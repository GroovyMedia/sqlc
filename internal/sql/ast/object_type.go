package ast

type ObjectType uint

// ObjectTypeMatview is the object type PostgreSQL's parser reports for
// CREATE MATERIALIZED VIEW, which it converts as a CreateTableAsStmt.
// The values are the parser's own; the postgresql engine's tests pin this
// one.
const ObjectTypeMatview ObjectType = 24

func (n *ObjectType) Pos() int {
	return 0
}
