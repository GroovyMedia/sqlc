package duckdb

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	"github.com/sqlc-dev/sqlc/internal/goldeneye/dialect"
)

// A built-in macro is an expression over its parameters, typed when a call
// is bound, so duckdb_functions() lists one with no parameter types and no
// return type. What the catalog does not say is written here: the
// parameter types of each macro's overloads, spelled the way the scalar
// functions' seeds spell theirs, "any" and "any[]" standing for a
// parameter that takes any type. The return type of each overload is what
// the CLI reports for a call over NULLs of those types, so that it is the
// binder's answer rather than a guess. A macro the CLI lists that is
// neither described here nor omitted fails the generator, so a new build's
// macros are described rather than dropped.

// macroArgs is the parameter types of each built-in macro's overloads.
// The macros that aggregate a list — list_sum(l) is list_aggr(l, 'sum')
// — are not listed: each is read from its definition and gets an
// overload per single-argument overload of the aggregate it names, over
// a list of that argument's type.
var macroArgs = map[string][][]string{
	"ago":                           {{"interval"}},
	"array_append":                  {{"any[]", "any"}},
	"array_pop_back":                {{"any[]"}},
	"array_pop_front":               {{"any[]"}},
	"array_prepend":                 {{"any", "any[]"}},
	"array_push_back":               {{"any[]", "any"}},
	"array_push_front":              {{"any[]", "any"}},
	"array_reverse":                 {{"any[]"}},
	"array_to_string":               {{"any[]", "varchar"}},
	"array_to_string_comma_default": {{"any[]", "varchar"}},
	"current_catalog":               {{}},
	"current_role":                  {{}},
	"current_user":                  {{}},
	"date_add": {
		{"date", "interval"},
		{"timestamp", "interval"},
		{"timestamp with time zone", "interval"},
		{"time", "interval"},
		{"time with time zone", "interval"},
		{"interval", "interval"},
	},
	"days_in_month": {{"date"}, {"timestamp"}, {"timestamp with time zone"}},
	// Division is floating point: a FLOAT stays one, anything else
	// becomes a DOUBLE.
	"fdiv":                  {{"double", "double"}, {"float", "float"}},
	"fmod":                  {{"double", "double"}, {"float", "float"}},
	"generate_subscripts":   {{"any[]", "bigint"}},
	"geomean":               {{"double"}},
	"geometric_mean":        {{"double"}},
	"get_block_size":        {{"varchar"}},
	"if":                    {{"boolean", "any", "any"}},
	"json":                  {{"json"}, {"varchar"}},
	"json_group_array":      {{"any"}},
	"json_group_object":     {{"any", "any"}},
	"json_group_structure":  {{"any"}},
	"list_append":           {{"any[]", "any"}},
	"list_prepend":          {{"any", "any[]"}},
	"list_reverse":          {{"any[]"}},
	"map_contains_entry":    {{"map", "any", "any"}},
	"map_contains_value":    {{"map", "any"}},
	"md5_number_lower":      {{"varchar"}, {"blob"}},
	"md5_number_upper":      {{"varchar"}, {"blob"}},
	"nullif":                {{"any", "any"}},
	"regexp_split_to_table": {{"varchar", "varchar"}},
	"session_user":          {{}},
	"split_part":            {{"varchar", "varchar", "bigint"}},
	"user":                  {{}},
	"variant_group_array":   {{"any"}},
	"wavg":                  {{"double", "double"}},
	"weighted_avg":          {{"double", "double"}},
}

// aggregateMacros are the macros that expand to an aggregate. The seed
// lists them as aggregates, NULL over no rows like the rest.
var aggregateMacros = map[string]bool{
	"geomean":              true,
	"geometric_mean":       true,
	"json_group_array":     true,
	"json_group_object":    true,
	"json_group_structure": true,
	"variant_group_array":  true,
	"wavg":                 true,
	"weighted_avg":         true,
}

