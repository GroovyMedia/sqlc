package compiler

import (
	"slices"
	"strings"

	"github.com/sqlc-dev/sqlc/internal/core"
	"github.com/sqlc-dev/sqlc/internal/sql/ast"
	"github.com/sqlc-dev/sqlc/internal/sql/catalog"
)

// coreResultCatalog dumps the core catalog into the legacy catalog shape a
// Result carries, so codegen sees the same table models either way a query
// set was analyzed. Relations and the enums a schema declared make the
// trip: codegen reads tables and their columns to build models, and enums
// to build a type per enum, and none of the other types, functions or
// operators the core catalog also holds.
func coreResultCatalog(c *core.Catalog) (*catalog.Catalog, error) {
	cat := catalog.New("public")
	namespaces, err := c.Namespaces()
	if err != nil {
		return nil, err
	}
	// The legacy catalog starts with its default schema, which the
	// namespace of the same name fills rather than repeats.
	schemas := make(map[string]*catalog.Schema, len(namespaces))
	for _, s := range cat.Schemas {
		schemas[s.Name] = s
	}
	defaults := c.DefaultNamespaces()
	for _, ns := range namespaces {
		schema, ok := schemas[ns.Name]
		if !ok {
			schema = &catalog.Schema{Name: ns.Name}
			schemas[ns.Name] = schema
			cat.Schemas = append(cat.Schemas, schema)
		}
		tables, err := c.TablesInNamespace(ns.OID)
		if err != nil {
			return nil, err
		}
		for _, table := range tables {
			cols, err := c.ClassCodegenColumns(table.OID)
			if err != nil {
				return nil, err
			}
			t := &catalog.Table{Rel: &ast.TableName{Schema: ns.Name, Name: table.Name}}
			for _, col := range cols {
				// Codegen reads a data type and an array flag, and renders
				// a "[]" per dimension, so an array of arrays of integers
				// is the type integer with two dimensions.
				expr, err := c.TypeExprOf(col.TypeOID)
				if err != nil {
					return nil, err
				}
				inner := expr.Innermost()
				column := &catalog.Column{
					Name:       col.Name,
					Type:       ast.TypeName{Name: strings.TrimSuffix(inner.Name, " unsigned")},
					IsNotNull:  col.NotNull,
					IsArray:    expr.IsArray(),
					ArrayDims:  expr.ArrayDims(),
					IsUnsigned: strings.HasSuffix(inner.Name, " unsigned"),
				}
				if len(inner.Args) > 0 && inner.Args[0].Int != nil {
					l := int(*inner.Args[0].Int)
					column.Length = &l
				}
				t.Columns = append(t.Columns, column)
			}
			schema.Tables = append(schema.Tables, t)
		}
		// A column's type names an enum the way the analyzer spells it:
		// bare when the enum sits in one of the dialect's default
		// namespaces (main for DuckDB, dbo for SQL Server), qualified
		// otherwise. Codegen looks a bare name up in the catalog's default
		// schema, so that is where such an enum goes; a qualified one
		// stays with its namespace.
		enums, err := c.EnumsInNamespace(ns.OID)
		if err != nil {
			return nil, err
		}
		target := schema
		if slices.Contains(defaults, ns.Name) {
			target = schemas[cat.DefaultSchema]
		}
		for _, e := range enums {
			target.Types = append(target.Types, &catalog.Enum{Name: e.Name, Vals: e.Labels})
		}
	}
	return cat, nil
}
