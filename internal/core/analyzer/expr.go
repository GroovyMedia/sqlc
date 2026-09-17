package analyzer

import (
	"fmt"
	"slices"
	"strings"

	"github.com/sqlc-dev/sqlc/internal/core"
	"github.com/sqlc-dev/sqlc/internal/sql/ast"
)

type exprType struct {
	// typeOID is the type's row: the instance when the catalog holds one,
	// else the family, else 0 for a type no dialect seeded and no schema
	// declared.
	typeOID int64
	// expr is the whole expression when it says more than the row does — a
	// cast to numeric(5, 1) when only numeric is a row — or names a type
	// the catalog does not hold. Analysis never adds a type: it reports the
	// expression the query used and carries on, so a query can be analyzed
	// against a catalog it cannot write to.
	expr               *core.TypeExpr
	nullable           bool
	sourceClassOID     int64
	sourceAttributeOID int64
	sourceTableAlias   string
	// columnNotNull is what a referenced column declares, kept apart from
	// nullable because a column read from the outer side of an outer join
	// is nullable whatever it declares.
	columnNotNull bool
	// untyped says why an expression has no type when the expression
	// itself is the reason: syntax sqlc has no node for, a function the
	// dialect does not list. A strict dialect reports it.
	untyped string
}

func (a *analyzer) typeExpr(n ast.Node) (exprType, error) {
	switch e := n.(type) {
	case nil:
		return exprType{}, nil

	case *ast.TODO:
		return exprType{untyped: unsupported(e)}, nil

	case *ast.LambdaExpr:
		return a.typeLambda(e)

	case *ast.A_Const:
		return a.typeConst(e)

	case *ast.ColumnRef:
		return a.typeColumnRef(e)

	case *ast.ParamRef:
		return a.typeParamRef(e)

	case *ast.A_Expr:
		return a.typeAExpr(e)

	case *ast.BoolExpr:
		return a.typeBoolExpr(e)

	case *ast.FuncCall:
		return a.typeFuncCall(e)

	case *ast.TypeCast:
		return a.typeTypeCast(e)

	case *ast.NullTest:
		if _, err := a.typeExpr(e.Arg); err != nil {
			return exprType{}, err
		}
		return a.boolType(false)

	case *ast.In:
		return a.typeIn(e)

	case *ast.BetweenExpr:
		return a.typeBetween(e)

	case *ast.CaseExpr:
		return a.typeCase(e)

	case *ast.CoalesceExpr:
		return a.typeCoalesce(e)

	case *ast.MinMaxExpr:
		return a.typeFirstOf(listItems(e.Args), false)

	case *ast.A_ArrayExpr:
		return a.typeArrayExpr(e)

	case *ast.SubLink:
		return a.typeSubLink(e)

	case *ast.CollateExpr:
		return a.typeExpr(e.Arg)

	case *ast.IntervalExpr:
		// A dialect with no interval type of its own leaves it untyped.
		oid, err := a.cat.TypeOID("interval")
		if err != nil {
			return exprType{}, nil
		}
		return exprType{typeOID: oid}, nil

	// A user variable and an ordinal in GROUP BY / ORDER BY carry no type of
	// their own, and neither is something to resolve.
	case *ast.VariableExpr, *ast.Integer, *ast.String, *ast.Float:
		return exprType{}, nil

	case *ast.List:
		for _, item := range listItems(e) {
			if _, err := a.typeExpr(item); err != nil {
				return exprType{}, err
			}
		}
		return exprType{}, nil
	}
	return exprType{}, fmt.Errorf("typeExpr: unsupported %T", n)
}

func (a *analyzer) typeConst(c *ast.A_Const) (exprType, error) {
	switch v := c.Val.(type) {
	case *ast.Integer:
		_ = v
		oid, err := a.cat.ConstTypeOID(core.ConstInteger)
		if err != nil {
			return exprType{}, err
		}
		return exprType{typeOID: oid}, nil
	case *ast.Float:
		oid, err := a.cat.ConstTypeOID(core.ConstFloat)
		if err != nil {
			return exprType{}, err
		}
		return exprType{typeOID: oid}, nil
	case *ast.String:
		oid, err := a.cat.ConstTypeOID(core.ConstString)
		if err != nil {
			return exprType{}, err
		}
		return exprType{typeOID: oid}, nil
	case *ast.Boolean:
		return a.boolType(false)
	case *ast.Null, nil:
		return exprType{nullable: true, untyped: "NULL has no type"}, nil
	}
	return exprType{}, fmt.Errorf("typeConst: unsupported %T", c.Val)
}

func (a *analyzer) boolType(nullable bool) (exprType, error) {
	oid, err := a.cat.BoolTypeOID()
	if err != nil {
		return exprType{}, err
	}
	return exprType{typeOID: oid, nullable: nullable}, nil
}

func (a *analyzer) typeColumnRef(c *ast.ColumnRef) (exprType, error) {
	parts := flattenFields(c.Fields)
	if len(parts) == 0 {
		return exprType{}, fmt.Errorf("column ref: empty")
	}
	relation := ""
	column := parts[0]
	if len(parts) >= 2 {
		relation = parts[0]
		column = parts[1]
	}
	// A lambda's parameter hides a column of its name. What it holds is
	// the function's to say, which the seed does not.
	if relation == "" && a.lambdas[column] > 0 {
		return exprType{untyped: fmt.Sprintf("lambda parameter %q has no type", column)}, nil
	}
	rel, col, ok, err := a.resolveColumn(relation, column)
	if err != nil {
		return exprType{}, err
	}
	// A dotted name may be a column's own, as ClickHouse names the columns
	// a Nested column stores as n.a.
	if !ok && relation != "" {
		rel, col, ok, err = a.resolveColumn("", relation+"."+column)
		if err != nil {
			return exprType{}, err
		}
	}
	// Or reach into a struct column: point.a is field a of column point.
	if !ok && relation != "" {
		if t, ok, err := a.typeStructField(parts); err != nil || ok {
			return t, err
		}
	}
	if !ok {
		if relation != "" {
			return exprType{}, fmt.Errorf("unknown column %q.%q", relation, column)
		}
		// The name may be one the target list assigned rather than one a
		// relation offers.
		if t, ok, err := a.typeAlias(column); err != nil || ok {
			return t, err
		}
		// Or one that calls a function the dialect spells without
		// parentheses, as CURRENT_DATE does.
		if fn, ok := a.cat.ValueFunction(column); ok {
			return a.typeFuncCall(&ast.FuncCall{Funcname: &ast.List{Items: []ast.Node{&ast.String{Str: fn}}}})
		}
		return exprType{}, fmt.Errorf("unknown column %q", column)
	}
	if len(parts) > 2 {
		// What follows a qualified column, as in t.point.a, names fields
		// of its struct.
		if t, ok := a.typeFieldPath(columnType(rel, col), parts[2:]); ok {
			return t, nil
		}
		return exprType{}, fmt.Errorf("unknown column %q", strings.Join(parts, "."))
	}
	return columnType(rel, col), nil
}

