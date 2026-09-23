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
		b.Exprs = append(b.Exprs, fullSpan(e))
		walkDW(e, func(n dw.Node) {
			if p, ok := n.(*dw.ParameterExpression); ok {
				b.Params = append(b.Params, ast.BatchParam{Start: p.Pos(), End: p.End(), Number: p.Number})
			}
		})
	}
	return b
}

// unnestSelect is the select list of a "SELECT unnest(@a), unnest(@b), ..."
// with no FROM, and the span of that SELECT. unnest[i] is the placeholder
// the i-th item unnests (the placeholder, or a cast of it), or 0.
func unnestSelect(sel *dw.SelectStatement) ([]dw.Expr, [2]int, []int, bool) {
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
	unnest := make([]int, len(node.SelectList))
	found := false
	for i, e := range node.SelectList {
		fn, ok := e.(*dw.FunctionExpression)
		if !ok || !strings.EqualFold(fn.FunctionName, "unnest") || len(fn.Arguments) != 1 {
			continue
		}
		arg := fn.Arguments[0].Expr
		if cast, ok := arg.(*dw.CastExpression); ok {
			arg = cast.Child
		}
		p, ok := arg.(*dw.ParameterExpression)
		if !ok {
			return nil, [2]int{}, nil, false
		}
		unnest[i] = p.Number
		found = true
	}
	if !found {
		return nil, [2]int{}, nil, false
	}
	return node.SelectList, fullSpan(node), unnest, true
}

func (c *cc) insertBatch(n *dw.InsertStatement) *ast.BatchSource {
	row, span, ok := singleValues(n.Query)
	var unnest []int
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
		b.ReturningItems = append(b.ReturningItems, fullSpan(item))
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
