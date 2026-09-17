package ast

type GroupingSetKind uint

// The kinds PostgreSQL's parser reports, in its numbering.
const (
	GroupingSetEmpty  GroupingSetKind = 1
	GroupingSetSimple GroupingSetKind = 2
	GroupingSetRollup GroupingSetKind = 3
	GroupingSetCube   GroupingSetKind = 4
	GroupingSetSets   GroupingSetKind = 5
)

func (n *GroupingSetKind) Pos() int {
	return 0
}
