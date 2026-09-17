package analyzer

import (
	"fmt"
	"slices"

	"github.com/sqlc-dev/sqlc/internal/core"
	"github.com/sqlc-dev/sqlc/internal/sql/ast"
)

type scope struct {
	rels []scopeRel
	// joined are the columns a USING clause or a NATURAL JOIN merged, each
	// standing in for the copies the two sides of its join hold.
	joined []joinedColumn
}

type scopeRel struct {
	alias    string
	classOID int64
	// cols is the catalog's column list, held as-is rather than copied into a
	// scope-local column type.
	cols []core.ClassColumn
	// nullable marks the outer side of an outer join: a row the other side
	// has no match for holds NULL in every column of this relation, whatever
	// the column declares.
	nullable bool
	// values marks a relation a VALUES list produces, whose column names
	// are the engine's to give.
	values bool
	// nulled names the columns a grouping set leaves out, which are NULL
	// in the rows of that set, whatever the column declares.
	nulled map[string]bool
	// qualifiedOnly marks a relation a bare column name never resolves
	// to: the row an INSERT proposed is read as excluded.col in ON
	// CONFLICT DO UPDATE, where a bare name is the target's.
	qualifiedOnly bool
	// causes says, by column name, why a column of a derived relation has
	// no type, for a strict dialect's error.
	causes map[string]string
}

// joinedColumn is a column both sides of a join have under one name and a
// USING clause or NATURAL merged into one. An unqualified reference to the
// name resolves to it rather than being ambiguous, and a star lists it once,
// where the side it is read from lists it.
type joinedColumn struct {
	name string
	// from and to bound the relations the join covers, as indexes into rels.
	from, to int
	// rel is the relation the merged column is read from — the left side's,
	// or the right side's in a RIGHT JOIN — and col is that copy, with
	// NotNull saying whether the merged column can be NULL.
	rel int
	col core.ClassColumn
	// dup is the relation whose copy the merged column hides.
	dup int
}

// match is a column a scope resolved: the relation it is read from, as an
// index into rels and as the relation itself, and the column.
type match struct {
	idx int
	rel scopeRel
	col core.ClassColumn
}

func (a *analyzer) buildScope(from *ast.List) (*scope, error) {
	items := listItems(from)
	sc := &scope{rels: make([]scopeRel, 0, len(items))}
	defer a.binding(sc)()
	for _, item := range items {
		if err := a.appendFromItem(sc, item); err != nil {
			return nil, err
		}
	}
	return sc, nil
}

// binding makes the scope under construction the one the analyzer resolves
// against, and returns the func that puts back the scope it replaced. A FROM
// item can refer to the ones before it — a set-returning function takes its
// arguments from them, and a LATERAL subquery reads their columns — so
// binding an item has to see what is bound so far rather than no scope at
// all.
func (a *analyzer) binding(sc *scope) func() {
	prev := a.scope
	a.scope = sc
	return func() { a.scope = prev }
}

func (a *analyzer) appendFromItem(sc *scope, item ast.Node) error {
	switch v := item.(type) {
	case *ast.RangeVar:
		rel, err := a.bindRangeVar(v)
		if err != nil {
			return err
		}
		sc.rels = append(sc.rels, rel)
		return nil
	case *ast.JoinExpr:
		return a.appendJoin(sc, v)
	case *ast.RangeFunction:
		rel, err := a.bindRangeFunction(v)
		if err != nil {
			return err
		}
		sc.rels = append(sc.rels, rel)
		return nil
	case *ast.RangeSubselect:
		rel, err := a.bindRangeSubselect(v)
		if err != nil {
			return err
		}
		sc.rels = append(sc.rels, rel)
		return nil
	case *ast.RangeTableSample:
		// A sample has the relation's columns; how many rows is not a
		// question of type.
		return a.appendFromItem(sc, v.Relation)
	case *ast.List:
		// Some engines report a comma-separated FROM as a nested list.
		for _, item := range listItems(v) {
			if err := a.appendFromItem(sc, item); err != nil {
				return err
			}
		}
		return nil
	default:
		return fmt.Errorf("scope: unsupported FROM item %T", item)
	}
}