// typeStructField types a dotted name whose head is a column of a struct
// type and whose tail names a field of it, at any depth.
func (a *analyzer) typeStructField(parts []string) (exprType, bool, error) {
	rel, col, ok, err := a.resolveColumn("", parts[0])
	if err != nil || !ok {
		return exprType{}, false, err
	}
	t, ok := a.typeFieldPath(columnType(rel, col), parts[1:])
	return t, ok, nil
}

// typeFieldPath types the field a path of names reaches in a struct. The
// field may be NULL whatever the struct is.
func (a *analyzer) typeFieldPath(t exprType, fields []string) (exprType, bool) {
	e := a.exprOf(t)
	for _, field := range fields {
		if e = structField(e, field); e == nil {
			return exprType{}, false
		}
	}
	out := a.lookupType(e)
	out.nullable = true
	return out, true
}

// structField is the type of a struct's named field, or nil when the type
// is not a struct that names it.
func structField(t *core.TypeExpr, name string) *core.TypeExpr {
	if t == nil {
		return nil
	}
	for _, arg := range t.Args {
		if arg.Type != nil && arg.Label != "" && strings.EqualFold(arg.Label, name) {
			return arg.Type
		}
	}
	return nil
}

func flattenFields(fields *ast.List) []string {
	if fields == nil {
		return nil
	}
	out := make([]string, 0, len(fields.Items))
	for _, item := range fields.Items {
		switch v := item.(type) {
		case *ast.String:
			out = append(out, v.Str)
		case *ast.A_Star:
			out = append(out, "*")
			return out
		}
	}
	return out
}

func (a *analyzer) typeParamRef(p *ast.ParamRef) (exprType, error) {
	a.locate(p)
	cur, ok := a.params[p.Number]
	if !ok {
		cur = core.Parameter{Number: p.Number, Name: p.Name}
		a.params[p.Number] = cur
	}
	return exprType{typeOID: cur.TypeOID, expr: cur.Type.WithNullable(false), nullable: !cur.NotNull}, nil
}

func (a *analyzer) inferParam(number int, t exprType) {
	cur, ok := a.params[number]
	if !ok {
		cur = core.Parameter{Number: number}
	}
	typed := cur.TypeOID == 0 && cur.Type == nil && (t.typeOID != 0 || t.expr != nil)
	if !typed {
		a.noteConflict(number, cur, t)
	}
	if typed {
		// A placeholder compared with a column holds a value for that
		// column, so it is nullable when the column is, not when the join
		// is: an outer join makes the column NULL only for the rows the
		// comparison cannot match.
		if t.sourceAttributeOID != 0 {
			t.nullable = !t.columnNotNull
		}
		cur.TypeOID = t.typeOID
		cur.DataType, cur.IsArray = a.typeNameOf(t)
		cur.NotNull = !t.nullable
		cur.Type = a.typeExprOf(t)
	}
	if cur.Source == nil && t.sourceAttributeOID != 0 {
		ad, err := a.cat.LookupAttribute(t.sourceAttributeOID)
		if err == nil {
			cur.Source = &core.ColumnSource{
				Schema:     ad.Schema,
				Table:      ad.Table,
				TableAlias: t.sourceTableAlias,
				Column:     ad.Column,
			}
		}
	}
	a.params[number] = cur
}

// nameParamAfter names a placeholder compared with a function call after
// the function, the way a placeholder compared with a column is named after
// the column.
func (a *analyzer) nameParamAfter(number int, other ast.Node) {
	fc, ok := other.(*ast.FuncCall)
	if !ok {
		return
	}
	cur := a.params[number]
	if cur.Name == "" && cur.Source == nil {
		cur.Name = funcCallName(fc)
		a.params[number] = cur
	}
}

