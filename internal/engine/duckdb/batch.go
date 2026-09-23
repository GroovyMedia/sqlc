package duckdb

import (
	"strings"

	dw "github.com/sqlc-dev/darkwing/ast"

	"github.com/sqlc-dev/sqlc/internal/sql/ast"
)

// singleValues is the one row of a "VALUES (...)" query, and the span of
// that VALUES clause. It reports false for any other query.
func singleValues(sel *dw.SelectStatement) ([]dw.Expr, [2]int, bool) {
	if sel == nil {
		return nil, [2]int{}, false
	}
	node, ok := sel.Node.(*dw.SelectNode)
	if !ok || node.Where != nil || node.Qualify != nil || node.Having != nil || len(node.GroupExpressions) > 0 {
		return nil, [2]int{}, false
	}
	list, ok := node.FromTable.(*dw.ExpressionListRef)
	if !ok || len(list.Values) != 1 || node.Pos() < 0 {
		return nil, [2]int{}, false
	}
	return list.Values[0], [2]int{node.Pos(), node.End()}, true
}

// batchValues fills the part of a BatchSource the VALUES row gives.
func (c *cc) batchValues(row []dw.Expr, span [2]int) *ast.BatchSource {
	b := &ast.BatchSource{Values: span}
	for _, e := range row {
		b.Exprs = append(b.Exprs, balance(c.src, fullSpan(e)))
		walkDW(e, func(n dw.Node) {
			if p, ok := n.(*dw.ParameterExpression); ok {
				b.Params = append(b.Params, ast.BatchParam{Start: p.Pos(), End: p.End(), Number: p.Number})
			}
		})
	}
	return b
}

// unnestSelect is the select list of a "SELECT ..., unnest(@a), ..." with
// no FROM, the span of that SELECT, and each unnest of a placeholder (or of
// a cast of one) the list holds. An unnest of anything else is refused.
func unnestSelect(sel *dw.SelectStatement) ([]dw.Expr, [2]int, []ast.BatchParam, bool) {
	if sel == nil {
		return nil, [2]int{}, nil, false
	}
	node, ok := sel.Node.(*dw.SelectNode)
	if !ok || node.Where != nil || node.Qualify != nil || node.Having != nil || len(node.GroupExpressions) > 0 {
		return nil, [2]int{}, nil, false
	}
	if _, empty := node.FromTable.(*dw.EmptyTableRef); node.FromTable != nil && !empty {
		return nil, [2]int{}, nil, false
	}
	var unnest []ast.BatchParam
	ok = true
	for _, e := range node.SelectList {
		walkDW(e, func(n dw.Node) {
			fn, isFn := n.(*dw.FunctionExpression)
			if !isFn || !strings.EqualFold(fn.FunctionName, "unnest") {
				return
			}
			if len(fn.Arguments) != 1 {
				ok = false
				return
			}
			arg := fn.Arguments[0].Expr
			if cast, isCast := arg.(*dw.CastExpression); isCast {
				arg = cast.Child
			}
			p, isParam := arg.(*dw.ParameterExpression)
			if !isParam {
				ok = false
				return
			}
			span := fullSpan(fn)
			unnest = append(unnest, ast.BatchParam{Start: span[0], End: span[1], Number: p.Number})
		})
	}
	if !ok || len(unnest) == 0 {
		return nil, [2]int{}, nil, false
	}
	return node.SelectList, fullSpan(node), unnest, true
}

func (c *cc) insertBatch(n *dw.InsertStatement) *ast.BatchSource {
	row, span, ok := singleValues(n.Query)
	var unnest []ast.BatchParam
	if !ok {
		row, span, unnest, ok = unnestSelect(n.Query)
	}
	if !ok {
		return nil
	}
	b := c.batchValues(row, span)
	b.Unnest = unnest
	b.Columns = n.Columns
	b.Table = qualifiedName(n.Catalog, n.Schema, n.Table)
	b.Qualifier = n.Table
	if n.TableAlias != "" {
		b.Table += " AS " + n.TableAlias
		b.Qualifier = n.TableAlias
	}
	if oc := n.OnConflict; oc != nil {
		b.OnConflict = true
		b.DoNothing = oc.Action != dw.OnConflictUpdate
		b.Keys = oc.IndexedColumns
	}
	c.batchReturning(b, n.Returning)
	return onlyRowParams(n, b)
}

