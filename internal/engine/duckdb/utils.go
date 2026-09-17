package duckdb

import (
	"log"
	"reflect"
	"strings"

	dw "github.com/sqlc-dev/darkwing/ast"

	"github.com/sqlc-dev/sqlc/internal/debug"
	"github.com/sqlc-dev/sqlc/internal/sql/ast"
)

// todo stands in for syntax sqlc has no node for, placed where it was
// written and carrying its text, so that the analysis can name it.
func (c *cc) todo(n dw.Node) *ast.TODO {
	if debug.Active {
		log.Printf("duckdb.convert: Unknown node type %T\n", n)
	}
	start, end := extent(n)
	if start < 0 {
		return &ast.TODO{}
	}
	out := &ast.TODO{Location: start}
	if end <= len(c.src) {
		out.Text = strings.TrimSpace(c.src[start:end])
	}
	return out
}

// extent is the span of a node and everything under it. A node's own span
// mirrors DuckDB's, which for an operator is the operator token alone; the
// children give the whole expression. A synthesized node has a negative
// span and no extent.
func extent(n dw.Node) (int, int) {
	start, end := -1, -1
	var visit func(dw.Node)
	visit = func(n dw.Node) {
		// A child that is absent may be a typed nil, as a CREATE MACRO's
		// query is.
		if n == nil || (reflect.ValueOf(n).Kind() == reflect.Pointer && reflect.ValueOf(n).IsNil()) {
			return
		}
		if s, e := n.Pos(), n.End(); s >= 0 && e >= s {
			if start < 0 || s < start {
				start = s
			}
			if e > end {
				end = e
			}
		}
		for _, child := range n.Children() {
			visit(child)
		}
	}
	visit(n)
	return start, end
}

// identifier normalizes an identifier. DuckDB identifiers are
// case-insensitive (though case-preserving); the catalog matches them
// lowercased.
func identifier(id string) string {
	return strings.ToLower(id)
}

func NewIdentifier(t string) *ast.String {
	return &ast.String{Str: identifier(t)}
}

// schemaName normalizes a schema qualifier. DuckDB's default "main" schema
// maps to the catalog's default namespace, so it is treated as unqualified.
func schemaName(s string) string {
	s = identifier(s)
	if s == "main" {
		return ""
	}
	return s
}

func parseTableName(catalog, schema, name string) *ast.TableName {
	return &ast.TableName{
		Catalog: identifier(catalog),
		Schema:  schemaName(schema),
		Name:    identifier(name),
	}
}
