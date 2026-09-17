package ast

// LambdaExpr is a lambda passed to a function, as DuckDB's
// list_transform(l, lambda x: x + 1) takes one: the names of its
// parameters, as Strings, and its body, an expression over them.
type LambdaExpr struct {
	Tag NodeTag[LambdaExpr] `json:"tag"`

	Params   *List `json:"params,omitempty"`
	Body     Node  `json:"body,omitempty"`
	Location int   `json:"location"`
}

func (n *LambdaExpr) Pos() int {
	return n.Location
}
