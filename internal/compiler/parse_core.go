package compiler

import (
	"errors"
	"fmt"
	"maps"
	"slices"
	"strings"

	"github.com/sqlc-dev/sqlc/internal/config"
	"github.com/sqlc-dev/sqlc/internal/core"
	coreanalyzer "github.com/sqlc-dev/sqlc/internal/core/analyzer"
	"github.com/sqlc-dev/sqlc/internal/metadata"
	"github.com/sqlc-dev/sqlc/internal/source"
	"github.com/sqlc-dev/sqlc/internal/sql/ast"
	"github.com/sqlc-dev/sqlc/internal/sql/named"
	"github.com/sqlc-dev/sqlc/internal/sql/preprocess"
	"github.com/sqlc-dev/sqlc/internal/sql/sqlerr"
	"github.com/sqlc-dev/sqlc/internal/sql/validate"
)

func (c *Compiler) parseQueryCore(raw *ast.RawStmt, src string, pre *preprocess.Statement) (*Query, error) {
	rawSQL, err := source.Pluck(src, raw.StmtLocation, raw.StmtLen)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(rawSQL) == "" {
		return nil, errors.New("missing semicolon at end of file")
	}

	name, cmd, err := metadata.ParseQueryNameAndType(rawSQL, metadata.CommentSyntax(c.parser.CommentSyntax()))
	if err != nil {
		return nil, err
	}
	// From here on an error names the query it is about, so that it can be
	// told apart in a file of many. A statement with no name is skipped,
	// but not one that misuses sqlc syntax.
	if pre.Err != nil {
		return nil, queryError(name, pre.Err)
	}
	if name == "" {
		return nil, nil
	}
	// A statement sqlc has no node for converts to a TODO. Left alone in
	// a schema it is harmless, but a named query has to come out the
	// other end, so one that cannot is an error rather than a silence.
	if todo, ok := raw.Stmt.(*ast.TODO); ok {
		return nil, &sqlerr.Error{
			Message:  fmt.Sprintf("%s: unsupported statement", name),
			Location: todo.Location,
		}
	}
	if err := validate.Cmd(raw.Stmt, name, cmd); err != nil {
		return nil, err
	}

	md := metadata.Metadata{Name: name, Cmd: cmd}
	cleanedComments, err := source.CleanedComments(rawSQL, c.parser.CommentSyntax())
	if err != nil {
		return nil, err
	}
	md.Params, md.Flags, md.RuleSkiplist, err = metadata.ParseCommentFlags(cleanedComments)
	if err != nil {
		return nil, err
	}

	if pre.ParamErr != nil {
		return nil, queryError(name, pre.ParamErr)
	}
	namedParams := pre.Params
	expanded := rawSQL

	var cols []*Column
	var params []Parameter
	switch raw.Stmt.(type) {
	case *ast.SelectStmt, *ast.InsertStmt, *ast.UpdateStmt, *ast.DeleteStmt, *ast.MergeStmt:
		res, err := coreanalyzer.PrepareWith(c.coreCatalog, raw, coreanalyzer.Options{NullableParams: namedParams.Nullable()})
		if err != nil {
			return nil, queryError(name, err)
		}
		for _, col := range res.Columns {
			cols = append(cols, coreColumn(col))
		}
		cols, err = c.embedCore(raw, res, pre.Embeds, cols)
		if err != nil {
			return nil, queryError(name, err)
		}
		for _, p := range res.Parameters {
			params = append(params, Parameter{Number: p.Number, Column: coreParamColumn(p, namedParams)})
		}
		if err := unanalyzedParam(name, pre, res.Parameters); err != nil {
			return nil, err
		}
		expanded, err = source.Mutate(rawSQL, c.expandCore(raw, res.Stars))
		if err != nil {
			return nil, err
		}
	}

	var batch *BatchPlan
	if c.conf.Engine == config.EngineDuckDB && strings.HasPrefix(cmd, ":batch") {
		write, plan, err := planDuckDBBatch(raw, rawSQL, params)
		if err != nil {
			return nil, queryError(name, err)
		}
		batch = plan
		if plan.Mode != BatchLoop {
			expanded = write
			for _, extra := range []string{plan.Rounds, plan.Read} {
				if extra == "" {
					continue
				}
				if _, err := c.newParser().Parse(strings.NewReader(extra + ";")); err != nil {
					return nil, queryError(name, fmt.Errorf("batch rewrite: %w", err))
				}
			}
		}
	}

	// If the query string was edited, make sure the syntax is valid
	if expanded != rawSQL {
		if _, err := c.newParser().Parse(strings.NewReader(expanded)); err != nil {
			return nil, fmt.Errorf("edited query syntax is invalid: %w", err)
		}
	}

	trimmed, comments, err := source.StripComments(expanded)
	if err != nil {
		return nil, err
	}
	md.Comments = comments

	var insertTable *ast.TableName
	if ins, ok := raw.Stmt.(*ast.InsertStmt); ok {
		insertTable, _ = ParseTableName(ins.Relation)
	}

	return &Query{
		RawStmt:         raw,
		Metadata:        md,
		Params:          params,
		Columns:         cols,
		SQL:             trimmed,
		InsertIntoTable: insertTable,
		Batch:           batch,
	}, nil
}

