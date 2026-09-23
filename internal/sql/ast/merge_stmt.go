package ast

import (
	"strings"

	"github.com/sqlc-dev/sqlc/internal/sql/format"
)

// MergeStmt is MERGE INTO. Only the DuckDB engine produces it.
type MergeStmt struct {
	Tag NodeTag[MergeStmt] `json:"tag"`

	Relation *RangeVar `json:"relation,omitempty"`
	// Source is the USING relation: a table, a subquery or a function.
	Source Node `json:"source,omitempty"`
	// JoinCondition is the ON condition. UsingColumns is the USING (cols)
	// shorthand instead; each item is a *String.
	JoinCondition Node  `json:"join_condition,omitempty"`
	UsingColumns  *List `json:"using_columns,omitempty"`
	// WhenClauses holds *MergeWhenClause items, in source order.
	WhenClauses   *List       `json:"when_clauses,omitempty"`
	ReturningList *List       `json:"returning_list,omitempty"`
	WithClause    *WithClause `json:"with_clause,omitempty"`
	// Batch is set when the source is a VALUES list of one row.
	Batch *BatchSource `json:"batch,omitempty"`
}

func (n *MergeStmt) Pos() int {
	return 0
}

type MergeMatchKind int

const (
	MergeWhenMatched MergeMatchKind = iota
	MergeWhenNotMatchedByTarget
	MergeWhenNotMatchedBySource
)

type MergeAction int

const (
	MergeActionUpdate MergeAction = iota
	MergeActionDelete
	MergeActionInsert
	MergeActionDoNothing
	MergeActionError
)

// MergeWhenClause is one WHEN ... THEN arm of MERGE INTO.
type MergeWhenClause struct {
	Tag NodeTag[MergeWhenClause] `json:"tag"`

	Kind      MergeMatchKind `json:"kind"`
	Condition Node           `json:"condition,omitempty"`
	Action    MergeAction    `json:"action"`
	// Star is UPDATE SET * or INSERT *: every column, from the source
	// column of the same name (ByName) or position.
	Star   bool `json:"star"`
	ByName bool `json:"by_name"`
	// TargetList is the UPDATE SET list of *ResTarget.
	TargetList *List `json:"target_list,omitempty"`
	// Cols and Values are the INSERT column list (*ResTarget names) and
	// VALUES expressions.
	Cols          *List `json:"cols,omitempty"`
	Values        *List `json:"values,omitempty"`
	DefaultValues bool  `json:"default_values"`
	ErrorExpr     Node  `json:"error_expr,omitempty"`
	Location      int   `json:"location"`
}

func (n *MergeWhenClause) Pos() int {
	return n.Location
}

func (n *MergeStmt) Format(buf *TrackedBuffer, d format.Dialect) {
	if n == nil {
		return
	}
	if n.WithClause != nil {
		buf.astFormat(n.WithClause, d)
		buf.Line()
	}
	buf.WriteString("MERGE INTO ")
	buf.astFormat(n.Relation, d)
	buf.Line()
	buf.WriteString("USING ")
	buf.astFormat(n.Source, d)
	if items(n.UsingColumns) {
		var names []string
		for _, item := range n.UsingColumns.Items {
			if s, ok := item.(*String); ok {
				names = append(names, s.Str)
			}
		}
		buf.WriteString(" USING (" + strings.Join(names, ", ") + ")")
	} else if set(n.JoinCondition) {
		buf.WriteString(" ON ")
		buf.condition(n.JoinCondition, d)
	}
	if n.WhenClauses != nil {
		for _, item := range n.WhenClauses.Items {
			buf.Line()
			buf.astFormat(item, d)
		}
	}
	if items(n.ReturningList) {
		buf.Line()
		buf.WriteString("RETURNING ")
		buf.astFormat(n.ReturningList, d)
	}
}

func (n *MergeWhenClause) Format(buf *TrackedBuffer, d format.Dialect) {
	if n == nil {
		return
	}
	switch n.Kind {
	case MergeWhenMatched:
		buf.WriteString("WHEN MATCHED")
	case MergeWhenNotMatchedByTarget:
		buf.WriteString("WHEN NOT MATCHED")
	case MergeWhenNotMatchedBySource:
		buf.WriteString("WHEN NOT MATCHED BY SOURCE")
	}
	if set(n.Condition) {
		buf.WriteString(" AND ")
		buf.condition(n.Condition, d)
	}
	buf.WriteString(" THEN ")
	switch n.Action {
	case MergeActionUpdate:
		buf.WriteString("UPDATE")
		if n.ByName {
			buf.WriteString(" BY NAME")
		}
		if n.Star {
			buf.WriteString(" SET *")
			return
		}
		buf.WriteString(" SET ")
		for i, item := range listItemsOf(n.TargetList) {
			if i > 0 {
				buf.WriteString(", ")
			}
			if rt, ok := item.(*ResTarget); ok && rt.Name != nil {
				buf.WriteString(*rt.Name + " = ")
				buf.astFormat(rt.Val, d)
			}
		}
	case MergeActionDelete:
		buf.WriteString("DELETE")
	case MergeActionInsert:
		buf.WriteString("INSERT")
		if n.ByName {
			buf.WriteString(" BY NAME")
		}
		switch {
		case n.DefaultValues:
			buf.WriteString(" DEFAULT VALUES")
		case n.Star:
			buf.WriteString(" *")
		default:
			if items(n.Cols) {
				buf.WriteString(" (")
				buf.joinComma(n.Cols, d)
				buf.WriteString(")")
			}
			if items(n.Values) {
				buf.WriteString(" VALUES (")
				buf.joinComma(n.Values, d)
				buf.WriteString(")")
			}
		}
	case MergeActionDoNothing:
		buf.WriteString("DO NOTHING")
	case MergeActionError:
		buf.WriteString("ERROR")
		if set(n.ErrorExpr) {
			buf.WriteString(" ")
			buf.astFormat(n.ErrorExpr, d)
		}
	}
}

func listItemsOf(l *List) []Node {
	if l == nil {
		return nil
	}
	return l.Items
}
