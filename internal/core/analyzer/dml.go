package analyzer

import (
	"fmt"
	"slices"

	"github.com/sqlc-dev/sqlc/internal/core"
	"github.com/sqlc-dev/sqlc/internal/sql/ast"
)

func (a *analyzer) analyzeInsert(s *ast.InsertStmt) error {
	if err := a.bindCTEs(s.WithClause); err != nil {
		return err
	}
	if s.Relation == nil {
		return fmt.Errorf("insert: missing relation")
	}
	rel, err := a.bindRangeVar(s.Relation)
	if err != nil {
		return err
	}
	a.scope = &scope{rels: []scopeRel{rel}}

	targets, err := insertTargets(rel, s.Cols)
	if err != nil {
		return err
	}
	if err := a.bindInsertValues(s.SelectStmt, rel, targets); err != nil {
		return err
	}
	if s.OnConflictClause != nil {
		if err := a.analyzeOnConflict(s.OnConflictClause, rel); err != nil {
			return fmt.Errorf("on conflict: %w", err)
		}
	}
	return a.projectReturning(s.ReturningList)
}

// analyzeOnConflict types an ON CONFLICT clause. DO UPDATE sets the
// target's columns the way UPDATE does, and its expressions read the
// target's row by a bare name or the table's, and the row the INSERT
// proposed as excluded, which is in scope for the clause alone and only
// by that name.
func (a *analyzer) analyzeOnConflict(c *ast.OnConflictClause, target scopeRel) error {
	if c.Infer != nil && c.Infer.WhereClause != nil {
		if _, err := a.typeExpr(c.Infer.WhereClause); err != nil {
			return fmt.Errorf("where: %w", err)
		}
	}
	excluded := target
	excluded.alias = "excluded"
	excluded.qualifiedOnly = true
	sc := &scope{rels: append(slices.Clone(a.scope.rels), excluded), joined: a.scope.joined}
	defer a.binding(sc)()

	for _, item := range listItems(c.TargetList) {
		rt, ok := item.(*ast.ResTarget)
		if !ok || rt.Name == nil {
			continue
		}
		col, ok := findColumn(target, *rt.Name)
		if !ok {
			return fmt.Errorf("unknown column %q", *rt.Name)
		}
		if err := a.bindValue(target, &col, rt.Val); err != nil {
			return fmt.Errorf("set %s: %w", *rt.Name, err)
		}
	}
	if c.WhereClause != nil {
		if _, err := a.typeExpr(c.WhereClause); err != nil {
			return fmt.Errorf("where: %w", err)
		}
	}
	return nil
}

func (a *analyzer) analyzeUpdate(s *ast.UpdateStmt) error {
	if err := a.bindCTEs(s.WithClause); err != nil {
		return err
	}
	sc, err := a.relationScope(s.Relations, s.FromClause, nil)
	if err != nil {
		return err
	}
	a.scope = sc
	target := sc.rels[0]

	for _, item := range listItems(s.TargetList) {
		rt, ok := item.(*ast.ResTarget)
		if !ok || rt.Name == nil {
			continue
		}
		col, ok := findColumn(target, *rt.Name)
		if !ok {
			return fmt.Errorf("unknown column %q", *rt.Name)
		}
		if err := a.bindValue(target, &col, rt.Val); err != nil {
			return fmt.Errorf("set %s: %w", *rt.Name, err)
		}
	}

	if s.WhereClause != nil {
		if _, err := a.typeExpr(s.WhereClause); err != nil {
			return fmt.Errorf("where: %w", err)
		}
	}
	return a.projectReturning(s.ReturningList)
}

func (a *analyzer) analyzeDelete(s *ast.DeleteStmt) error {
	if err := a.bindCTEs(s.WithClause); err != nil {
		return err
	}
	sc, err := a.relationScope(s.Relations, s.UsingClause, s.FromClause)
	if err != nil {
		return err
	}
	a.scope = sc

	if s.WhereClause != nil {
		if _, err := a.typeExpr(s.WhereClause); err != nil {
			return fmt.Errorf("where: %w", err)
		}
	}
	return a.projectReturning(s.ReturningList)
}

// relationScope builds the scope a DML statement operates on: the relations it
// targets, whatever a USING or FROM clause joins in, and — for the engines that
// report a multi-table DELETE that way — a single FROM node.
func (a *analyzer) relationScope(relations, extra *ast.List, from ast.Node) (*scope, error) {
	sc := &scope{}
	defer a.binding(sc)()
	for _, item := range listItems(relations) {
		if err := a.appendFromItem(sc, item); err != nil {
			return nil, err
		}
	}
	for _, item := range listItems(extra) {
		if err := a.appendFromItem(sc, item); err != nil {
			return nil, err
		}
	}
	if from != nil {
		if err := a.appendFromItem(sc, from); err != nil {
			return nil, err
		}
	}
	if len(sc.rels) == 0 {
		return nil, fmt.Errorf("missing target relation")
	}
	return sc, nil
}

func insertTargets(rel scopeRel, cols *ast.List) ([]core.ClassColumn, error) {
	items := listItems(cols)
	if len(items) == 0 {
		out := make([]core.ClassColumn, 0, len(rel.cols))
		for _, col := range rel.cols {
			if !col.Hidden {
				out = append(out, col)
			}
		}
		return out, nil
	}
	out := make([]core.ClassColumn, 0, len(items))
	for _, item := range items {
		rt, ok := item.(*ast.ResTarget)
		if !ok || rt.Name == nil {
			return nil, fmt.Errorf("insert: unsupported column target %T", item)
		}
		col, ok := findColumn(rel, *rt.Name)
		if !ok {
			return nil, fmt.Errorf("unknown column %q", *rt.Name)
		}
		out = append(out, col)
	}
	return out, nil
}