// onlyRowParams is b, or nil when a placeholder appears outside the row
// expressions (in a SET, a WHERE, a MERGE arm, RETURNING, a CTE): the
// rewrite would leave it bound to the JSON array or the round.
func onlyRowParams(n dw.Node, b *ast.BatchSource) *ast.BatchSource {
	count := 0
	walkDW(n, func(node dw.Node) {
		if _, ok := node.(*dw.ParameterExpression); ok {
			count++
		}
	})
	if count != len(b.Params) {
		return nil
	}
	return b
}

func (c *cc) mergeBatch(n *dw.MergeIntoStatement) *ast.BatchSource {
	src, ok := n.Source.(*dw.SubqueryRef)
	if !ok {
		return nil
	}
	row, span, ok := singleValues(src.Subquery)
	if !ok {
		return nil
	}
	b := c.batchValues(row, span)
	b.Merge = true
	b.Columns = src.ColumnNameAlias
	b.Keys = mergeKeys(n, src.Alias)
	if b.Keys == nil {
		return nil
	}
	c.batchReturning(b, n.Returning)
	return onlyRowParams(n, b)
}

// mergeKeys is the source columns the rounds split on, or nil when the
// MERGE does not fit the rounds. Two rows can then touch the same target
// row only when their keys are equal: ON is an AND of "t.x = s.y", each
// arm that inserts writes s.y into x, and no UPDATE sets x. A WHEN NOT
// MATCHED BY SOURCE arm would, in a later round, delete what an earlier
// one wrote.
func mergeKeys(n *dw.MergeIntoStatement, alias string) []string {
	pairs := map[string]string{} // target column -> source column
	var keys []string
	if len(n.UsingColumns) > 0 {
		for _, col := range n.UsingColumns {
			pairs[strings.ToLower(col)] = col
			keys = append(keys, col)
		}
	} else {
		if alias == "" || !mergeOn(n.JoinCondition, alias, pairs, &keys) {
			return nil
		}
	}
	isSource := func(e dw.Expr, col string) bool {
		ref, ok := e.(*dw.ColumnRefExpression)
		if !ok {
			return false
		}
		names := ref.ColumnNames
		if len(names) == 2 && !strings.EqualFold(names[0], alias) {
			return false
		}
		return (len(names) == 1 || len(names) == 2) && strings.EqualFold(names[len(names)-1], col)
	}
	for _, a := range n.Actions {
		if a.Kind == dw.MergeWhenNotMatchedBySource {
			return nil
		}
		switch a.Action {
		case dw.MergeUpdate:
			if a.SetInfo == nil {
				return nil
			}
			for _, path := range a.SetInfo.Columns {
				if _, key := pairs[strings.ToLower(path[0])]; key {
					return nil
				}
			}
		case dw.MergeInsert:
			if len(a.Columns) != len(a.Expressions) {
				return nil
			}
			for tcol, scol := range pairs {
				found := false
				for i, col := range a.Columns {
					if strings.EqualFold(col, tcol) {
						found = isSource(a.Expressions[i], scol)
					}
				}
				if !found {
					return nil
				}
			}
		}
	}
	return keys
}

