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
	if len(n.UsingColumns) > 0 {
		b.Keys = n.UsingColumns
	} else if src.Alias != "" {
		seen := map[string]bool{}
		walkDW(n.JoinCondition, func(node dw.Node) {
			ref, ok := node.(*dw.ColumnRefExpression)
			if !ok || len(ref.ColumnNames) != 2 || !strings.EqualFold(ref.ColumnNames[0], src.Alias) {
				return
			}
			if name := ref.ColumnNames[1]; !seen[name] {
				seen[name] = true
				b.Keys = append(b.Keys, name)
			}
		})
	}
	c.batchReturning(b, n.Returning)
	return b
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