func (a *analyzer) typeAExpr(e *ast.A_Expr) (exprType, error) {
	// Not every engine classifies its operators, so a zero kind is a plain
	// operator application rather than an unset field. LIKE and its relatives
	// name a real operator too, so they resolve the same way.
	switch e.Kind {
	case ast.A_Expr_Kind_OP, ast.A_Expr_Kind_LIKE, ast.A_Expr_Kind_ILIKE, ast.A_Expr_Kind_SIMILAR, 0:
	case ast.A_Expr_Kind_OP_ANY, ast.A_Expr_Kind_OP_ALL:
		return a.typeQuantifiedExpr(e)
	case ast.A_Expr_Kind_IN, ast.A_Expr_Kind_BETWEEN, ast.A_Expr_Kind_NOT_BETWEEN,
		ast.A_Expr_Kind_BETWEEN_SYM, ast.A_Expr_Kind_NOT_BETWEEN_SYM:
		return a.typePredicateList(e)
	case ast.A_Expr_Kind_DISTINCT, ast.A_Expr_Kind_NOT_DISTINCT:
		return a.typeComparison(e)
	case ast.A_Expr_Kind_NULLIF:
		return a.typeNullIf(e)
	default:
		return exprType{}, fmt.Errorf("a_expr: unsupported kind %v", e.Kind)
	}
	opName := normalizeOperator(opNameFromList(e.Name))
	if opName == "" {
		return exprType{}, fmt.Errorf("a_expr: unnamed operator")
	}

	// Engines that do not have a dedicated boolean node report AND and OR as
	// operators. They combine predicates whatever the dialect.
	if opName == "AND" || opName == "OR" {
		if _, err := a.typeExpr(e.Lexpr); err != nil {
			return exprType{}, err
		}
		if _, err := a.typeExpr(e.Rexpr); err != nil {
			return exprType{}, err
		}
		return a.boolType(false)
	}

	leftT, err := a.typeExpr(e.Lexpr)
	if err != nil {
		return exprType{}, err
	}
	// The postfix null tests SQLite has — x ISNULL, x NOTNULL, x NOT NULL —
	// arrive as operators with no right operand, and are predicates that
	// are never NULL.
	if e.Rexpr == nil && isNullTest(opName) {
		return a.boolType(false)
	}
	rightT, err := a.typeExpr(e.Rexpr)
	if err != nil {
		return exprType{}, err
	}

	if pr, ok := e.Lexpr.(*ast.ParamRef); ok && rightT.typeOID != 0 {
		a.inferParam(pr.Number, rightT)
		a.nameParamAfter(pr.Number, e.Rexpr)
		leftT = rightT
	} else if pr := castParamRef(e.Lexpr); pr != nil {
		a.inferParam(pr.Number, rightT)
		a.nameParamAfter(pr.Number, e.Rexpr)
	}
	if pr, ok := e.Rexpr.(*ast.ParamRef); ok && leftT.typeOID != 0 {
		a.inferParam(pr.Number, leftT)
		a.nameParamAfter(pr.Number, e.Lexpr)
		rightT = leftT
	} else if pr := castParamRef(e.Rexpr); pr != nil {
		a.inferParam(pr.Number, leftT)
		a.nameParamAfter(pr.Number, e.Lexpr)
	}
	a.noteCause(e.Lexpr, rightT)
	a.noteCause(e.Rexpr, leftT)

	overload, err := a.resolveOperator(opName, leftT, rightT)
	if err != nil {
		return exprType{}, err
	}
	// An operator's result is NULL when an operand is, except for IS and
	// IS NOT, which test for NULL rather than propagate it: x IS NULL and
	// x IS y are never NULL, whatever x and y are.
	return exprType{
		typeOID:  overload.ResultTypeOID,
		nullable: (leftT.nullable || rightT.nullable) && !isNullTest(opName),
	}, nil
}

// castParamRef is the placeholder a cast wraps, as ClickHouse's {p:UInt64}
// is written, or nil. The cast has typed it already; what it is compared
// with still names it and says which column it stands in for.
func castParamRef(n ast.Node) *ast.ParamRef {
	tc, ok := n.(*ast.TypeCast)
	if !ok {
		return nil
	}
	pr, _ := tc.Arg.(*ast.ParamRef)
	return pr
}

// isNullTest reports whether an operator compares with NULL as a value
// rather than propagating it, so that its result is never NULL.
func isNullTest(opName string) bool {
	switch opName {
	case "IS", "IS NOT", "ISNULL", "NOTNULL", "NOT NULL":
		return true
	}
	return false
}

// typeQuantifiedExpr types "x = ANY($1)" and "x > ALL(...)": the right side
// holds values of the left side's type, and the result is a predicate.
func (a *analyzer) typeQuantifiedExpr(e *ast.A_Expr) (exprType, error) {
	leftT, err := a.typeExpr(e.Lexpr)
	if err != nil {
		return exprType{}, err
	}
	if err := a.typeOperands(e.Rexpr, leftT); err != nil {
		return exprType{}, err
	}
	return a.boolType(false)
}

// typePredicateList types IN and BETWEEN, where the right side is a list of
// values compared against the left.
func (a *analyzer) typePredicateList(e *ast.A_Expr) (exprType, error) {
	leftT, err := a.typeExpr(e.Lexpr)
	if err != nil {
		return exprType{}, err
	}
	if l, ok := e.Rexpr.(*ast.List); ok {
		for _, item := range listItems(l) {
			if err := a.typeOperands(item, leftT); err != nil {
				return exprType{}, err
			}
		}
		return a.boolType(false)
	}
	if err := a.typeOperands(e.Rexpr, leftT); err != nil {
		return exprType{}, err
	}
	return a.boolType(false)
}

// typeIn types the IN node the engines that have one report, where the values
// compared against are held apart from the expression.
func (a *analyzer) typeIn(e *ast.In) (exprType, error) {
	leftT, err := a.typeExpr(e.Expr)
	if err != nil {
		return exprType{}, err
	}
	for _, item := range e.List {
		if err := a.typeOperands(item, leftT); err != nil {
			return exprType{}, err
		}
	}
	// "x IN (SELECT ...)" compares x against the subquery's column, and the
	// subquery's own placeholders are reported with the rest.
	if sel, ok := e.Sel.(*ast.SelectStmt); ok {
		a.inferUnnestParam(sel, leftT)
		cols, err := a.subqueryColumns(sel)
		if err != nil {
			return exprType{}, err
		}
		if pr, ok := e.Expr.(*ast.ParamRef); ok && len(cols) > 0 {
			if err := a.typeOperands(pr, columnExprType(cols[0])); err != nil {
				return exprType{}, err
			}
		}
	}
	return a.boolType(false)
}

// typeBetween types the BETWEEN node the engines that have one report.
func (a *analyzer) typeBetween(e *ast.BetweenExpr) (exprType, error) {
	leftT, err := a.typeExpr(e.Expr)
	if err != nil {
		return exprType{}, err
	}
	for _, bound := range []ast.Node{e.Left, e.Right} {
		if err := a.typeOperands(bound, leftT); err != nil {
			return exprType{}, err
		}
	}
	return a.boolType(false)
}

// typeCase types CASE. Its result is the first branch's, and it is nullable
// unless every branch and the default are.
func (a *analyzer) typeCase(e *ast.CaseExpr) (exprType, error) {
	argT, err := a.typeExpr(e.Arg)
	if err != nil {
		return exprType{}, err
	}
	results := make([]ast.Node, 0, len(listItems(e.Args))+1)
	for _, item := range listItems(e.Args) {
		when, ok := item.(*ast.CaseWhen)
		if !ok {
			continue
		}
		// "CASE x WHEN y" compares y against x; "CASE WHEN y" is a predicate.
		if e.Arg != nil {
			if err := a.typeOperands(when.Expr, argT); err != nil {
				return exprType{}, err
			}
		} else if _, err := a.typeExpr(when.Expr); err != nil {
			return exprType{}, err
		}
		results = append(results, when.Result)
	}
	if e.Defresult != nil {
		results = append(results, e.Defresult)
	}
	t, err := a.typeFirstOf(results, e.Defresult == nil)
	if err != nil {
		return exprType{}, err
	}
	return t, nil
}

