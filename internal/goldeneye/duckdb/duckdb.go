// Package duckdb generates the DuckDB dialect seed under
// internal/engine/duckdb/dialect — types.jsonl, functions.jsonl and
// operators.jsonl — from a live DuckDB CLI, the same way the postgresql
// package generates PostgreSQL's from a live server, and verifies the
// DuckDB analyze cases under internal/endtoend/testdata against the same
// CLI.
//
// The CLI is a DuckDB 2.0 build, the release darkwing is pinned against,
// which has no release to download yet: Install fetches the current build
// of DuckDB's v2.0 preview channel into the user cache directory. The CLI
// is located through the DUCKDB environment variable, then the cached
// build, then "duckdb" on PATH.
package duckdb

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"slices"
	"sort"
	"strings"

	"github.com/sqlc-dev/sqlc/internal/goldeneye/dialect"
)

// Engine is the name of the engine directory the dialect lives under.
const Engine = "duckdb"

// Locate finds the DuckDB CLI: the DUCKDB environment variable wins, then
// the cached build of DefaultVersion, then "duckdb" on PATH.
func Locate() (string, error) {
	if path := os.Getenv("DUCKDB"); path != "" {
		return path, nil
	}
	if path, err := cachedBinary(DefaultVersion); err == nil {
		if _, err := os.Stat(path); err == nil {
			return path, nil
		}
	}
	path, err := exec.LookPath("duckdb")
	if err != nil {
		return "", errors.New("no duckdb CLI found: run `go run ./cmd/goldeneye install duckdb` in internal/goldeneye, set DUCKDB to a DuckDB 2.0 binary, or put duckdb on PATH")
	}
	return path, nil
}

// Version reports the release a CLI is.
func Version(ctx context.Context, binary string) (string, error) {
	out, err := exec.CommandContext(ctx, binary, "--version").Output()
	if err != nil {
		return "", fmt.Errorf("duckdb --version: %w", err)
	}
	return "DuckDB " + strings.TrimSpace(string(out)), nil
}

// query runs a SQL statement against an in-memory database and decodes the
// CLI's JSON output into rows.
func query(ctx context.Context, binary, sql string, rows any) error {
	cmd := exec.CommandContext(ctx, binary, "-json", ":memory:", "-c", sql)
	out, err := cmd.Output()
	if err != nil {
		if exit, ok := err.(*exec.ExitError); ok {
			return fmt.Errorf("duckdb: %s: %s", err, exit.Stderr)
		}
		return fmt.Errorf("duckdb: %w", err)
	}
	// An empty result set prints nothing rather than [].
	if len(out) == 0 {
		return nil
	}
	return json.Unmarshal(out, rows)
}

type typeRow struct {
	TypeName    string  `json:"type_name"`
	LogicalType string  `json:"logical_type"`
	Category    *string `json:"type_category"`
}

type functionRow struct {
	Name           string   `json:"function_name"`
	FunctionType   string   `json:"function_type"`
	ParameterTypes []string `json:"parameter_types"`
	Varargs        *string  `json:"varargs"`
	ReturnType     *string  `json:"return_type"`
}

// metaTypes are type ids that never describe a column's value: sentinels and
// binder-internal types the dump lists alongside the real ones.
var metaTypes = map[string]bool{
	"null":    true,
	"unknown": true,
	"invalid": true,
	"any":     true,
	"type":    true,
	"lambda":  true,
	"table":   true,
	"pointer": true,
}

// categoryLetter maps duckdb_types() categories onto the PostgreSQL category
// letters the seed package uses.
func categoryLetter(category *string) string {
	if category == nil {
		return "U"
	}
	switch *category {
	case "NUMERIC":
		return "N"
	case "STRING":
		return "S"
	case "DATETIME":
		return "D"
	case "BOOLEAN":
		return "B"
	case "COMPOSITE":
		return "C"
	default:
		return "U"
	}
}

