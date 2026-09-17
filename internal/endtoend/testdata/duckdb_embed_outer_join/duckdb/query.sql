-- name: EmbedLeft :many
SELECT b.id, sqlc.embed(a) FROM books b
LEFT JOIN authors a ON a.id = b.author_id;

-- name: EmbedRight :many
SELECT sqlc.embed(b), a.id AS author_id FROM books b
RIGHT JOIN authors a ON a.id = b.author_id;

-- name: EmbedFull :many
SELECT sqlc.embed(b), sqlc.embed(a) FROM books b
FULL JOIN authors a ON a.id = b.author_id;