// typeCoalesce types COALESCE, which is its first typed argument's type and
// is null only when every argument is.
func (a *analyzer) typeCoalesce(e *ast.CoalesceExpr) (exprType, error) {
	var out exprType
	found := false
	nullable := true
	for _, n := range listItems(e.Args) {
		t, err := a.typeExpr(n)
		if err != nil {
			return exprType{}, err
		}
		if !found && (t.typeOID != 0 || t.expr != nil) {
			// The result is an expression's, not the column's it came from.
			out = exprType{typeOID: t.typeOID, expr: t.expr}
			found = true
		}
		nullable = nullable && t.nullable
	}
	out.nullable = nullable
	a.inferParams(listItems(e.Args), out)
	return out, nil
}

// inferParams gives the bare placeholders among a set of alternatives the
// type the set was found to have, as a placeholder among the arguments of
// COALESCE or the results of CASE holds what the others do.
func (a *analyzer) inferParams(nodes []ast.Node, t exprType) {
	if t.typeOID == 0 && t.expr == nil {
		return
	}
	for _, n := range nodes {
		if pr, ok := n.(*ast.ParamRef); ok {
			a.inferParam(pr.Number, exprType{typeOID: t.typeOID, expr: t.expr})
		}
	}
}

// typeFirstOf types a set of alternative results, taking the first one that has
// a type. The result is nullable when any alternative is, or when the caller
// says the set is not exhaustive.
func (a *analyzer) typeFirstOf(nodes []ast.Node, nullable bool) (exprType, error) {
	var out exprType
	found := false
	for _, n := range nodes {
		t, err := a.typeExpr(n)
		if err != nil {
			return exprType{}, err
		}
		if !found && (t.typeOID != 0 || t.expr != nil) {
			out = t
			found = true
		}
		nullable = nullable || t.nullable
	}
	out.nullable = nullable
	a.inferParams(nodes, out)
	return out, nil
}

// typeArrayExpr types ARRAY[...], whose type is an array of its elements'.
func (a *analyzer) typeArrayExpr(e *ast.A_ArrayExpr) (exprType, error) {
	elemT, err := a.typeFirstOf(listItems(e.Elements), false)
	if err != nil {
		return exprType{}, err
	}
	element := a.exprOf(elemT)
	if element == nil {
		return exprType{}, nil
	}
	return a.lookupType(core.Array(element.WithNullable(false))), nil
}

// lookupType is the type an expression refers to: its row when the catalog
// holds one, its family's row when it holds only that, and the expression
// alone when it holds neither.
func (a *analyzer) lookupType(t *core.TypeExpr) exprType {
	if t == nil {
		return exprType{}
	}
	found, ok := a.cat.LookupTypeExpr(t)
	if !ok {
		return exprType{expr: t.WithNullable(false)}
	}
	out := exprType{typeOID: found.OID}
	if found.OID == found.FamilyOID && len(found.Expr.Args) > 0 {
		out.expr = found.Expr
	}
	return out
}

// namedType is the type a name refers to, or the name itself when the catalog
// has no such type.
func (a *analyzer) namedType(name string) exprType {
	return a.lookupType(core.ParseTypeExpr(name))
}

// exprOf is a type's expression: the one the analysis carries, or the one
// its row stands for. It is a copy, and nil for an untyped expression.
func (a *analyzer) exprOf(t exprType) *core.TypeExpr {
	if t.expr != nil {
		return t.expr.Clone()
	}
	if t.typeOID == 0 {
		return nil
	}
	e, err := a.cat.TypeExprOf(t.typeOID)
	if err != nil {
		return nil
	}
	return e
}

// typeExprOf writes a type as the expression a result reports, with the
// expression's own nullability set from the analysis.
func (a *analyzer) typeExprOf(t exprType) *core.TypeExpr {
	e := a.exprOf(t)
	if e == nil {
		return nil
	}
	e.Nullable = t.nullable
	return e
}

// typeNameOf reports a type's innermost family name and whether the type is
// an array, which is the flat view the legacy compiler reads.
func (a *analyzer) typeNameOf(t exprType) (string, bool) {
	e := a.exprOf(t)
	if e == nil {
		return "", false
	}
	// MySQL's unsigned families are their own types, but codegen reads
	// the signed family and an unsigned flag, which the bridge derives
	// from the expression.
	return strings.TrimSuffix(e.Innermost().Name, " unsigned"), e.IsArray()
}

// columnExprType is the type a result column of a nested query has, as an
// operand of the query around it.
func columnExprType(col core.Column) exprType {
	return exprType{typeOID: col.TypeOID, expr: col.Type.WithNullable(false), nullable: !col.NotNull}
}

// familyOID is the row everything about a type is registered on: the end of
// its resolution chain.
func (a *analyzer) familyOID(oid int64) int64 {
	chain := a.cat.ResolutionChain(oid)
	return chain[len(chain)-1]
}

