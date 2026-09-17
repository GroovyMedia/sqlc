package analyzer

import (
	"fmt"

	"github.com/sqlc-dev/sqlc/internal/core"
	"github.com/sqlc-dev/sqlc/internal/sql/ast"
)

type scope struct {
	rels []scopeRel
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
// them: an outer join makes its outer side nullable.
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
	return nil
}

// markNullable makes the relations in [from, to) nullable.
func (s *scope) markNullable(from, to int) {
	for i := from; i < to; i++ {
		s.rels[i].nullable = true
	}
}

// bindRangeFunction binds a function called in FROM. A set-returning function
// stands in for a relation of a single column named after it.
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
	if len(overloads) == 0 {
		return scopeRel{}, fmt.Errorf("unknown function %q", name)
	}

	// The arguments are typed against the scope built so far, which is what
	// gives a placeholder passed to the function its type.
	if _, err := a.typeFuncCall(call); err != nil {
		return scopeRel{}, err
	}

	p := overloads[0]
	rel := scopeRel{
		alias: name,
		cols:  []core.ClassColumn{{Name: name, TypeOID: p.ReturnTypeOID, NotNull: !p.ReturnNullable}},
	}
	if rf.Alias != nil && rf.Alias.Aliasname != nil && *rf.Alias.Aliasname != "" {
		rel.alias = *rf.Alias.Aliasname
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
	cols, err := a.subqueryColumns(sel)
	if err != nil {
		return scopeRel{}, err
	}
	alias := ""
	if rs.Alias != nil && rs.Alias.Aliasname != nil {
		alias = *rs.Alias.Aliasname
	}
	rel := derivedRel(alias, cols)
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
	found := 0
	for _, r := range s.rels {
		if relation != "" && r.alias != relation {
			continue
		}
		for _, c := range r.cols {
			if c.Name != column {
				continue
			}
			found++
			if found > 1 {
				return scopeRel{}, core.ClassColumn{}, false, fmt.Errorf("ambiguous column reference %q", column)
			}
			rel, col = r, c
		}
	}
	if found == 0 {
		return scopeRel{}, core.ClassColumn{}, false, nil
	}
	return rel, col, true, nil
}