// appendJoin binds both sides of a join, then applies what the join does to
// them: an outer join makes its outer side nullable, and a USING clause or
// NATURAL merges the columns the sides share.
func (a *analyzer) appendJoin(sc *scope, je *ast.JoinExpr) error {
	from := len(sc.rels)
	if err := a.appendFromItem(sc, je.Larg); err != nil {
		return err
	}
	mid := len(sc.rels)
	if err := a.appendFromItem(sc, je.Rarg); err != nil {
		return err
	}
	to := len(sc.rels)

	// The condition is typed as part of binding the join, against both
	// sides as they are before the join: a placeholder in it is typed by
	// the column beside it, whichever statement the join is in.
	if je.Quals != nil {
		if _, err := a.typeExpr(je.Quals); err != nil {
			return fmt.Errorf("join: ON: %w", err)
		}
	}
	joined, err := sc.mergeColumns(je, from, mid, to)
	if err != nil {
		return fmt.Errorf("join: %w", err)
	}

	// An outer join keeps the rows one side has no match for, with NULL in
	// every column of the other side, unless the dialect fills that side
	// with defaults instead, as ClickHouse does unless join_use_nulls is
	// set.
	outerFrom, outerTo := 0, 0
	switch je.Jointype {
	case ast.JoinTypeLeft:
		outerFrom, outerTo = mid, to
	case ast.JoinTypeRight:
		outerFrom, outerTo = from, mid
	case ast.JoinTypeFull:
		outerFrom, outerTo = from, to
	}
	if outerFrom < outerTo && !a.cat.OuterJoinDefaults() {
		sc.markNullable(outerFrom, outerTo)
	}
	// The merged columns were typed before the mark, so a FULL JOIN's own
	// mark does not reach them: its merged column is NULL only when both
	// copies are.
	sc.joined = append(sc.joined, joined...)
	return nil
}

// markNullable makes the relations in [from, to), and the columns merged
// among them, nullable.
func (s *scope) markNullable(from, to int) {
	for i := from; i < to; i++ {
		s.rels[i].nullable = true
	}
	for j := range s.joined {
		if from <= s.joined[j].from && s.joined[j].to <= to {
			s.joined[j].col.NotNull = false
		}
	}
}

// mergeColumns resolves the columns a USING clause names, or the ones both
// sides of a NATURAL JOIN have, on each side of the join. A merged column is
// read from the side an outer join keeps whole, and is nullable when that
// copy is; a FULL JOIN's is NULL only when both copies are.
func (s *scope) mergeColumns(je *ast.JoinExpr, from, mid, to int) ([]joinedColumn, error) {
	var names []string
	switch {
	case je.UsingClause != nil:
		for _, item := range listItems(je.UsingClause) {
			if str, ok := item.(*ast.String); ok {
				names = append(names, str.Str)
			}
		}
	case je.IsNatural:
		names = s.commonColumns(from, mid, to)
	}
	out := make([]joinedColumn, 0, len(names))
	for _, name := range names {
		left, ok, err := s.resolveIn("", name, from, mid)
		if err != nil {
			return nil, err
		}
		if !ok {
			return nil, fmt.Errorf("column %q specified in USING clause does not exist in left table", name)
		}
		right, ok, err := s.resolveIn("", name, mid, to)
		if err != nil {
			return nil, err
		}
		if !ok {
			return nil, fmt.Errorf("column %q specified in USING clause does not exist in right table", name)
		}
		src, other := left, right
		if je.Jointype == ast.JoinTypeRight {
			src, other = right, left
		}
		nullable := columnType(src.rel, src.col).nullable
		if je.Jointype == ast.JoinTypeFull {
			nullable = nullable || columnType(other.rel, other.col).nullable
		}
		col := src.col
		col.NotNull = !nullable
		out = append(out, joinedColumn{name: name, from: from, to: to, rel: src.idx, col: col, dup: other.idx})
	}
	return out, nil
}