// typeSubLink types a subquery used as an expression: EXISTS and IN yield a
// predicate, and a scalar subquery yields its first column.
func (a *analyzer) typeSubLink(e *ast.SubLink) (exprType, error) {
	testT, err := a.typeExpr(e.Testexpr)
	if err != nil {
		return exprType{}, err
	}
	sel, ok := e.Subselect.(*ast.SelectStmt)
	if !ok {
		if e.SubLinkType == ast.EXPR_SUBLINK || e.SubLinkType == ast.ARRAY_SUBLINK {
			return exprType{nullable: true}, nil
		}
		return a.boolType(false)
	}
	if e.SubLinkType == ast.ANY_SUBLINK || e.SubLinkType == ast.ALL_SUBLINK {
		a.inferUnnestParam(sel, testT)
	}
	sub, err := a.subquery(sel)
	if err != nil {
		return exprType{}, err
	}
	cols := sub.columns
	// "$1 = ANY(SELECT ...)" compares the placeholder against the
	// subquery's column, and holds a value of its type.
	if pr, ok := e.Testexpr.(*ast.ParamRef); ok && len(cols) > 0 {
		if err := a.typeOperands(pr, exprType{typeOID: cols[0].TypeOID, expr: cols[0].Type.WithNullable(false)}); err != nil {
			return exprType{}, err
		}
	}
	switch e.SubLinkType {
	case ast.EXPR_SUBLINK, ast.ARRAY_SUBLINK:
		if len(cols) == 0 {
			return exprType{nullable: true}, nil
		}
		// A subquery that matches no row yields NULL.
		t := columnExprType(cols[0])
		t.nullable = true
		if len(sub.untypedColumns) > 0 {
			t.untyped = sub.untypedColumns[0].cause
		}
		return t, nil
	default:
		return a.boolType(false)
	}
}

// inferUnnestParam types the placeholder of "x = ANY($1)", which DuckDB's
// parser spells as "x = ANY(SELECT unnest($1))", as a list of x's type,
// standing in for x's column when x is one. A subquery that is anything but
// a bare placeholder unnested is left to say what it holds.
func (a *analyzer) inferUnnestParam(sel *ast.SelectStmt, testT exprType) {
	if sel.FromClause != nil || sel.WhereClause != nil || len(listItems(sel.TargetList)) != 1 {
		return
	}
	rt, ok := sel.TargetList.Items[0].(*ast.ResTarget)
	if !ok {
		return
	}
	fc, ok := rt.Val.(*ast.FuncCall)
	if !ok || funcCallName(fc) != "unnest" || len(listItems(fc.Args)) != 1 {
		return
	}
	pr, ok := fc.Args.Items[0].(*ast.ParamRef)
	if !ok {
		return
	}
	element := a.exprOf(testT)
	if element == nil {
		return
	}
	list := a.lookupType(listOf(element, 1))
	list.nullable = testT.nullable
	list.columnNotNull = testT.columnNotNull
	list.sourceClassOID = testT.sourceClassOID
	list.sourceAttributeOID = testT.sourceAttributeOID
	list.sourceTableAlias = testT.sourceTableAlias
	a.inferParam(pr.Number, list)
}

// typeComparison types IS DISTINCT FROM and its negation, which compare any two
// values and never return NULL.
func (a *analyzer) typeComparison(e *ast.A_Expr) (exprType, error) {
	leftT, err := a.typeExpr(e.Lexpr)
	if err != nil {
		return exprType{}, err
	}
	rightT, err := a.typeExpr(e.Rexpr)
	if err != nil {
		return exprType{}, err
	}
	if err := a.typeOperands(e.Rexpr, leftT); err != nil {
		return exprType{}, err
	}
	if err := a.typeOperands(e.Lexpr, rightT); err != nil {
		return exprType{}, err
	}
	return a.boolType(false)
}

// typeNullIf types NULLIF(x, y), which is x's type, made nullable. The
// result is an expression's, not the column's x may be.
func (a *analyzer) typeNullIf(e *ast.A_Expr) (exprType, error) {
	leftT, err := a.typeExpr(e.Lexpr)
	if err != nil {
		return exprType{}, err
	}
	if err := a.typeOperands(e.Rexpr, leftT); err != nil {
		return exprType{}, err
	}
	return exprType{typeOID: leftT.typeOID, expr: leftT.expr, nullable: true}, nil
}

// typeOperands types a node standing opposite one of known type, giving a bare
// placeholder that type.
func (a *analyzer) typeOperands(n ast.Node, other exprType) error {
	if pr, ok := n.(*ast.ParamRef); ok {
		// Registering the placeholder first keeps the name its syntax gave
		// it, as ClickHouse's {name:Type} does.
		if _, err := a.typeParamRef(pr); err != nil {
			return err
		}
		if other.typeOID != 0 || other.expr != nil {
			a.inferParam(pr.Number, other)
		}
		a.noteCause(pr, other)
		return nil
	}
	_, err := a.typeExpr(n)
	return err
}

// normalizeOperator upper-cases a word operator ("like", "is not") so that the
// spelling a user wrote matches the one a dialect seeded. Symbolic operators
// are already canonical.
func normalizeOperator(name string) string {
	for _, r := range name {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') {
			return strings.ToUpper(name)
		}
	}
	return name
}

func opNameFromList(l *ast.List) string {
	if l == nil {
		return ""
	}
	parts := make([]string, 0, len(l.Items))
	for _, item := range l.Items {
		if s, ok := item.(*ast.String); ok {
			parts = append(parts, s.Str)
		}
	}
	return strings.Join(parts, ".")
}

// resolveOperator finds the overload of an operator over two operand types.
// An operator is registered on a family, so an operand that is an instance,
// an alias or a domain is looked up along its resolution chain: numeric(10,
// 2) + numeric(5, 1) resolves on numeric.
func (a *analyzer) resolveOperator(name string, leftT, rightT exprType) (core.OperatorOverload, error) {
	leftChain := a.cat.ResolutionChain(leftT.typeOID)
	rightChain := a.cat.ResolutionChain(rightT.typeOID)
	all, err := a.cat.FindOperators(name, 0, 0)
	if err != nil {
		return core.OperatorOverload{}, err
	}
	// The operator's overloads are read once; the pairs along the two
	// chains are tried against them in order, nearest first.
	byOperands := make(map[[2]int64]core.OperatorOverload, len(all))
	for _, ov := range all {
		key := [2]int64{ov.LeftTypeOID, ov.RightTypeOID}
		if _, seen := byOperands[key]; !seen {
			byOperands[key] = ov
		}
	}
	for _, l := range leftChain {
		for _, r := range rightChain {
			if ov, ok := byOperands[[2]int64{l, r}]; ok && l != 0 && r != 0 {
				return ov, nil
			}
		}
	}
	leftOID := leftChain[len(leftChain)-1]
	rightOID := rightChain[len(rightChain)-1]

	// No overload takes both operands as they are: among the ones the
	// operands cast to implicitly, the nearest wins, an operand matching
	// one side counting for more than a cast on it.
	best := -1
	bestScore := 0
	for i, ov := range all {
		if (leftOID == 0) != (ov.LeftTypeOID == 0) || (rightOID == 0) != (ov.RightTypeOID == 0) {
			continue
		}
		left, ok := a.operandScore(leftChain, ov.LeftTypeOID)
		if !ok {
			continue
		}
		right, ok := a.operandScore(rightChain, ov.RightTypeOID)
		if !ok {
			continue
		}
		if score := left + right; best < 0 || score > bestScore {
			best, bestScore = i, score
		}
	}
	if best >= 0 {
		return all[best], nil
	}

	// No dialect declares every operator over every type — extensions add
	// them, and an operator over a type the schema declared may never have
	// been registered. Rather than fail the query, assume the shape the
	// operator's name implies: a comparison yields a boolean and anything
	// else yields the type it was applied to.
	if a.cat.IsComparisonOperator(name) {
		boolOID, err := a.cat.BoolTypeOID()
		if err != nil {
			return core.OperatorOverload{}, err
		}
		return core.OperatorOverload{Name: name, ResultTypeOID: boolOID}, nil
	}
	result := leftOID
	if result == 0 {
		result = rightOID
	}
	return core.OperatorOverload{Name: name, ResultTypeOID: result}, nil
}