func (a *analyzer) bindInsertValues(n ast.Node, rel scopeRel, targets []core.ClassColumn) error {
	if n == nil {
		return nil
	}
	sel, ok := n.(*ast.SelectStmt)
	if !ok {
		return fmt.Errorf("insert: unsupported source %T", n)
	}
	// INSERT ... SELECT inserts whatever the query returns. The rows are not
	// the statement's result, but the query still holds placeholders, and
	// one the query projects straight into a target column holds what the
	// column does.
	if len(listItems(sel.ValuesLists)) == 0 {
		if err := a.bindInsertSelect(sel, rel, targets); err != nil {
			return err
		}
		_, err := a.subqueryColumns(sel)
		return err
	}
	for _, row := range listItems(sel.ValuesLists) {
		values, ok := row.(*ast.List)
		if !ok {
			continue
		}
		for i, v := range values.Items {
			var target *core.ClassColumn
			if i < len(targets) {
				target = &targets[i]
			}
			if err := a.bindValue(rel, target, v); err != nil {
				return err
			}
		}
	}
	return nil
}

// bindInsertSelect types the placeholders an INSERT ... SELECT projects
// into its target columns, by position: a bare placeholder holds the
// column's type, and one unnested into the column holds a list of it. A
// SELECT * over a VALUES list is bound row by row, as INSERT ... VALUES is.
// The query's own context types its other placeholders.
func (a *analyzer) bindInsertSelect(sel *ast.SelectStmt, rel scopeRel, targets []core.ClassColumn) error {
	if values := selectedValues(sel); values != nil {
		for _, row := range listItems(values) {
			items, ok := row.(*ast.List)
			if !ok {
				continue
			}
			for i, v := range items.Items {
				pr, ok := v.(*ast.ParamRef)
				if !ok || i >= len(targets) {
					continue
				}
				a.locate(pr)
				a.inferParam(pr.Number, columnType(rel, targets[i]))
			}
		}
		return nil
	}
	for i, item := range listItems(sel.TargetList) {
		rt, ok := item.(*ast.ResTarget)
		if !ok || i >= len(targets) {
			return nil
		}
		// A star shifts every position after it.
		if isStarRef(rt.Val) {
			return nil
		}
		t := columnType(rel, targets[i])
		switch v := rt.Val.(type) {
		case *ast.ParamRef:
			a.locate(v)
			a.inferParam(v.Number, t)
		case *ast.FuncCall:
			if funcCallName(v) != "unnest" || len(listItems(v.Args)) != 1 {
				continue
			}
			pr, ok := v.Args.Items[0].(*ast.ParamRef)
			if !ok {
				continue
			}
			element := a.exprOf(t)
			if element == nil {
				continue
			}
			list := a.lookupType(core.Array(element.WithNullable(false)))
			list.nullable, list.columnNotNull = t.nullable, t.columnNotNull
			list.sourceAttributeOID, list.sourceTableAlias = t.sourceAttributeOID, t.sourceTableAlias
			a.locate(pr)
			a.inferParam(pr.Number, list)
		}
	}
	return nil
}

// selectedValues is the VALUES list a "SELECT * FROM (VALUES ...)" selects
// everything from, and nil for any other query.
func selectedValues(sel *ast.SelectStmt) *ast.List {
	targets := listItems(sel.TargetList)
	from := listItems(sel.FromClause)
	if len(targets) != 1 || len(from) != 1 || sel.WhereClause != nil || sel.GroupClause != nil {
		return nil
	}
	rt, ok := targets[0].(*ast.ResTarget)
	if !ok || !isStarRef(rt.Val) {
		return nil
	}
	rs, ok := from[0].(*ast.RangeSubselect)
	if !ok {
		return nil
	}
	sub, ok := rs.Subquery.(*ast.SelectStmt)
	if !ok || len(listItems(sub.ValuesLists)) == 0 {
		return nil
	}
	return sub.ValuesLists
}

func (a *analyzer) bindValue(rel scopeRel, target *core.ClassColumn, v ast.Node) error {
	if target != nil {
		switch value := v.(type) {
		case *ast.ParamRef:
			a.locate(value)
			a.inferParam(value.Number, columnType(rel, *target))
			return nil
		case *ast.A_Const:
			return nil
		}
	}
	_, err := a.typeExpr(v)
	return err
}

func (a *analyzer) projectReturning(l *ast.List) error {
	for _, item := range listItems(l) {
		rt, ok := item.(*ast.ResTarget)
		if !ok {
			continue
		}
		if err := a.projectTarget(rt); err != nil {
			return err
		}
	}
	return nil
}

func findColumn(rel scopeRel, name string) (core.ClassColumn, bool) {
	for _, col := range rel.cols {
		if col.Name == name {
			return col, true
		}
	}
	return core.ClassColumn{}, false
}

func columnType(rel scopeRel, col core.ClassColumn) exprType {
	return exprType{
		typeOID:            col.TypeOID,
		expr:               col.Type,
		untyped:            rel.causes[col.Name],
		nullable:           !col.NotNull || rel.nullable,
		sourceClassOID:     rel.classOID,
		sourceAttributeOID: col.AttOID,
		sourceTableAlias:   rel.alias,
		columnNotNull:      col.NotNull,
	}
}
