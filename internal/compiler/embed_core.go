package compiler

import (
	"fmt"

	"github.com/sqlc-dev/sqlc/internal/core"
	"github.com/sqlc-dev/sqlc/internal/sql/ast"
	"github.com/sqlc-dev/sqlc/internal/sql/preprocess"
)

// embedCore folds the columns each sqlc.embed(table) expanded to into the
// one column the legacy path reports for an embed: named after the table,
// with EmbedTable set, so codegen nests the table's model in the row
// struct. The analyzer reports columns flat and in target order, so the
// targets are walked alongside them: a star target covers as many columns
// as it expanded to, any other target one.
func (c *Compiler) embedCore(raw *ast.RawStmt, res core.PrepareResult, embeds preprocess.EmbedSet, cols []*Column) ([]*Column, error) {
	if len(embeds) == 0 {
		return cols, nil
	}
	targets := targetList(raw.Stmt)
	if targets == nil {
		return cols, nil
	}
	stars := make(map[int]core.StarExpansion, len(res.Stars))
	for _, star := range res.Stars {
		stars[star.Location] = star
	}

	out := make([]*Column, 0, len(cols))
	next := 0
	for _, item := range targets.Items {
		rt, ok := item.(*ast.ResTarget)
		if !ok {
			continue
		}
		star, isStar := stars[rt.Location]
		width := 1
		if isStar {
			width = len(star.Columns)
		}
		if next+width > len(cols) {
			return cols, nil
		}
		locations := []int{rt.Location}
		if rt.Val != nil {
			locations = append(locations, rt.Val.Pos())
		}
		embed, isEmbed := embeds.Find(locations...)
		if !isStar || !isEmbed {
			out = append(out, cols[next:next+width]...)
			next += width
			continue
		}

		// The star's columns name the relation it resolved to, alias and
		// schema included, so the model to nest is theirs. A relation with
		// no class behind it, a CTE or a subquery, has no model, and
		// neither has one the catalog codegen reads does not list.
		table, ok := starTable(res.Columns[next : next+width])
		if ok {
			_, err := c.catalog.GetTable(table)
			ok = err == nil
		}
		if !ok {
			return nil, fmt.Errorf("unable to resolve table with %q", embed.Orig())
		}
		out = append(out, &Column{Name: table.Name, EmbedTable: table})
		next += width
	}
	return append(out, cols[next:]...), nil
}

// starTable is the table every column of a star came from.
func starTable(cols []core.Column) (*ast.TableName, bool) {
	if len(cols) == 0 {
		return nil, false
	}
	first := cols[0]
	if first.SourceClassOID == 0 || first.Source == nil || first.Source.Table == "" {
		return nil, false
	}
	for _, col := range cols[1:] {
		if col.SourceClassOID != first.SourceClassOID {
			return nil, false
		}
	}
	return &ast.TableName{Schema: first.Source.Schema, Name: first.Source.Table}, true
}

// targetList is the list of targets a statement's result columns come from.
// A set operation reports the columns of its leftmost branch.
func targetList(n ast.Node) *ast.List {
	switch s := n.(type) {
	case *ast.SelectStmt:
		if s.Larg != nil && (s.TargetList == nil || len(s.TargetList.Items) == 0) {
			return targetList(s.Larg)
		}
		return s.TargetList
	case *ast.InsertStmt:
		return s.ReturningList
	case *ast.UpdateStmt:
		return s.ReturningList
	case *ast.DeleteStmt:
		return s.ReturningList
	}
	return nil
}