// operandScore rates how an operand, given as its resolution chain, fits
// one side of an operator: 3 for its own type, 2 for a type along its chain,
// 0 for a type it casts to implicitly, and false for one it does not.
func (a *analyzer) operandScore(chain []int64, typeOID int64) (int, bool) {
	family := chain[len(chain)-1]
	if family == 0 || typeOID == 0 {
		return 0, true
	}
	switch i := slices.Index(chain, typeOID); {
	case i == 0:
		return 3, true
	case i > 0:
		return 2, true
	}
	ok, _ := a.cat.CastAllowed(family, typeOID, "i")
	return 0, ok
}

func (a *analyzer) typeBoolExpr(b *ast.BoolExpr) (exprType, error) {
	for _, item := range listItems(b.Args) {
		if _, err := a.typeExpr(item); err != nil {
			return exprType{}, err
		}
	}
	return a.boolType(false)
}

func (a *analyzer) typeFuncCall(f *ast.FuncCall) (exprType, error) {
	name := funcCallName(f)
	if name == "" {
		return exprType{}, fmt.Errorf("func call: missing name")
	}
	if err := a.typeFuncClauses(f); err != nil {
		return exprType{}, err
	}

	if f.AggStar && (name == "count" || name == "count.*") {
		if overloads, err := a.cat.FindProcs("count", nil); err == nil && len(overloads) > 0 {
			return exprType{typeOID: overloads[0].ReturnTypeOID, nullable: overloads[0].ReturnNullable}, nil
		}
		oid, err := a.cat.TypeOID("bigint")
		if err != nil {
			return exprType{}, err
		}
		return exprType{typeOID: oid, nullable: false}, nil
	}

	args := listItems(f.Args)
	argTypes := make([]exprType, 0, len(args))
	argOIDs := make([]int64, 0, len(args))
	anyNullable := false
	for _, arg := range args {
		t, err := a.typeExpr(arg)
		if err != nil {
			return exprType{}, err
		}
		argTypes = append(argTypes, t)
		argOIDs = append(argOIDs, t.typeOID)
		anyNullable = anyNullable || t.nullable
	}

	overloads, err := a.cat.FindProcs(name, nil)
	if err != nil {
		return exprType{}, err
	}
	if len(overloads) == 0 {
		if name == "unnest" {
			return a.typeUnnest(argTypes), nil
		}
		// A dialect's function list is never complete — extensions add to it,
		// and so does the user. An unknown function leaves the result untyped
		// rather than failing the query, and says so for the dialects that
		// report an untyped result; so does a placeholder it was passed.
		unknown := exprType{nullable: true, untyped: fmt.Sprintf("unknown function %q", name)}
		for _, arg := range args {
			a.noteCause(arg, unknown)
		}
		return unknown, nil
	}
	p := a.pickOverload(overloads, argOIDs)
	generic := a.polymorphicBinding(p, argTypes)
	// An argument that is a bare placeholder takes the parameter's type: its
	// own, or the generic type the other arguments bound, at the
	// parameter's list depth.
	for i, arg := range args {
		if i >= len(p.ArgTypes) {
			break
		}
		pr, ok := arg.(*ast.ParamRef)
		if !ok {
			continue
		}
		dims, polymorphic := a.polymorphicDims(p.ArgTypes[i])
		var t exprType
		switch {
		case !polymorphic:
			t = exprType{typeOID: p.ArgTypes[i]}
		case generic != nil:
			t = a.lookupType(listOf(generic, dims))
		default:
			continue
		}
		if err := a.typeOperands(pr, t); err != nil {
			return exprType{}, err
		}
	}
	ret := a.returnType(p, argTypes, generic)
	// A result that depends on an argument's value is spelled by the seed
	// as a template over the arguments, filled in from the call.
	if computed := a.returnTemplate(p, args, argTypes); computed != nil {
		ret = a.lookupType(computed)
	}
	// A field read from a struct by name — struct_extract(s, 'a'), which is
	// also how DuckDB binds s['a'] — has the field's type when the struct's
	// type names it, and may be NULL whatever the struct is.
	if (name == "struct_extract" || name == "array_extract") && len(args) == 2 {
		if t, ok := a.typeFieldPath(argTypes[0], []string{stringConst(args[1])}); ok {
			return t, nil
		}
	}
	ret.nullable = p.ReturnNullable
	if !p.NeverNull && anyNullable && a.cat.PropagatesNullable() {
		ret.nullable = true
	}
	if a.strict != nil && a.isUntyped(ret) {
		ret.untyped = fmt.Sprintf("the result of %q has no known type here", name)
	}
	return ret, nil
}