func readTypes(ctx context.Context, binary string) ([]dialect.Type, error) {
	var rows []typeRow
	err := query(ctx, binary, `
SELECT type_name, logical_type, type_category
FROM duckdb_types()
WHERE database_name = 'system'
ORDER BY type_name`, &rows)
	if err != nil {
		return nil, err
	}

	// Group the dump's one-row-per-spelling by logical type: the spelling
	// matching the logical type id is the canonical name, the rest are
	// aliases. A spelling is listed once per schema it is visible in, so
	// an alias is kept once.
	grouped := map[string]*dialect.Type{}
	var order []string
	seen := map[string]bool{}
	for _, row := range rows {
		logical := strings.ToLower(row.LogicalType)
		if metaTypes[logical] {
			continue
		}
		t, ok := grouped[logical]
		if !ok {
			t = &dialect.Type{Name: logical, Category: categoryLetter(row.Category)}
			grouped[logical] = t
			order = append(order, logical)
		}
		if t.Category == "U" {
			t.Category = categoryLetter(row.Category)
		}
		if name := strings.ToLower(row.TypeName); name != logical && !seen[logical+"\x00"+name] {
			seen[logical+"\x00"+name] = true
			t.Aliases = append(t.Aliases, name)
		}
	}

	sort.Strings(order)
	types := make([]dialect.Type, 0, len(order)+2)
	// "any" stands in for the generic parameters of DuckDB's polymorphic
	// functions (ANY, T, K, V); the analyzer resolves a call returning it
	// to the type of the call's first argument. "lambda" is the parameter
	// list_transform and its relatives take a lambda in, which no value
	// has the type of.
	types = append(types, dialect.Type{Name: "any", Category: "U"}, dialect.Type{Name: "lambda", Category: "U"})
	for _, name := range order {
		types = append(types, *grouped[name])
	}
	return types, nil
}

// typeNames is the set of names the seed declares, for filtering out function
// overloads over generic or binder-internal types.
func typeNames(types []dialect.Type) map[string]bool {
	names := map[string]bool{}
	for _, t := range types {
		names[t.Name] = true
		for _, alias := range t.Aliases {
			names[alias] = true
		}
	}
	return names
}

// seedTypeName maps a duckdb_functions() type spelling to a seeded type name:
// lowercased, with modifiers dropped (DECIMAL(18,3) is a decimal), array
// markers kept (fixed sizes and generic bounds included: DOUBLE[3] and
// ANY[] are arrays), and the generic parameters of polymorphic functions
// mapped to "any". It reports false for the internal types a seed cannot
// describe.
func seedTypeName(name string, known map[string]bool) (string, bool) {
	name = strings.ToLower(strings.TrimSpace(name))
	if close := strings.LastIndexByte(name, ']'); close == len(name)-1 {
		if open := strings.LastIndexByte(name, '['); open != -1 {
			elementName, ok := seedTypeName(name[:open], known)
			return elementName + "[]", ok
		}
	}
	if open := strings.IndexByte(name, '('); open != -1 {
		name = name[:open]
	}
	switch name {
	case "any", "t", "k", "v":
		return "any", true
	case "lambda":
		return "lambda", true
	}
	if !known[name] {
		return "", false
	}
	return name, true
}

// isOperatorName reports a function named by symbols rather than an
// identifier — the spelling of a binary or prefix operator.
func isOperatorName(name string) bool {
	return strings.IndexFunc(name, func(r rune) bool {
		return (r < 'a' || r > 'z') && (r < '0' || r > '9') && r != '_' && r != '$'
	}) != -1
}

// neverNull lists the scalar functions whose result is not NULL when an
// argument is: DuckDB binds them with special NULL handling rather than
// the default, which returns NULL for any NULL argument.
var neverNull = map[string]bool{
	"concat":      true,
	"concat_ws":   true,
	"greatest":    true,
	"hash":        true,
	"json_object": true,
	"least":       true,
	"list_pack":   true,
	"list_value":  true,
	"row":         true,
	"struct_pack": true,
	"typeof":      true,
}

