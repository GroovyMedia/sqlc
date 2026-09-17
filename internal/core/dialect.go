package core

import (
	"context"
	"fmt"
	"slices"
	"strings"

	"github.com/sqlc-dev/sqlc/internal/core/catalogdb"
)

func (c *Catalog) CreateDialect(name string) (int64, error) {
	oid, err := c.q.CreateDialect(context.Background(), name)
	if err != nil {
		return 0, fmt.Errorf("create dialect %q: %w", name, err)
	}
	c.dialectOID = oid
	return oid, nil
}

func (c *Catalog) DialectOID(name string) (int64, error) {
	oid, err := c.q.DialectOID(context.Background(), name)
	if err != nil {
		return 0, fmt.Errorf("dialect %q: %w", name, err)
	}
	return oid, nil
}

func (c *Catalog) SetDialectFlag(dialectOID int64, key, value string) error {
	err := c.q.SetDialectFlag(context.Background(), catalogdb.SetDialectFlagParams{
		DialectOid: dialectOID,
		Key:        key,
		Value:      value,
	})
	if err != nil {
		return fmt.Errorf("set dialect flag %s.%s: %w", key, value, err)
	}
	return nil
}

func (c *Catalog) DialectFlag(dialectOID int64, key string) (string, error) {
	value, err := c.q.DialectFlag(context.Background(), catalogdb.DialectFlagParams{
		DialectOid: dialectOID,
		Key:        key,
	})
	if err != nil {
		return "", nil
	}
	return value, nil
}

// FlagComparisonOperators holds the dialect's comparison operators as a
// comma-separated list, so that a type declared by a schema can be given the
// same ones the seeded types have.
const FlagComparisonOperators = "operators.comparison"

// FlagBoolType holds the type a comparison or predicate yields, when it is
// not the type a boolean literal has: ClickHouse compares to UInt8 while
// writing true as Bool.
const FlagBoolType = "types.bool"

// FlagLimitType holds the type a LIMIT or OFFSET count has, which is what a
// placeholder in one is typed as.
const FlagLimitType = "types.limit"

// FlagUntypedType holds the type a placeholder takes when nothing in the
// query constrains it, for a dialect that gives such a placeholder one.
const FlagUntypedType = "types.untyped"

// FlagPropagateNullable is set when a function's result is nullable whenever
// one of its arguments is, the way ClickHouse's ordinary functions behave.
const FlagPropagateNullable = "functions.propagate_nullable"

// FlagValueFunctions holds the functions a dialect calls by a bare name,
// without parentheses, as CURRENT_DATE is: each name and the function it
// calls, as name:function pairs separated by commas.
const FlagValueFunctions = "functions.values"

// FlagStrict is set for a dialect whose analysis reports what it cannot
// type rather than leaving it untyped: a result column or a placeholder
// with no type, and a placeholder two uses type differently, fail the
// query. DuckDB gives an unconstrained placeholder no type at all, so a
// query it prepares has every type settled.
const FlagStrict = "analysis.strict"

// FlagQualifyDuplicateColumns is set for a dialect that names a result
// column after its relation when an earlier result column from another
// relation has the same name, as ClickHouse names the second id of a join
// e.id.
const FlagQualifyDuplicateColumns = "columns.qualify_duplicates"

// FlagOuterJoinDefaults is set for a dialect whose outer joins fill the
// columns of the side a row has no match on with the column type's default
// value rather than NULL, as ClickHouse does unless join_use_nulls is set.
const FlagOuterJoinDefaults = "joins.outer_defaults"

// FlagDefaultSchema holds the schema the dialect puts an unqualified object
// in, when that is not the catalog's own default: a type in it is reported
// without its schema.
const FlagDefaultSchema = "schema.default"

// FlagCastCategories holds the categories whose types are all implicitly
// castable to one another, as the dialect's seed declared them, so that a type
// arriving after the seed — an extension's, say — can join its category.
const FlagCastCategories = "casts.categories"

// Constant kinds. A literal in a query has no declared type, so each dialect
// names the type its literals take on.
const (
	ConstInteger = "integer"
	ConstFloat   = "float"
	ConstString  = "string"
	ConstBool    = "bool"
)

// defaultConstTypes are the PostgreSQL type names, used by any dialect that
// does not name its own.
var defaultConstTypes = map[string]string{
	ConstInteger: "int4",
	ConstFloat:   "numeric",
	ConstString:  "text",
	ConstBool:    "bool",
}