// typeLambda types a lambda passed to a function: its body, with the
// parameters in scope, for the placeholders the body holds. The lambda
// itself is of the type the dialect seeds its parameters as, when it
// seeds one, which is how list_transform and its relatives list it.
func (a *analyzer) typeLambda(l *ast.LambdaExpr) (exprType, error) {
	if a.lambdas == nil {
		a.lambdas = map[string]int{}
	}
	for _, item := range listItems(l.Params) {
		if s, ok := item.(*ast.String); ok {
			a.lambdas[s.Str]++
		}
	}
	_, err := a.typeExpr(l.Body)
	for _, item := range listItems(l.Params) {
		if s, ok := item.(*ast.String); ok {
			a.lambdas[s.Str]--
		}
	}
	if err != nil {
		return exprType{}, fmt.Errorf("lambda: %w", err)
	}
	if oid, err := a.cat.TypeOID("lambda"); err == nil {
		return exprType{typeOID: oid}, nil
	}
	return exprType{untyped: "a lambda has no type"}, nil
}

// typeFuncClauses types the clauses a call carries besides its arguments:
// FILTER, an aggregate's ORDER BY, and a window's PARTITION BY, ORDER BY
// and frame bounds, each of which may hold a placeholder.
func (a *analyzer) typeFuncClauses(f *ast.FuncCall) error {
	if f.AggFilter != nil {
		if _, err := a.typeExpr(f.AggFilter); err != nil {
			return fmt.Errorf("filter: %w", err)
		}
	}
	if err := a.typeSortClause(f.AggOrder); err != nil {
		return err
	}
	if f.Over != nil {
		for _, item := range listItems(f.Over.PartitionClause) {
			if _, err := a.typeExpr(item); err != nil {
				return fmt.Errorf("partition by: %w", err)
			}
		}
		if err := a.typeSortClause(f.Over.OrderClause); err != nil {
			return err
		}
		for _, off := range []ast.Node{f.Over.StartOffset, f.Over.EndOffset} {
			if off == nil {
				continue
			}
			// A ROWS or GROUPS bound counts rows, as LIMIT does. A RANGE
			// bound is a value in the ORDER BY column's domain, or an
			// interval over a date or time, which a bare placeholder
			// cannot say.
			if f.Over.FrameOptions&ast.FrameOptionRange == 0 {
				if err := a.typeLimit(off); err != nil {
					return fmt.Errorf("frame: %w", err)
				}
				continue
			}
			if _, err := a.typeExpr(off); err != nil {
				return fmt.Errorf("frame: %w", err)
			}
		}
	}
	return nil
}

func (a *analyzer) typeSortClause(l *ast.List) error {
	for _, item := range listItems(l) {
		if sb, ok := item.(*ast.SortBy); ok {
			if _, err := a.typeExpr(sb.Node); err != nil {
				return fmt.Errorf("order by: %w", err)
			}
		}
	}
	return nil
}

// typeUnnest types unnest in a dialect whose seed cannot describe it —
// DuckDB lists it as a table function with no return type: the element of
// its list argument, which may be NULL.
func (a *analyzer) typeUnnest(argTypes []exprType) exprType {
	if len(argTypes) == 0 {
		return exprType{nullable: true}
	}
	t := a.exprOf(argTypes[0])
	if !t.IsArray() {
		return exprType{nullable: true}
	}
	out := a.lookupType(t.Element().WithNullable(false))
	out.nullable = true
	return out
}

// stringConst is the value of a string literal, or "" for anything else.
func stringConst(n ast.Node) string {
	if c, ok := n.(*ast.A_Const); ok {
		if s, ok := c.Val.(*ast.String); ok {
			return s.Str
		}
	}
	return ""
}

// listOf wraps a type in dims list dimensions.
func listOf(t *core.TypeExpr, dims int) *core.TypeExpr {
	t = t.WithNullable(false)
	for range dims {
		t = core.Array(t)
	}
	return t
}

// returnTemplate fills a seed's return template — Decimal(18, $2) — from
// the call: a $n that stands for an integer literal takes its value, and
// one that stands for a typed argument takes its type. A template that
// cannot be filled leaves the answer to the catalog.
func (a *analyzer) returnTemplate(p core.ProcOverload, args []ast.Node, argTypes []exprType) *core.TypeExpr {
	if p.ReturnTemplate == "" {
		return nil
	}
	template := core.ParseTypeExpr(p.ReturnTemplate)
	fill := func(arg core.TypeArg) (core.TypeArg, bool) {
		n, ok := argIndex(arg.Type.Name)
		if !ok || n >= len(args) {
			return core.TypeArg{}, false
		}
		if c, ok := args[n].(*ast.A_Const); ok {
			if lit, ok := c.Val.(*ast.Integer); ok {
				v := lit.Ival
				return core.TypeArg{Label: arg.Label, Int: &v}, true
			}
			if lit, ok := c.Val.(*ast.String); ok {
				v := lit.Str
				return core.TypeArg{Label: arg.Label, String: &v}, true
			}
		}
		if t := a.exprOf(argTypes[n]); t != nil {
			return core.TypeArg{Label: arg.Label, Type: t.WithNullable(false)}, true
		}
		return core.TypeArg{}, false
	}
	for i, arg := range template.Args {
		if arg.Type == nil || !strings.HasPrefix(arg.Type.Name, "$") {
			continue
		}
		filled, ok := fill(arg)
		if !ok {
			return nil
		}
		template.Args[i] = filled
	}
	return template
}

// returnType resolves a polymorphic return type — max(anyelement), or a
// seed's "$2" for the type of the second argument — to the type the call was
// made with. A seed relates a polymorphic parameter to the result by list
// depth: unnest(any[]) returns any, the element of its argument, and
// array_agg(any) returns any[], a list of it; generic is that element, as
// polymorphicBinding found it.
func (a *analyzer) returnType(p core.ProcOverload, argTypes []exprType, generic *core.TypeExpr) exprType {
	if p.ReturnTypeOID == 0 || len(argTypes) == 0 {
		return exprType{typeOID: p.ReturnTypeOID}
	}
	name, err := a.cat.TypeName(p.ReturnTypeOID)
	if err != nil {
		return exprType{typeOID: p.ReturnTypeOID}
	}
	if n, ok := argIndex(name); ok {
		if n < len(argTypes) {
			return exprType{typeOID: argTypes[n].typeOID, expr: argTypes[n].expr}
		}
		return exprType{}
	}
	dims, ok := a.polymorphicDims(p.ReturnTypeOID)
	if !ok {
		return exprType{typeOID: p.ReturnTypeOID}
	}
	if generic != nil {
		return a.lookupType(listOf(generic, dims))
	}
	if argTypes[0].typeOID != 0 || argTypes[0].expr != nil {
		return exprType{typeOID: argTypes[0].typeOID, expr: argTypes[0].expr}
	}
	return exprType{typeOID: p.ReturnTypeOID}
}

