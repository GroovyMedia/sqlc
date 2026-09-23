package compiler

import (
	"fmt"
	"sort"
	"strings"

	"github.com/sqlc-dev/sqlc/internal/sql/ast"
)

// BatchPlan says how a DuckDB :batch query runs.
//
// DuckDB runs one statement per row slowly (an upsert costs about 400 µs)
// and cannot bind a NULL inside a list parameter. So a batch whose row
// comes from a one-row VALUES list is rewritten to take every row at once,
// as one JSON array in $1, with the VALUES list replaced by a SELECT over
// the array. The rows run in rounds so that the result is the one the
// rows would give one statement at a time: round r ($2) takes the r-th
// row of each key, where the key is the ON CONFLICT target or the columns
// a MERGE joins on. Rounds is the query that counts the rounds. For a
// batch with RETURNING, Read reads the rows back by key after the write,
// with the index of the batch element each came from first.
//
// A plain INSERT with RETURNING has no key to read back by, so it runs
// once per batch element (BatchJSONEach): the element's rows travel as
// JSON, and every row the statement returns belongs to that element.
//
// Any other batch runs its statement once per row (BatchLoop).
//
// The row may also come from "SELECT unnest(@a), unnest(@b), ...": each
// batch element then stands for as many rows as its longest list, the
// lists zipped by position as unnest zips them. Unnest lists those
// placeholders; each JSON row carries one element of each list.
type BatchPlan struct {
	Mode   string
	Rounds string
	Read   string
	Unnest []int
}

const (
	BatchJSON     = "json"
	BatchJSONEach = "json_each"
	BatchLoop     = "loop"
)

// duckdbBatchType is the from_json type of a parameter, and whether the
// value travels as base64 text. ok is false for a type the JSON form does
// not carry; such a batch runs row by row.
func duckdbBatchType(col *Column) (typ string, base64, ok bool) {
	name := strings.ToLower(col.DataType)
	if col.TypeExpr != nil && col.TypeExpr.IsArray() {
		inner := col.TypeExpr.Innermost()
		switch strings.ToLower(inner.Name) {
		case "date", "timestamp", "timestamp with time zone", "timestamptz", "blob", "bytea", "decimal", "numeric":
			return "", false, false
		}
		elem, _, ok := duckdbBatchType(&Column{DataType: inner.Name})
		if !ok {
			return "", false, false
		}
		return elem + strings.Repeat("[]", col.TypeExpr.ArrayDims()), false, true
	}
	switch name {
	case "boolean", "bool", "tinyint", "smallint", "integer", "int", "bigint", "hugeint",
		"utinyint", "usmallint", "uinteger", "ubigint", "uhugeint", "float", "real", "double",
		"varchar", "text", "string", "uuid", "json", "date", "timestamp":
		return strings.ToUpper(name), false, true
	case "timestamp with time zone", "timestamptz":
		return "TIMESTAMP WITH TIME ZONE", false, true
	case "blob", "bytea":
		return "VARCHAR", true, true
	case "decimal", "numeric", "interval", "time", "time with time zone", "struct", "map", "union", "bit", "any", "":
		return "", false, false
	}
	// An enum or other type the schema declares travels as its text.
	return "VARCHAR", false, true
}