// IsComparisonOperator reports whether the dialect counts name among the
// operators that compare two values and yield a boolean.
func (c *Catalog) IsComparisonOperator(name string) bool {
	if c.dialectOID == 0 {
		return false
	}
	ops, _ := c.DialectFlag(c.dialectOID, FlagComparisonOperators)
	if ops == "" {
		return false
	}
	return slices.Contains(strings.Split(ops, ","), name)
}

// SetConstType records the type a literal of the given kind takes on in this
// dialect.
func (c *Catalog) SetConstType(dialectOID int64, kind, typeName string) error {
	return c.SetDialectFlag(dialectOID, "const."+kind, typeName)
}

// ConstTypeOID returns the type a literal of the given kind takes on.
func (c *Catalog) ConstTypeOID(kind string) (int64, error) {
	if c.dialectOID != 0 {
		if name, _ := c.DialectFlag(c.dialectOID, "const."+kind); name != "" {
			return c.TypeOID(name)
		}
	}
	name, ok := defaultConstTypes[kind]
	if !ok {
		return 0, fmt.Errorf("unknown constant kind %q", kind)
	}
	return c.TypeOID(name)
}

// BoolTypeOID returns the type a comparison or predicate yields.
func (c *Catalog) BoolTypeOID() (int64, error) {
	if c.dialectOID != 0 {
		if name, _ := c.DialectFlag(c.dialectOID, FlagBoolType); name != "" {
			return c.TypeOID(name)
		}
	}
	return c.ConstTypeOID(ConstBool)
}

// LimitTypeOID returns the type a LIMIT or OFFSET count has, falling back to
// the type of an integer literal.
func (c *Catalog) LimitTypeOID() (int64, error) {
	if c.dialectOID != 0 {
		if name, _ := c.DialectFlag(c.dialectOID, FlagLimitType); name != "" {
			return c.TypeOID(name)
		}
	}
	return c.ConstTypeOID(ConstInteger)
}

// UntypedTypeOID returns the type an unconstrained placeholder takes, and
// whether the dialect gives it one at all.
func (c *Catalog) UntypedTypeOID() (int64, bool) {
	if c.dialectOID == 0 {
		return 0, false
	}
	name, _ := c.DialectFlag(c.dialectOID, FlagUntypedType)
	if name == "" {
		return 0, false
	}
	oid, err := c.TypeOID(name)
	if err != nil {
		return 0, false
	}
	return oid, true
}

// PropagatesNullable reports whether a function's result is nullable
// whenever one of its arguments is.
func (c *Catalog) PropagatesNullable() bool {
	if c.dialectOID == 0 {
		return false
	}
	v, _ := c.DialectFlag(c.dialectOID, FlagPropagateNullable)
	return v == "true"
}

// Strict reports whether the dialect fails a query whose result column or
// placeholder has no type.
func (c *Catalog) Strict() bool {
	if c.dialectOID == 0 {
		return false
	}
	v, _ := c.DialectFlag(c.dialectOID, FlagStrict)
	return v == "true"
}

// QualifiesDuplicateColumns reports whether a result column that repeats an
// earlier one's name from another relation is named after its relation.
func (c *Catalog) QualifiesDuplicateColumns() bool {
	if c.dialectOID == 0 {
		return false
	}
	v, _ := c.DialectFlag(c.dialectOID, FlagQualifyDuplicateColumns)
	return v == "true"
}

// OuterJoinDefaults reports whether an outer join fills the columns of the
// side a row has no match on with default values rather than NULL, so that
// the join leaves them as nullable as they declare.
func (c *Catalog) OuterJoinDefaults() bool {
	if c.dialectOID == 0 {
		return false
	}
	v, _ := c.DialectFlag(c.dialectOID, FlagOuterJoinDefaults)
	return v == "true"
}

// ValueFunction is the function a bare name calls in this dialect, when it
// names no column: get_current_timestamp for DuckDB's CURRENT_TIMESTAMP.
func (c *Catalog) ValueFunction(name string) (string, bool) {
	if c.dialectOID == 0 {
		return "", false
	}
	pairs, _ := c.DialectFlag(c.dialectOID, FlagValueFunctions)
	for _, pair := range strings.Split(pairs, ",") {
		if bare, fn, ok := strings.Cut(pair, ":"); ok && bare == strings.ToLower(name) {
			return fn, true
		}
	}
	return "", false
}

// DefaultNamespaces lists the namespaces a type is reported from without
// qualification: the catalog's default, PostgreSQL's system catalog, and
// the dialect's own default schema when it names one.
func (c *Catalog) DefaultNamespaces() []string {
	out := []string{"public", "pg_catalog"}
	if c.dialectOID != 0 {
		if name, _ := c.DialectFlag(c.dialectOID, FlagDefaultSchema); name != "" {
			out = append(out, name)
		}
	}
	return out
}