// mayBeNull lists the scalar and window functions whose result can be NULL
// when no argument is: a lookup that finds nothing, a window row with no
// neighbour, an aggregate over an empty list. The number is the fewest
// arguments an overload takes for that to hold — json_type(j) always has an
// answer, json_type(j, path) has none for a path that is not there.
var mayBeNull = map[string]int{
	"aggregate":              2,
	"array_aggr":             2,
	"array_aggregate":        2,
	"array_extract":          2,
	"array_indexof":          2,
	"array_position":         2,
	"json_array_length":      2,
	"json_extract":           2,
	"json_extract_path":      2,
	"json_extract_path_text": 2,
	"json_extract_string":    2,
	"json_keys":              2,
	"json_type":              2,
	"json_value":             2,
	"lag":                    1,
	"lead":                   1,
	"list_aggr":              2,
	"list_aggregate":         2,
	"list_element":           2,
	"list_extract":           2,
	"list_indexof":           2,
	"list_position":          2,
	"map_extract_value":      2,
	"nth_value":              2,
	"try_strptime":           2,
}

func functionKind(functionType string) string {
	switch functionType {
	case "aggregate":
		return "a"
	case "window":
		return "w"
	default:
		return ""
	}
}

func readFunctions(ctx context.Context, binary string, known map[string]bool) ([]dialect.Function, []dialect.Operator, error) {
	var rows []functionRow
	err := query(ctx, binary, `
SELECT DISTINCT function_name, function_type, parameter_types, varargs, return_type
FROM duckdb_functions()
WHERE database_name = 'system'
  AND schema_name = 'main'
  AND function_type IN ('scalar', 'aggregate', 'window', 'macro')
ORDER BY function_name, parameter_types::VARCHAR, return_type`, &rows)
	if err != nil {
		return nil, nil, err
	}

	var funcs []dialect.Function
	var operators []dialect.Operator
	seenFunc := map[string]bool{}
	seenOp := map[string]bool{}
	for _, row := range rows {
		if row.ReturnType == nil {
			continue
		}
		returns, ok := seedTypeName(*row.ReturnType, known)
		if !ok {
			continue
		}
		args := make([]string, 0, len(row.ParameterTypes))
		resolved := true
		for _, param := range row.ParameterTypes {
			arg, ok := seedTypeName(param, known)
			if !ok {
				resolved = false
				break
			}
			args = append(args, arg)
		}
		if !resolved {
			continue
		}

		if isOperatorName(row.Name) {
			// Operator-named functions become operator entries; sqlc's
			// engine reports them as operator applications. Only the
			// binary form over concrete scalar types is representable —
			// the seed's operator table, unlike its function table, does
			// not register generic or array types on demand.
			if row.FunctionType != "scalar" || len(args) != 2 || row.Varargs != nil {
				continue
			}
			if concrete := !strings.Contains(returns, "any") && !strings.Contains(returns, "[]") &&
				!strings.Contains(args[0], "any") && !strings.Contains(args[0], "[]") &&
				!strings.Contains(args[1], "any") && !strings.Contains(args[1], "[]"); !concrete {
				continue
			}
			key := row.Name + "\x00" + strings.Join(args, "\x00")
			if seenOp[key] {
				continue
			}
			seenOp[key] = true
			operators = append(operators, dialect.Operator{
				Name:   row.Name,
				Left:   args[0],
				Right:  args[1],
				Result: returns,
			})
			continue
		}

		fn := dialect.Function{
			Name:    row.Name,
			Kind:    functionKind(row.FunctionType),
			Returns: returns,
			// An aggregate over no rows returns NULL — except count,
			// which returns 0 — and so does a lookup that finds nothing.
			Nullable: (row.FunctionType == "aggregate" && !strings.HasPrefix(row.Name, "count")) ||
				(mayBeNull[row.Name] > 0 && len(args) >= mayBeNull[row.Name]),
			// A function's result is NULL when an argument is, except for
			// the ones that handle NULL themselves.
			NeverNull: strings.HasPrefix(row.Name, "count") || neverNull[row.Name],
		}
		for _, arg := range args {
			fn.Args = append(fn.Args, dialect.Arg{Type: arg})
		}
		if row.Varargs != nil {
			vararg, ok := seedTypeName(*row.Varargs, known)
			if !ok {
				continue
			}
			fn.Args = append(fn.Args, dialect.Arg{Type: vararg, Mode: "v"})
		}

		key := fn.Name + "\x00" + fn.Kind + "\x00" + strings.Join(args, "\x00")
		if seenFunc[key] {
			continue
		}
		seenFunc[key] = true
		funcs = append(funcs, fn)
	}
	return funcs, operators, nil
}

