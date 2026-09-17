package golang

import (
	"log"
	"strings"

	"github.com/sqlc-dev/sqlc/internal/codegen/golang/opts"
	"github.com/sqlc-dev/sqlc/internal/codegen/sdk"
	"github.com/sqlc-dev/sqlc/internal/debug"
	"github.com/sqlc-dev/sqlc/internal/plugin"
)

// duckdbType maps a DuckDB type to the Go type a column of it scans into
// through database/sql and github.com/duckdb/duckdb-go/v2. The analysis
// core hands over canonical names (integer, timestamp with time zone,
// decimal), so the aliases here are for the spellings a schema may use.
//
// The driver hands database/sql a Go value of the column's own width, a
// *big.Int for the 128-bit and arbitrary-precision integers, 16 raw bytes
// for a UUID, its own Interval and Decimal for those, the decoded value of
// a JSON and []any for a LIST. A LIST, a JSON and a DECIMAL need a scanner
// from the duckdb helper file to land in the type chosen here; the rest
// scan as they are.
func duckdbType(req *plugin.GenerateRequest, options *opts.Options, col *plugin.Column) string {
	columnType := sdk.DataType(col.Type)
	notNull := col.NotNull || col.IsArray
	emitPointersForNull := options.EmitPointersForNullTypes
	emitPointersForNullEnums := emitPointersForNull
	if options.EmitPointersForNullEnumTypes != nil {
		emitPointersForNullEnums = *options.EmitPointersForNullEnumTypes
	}

	switch columnType {
	case "boolean", "bool", "logical":
		if notNull {
			return "bool"
		}
		if emitPointersForNull {
			return "*bool"
		}
		return "sql.NullBool"

	case "tinyint", "int1":
		if notNull {
			return "int8"
		}
		if emitPointersForNull {
			return "*int8"
		}
		// database/sql has no NullInt8.
		return "sql.NullInt16"

	case "smallint", "int2", "int16", "short":
		if notNull {
			return "int16"
		}
		if emitPointersForNull {
			return "*int16"
		}
		return "sql.NullInt16"

	case "integer", "int", "int4", "int32", "integral", "signed":
		if notNull {
			return "int32"
		}
		if emitPointersForNull {
			return "*int32"
		}
		return "sql.NullInt32"

	case "bigint", "int8", "int64", "long", "oid":
		if notNull {
			return "int64"
		}
		if emitPointersForNull {
			return "*int64"
		}
		return "sql.NullInt64"

	case "utinyint", "uint8":
		if notNull {
			return "uint8"
		}
		if emitPointersForNull {
			return "*uint8"
		}
		return "sql.Null[uint8]"

	case "usmallint", "uint16":
		if notNull {
			return "uint16"
		}
		if emitPointersForNull {
			return "*uint16"
		}
		return "sql.Null[uint16]"

	case "uinteger", "uint32":
		if notNull {
			return "uint32"
		}
		if emitPointersForNull {
			return "*uint32"
		}
		return "sql.Null[uint32]"

	case "ubigint", "uint64":
		if notNull {
			return "uint64"
		}
		if emitPointersForNull {
			return "*uint64"
		}
		return "sql.Null[uint64]"

	case "hugeint", "int128", "uhugeint", "uint128", "bignum", "varint":
		// The driver hands these over as a *big.Int, and nil is NULL.
		return "*big.Int"

	case "float", "float4", "real":
		if notNull {
			return "float32"
		}
		if emitPointersForNull {
			return "*float32"
		}
		return "sql.Null[float32]"

	case "double", "float8":
		if notNull {
			return "float64"
		}
		if emitPointersForNull {
			return "*float64"
		}
		return "sql.NullFloat64"

	case "decimal", "dec", "numeric":
		// The Go standard library has no decimal type, and DECIMAL(18, 3)
		// does not fit a float64, so the value is kept as its text, the
		// way the PostgreSQL and MySQL mappings keep a numeric.
		if notNull {
			return "string"
		}
		if emitPointersForNull {
			return "*string"
		}
		return "sql.NullString"

	case "varchar", "text", "string", "char", "bpchar", "nvarchar":
		if notNull {
			return "string"
		}
		if emitPointersForNull {
			return "*string"
		}
		return "sql.NullString"

	case "blob", "binary", "bytea", "varbinary":
		return "[]byte"

	case "date", "time", "time_ns", "time with time zone", "timetz",
		"timestamp", "datetime", "timestamp_us", "timestamp_s", "timestamp_ms", "timestamp_ns",
		"timestamp with time zone", "timestamptz", "timestamptz_ns":
		if notNull {
			return "time.Time"
		}
		if emitPointersForNull {
			return "*time.Time"
		}
		return "sql.NullTime"

	case "interval":
		if notNull {
			return "duckdb.Interval"
		}
		if emitPointersForNull {
			return "*duckdb.Interval"
		}
		return "sql.Null[duckdb.Interval]"

	case "uuid", "guid":
		if notNull {
			return "uuid.UUID"
		}
		if emitPointersForNull {
			return "*uuid.UUID"
		}
		return "uuid.NullUUID"

	case "json":
		// A nil message is NULL.
		return "json.RawMessage"

	case "struct", "row":
		if notNull {
			return "map[string]any"
		}
		if emitPointersForNull {
			return "*map[string]any"
		}
		return "sql.Null[map[string]any]"

	case "map":
		if notNull {
			return "duckdb.Map"
		}
		if emitPointersForNull {
			return "*duckdb.Map"
		}
		return "sql.Null[duckdb.Map]"

	case "enum":
		// An enum declared inline on the column, with no type name to
		// generate a Go type from.
		if notNull {
			return "string"
		}
		if emitPointersForNull {
			return "*string"
		}
		return "sql.NullString"

	case "any":
		return "any"

	default:
		rel, err := parseIdentifierString(columnType)
		if err != nil {
			return "any"
		}
		if rel.Schema == "" {
			rel.Schema = req.Catalog.DefaultSchema
		}
		for _, schema := range req.Catalog.Schemas {
			for _, enum := range schema.Enums {
				if rel.Name != enum.Name || rel.Schema != schema.Name {
					continue
				}
				name := enum.Name
				if schema.Name != req.Catalog.DefaultSchema {
					name = schema.Name + "_" + enum.Name
				}
				if notNull {
					return StructName(name, options)
				}
				if emitPointersForNullEnums {
					return "*" + StructName(name, options)
				}
				return "Null" + StructName(name, options)
			}
		}
	}

	if debug.Active {
		log.Printf("unknown DuckDB type: %s\n", columnType)
	}
	return "any"
}

// duckdbScanner names the helper in the duckdb file that scans a column
// into the Go type chosen for it, or "" when the driver's value lands in
// the type as it is. The helper follows the column's type, not the Go
// type, so an override still gets the driver's value converted.
func duckdbScanner(typ string, col *plugin.Column) string {
	if isSlice(typ) {
		if elem := strings.TrimPrefix(typ, "[]"); isSlice(elem) {
			// A LIST of LISTs. The helper needs the leaf type spelled
			// out: a generic function cannot take it from a nested slice.
			for isSlice(elem) {
				elem = strings.TrimPrefix(elem, "[]")
			}
			return "duckdbNested[" + elem + "]"
		}
		return "duckdbList"
	}
	if col == nil || col.Type == nil {
		return ""
	}
	switch strings.ToLower(col.Type.Name) {
	case "json":
		return "duckdbJSON"
	case "decimal", "dec", "numeric":
		return "duckdbDecimal"
	case "uuid", "guid":
		switch typ {
		case "uuid.UUID", "*uuid.UUID", "uuid.NullUUID", "[]byte":
			// These take the 16 raw bytes the driver hands over.
			return ""
		}
		return "duckdbUUID"
	}
	return ""
}
