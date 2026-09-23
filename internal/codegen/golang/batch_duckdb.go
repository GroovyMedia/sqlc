package golang

import (
	"fmt"
	"strings"

	"github.com/sqlc-dev/sqlc/internal/plugin"
)

// batchFields maps each parameter of a DuckDB JSON batch to a field of the
// JSON row. The compiler typed the row with from_json; a DATE or TIMESTAMP
// travels as text, formatted here the way the statement parses it.
func batchFields(gq Query, query *plugin.Query) ([]BatchField, error) {
	type param struct {
		number int32
		typ    string
		expr   string
		dbType string
		array  bool
	}
	var params []param
	if gq.Arg.Struct == nil {
		if len(query.Params) != 1 {
			return nil, fmt.Errorf("batch: %d parameters without a params struct", len(query.Params))
		}
		p := query.Params[0]
		params = append(params, param{p.Number, gq.Arg.Typ, escape(gq.Arg.Name), p.Column.GetType().GetName(), p.Column.GetIsArray()})
	} else {
		if len(gq.Arg.Struct.Fields) != len(query.Params) {
			return nil, fmt.Errorf("batch: %d fields for %d parameters", len(gq.Arg.Struct.Fields), len(query.Params))
		}
		for i, f := range gq.Arg.Struct.Fields {
			p := query.Params[i]
			params = append(params, param{p.Number, f.Type, gq.Arg.VariableForField(f), p.Column.GetType().GetName(), p.Column.GetIsArray()})
		}
	}

	unnested := map[int32]bool{}
	for _, n := range query.Batch.GetUnnestParams() {
		unnested[n] = true
	}

	var out []BatchField
	for _, p := range params {
		if unnested[p.number] {
			f, err := unnestField(p.number, p.typ, p.expr, p.dbType)
			if err != nil {
				return nil, err
			}
			out = append(out, f)
			continue
		}
		f := BatchField{
			Name:  fmt.Sprintf("P%d", p.number),
			Key:   fmt.Sprintf("p%d", p.number),
			Type:  p.typ,
			Value: p.expr,
		}
		helper := ""
		switch db := strings.ToLower(p.dbType); {
		case p.array:
		case db == "date":
			helper = "duckdbBatchDate"
		case db == "timestamp":
			helper = "duckdbBatchTimestamp"
		case db == "timestamp with time zone" || db == "timestamptz":
			helper = "duckdbBatchTimestamptz"
		}
		switch {
		case helper != "":
			switch p.typ {
			case "time.Time":
				f.Type, f.Value = "string", helper+"("+p.expr+")"
			case "*time.Time":
				f.Type, f.Value = "*string", helper+"Ptr("+p.expr+")"
			case "sql.NullTime":
				f.Type, f.Value = "*string", helper+"Null("+p.expr+")"
			default:
				return nil, fmt.Errorf("batch: parameter %d: %s is not a time type", p.number, p.typ)
			}
		case p.typ == "[]byte" && strings.EqualFold(p.dbType, "json"):
			f.Type, f.Value = "json.RawMessage", "json.RawMessage("+p.expr+")"
		case strings.HasPrefix(p.typ, "sql.Null") || strings.HasPrefix(p.typ, "Null"):
			f.Type, f.Value = "any", "duckdbBatchValuer("+p.expr+")"
		}
		out = append(out, f)
	}
	return out, nil
}

// unnestField reads element sqlcI of an unnested list, or null past its
// end, as unnest pads a shorter list.
func unnestField(number int32, typ, expr, dbType string) (BatchField, error) {
	f := BatchField{
		Name:  fmt.Sprintf("P%d", number),
		Key:   fmt.Sprintf("p%d", number),
		Type:  "any",
		Value: "duckdbBatchAt(" + expr + ", sqlcI)",
		List:  expr,
	}
	if !strings.HasPrefix(typ, "[]") {
		return f, fmt.Errorf("batch: parameter %d: unnest of %s, which is not a slice", number, typ)
	}
	elem := strings.TrimPrefix(typ, "[]")
	helper := ""
	switch db := strings.ToLower(dbType); db {
	case "date":
		helper = "duckdbBatchDate"
	case "timestamp":
		helper = "duckdbBatchTimestamp"
	case "timestamp with time zone", "timestamptz":
		helper = "duckdbBatchTimestamptz"
	}
	switch {
	case helper != "" && elem == "time.Time":
		f.Value = "duckdbBatchAtWith(" + expr + ", sqlcI, " + helper + ")"
	case helper != "" && elem == "*time.Time":
		f.Value = "duckdbBatchAtWith(" + expr + ", sqlcI, " + helper + "Ptr)"
	case helper != "":
		return f, fmt.Errorf("batch: parameter %d: %s is not a list of times", number, typ)
	case strings.HasPrefix(elem, "sql.Null"):
		return f, fmt.Errorf("batch: parameter %d: unnest of %s is not supported", number, typ)
	}
	return f, nil
}