// planDuckDBBatch rewrites a :batch statement for the JSON form. It returns
// the statement to run and the plan, or a BatchLoop plan and the original
// text when the statement does not fit the JSON form.
func planDuckDBBatch(raw *ast.RawStmt, rawSQL string, params []Parameter) (string, *BatchPlan, error) {
	loop := &BatchPlan{Mode: BatchLoop}
	var b *ast.BatchSource
	switch s := raw.Stmt.(type) {
	case *ast.InsertStmt:
		b = s.Batch
	case *ast.MergeStmt:
		b = s.Batch
	}
	if b == nil || len(params) == 0 {
		return rawSQL, loop, nil
	}
	returning := len(b.ReturningItems) > 0
	each := returning && !b.OnConflict && !b.Merge
	if returning && !each && (b.Merge || b.DoNothing) {
		return rawSQL, loop, nil
	}
	if (b.OnConflict || b.Merge) && len(b.Keys) == 0 {
		return rawSQL, loop, nil
	}

	// Every parameter must be one the VALUES row holds.
	inValues := map[int]bool{}
	for _, p := range b.Params {
		inValues[p.Number] = true
	}
	// An unnested placeholder must appear only inside its unnest.
	unnested := map[int]bool{}
	for _, u := range b.Unnest {
		unnested[u.Number] = true
	}
	for _, p := range b.Params {
		if !unnested[p.Number] {
			continue
		}
		inside := false
		for _, u := range b.Unnest {
			if u.Number == p.Number && p.Start >= u.Start && p.End <= u.End {
				inside = true
			}
		}
		if !inside {
			return rawSQL, loop, nil
		}
	}
	types := map[int]string{}
	blob := map[int]bool{}
	for _, p := range params {
		if !inValues[p.Number] || p.Column == nil {
			return rawSQL, loop, nil
		}
		col := p.Column
		if unnested[p.Number] {
			if col.TypeExpr == nil || !col.TypeExpr.IsArray() {
				return rawSQL, loop, nil
			}
			elem := col.TypeExpr.Element()
			col = &Column{DataType: elem.Name, TypeExpr: elem}
		}
		typ, b64, ok := duckdbBatchType(col)
		if !ok {
			return rawSQL, loop, nil
		}
		types[p.Number] = typ
		blob[p.Number] = b64
	}

	off := raw.StmtLocation
	text := func(span [2]int) string { return rawSQL[span[0]-off : span[1]-off] }

	// Each expression is rewritten by replacing, left to right, every
	// unnest(@x) with the JSON column of x and every other placeholder
	// with its own.
	type edit struct {
		start, end int
		text       string
	}
	ref := func(n int) string {
		if blob[n] {
			return fmt.Sprintf("from_base64(sqlc_b.p%d)", n)
		}
		return fmt.Sprintf("sqlc_b.p%d", n)
	}
	var edits []edit
	for _, u := range b.Unnest {
		edits = append(edits, edit{u.Start, u.End, ref(u.Number)})
	}
	for _, p := range b.Params {
		if !unnested[p.Number] {
			edits = append(edits, edit{p.Start, p.End, ref(p.Number)})
		}
	}
	sort.Slice(edits, func(i, j int) bool { return edits[i].start < edits[j].start })
	exprs := make([]string, len(b.Exprs))
	for i, span := range b.Exprs {
		var sb strings.Builder
		at := span[0]
		for _, e := range edits {
			if e.start < span[0] || e.end > span[1] {
				continue
			}
			sb.WriteString(text([2]int{at, e.start}))
			sb.WriteString(e.text)
			at = e.end
		}
		sb.WriteString(text([2]int{at, span[1]}))
		exprs[i] = sb.String()
	}

	var keys []string
	for _, k := range b.Keys {
		idx := -1
		for i, col := range b.Columns {
			if strings.EqualFold(col, k) {
				idx = i
			}
		}
		if idx < 0 || idx >= len(exprs) {
			return rawSQL, loop, nil
		}
		keys = append(keys, exprs[idx])
	}

	numbers := make([]int, 0, len(types))
	for n := range types {
		numbers = append(numbers, n)
	}
	sort.Ints(numbers)
	var fields []string
	for _, n := range numbers {
		fields = append(fields, fmt.Sprintf("%q:%q", fmt.Sprintf("p%d", n), types[n]))
	}
	fields = append(fields, `"sqlc_ord":"BIGINT"`, `"sqlc_elem":"BIGINT"`)
	structure := strings.ReplaceAll("[{"+strings.Join(fields, ",")+"}]", "'", "''")
	source := "(SELECT unnest(from_json($1::JSON, '" + structure + "'), recursive := true)) AS sqlc_b"

	sel := "SELECT " + strings.Join(exprs, ", ") + " FROM " + source
	plan := &BatchPlan{Mode: BatchJSON}
	if each {
		plan.Mode = BatchJSONEach
	}
	for n := range unnested {
		plan.Unnest = append(plan.Unnest, n)
	}
	sort.Ints(plan.Unnest)
	if len(keys) > 0 {
		partition := strings.Join(keys, ", ")
		sel += " QUALIFY row_number() OVER (PARTITION BY " + partition + " ORDER BY sqlc_b.sqlc_ord) = $2"
		plan.Rounds = "SELECT coalesce(max(n), 0)::BIGINT FROM (SELECT count(*) AS n FROM " + source + " GROUP BY " + partition + ")"
	}

	write := rawSQL[:b.Values[0]-off] + sel
	if returning && !each {
		write += rawSQL[b.Values[1]-off:b.Returning[0]-off] + rawSQL[b.Returning[1]-off:]
		var items []string
		for i, span := range b.ReturningItems {
			if b.ReturningStar[i] {
				items = append(items, b.Qualifier+".*")
			} else {
				items = append(items, text(span))
			}
		}
		var on []string
		for i, k := range b.Keys {
			on = append(on, b.Qualifier+"."+k+" IS NOT DISTINCT FROM ("+keys[i]+")")
		}
		plan.Read = "SELECT sqlc_b.sqlc_elem, " + strings.Join(items, ", ") + " FROM " + source +
			" JOIN " + b.Table + " ON " + strings.Join(on, " AND ") + " ORDER BY sqlc_b.sqlc_ord"
	} else {
		write += rawSQL[b.Values[1]-off:]
	}
	return write, plan, nil
}