// commonColumns lists the names the left side of a join offers, in its
// order, that the right side offers too.
func (s *scope) commonColumns(from, mid, to int) []string {
	var names []string
	for i := from; i < mid; i++ {
		for _, c := range s.rels[i].cols {
			if c.Hidden || s.hides(i, c.Name) || slices.Contains(names, c.Name) {
				continue
			}
			if _, ok, _ := s.resolveIn("", c.Name, mid, to); ok {
				names = append(names, c.Name)
			}
		}
	}
	return names
}

// joinedAt is the outermost merged column named name that covers relation
// i, or -1. A join registers its merged columns after the joins nested in
// it, so the last one found is the outermost.
func (s *scope) joinedAt(i int, name string) int {
	for j := len(s.joined) - 1; j >= 0; j-- {
		jc := &s.joined[j]
		if jc.name == name && jc.from <= i && i < jc.to {
			return j
		}
	}
	return -1
}

// hides reports whether relation i's column named name is a copy a merged
// column stands in for.
func (s *scope) hides(i int, name string) bool {
	for _, jc := range s.joined {
		if jc.name == name && jc.dup == i {
			return true
		}
	}
	return false
}

// joinedMatch is a merged column as a resolution: read from its relation,
// with its own nullability standing in for the relation's.
func (s *scope) joinedMatch(j int) match {
	jc := s.joined[j]
	rel := s.rels[jc.rel]
	rel.nullable = false
	return match{idx: jc.rel, rel: rel, col: jc.col}
}

// bindRangeFunction binds a function called in FROM. A set-returning function
// stands in for a relation of a single column named after it, or after a
// bare alias, as "FROM unnest(x) AS t" makes t the column. WITH ORDINALITY
// adds the row's number as a second column, and then both keep their names.
func (a *analyzer) bindRangeFunction(rf *ast.RangeFunction) (scopeRel, error) {
	call := findFuncCall(rf.Functions)
	if call == nil {
		return scopeRel{}, fmt.Errorf("range function: no function call")
	}
	name := funcCallName(call)
	overloads, err := a.cat.FindProcs(name, nil)
	if err != nil {
		return scopeRel{}, err
	}
	// unnest is typed by the analyzer where the seed cannot describe it.
	if len(overloads) == 0 && name != "unnest" {
		return scopeRel{}, fmt.Errorf("unknown function %q", name)
	}

	// The arguments are typed against the scope built so far, which is what
	// gives a placeholder passed to the function its type.
	t, err := a.typeFuncCall(call)
	if err != nil {
		return scopeRel{}, err
	}

	rel := scopeRel{
		alias: name,
		cols:  []core.ClassColumn{{Name: name, TypeOID: t.typeOID, Type: a.exprOf(t), NotNull: !t.nullable}},
	}
	if rf.Alias != nil && rf.Alias.Aliasname != nil && *rf.Alias.Aliasname != "" {
		rel.alias = *rf.Alias.Aliasname
		if len(listItems(rf.Alias.Colnames)) == 0 && !rf.Ordinality {
			rel.cols[0].Name = rel.alias
		}
	}
	if rf.Ordinality {
		ordinal := core.ClassColumn{Name: "ordinality", NotNull: true}
		if oid, err := a.cat.TypeOID("bigint"); err == nil {
			ordinal.TypeOID = oid
		}
		rel.cols = append(rel.cols, ordinal)
	}
	if rf.Alias != nil {
		renameColumns(&rel, rf.Alias.Colnames)
	}
	return rel, nil
}

func findFuncCall(l *ast.List) *ast.FuncCall {
	for _, item := range listItems(l) {
		switch v := item.(type) {
		case *ast.FuncCall:
			return v
		case *ast.List:
			if call := findFuncCall(v); call != nil {
				return call
			}
		}
	}
	return nil
}

// bindRangeSubselect binds a subquery used as a relation in FROM.
func (a *analyzer) bindRangeSubselect(rs *ast.RangeSubselect) (scopeRel, error) {
	sel, ok := rs.Subquery.(*ast.SelectStmt)
	if !ok {
		return scopeRel{}, fmt.Errorf("subquery: unsupported %T", rs.Subquery)
	}
	sub, err := a.subquery(sel)
	if err != nil {
		return scopeRel{}, err
	}
	alias := ""
	if rs.Alias != nil && rs.Alias.Aliasname != nil {
		alias = *rs.Alias.Aliasname
	}
	rel := sub.derivedRel(alias)
	rel.values = len(listItems(sel.ValuesLists)) > 0
	if rs.Alias != nil {
		renameColumns(&rel, rs.Alias.Colnames)
	}
	return rel, nil
}