// mixedOperators measures what the binder makes of an operator over two
// numeric types it lists no overload for. DuckDB promotes mixed numeric
// operands to a common type when it binds them — DECIMAL * INTEGER is a
// DECIMAL, INTEGER / INTEGER a DOUBLE, SMALLINT + UTINYINT a SMALLINT —
// which duckdb_functions() does not describe, listing arithmetic over
// each type alone. Every operator seeded over two numeric types is tried
// over every ordered pair of numeric families it is not seeded over, in
// one CLI process, and a pair the binder accepts becomes an overload
// beside the seeded ones; one it rejects, as & over a DOUBLE is, is left
// out. The pairs of an operator are listed after its seeded overloads,
// in the order of the families in types.jsonl.
func mixedOperators(ctx context.Context, binary string, types []dialect.Type, operators []dialect.Operator) ([]dialect.Operator, error) {
	var numeric []string
	for _, t := range types {
		if t.Category == "N" {
			numeric = append(numeric, t.Name)
		}
	}
	known := typeNames(types)
	seeded := map[string]bool{}
	var names []string
	for _, op := range operators {
		seeded[op.Name+"\x00"+op.Left+"\x00"+op.Right] = true
		if slices.Contains(numeric, op.Left) && slices.Contains(numeric, op.Right) && !slices.Contains(names, op.Name) {
			names = append(names, op.Name)
		}
	}

	type probeRow struct {
		Op, Left, Right, Result string
	}
	var script strings.Builder
	for _, name := range names {
		for _, left := range numeric {
			for _, right := range numeric {
				if seeded[name+"\x00"+left+"\x00"+right] {
					continue
				}
				fmt.Fprintf(&script, "SELECT %s AS op, %s AS left, %s AS right, typeof(l %s r) AS result FROM (SELECT 1::%s AS l, 1::%s AS r);\n",
					quote(name), quote(left), quote(right), name, left, right)
			}
		}
	}
	// A statement the binder rejects fails on its own; the CLI runs the
	// rest and exits non-zero, with the results of the ones it ran.
	cmd := exec.CommandContext(ctx, binary, "-json", ":memory:")
	cmd.Stdin = strings.NewReader(script.String())
	out, err := cmd.Output()
	if err != nil {
		if _, ok := err.(*exec.ExitError); !ok {
			return nil, fmt.Errorf("duckdb: %w", err)
		}
	}
	measured := map[string][]dialect.Operator{}
	dec := json.NewDecoder(bytes.NewReader(out))
	for {
		var rows []probeRow
		if err := dec.Decode(&rows); err == io.EOF {
			break
		} else if err != nil {
			return nil, fmt.Errorf("duckdb: reading mixed operators: %w", err)
		}
		for _, row := range rows {
			result, ok := seedTypeName(row.Result, known)
			if !ok {
				continue
			}
			measured[row.Op] = append(measured[row.Op], dialect.Operator{Name: row.Op, Left: row.Left, Right: row.Right, Result: result})
		}
	}

	var merged []dialect.Operator
	for i, op := range operators {
		merged = append(merged, op)
		if i+1 == len(operators) || operators[i+1].Name != op.Name {
			merged = append(merged, measured[op.Name]...)
		}
	}
	return merged, nil
}

// quote writes a string as a SQL literal.
func quote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "''") + "'"
}

// Generate reads the dialect from the CLI.
func Generate(ctx context.Context, binary string) (dialect.Files, error) {
	types, err := readTypes(ctx, binary)
	if err != nil {
		return nil, err
	}
	funcs, operators, err := readFunctions(ctx, binary, typeNames(types))
	if err != nil {
		return nil, err
	}
	if operators, err = mixedOperators(ctx, binary, types, operators); err != nil {
		return nil, err
	}
	files := dialect.Files{}
	if files[dialect.TypesFile], err = dialect.JSONL(types); err != nil {
		return nil, err
	}
	if files[dialect.FunctionsFile], err = dialect.JSONL(funcs); err != nil {
		return nil, err
	}
	if files[dialect.OperatorsFile], err = dialect.JSONL(operators); err != nil {
		return nil, err
	}
	return files, nil
}
