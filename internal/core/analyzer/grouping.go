package analyzer

import (
	"github.com/sqlc-dev/sqlc/internal/sql/ast"
	"github.com/sqlc-dev/sqlc/internal/sql/astutils"
)

// A GROUP BY with grouping sets — ROLLUP, CUBE or GROUPING SETS — puts out
// the rows of every set, and in the rows of a set each grouping column
// the set leaves out is NULL, whatever the column declares. So a column
// one set groups by and another leaves out is nullable from the GROUP BY
// on: in the target list, HAVING and ORDER BY.

// groupedColumn is a column of the scope a grouping expression reads:
// the relation, as an index into the scope, and the column's name.
type groupedColumn struct {
	rel  int
	name string
}

// typeGroupingSet types the expressions of a grouping set and marks the
// columns some set leaves out.
func (a *analyzer) typeGroupingSet(gs *ast.GroupingSet) error {
	sets, err := a.groupingSets(gs)
	if err != nil {
		return err
	}
	for _, set := range sets {
		for c := range set {
			if a.groupedByEverySet(c, sets) {
				continue
			}
			rel := &a.scope.rels[c.rel]
			if rel.nulled == nil {
				rel.nulled = map[string]bool{}
			}
			rel.nulled[c.name] = true
		}
	}
	return nil
}

func (a *analyzer) groupedByEverySet(c groupedColumn, sets []map[groupedColumn]bool) bool {
	for _, set := range sets {
		if !set[c] {
			return false
		}
	}
	return true
}

// groupingSets lists the sets a grouping set stands for, each as the
// columns it groups by. A ROLLUP or CUBE stands for sets among which is
// the empty one, so every column of its is one some set leaves out.
func (a *analyzer) groupingSets(gs *ast.GroupingSet) ([]map[groupedColumn]bool, error) {
	switch gs.Kind {
	case ast.GroupingSetSets:
		var sets []map[groupedColumn]bool
		for _, item := range listItems(gs.Content) {
			if sub, ok := item.(*ast.GroupingSet); ok {
				subsets, err := a.groupingSets(sub)
				if err != nil {
					return nil, err
				}
				sets = append(sets, subsets...)
				continue
			}
			// PostgreSQL's parser gives a parenthesized set as a row.
			if row, ok := item.(*ast.RowExpr); ok {
				item = row.Args
			}
			cols, err := a.groupedColumns(item)
			if err != nil {
				return nil, err
			}
			sets = append(sets, cols)
		}
		return sets, nil
	case ast.GroupingSetRollup, ast.GroupingSetCube:
		cols, err := a.groupedColumns(gs.Content)
		if err != nil {
			return nil, err
		}
		return []map[groupedColumn]bool{cols, {}}, nil
	default:
		cols, err := a.groupedColumns(gs.Content)
		if err != nil {
			return nil, err
		}
		return []map[groupedColumn]bool{cols}, nil
	}
}

// groupedColumns types a grouping expression, or a list of them, and
// collects the columns of the scope it reads, through the output names
// it may refer to.
func (a *analyzer) groupedColumns(n ast.Node) (map[groupedColumn]bool, error) {
	cols := map[groupedColumn]bool{}
	if l, ok := n.(*ast.List); ok && l == nil {
		return cols, nil
	}
	if _, err := a.typeExpr(n); err != nil {
		return nil, err
	}
	a.collectGrouped(n, cols, map[string]bool{})
	return cols, nil
}

func (a *analyzer) collectGrouped(n ast.Node, cols map[groupedColumn]bool, aliases map[string]bool) {
	astutils.Walk(astutils.VisitorFunc(func(node ast.Node) {
		cr, ok := node.(*ast.ColumnRef)
		if !ok {
			return
		}
		parts := flattenFields(cr.Fields)
		if len(parts) == 0 {
			return
		}
		relation, column := "", parts[0]
		if len(parts) >= 2 {
			relation, column = parts[0], parts[1]
		}
		// The expression was typed already, so an ambiguous name has
		// been reported and an unknown one is an alias or nothing.
		if m, ok, _ := a.scope.resolveIn(relation, column, 0, len(a.scope.rels)); ok {
			cols[groupedColumn{rel: m.idx, name: m.col.Name}] = true
			return
		}
		if val, ok := a.aliases[column]; ok && relation == "" && !aliases[column] {
			aliases[column] = true
			a.collectGrouped(val, cols, aliases)
		}
	}), n)
}