// omittedMacros are the macros the dialect leaves out: one that exists
// for its side effect and returns nothing a query can use, and the
// helpers COPY TO uses to format dates, which no query calls.
var omittedMacros = map[string]bool{
	"assert_true":                     true,
	"json_copy_strftime_if_date":      true,
	"json_copy_strftime_if_timestamp": true,
}

// listAggregate matches the definition of a macro that aggregates a list,
// capturing the aggregate's name: list_sum(l) is list_aggr(l, 'sum').
var listAggregate = regexp.MustCompile(`^list_aggr\([a-z_]+, '([a-z_]+)'\)$`)

// sampleType stands in for "any" in a probe: a type no macro returns for
// an argument of another type, so that a result of this type is the
// argument's own, and one the aggregates treat as they do any other,
// where a numeric type would have median return a DOUBLE.
const sampleType = "uuid"

// readMacros describes the built-in macros among the rows, keyed by name:
// each macro's overloads, with the return type the CLI reports for each.
// aggregates holds the single-argument overloads of each aggregate, for
// the macros that aggregate a list.
func readMacros(ctx context.Context, binary string, rows []functionRow, aggregates map[string][]dialect.Function, known map[string]bool) (map[string][]dialect.Function, error) {
	arities := map[string]map[int]bool{}
	definitions := map[string]string{}
	for _, row := range rows {
		if row.FunctionType != "macro" {
			continue
		}
		if arities[row.Name] == nil {
			arities[row.Name] = map[int]bool{}
		}
		arities[row.Name][len(row.Parameters)] = true
		if row.MacroDefinition != nil {
			definitions[row.Name] = *row.MacroDefinition
		}
	}
	for name := range macroArgs {
		if arities[name] == nil {
			return nil, fmt.Errorf("macroArgs describes %q, which the CLI does not list", name)
		}
	}

	macros := map[string][]dialect.Function{}
	var pending []*dialect.Function
	var calls []string
	for name, counts := range arities {
		overloads, err := macroOverloads(name, counts, definitions[name], aggregates)
		if err != nil {
			return nil, err
		}
		for i := range overloads {
			fn := &overloads[i]
			// An aggregate is NULL over no rows, and a lookup that finds
			// nothing is NULL; a macro that aggregates a list is NULL
			// over an empty list when its aggregate is over no rows.
			if fn.Kind == "a" || (mayBeNull[name] > 0 && len(fn.Args) >= mayBeNull[name]) {
				fn.Nullable = true
			}
			fn.NeverNull = neverNull[name]
			pending = append(pending, fn)
			calls = append(calls, macroCall(*fn))
		}
		if overloads != nil {
			macros[name] = overloads
		}
	}

	returns, err := probeMacros(ctx, binary, calls)
	if err != nil {
		return nil, err
	}
	for i, fn := range pending {
		typ, ok := macroReturn(returns[i], polymorphic(*fn), known)
		if !ok {
			return nil, fmt.Errorf("macro %s returns %s, which the seed has no type for", calls[i], returns[i])
		}
		fn.Returns = typ
	}
	return macros, nil
}

// macroOverloads lists a macro's overloads without their return types:
// from macroArgs, checked against the parameter counts the CLI lists, or
// from the aggregate a list macro names. An omitted macro has none.
func macroOverloads(name string, counts map[int]bool, definition string, aggregates map[string][]dialect.Function) ([]dialect.Function, error) {
	if omittedMacros[name] {
		return nil, nil
	}
	var overloads []dialect.Function
	if argLists, ok := macroArgs[name]; ok {
		for _, args := range argLists {
			if !counts[len(args)] {
				return nil, fmt.Errorf("macroArgs gives %q %d parameters, which the CLI does not list", name, len(args))
			}
			fn := dialect.Function{Name: name}
			if aggregateMacros[name] {
				fn.Kind = "a"
			}
			for _, arg := range args {
				fn.Args = append(fn.Args, dialect.Arg{Type: arg})
			}
			overloads = append(overloads, fn)
		}
		for count := range counts {
			if !hasArity(overloads, count) {
				return nil, fmt.Errorf("the CLI lists %q with %d parameters, which macroArgs does not give", name, count)
			}
		}
		return overloads, nil
	}
	m := listAggregate.FindStringSubmatch(definition)
	if m == nil {
		return nil, fmt.Errorf("macro %q is not described: add it to macroArgs or omittedMacros", name)
	}
	if len(aggregates[m[1]]) == 0 {
		return nil, fmt.Errorf("macro %q aggregates with %q, which has no single-argument overload", name, m[1])
	}
	for _, agg := range aggregates[m[1]] {
		overloads = append(overloads, dialect.Function{
			Name:     name,
			Args:     []dialect.Arg{{Type: agg.Args[0].Type + "[]"}},
			Nullable: agg.Nullable,
		})
	}
	return overloads, nil
}

