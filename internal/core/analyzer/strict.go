package analyzer

import (
	"fmt"

	"github.com/sqlc-dev/sqlc/internal/core"
	"github.com/sqlc-dev/sqlc/internal/sql/ast"
	"github.com/sqlc-dev/sqlc/internal/sql/sqlerr"
)

// strict is the bookkeeping behind a strict dialect's errors: where each
// placeholder first stands, why an untyped one has no type, and the first
// placeholder two uses typed differently. The analyzers of the nested
// queries share it, the way they share the parameters. It is nil for a
// dialect that is not strict.
type strict struct {
	paramLocation map[int]int
	paramCause    map[int]string
	// cast holds the placeholders a cast typed, which no other use can
	// contradict: the cast is the query's own word on the type.
	cast     map[int]bool
	conflict error
}

// untypedColumn is a result column that has no type, with why it has none
// when the expression said so, kept until the query's result is reported:
// a nested query's columns are not the statement's, and a cast around the
// expression types it after all.
type untypedColumn struct {
	name     string
	cause    string
	location int
}

func newStrict() *strict {
	return &strict{
		paramLocation: map[int]int{},
		paramCause:    map[int]string{},
		cast:          map[int]bool{},
	}
}

// markCast records that a cast typed a placeholder.
func (a *analyzer) markCast(p *ast.ParamRef) {
	if a.strict != nil {
		a.strict.cast[p.Number] = true
	}
}

// locate records where a placeholder first stands, for the error that
// names it.
func (a *analyzer) locate(p *ast.ParamRef) {
	if a.strict == nil {
		return
	}
	if _, ok := a.strict.paramLocation[p.Number]; !ok {
		a.strict.paramLocation[p.Number] = p.Location
	}
}

// noteCause records why a placeholder compared with an untyped expression
// has no type of its own, when the expression says why.
func (a *analyzer) noteCause(n ast.Node, other exprType) {
	if a.strict == nil || other.untyped == "" {
		return
	}
	pr, ok := n.(*ast.ParamRef)
	if !ok {
		return
	}
	if _, ok := a.strict.paramCause[pr.Number]; !ok {
		a.strict.paramCause[pr.Number] = other.untyped
	}
}

// noteConflict records a placeholder that two columns type differently,
// which a strict dialect reports rather than keeping the first type. A
// literal or a function argument says less about a placeholder than a
// column does, so only two columns can disagree.
func (a *analyzer) noteConflict(number int, cur core.Parameter, t exprType) {
	if a.strict == nil || a.strict.conflict != nil || a.strict.cast[number] {
		return
	}
	if cur.Source == nil || t.sourceAttributeOID == 0 {
		return
	}
	was := exprType{typeOID: cur.TypeOID, expr: cur.Type.WithNullable(false)}
	if a.isUntyped(was) || a.isUntyped(t) || a.sameFamily(was, t) {
		return
	}
	a.strict.conflict = &sqlerr.Error{
		Message: fmt.Sprintf("parameter $%d is used as %s and as %s; cast it to one type, as in $%d::BIGINT",
			number, a.spell(was), a.spell(t), number),
		Location: a.strict.paramLocation[number],
	}
}

// sameFamily reports whether two types are one type, whatever the spelling:
// an alias resolves to the type it names, and an instance to its family.
func (a *analyzer) sameFamily(x, y exprType) bool {
	if x.typeOID != 0 && y.typeOID != 0 {
		return a.familyOID(x.typeOID) == a.familyOID(y.typeOID)
	}
	return a.exprOf(x).Key() == a.exprOf(y).Key()
}

// isUntyped reports whether an expression has no usable type: none at all,
// or a polymorphic one, which says only that it stands for any type.
func (a *analyzer) isUntyped(t exprType) bool {
	e := a.exprOf(t)
	if e == nil {
		return true
	}
	name := e.Innermost().Name
	if _, ok := argIndex(name); ok {
		return true
	}
	return isPolymorphic(name)
}

func (a *analyzer) spell(t exprType) string {
	if e := a.exprOf(t); e != nil {
		return e.String()
	}
	return "?"
}

// recordUntyped keeps a result column that has no type, to report when the
// result is the statement's.
func (a *analyzer) recordUntyped(name string, t exprType, location int) {
	if a.strict == nil || !a.isUntyped(t) {
		return
	}
	a.untypedColumns = append(a.untypedColumns, untypedColumn{name: name, cause: t.untyped, location: location})
}

// checkTyped is the strict dialect's verdict on a result: a placeholder
// two uses disagree on, then a placeholder with no type, then a result
// column with none, each with what to do about it. A cast is the fix in
// every case, since a cast types whatever it wraps.
func (a *analyzer) checkTyped(res core.PrepareResult) error {
	if a.strict == nil {
		return nil
	}
	if a.strict.conflict != nil {
		return a.strict.conflict
	}
	for _, p := range res.Parameters {
		if !a.isUntyped(exprType{typeOID: p.TypeOID, expr: p.Type}) {
			continue
		}
		cause := a.strict.paramCause[p.Number]
		if cause == "" {
			cause = "nothing in the query says what it holds"
		}
		return &sqlerr.Error{
			Message:  fmt.Sprintf("parameter $%d has no type: %s; cast it to the type it holds, as in $%d::BIGINT", p.Number, cause, p.Number),
			Location: a.strict.paramLocation[p.Number],
		}
	}
	for _, col := range a.untypedColumns {
		cause := col.cause
		if cause == "" {
			cause = "nothing says what it holds"
		}
		return &sqlerr.Error{
			Message:  fmt.Sprintf("column %q has no type: %s; cast it to the type it holds, as in expr::BIGINT", col.name, cause),
			Location: col.location,
		}
	}
	return nil
}

// unsupported is why a TODO node has no type: sqlc has no node for what
// was written there.
func unsupported(n *ast.TODO) string {
	if n.Text == "" {
		return "unsupported expression"
	}
	return fmt.Sprintf("unsupported expression %q", n.Text)
}
