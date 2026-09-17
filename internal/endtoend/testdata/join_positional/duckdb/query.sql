-- name: PositionalJoin :many
SELECT a.id, a.name, b.id AS book_id, b.title
FROM authors a
POSITIONAL JOIN books b;

-- name: PositionalJoinSubquery :many
SELECT a.id, b.title
FROM authors a
POSITIONAL JOIN (SELECT * FROM books LIMIT 1) b;
