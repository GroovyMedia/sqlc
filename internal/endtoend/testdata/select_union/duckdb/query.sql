-- name: UnionNullLiteral :many
SELECT id, name FROM authors UNION ALL SELECT NULL, NULL;

-- name: UnionNullableRight :many
SELECT name AS label FROM editors UNION SELECT bio FROM authors;

-- name: UnionNotNull :many
SELECT id, name FROM authors UNION ALL SELECT id, name FROM editors;

-- name: UnionInCTE :many
WITH people AS (SELECT id, name FROM authors UNION ALL SELECT NULL, NULL)
SELECT id, name FROM people;

-- name: UnionInSubquery :many
SELECT p.id FROM (SELECT id FROM editors UNION ALL SELECT NULL) p;

-- name: ExceptKeepsLeft :many
SELECT bio FROM editors EXCEPT SELECT bio FROM authors;
