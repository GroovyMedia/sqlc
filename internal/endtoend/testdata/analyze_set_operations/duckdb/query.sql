-- name: UnionNullLiteral :many
SELECT id, name FROM authors UNION ALL SELECT NULL, NULL;

-- name: UnionNullableRight :many
SELECT name AS label FROM editors UNION SELECT bio FROM authors;

-- name: UnionNotNull :many
SELECT id, name FROM authors UNION ALL SELECT id, name FROM editors;

-- name: ExceptKeepsLeft :many
SELECT bio FROM editors EXCEPT SELECT bio FROM authors;

-- name: IntersectKeepsLeft :many
SELECT name FROM editors INTERSECT SELECT bio FROM authors;