func hasArity(overloads []dialect.Function, n int) bool {
	for _, fn := range overloads {
		if len(fn.Args) == n {
			return true
		}
	}
	return false
}

// polymorphic reports an overload with a parameter that takes any type.
func polymorphic(fn dialect.Function) bool {
	for _, arg := range fn.Args {
		if strings.TrimRight(arg.Type, "[]") == "any" {
			return true
		}
	}
	return false
}

// macroCall spells a call of an overload over NULLs of its parameter
// types, for DESCRIBE to type. The name is quoted, since some macros are
// named by keywords.
func macroCall(fn dialect.Function) string {
	casts := make([]string, len(fn.Args))
	for i, arg := range fn.Args {
		casts[i] = "CAST(NULL AS " + probeType(arg.Type) + ")"
	}
	return `"` + fn.Name + `"(` + strings.Join(casts, ", ") + ")"
}

// probeType spells a seeded type as one a NULL can be cast to: "any" as
// the sample type, and a composite type with the type arguments its
// spelling needs.
func probeType(seed string) string {
	if elem, ok := strings.CutSuffix(seed, "[]"); ok {
		return probeType(elem) + "[]"
	}
	switch seed {
	case "any":
		return sampleType
	case "decimal":
		return "DECIMAL(18,3)"
	case "map":
		return "MAP(" + sampleType + ", " + sampleType + ")"
	case "struct":
		return "STRUCT(a " + sampleType + ")"
	case "union":
		return "UNION(a " + sampleType + ")"
	}
	return seed
}

// macroReturn reads a type the CLI reports back into the seed's spelling:
// for a polymorphic overload, the sample type stands for "any".
func macroReturn(columnType string, polymorphic bool, known map[string]bool) (string, bool) {
	name, ok := seedTypeName(columnType, known)
	if !ok {
		return "", false
	}
	if elem := strings.TrimRight(name, "[]"); polymorphic && elem == sampleType {
		return "any" + name[len(elem):], true
	}
	return name, true
}

type describeRow struct {
	ColumnName string `json:"column_name"`
	ColumnType string `json:"column_type"`
}

// probeMacros asks the CLI what each call returns, all in one DESCRIBE,
// which binds the calls without running them. A batch the binder rejects
// is retried one call at a time, so that the error names the call.
func probeMacros(ctx context.Context, binary string, calls []string) ([]string, error) {
	if len(calls) == 0 {
		return nil, nil
	}
	items := make([]string, len(calls))
	for i, call := range calls {
		items[i] = fmt.Sprintf("%s AS \"%d\"", call, i)
	}
	var rows []describeRow
	err := query(ctx, binary, "DESCRIBE SELECT "+strings.Join(items, ", "), &rows)
	if err == nil && len(rows) != len(calls) {
		err = fmt.Errorf("DESCRIBE reported %d columns for %d calls", len(rows), len(calls))
	}
	if err != nil {
		if len(calls) == 1 {
			return nil, fmt.Errorf("macro %s: %w", calls[0], err)
		}
		returns := make([]string, 0, len(calls))
		for _, call := range calls {
			ret, err := probeMacros(ctx, binary, []string{call})
			if err != nil {
				return nil, err
			}
			returns = append(returns, ret...)
		}
		return returns, nil
	}
	returns := make([]string, len(calls))
	for i, row := range rows {
		returns[i] = row.ColumnType
	}
	return returns, nil
}
