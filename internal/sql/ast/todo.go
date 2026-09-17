package ast

// TODO stands in for syntax sqlc has no node for. An engine that reports
// what it stood in for gives it the location and the text, so that the
// analysis can name the expression it cannot type.
type TODO struct {
	Tag NodeTag[TODO] `json:"tag"`

	Location int    `json:"location,omitempty"`
	Text     string `json:"text,omitempty"`
}

func (n *TODO) Pos() int {
	return n.Location
}