// mergeOn fills pairs and keys from an ON that is an AND of equalities
// between a target column and a source column, and reports false for any
// other ON.
func mergeOn(e dw.Expr, alias string, pairs map[string]string, keys *[]string) bool {
	switch e := e.(type) {
	case *dw.ConjunctionExpression:
		if e.Type != dw.ConjunctionAnd {
			return false
		}
		for _, op := range e.Operands {
			if !mergeOn(op, alias, pairs, keys) {
				return false
			}
		}
		return true
	case *dw.ComparisonExpression:
		if e.Type != dw.CompareEqual {
			return false
		}
		l, lok := e.Left.(*dw.ColumnRefExpression)
		r, rok := e.Right.(*dw.ColumnRefExpression)
		if !lok || !rok || len(l.ColumnNames) != 2 || len(r.ColumnNames) != 2 {
			return false
		}
		if strings.EqualFold(l.ColumnNames[0], alias) {
			l, r = r, l
		}
		if strings.EqualFold(l.ColumnNames[0], alias) || !strings.EqualFold(r.ColumnNames[0], alias) {
			return false
		}
		tcol, scol := strings.ToLower(l.ColumnNames[1]), r.ColumnNames[1]
		if prev, ok := pairs[tcol]; ok && !strings.EqualFold(prev, scol) {
			return false
		}
		pairs[tcol] = scol
		*keys = append(*keys, scol)
		return true
	}
	return false
}

// batchReturning records the RETURNING clause: from its keyword, which the
// items follow, to the end of the last item.
func (c *cc) batchReturning(b *ast.BatchSource, items []dw.Expr) {
	if len(items) == 0 {
		return
	}
	first := fullSpan(items[0])[0]
	if first < 0 {
		return
	}
	kw := strings.LastIndex(strings.ToUpper(c.src[:first]), "RETURNING")
	if kw < 0 {
		return
	}
	b.Returning = [2]int{kw, fullSpan(items[len(items)-1])[1]}
	for _, item := range items {
		_, star := item.(*dw.StarExpression)
		b.ReturningItems = append(b.ReturningItems, balance(c.src, fullSpan(item)))
		b.ReturningStar = append(b.ReturningStar, star)
	}
}

// fullSpan is the text a node and everything under it covers. darkwing
// starts some nodes after their first child: a cast's span starts at "::".
func fullSpan(n dw.Node) [2]int {
	span := [2]int{-1, -1}
	walkDW(n, func(c dw.Node) {
		if c.Pos() < 0 {
			return
		}
		if span[0] < 0 || c.Pos() < span[0] {
			span[0] = c.Pos()
		}
		if c.End() > span[1] {
			span[1] = c.End()
		}
	})
	return span
}

// balance widens span over the parentheses around its start or end that
// darkwing leaves out of a node's span, as in "($1::JSON)::DOUBLE[]", until
// the text it covers has as many "(" as ")". Parentheses inside quotes are
// not told apart, which the batch expressions never need.
func balance(src string, span [2]int) [2]int {
	depth := func() (open, close int) {
		for _, r := range src[span[0]:span[1]] {
			switch r {
			case '(':
				open++
			case ')':
				close++
			}
		}
		return
	}
	for {
		open, close := depth()
		switch {
		case close > open:
			i := span[0] - 1
			for i >= 0 && (src[i] == ' ' || src[i] == '\t' || src[i] == '\n' || src[i] == '\r') {
				i--
			}
			if i < 0 || src[i] != '(' {
				return span
			}
			span[0] = i
		case open > close:
			i := span[1]
			for i < len(src) && (src[i] == ' ' || src[i] == '\t' || src[i] == '\n' || src[i] == '\r') {
				i++
			}
			if i >= len(src) || src[i] != ')' {
				return span
			}
			span[1] = i + 1
		default:
			return span
		}
	}
}

func walkDW(n dw.Node, f func(dw.Node)) {
	if n == nil {
		return
	}
	f(n)
	for _, child := range n.Children() {
		walkDW(child, f)
	}
}

func qualifiedName(parts ...string) string {
	var out []string
	for _, p := range parts {
		if p != "" {
			out = append(out, p)
		}
	}
	return strings.Join(out, ".")
}