func (a *analyzer) bindRangeVar(rv *ast.RangeVar) (scopeRel, error) {
	if rv.Relname == nil {
		return scopeRel{}, fmt.Errorf("range var: missing relation name")
	}
	relName := *rv.Relname

	// A WITH clause defines a relation that shadows nothing but is not in the
	// catalog, so it is resolved before one.
	if cte, ok := a.ctes[relName]; ok && (rv.Schemaname == nil || *rv.Schemaname == "") {
		if rv.Alias != nil && rv.Alias.Aliasname != nil && *rv.Alias.Aliasname != "" {
			cte.alias = *rv.Alias.Aliasname
		}
		return cte, nil
	}

	schema := ""
	if rv.Schemaname != nil {
		schema = *rv.Schemaname
	}
	if schema == "" {
		schema = "public"
	}
	nsOID, err := a.cat.NamespaceOID(schema)
	if err != nil {
		return scopeRel{}, fmt.Errorf("schema %q: %w", schema, err)
	}
	classOID, err := a.cat.ClassOID(nsOID, relName)
	if err != nil {
		return scopeRel{}, fmt.Errorf("relation %q.%q: %w", schema, relName, err)
	}
	rel := scopeRel{
		alias:    relName,
		classOID: classOID,
	}
	if rv.Alias != nil && rv.Alias.Aliasname != nil && *rv.Alias.Aliasname != "" {
		rel.alias = *rv.Alias.Aliasname
	}

	cols, err := a.cat.ClassColumns(classOID)
	if err != nil {
		return scopeRel{}, err
	}
	rel.cols = cols
	return rel, nil
}

// resolveColumn finds a column in this query's scope, falling back to the
// scopes of the queries it is nested in, outermost last, which is what a
// correlated subquery at any depth refers to.
func (a *analyzer) resolveColumn(relation, column string) (scopeRel, core.ClassColumn, bool, error) {
	for cur := a; cur != nil; cur = cur.outer {
		rel, col, ok, err := cur.scope.resolveColumn(relation, column)
		if err != nil || ok {
			return rel, col, ok, err
		}
	}
	return scopeRel{}, core.ClassColumn{}, false, nil
}

// resolveColumn finds the single column named column, optionally qualified by
// relation. It reports an error when more than one relation in scope offers
// that name.
func (s *scope) resolveColumn(relation, column string) (rel scopeRel, col core.ClassColumn, ok bool, err error) {
	// A statement whose scope is not built yet offers no columns. Report the
	// column as unresolved and let the caller say so, rather than crashing on
	// the way to the same answer.
	if s == nil {
		return rel, col, false, nil
	}
	m, ok, err := s.resolveIn(relation, column, 0, len(s.rels))
	if err != nil || !ok {
		return scopeRel{}, core.ClassColumn{}, false, err
	}
	return m.rel, m.col, true, nil
}

// resolveIn finds the single column named column among the relations in
// [from, to), optionally qualified by relation. The copies a join merged
// count as the one merged column, so an unqualified name both sides of a
// USING clause have is not ambiguous.
func (s *scope) resolveIn(relation, column string, from, to int) (match, bool, error) {
	var found match
	count := 0
	seen := -1
	for i := from; i < to; i++ {
		r := s.rels[i]
		if relation != "" && r.alias != relation {
			continue
		}
		if relation == "" && r.qualifiedOnly {
			continue
		}
		for _, c := range r.cols {
			if c.Name != column {
				continue
			}
			m := match{idx: i, rel: r, col: c}
			if relation == "" {
				if j := s.joinedAt(i, column); j >= 0 {
					if j == seen {
						continue
					}
					seen = j
					m = s.joinedMatch(j)
				}
			}
			count++
			if count > 1 {
				return match{}, false, fmt.Errorf("ambiguous column reference %q", column)
			}
			found = m
		}
	}
	return found, count == 1, nil
}
