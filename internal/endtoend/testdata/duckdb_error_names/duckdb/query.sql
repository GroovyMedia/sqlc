-- name: Gap :many
SELECT id FROM authors WHERE id = $1 AND name = $3;

-- name: Mix :many
SELECT id FROM authors WHERE id = $1 AND name = ?;

-- name: UnknownMacro :many
SELECT id FROM authors WHERE id = sqlc.foo(ids);

-- name: EmbedMissing :many
SELECT sqlc.embed(nope) FROM authors;

-- name: Untyped :many
SELECT id FROM authors WHERE frobnicate(name) = @needle;

-- name: Conflict :many
SELECT id FROM authors WHERE name = $v OR id = sqlc.arg(v);