// queryError prefixes an error with the name of the query it is about. The
// error is wrapped, so its position survives.
func queryError(name string, err error) error {
	if name == "" {
		return err
	}
	return fmt.Errorf("%s: %w", name, err)
}

// unanalyzedParam reports the first placeholder, in source order, that the
// analyzer did not see. The engine converts syntax it has no node for into
// a TODO, and a placeholder inside one is invisible to the analyzer: the
// query text still holds it, but the generated code would not bind it and
// every call would fail with too few arguments.
func unanalyzedParam(name string, pre *preprocess.Statement, params []core.Parameter) error {
	seen := make(map[int]bool, len(params))
	for _, p := range params {
		seen[p.Number] = true
	}
	for _, offset := range slices.Sorted(maps.Keys(pre.Numbers)) {
		number := pre.Numbers[offset]
		if seen[number] {
			continue
		}
		ref := fmt.Sprintf("$%d", number)
		if pname, ok := pre.Params.NameFor(number); ok && pname != "" {
			ref = "@" + pname
		}
		return queryError(name, &sqlerr.Error{
			Message:  fmt.Sprintf("parameter %s is inside an expression sqlc cannot analyze", ref),
			Location: offset,
		})
	}
	return nil
}

func coreColumn(c core.Column) *Column {
	col := &Column{
		Name:     c.Name,
		DataType: c.DataType,
		NotNull:  c.NotNull,
		IsArray:  c.IsArray,
		TypeExpr: c.Type,
	}
	describeType(col, c.Type)
	if c.Source != nil && c.Source.Table != "" {
		col.Table = &ast.TableName{Schema: c.Source.Schema, Name: c.Source.Table}
		col.TableAlias = c.Source.TableAlias
		col.OriginalName = c.Source.Column
	}
	return col
}

// describeType fills in what codegen reads about a type from its
// expression: one array dimension per nesting, the length that is the
// innermost type's first integer argument (which is how a MySQL tinyint(1)
// is told from a tinyint), and whether the innermost type is unsigned.
func describeType(col *Column, t *core.TypeExpr) {
	if t == nil {
		if col.IsArray {
			col.ArrayDims = 1
		}
		return
	}
	col.ArrayDims = t.ArrayDims()
	inner := t.Innermost()
	if len(inner.Args) > 0 && inner.Args[0].Int != nil {
		l := int(*inner.Args[0].Int)
		col.Length = &l
	}
	col.Unsigned = strings.HasSuffix(inner.Name, " unsigned")
}

func coreParamColumn(p core.Parameter, params *named.ParamSet) *Column {
	col := &Column{
		Name:     p.Name,
		DataType: p.DataType,
		NotNull:  p.NotNull,
		IsArray:  p.IsArray,
		TypeExpr: p.Type,
	}
	describeType(col, p.Type)
	if p.Source != nil && p.Source.Table != "" {
		col.Table = &ast.TableName{Schema: p.Source.Schema, Name: p.Source.Table}
		col.OriginalName = p.Source.Column
	}
	if col.Name == "" && p.Source != nil {
		col.Name = p.Source.Column
	}
	// Merge in what the user asked for: sqlc.narg() makes the parameter
	// nullable and sqlc.slice() marks it as a slice, whichever way the
	// analyzer typed it.
	if param, isNamed := params.FetchMerge(p.Number, named.NewInferredParam(col.Name, p.NotNull)); isNamed {
		col.Name = param.Name()
		col.NotNull = param.NotNull()
		col.TypeExpr = col.TypeExpr.WithNullable(!col.NotNull)
		col.IsSqlcSlice = param.IsSqlcSlice()
		col.IsNamedParam = true
	}
	return col
}
