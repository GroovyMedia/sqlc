package preprocess

import (
	"github.com/sqlc-dev/sqlc/internal/config"
)

// Style is the native placeholder syntax a dialect uses for bind parameters.
type Style int

const (
	// StyleDollar numbers parameters as $1, $2, ... (PostgreSQL)
	StyleDollar Style = iota
	// StyleQuestion uses an unnumbered ? for every parameter (MySQL)
	StyleQuestion
	// StyleOrdinal numbers parameters as ?1, ?2, ... (SQLite)
	StyleOrdinal
)

// Dialect describes the lexical rules the preprocessor needs to walk a query
// without parsing it: how the dialect quotes strings and identifiers, how it
// writes comments and which placeholder syntax it accepts.
//
// The preprocessor never interprets SQL. It only needs to know enough to skip
// the regions of a query where sqlc syntax must not be rewritten.
type Dialect struct {
	// Style is the native placeholder syntax used for rewritten parameters.
	Style Style

	// AtSign reports whether a bare "@name" is sqlc named-parameter syntax.
	// It is false for MySQL, where "@name" is a user variable.
	AtSign bool

	// DollarQuote reports whether $tag$ ... $tag$ string literals are valid.
	DollarQuote bool

	// DollarNumber reports whether $1 is a bind parameter.
	DollarNumber bool

	// Question reports whether ? is a bind parameter.
	Question bool

	// Backtick reports whether `ident` is a quoted identifier.
	Backtick bool

	// HashComment reports whether # starts a line comment.
	HashComment bool

	// DoubleQuoteString reports whether "..." is a string literal rather than
	// a quoted identifier. MySQL treats it as a string unless ANSI_QUOTES is
	// enabled.
	DoubleQuoteString bool

	// NestedBlockComment reports whether /* ... /* ... */ ... */ nests.
	NestedBlockComment bool

	// Backslash reports whether a backslash escapes the next character inside
	// a string literal.
	Backslash bool

	// FoldIdentifier reports whether the dialect lowercases unquoted
	// identifiers. It decides the case of a parameter named by a bare
	// reference, e.g. sqlc.arg(FooBar).
	FoldIdentifier bool

	// DollarName reports whether $name is a named bind parameter. It is
	// rewritten like @name: to a numbered placeholder that the parameter
	// set names, so a name used twice binds one value.
	DollarName bool

	// NoSlice reports that sqlc.slice() is refused with an error. A dialect
	// that binds a Go slice as a single list value has nothing to expand,
	// so the list is passed with sqlc.arg() instead.
	NoSlice bool
}

var dialects = map[config.Engine]Dialect{
	config.EnginePostgreSQL: {
		Style:              StyleDollar,
		AtSign:             true,
		DollarQuote:        true,
		DollarNumber:       true,
		NestedBlockComment: true,
		FoldIdentifier:     true,
	},
	config.EngineMySQL: {
		Style:             StyleQuestion,
		Question:          true,
		Backtick:          true,
		HashComment:       true,
		DoubleQuoteString: true,
		Backslash:         true,
		FoldIdentifier:    true,
	},
	config.EngineSQLite: {
		Style:     StyleOrdinal,
		AtSign:    true,
		Question:  true,
		Backtick:  true,
		Backslash: true,
	},
	// ClickHouse binds with an unnumbered ?, like MySQL, and its identifiers
	// keep their case.
	config.EngineClickHouse: {
		Style:     StyleQuestion,
		Question:  true,
		Backtick:  true,
		Backslash: true,
	},
	// DuckDB lexes like PostgreSQL and binds with $1, ? and $name. It does
	// not mix $name with numbered placeholders, so $name is rewritten to a
	// number as @name is and every placeholder ends up numbered. A Go
	// slice binds as one LIST value, so sqlc.slice() is refused: pass the
	// list with sqlc.arg() and compare with "= ANY(sqlc.arg(name))".
	config.EngineDuckDB: {
		Style:              StyleDollar,
		AtSign:             true,
		DollarQuote:        true,
		DollarNumber:       true,
		DollarName:         true,
		Question:           true,
		NestedBlockComment: true,
		FoldIdentifier:     true,
		NoSlice:            true,
	},
}

// DialectFor returns the lexical rules for an engine, and whether the engine is
// preprocessed at all. GoogleSQL is not: it handles its own parameter syntax,
// so its queries reach the parser unchanged.
func DialectFor(engine config.Engine) (Dialect, bool) {
	d, ok := dialects[engine]
	return d, ok
}

// hasNamedSupport reports whether repeated uses of the same parameter name can
// share a single placeholder. MySQL sends an argument per "?", so every
// occurrence needs its own number.
func (d Dialect) hasNamedSupport() bool {
	return d.Style != StyleQuestion
}