// polymorphicBinding is the type an overload's polymorphic parameters stand
// for, read from the first argument in such a position that has a type:
// the argument's own type for any, its element for any[]. It is nil when
// no argument says.
func (a *analyzer) polymorphicBinding(p core.ProcOverload, argTypes []exprType) *core.TypeExpr {
	for i, argT := range argTypes {
		if i >= len(p.ArgTypes) {
			break
		}
		dims, ok := a.polymorphicDims(p.ArgTypes[i])
		if !ok {
			continue
		}
		t := a.exprOf(argT)
		if t == nil || t.ArrayDims() < dims {
			continue
		}
		for range dims {
			t = t.Element()
		}
		return t.WithNullable(false)
	}
	return nil
}

// argIndex reads a seed's "$n" pseudo-type as the zero-based index of the
// argument whose type it stands for.
func argIndex(typeName string) (int, bool) {
	rest, ok := strings.CutPrefix(typeName, "$")
	if !ok {
		return 0, false
	}
	n := 0
	for _, r := range rest {
		if r < '0' || r > '9' {
			return 0, false
		}
		n = n*10 + int(r-'0')
	}
	if n == 0 {
		return 0, false
	}
	return n - 1, true
}

// polymorphicDims reports whether a parameter or result type accepts any
// type, and at what list depth: 0 for any, 1 for any[].
func (a *analyzer) polymorphicDims(oid int64) (int, bool) {
	if oid == 0 {
		return 0, true
	}
	name, err := a.cat.TypeName(oid)
	if err != nil {
		return 0, false
	}
	if _, ok := argIndex(name); ok {
		return 0, true
	}
	if isPolymorphic(name) {
		return 0, true
	}
	if name != core.ArrayTypeName {
		return 0, false
	}
	t, err := a.cat.TypeExprOf(oid)
	if err != nil || !isPolymorphic(t.Innermost().Name) {
		return 0, false
	}
	return t.ArrayDims(), true
}

// listDims is the list depth of a type: 0 for anything but a list.
func (a *analyzer) listDims(oid int64) int {
	t, err := a.cat.TypeExprOf(oid)
	if err != nil {
		return 0
	}
	return t.ArrayDims()
}

func isPolymorphic(typeName string) bool {
	switch typeName {
	case "any", "anyelement", "anyarray", "anynonarray", "anyenum", "anyrange",
		"anymultirange", "anycompatible", "anycompatiblearray",
		"anycompatiblenonarray", "anycompatiblerange":
		return true
	}
	return false
}

// pickOverload chooses the overload whose parameters the call's arguments
// match best: an exact type match on a parameter beats a match on the
// argument's family, which beats a polymorphic parameter, which beats a
// parameter the argument casts to implicitly, which beats one it does not,
// and any overload of the right arity beats one of the wrong arity.
func (a *analyzer) pickOverload(overloads []core.ProcOverload, argTypes []int64) core.ProcOverload {
	best := -1
	bestScore := 0
	chains := make([][]int64, len(argTypes))
	for j, oid := range argTypes {
		if oid != 0 {
			chains[j] = a.cat.ResolutionChain(oid)
		}
	}
	castable := map[[2]int64]bool{}
	for i := range overloads {
		ov := &overloads[i]
		if len(ov.ArgTypes) != len(argTypes) {
			continue
		}
		score := 0
		for j, oid := range argTypes {
			switch {
			case oid != 0 && oid == ov.ArgTypes[j]:
				score += 3
			case oid != 0 && slices.Contains(chains[j], ov.ArgTypes[j]):
				score += 2
			default:
				dims, polymorphic := a.polymorphicDims(ov.ArgTypes[j])
				switch {
				case polymorphic && (oid == 0 || a.listDims(oid) >= dims):
					// A list parameter takes a list, or an argument whose
					// type is not known.
					score++
				case polymorphic || oid == 0:
				default:
					pair := [2]int64{chains[j][len(chains[j])-1], a.familyOID(ov.ArgTypes[j])}
					ok, seen := castable[pair]
					if !seen {
						ok, _ = a.cat.CastAllowed(pair[0], pair[1], "i")
						castable[pair] = ok
					}
					if !ok {
						score -= 2
					}
				}
			}
		}
		if best < 0 || score > bestScore {
			best, bestScore = i, score
		}
	}
	if best >= 0 {
		return overloads[best]
	}
	return overloads[0]
}

func funcCallName(f *ast.FuncCall) string {
	if f.Funcname != nil {
		return opNameFromList(f.Funcname)
	}
	if f.Func != nil {
		return strings.ToLower(f.Func.Name)
	}
	return ""
}

func (a *analyzer) typeTypeCast(c *ast.TypeCast) (exprType, error) {
	if c.TypeName == nil {
		return exprType{}, fmt.Errorf("cast: missing target type")
	}
	target := core.TypeExprOfTypeName(c.TypeName)
	if target == nil {
		return exprType{}, fmt.Errorf("cast: missing target type")
	}
	t := a.lookupType(target)
	// A cast is how a query says what an otherwise untyped placeholder
	// holds, and a placeholder so typed is not null unless the type says
	// otherwise, as ClickHouse's Nullable(String) does. Anything else
	// cast is NULL when it was NULL before, or when the type says so.
	t.nullable = target.Nullable
	if pr, ok := c.Arg.(*ast.ParamRef); ok {
		a.markCast(pr)
		if err := a.typeOperands(pr, t); err != nil {
			return exprType{}, err
		}
		// A TRY_CAST is NULL where the cast would fail, whatever it
		// was given.
		t.nullable = t.nullable || c.Try
		return t, nil
	}
	arg, err := a.typeExpr(c.Arg)
	if err != nil {
		return exprType{}, err
	}
	t.nullable = t.nullable || arg.nullable || c.Try
	return t, nil
}
