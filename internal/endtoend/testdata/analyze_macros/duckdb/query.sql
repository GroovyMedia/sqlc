-- name: AtSign :many
SELECT id, name FROM authors WHERE name = @name AND royalties > @min_royalties;

-- name: Arg :many
SELECT id, name FROM authors WHERE name = sqlc.arg(name) AND royalties > sqlc.arg('min_royalties');

-- name: NargOnNotNull :many
SELECT id, name FROM authors WHERE name = sqlc.narg(name);

-- name: NargOnNullable :many
SELECT id, name FROM authors WHERE bio = sqlc.narg(bio);

-- name: RepeatedName :many
SELECT id FROM authors WHERE name = @q OR bio = sqlc.arg(q) OR name = $q;

-- name: RepeatedDollarName :many
SELECT id FROM authors WHERE name = $name OR bio = $name OR id = $id;

-- name: InsertAtSign :exec
INSERT INTO authors (id, name, bio, royalties) VALUES (@id, @name, sqlc.narg(bio), @royalties);

-- name: NargProjected :many
SELECT id, sqlc.narg('label')::VARCHAR AS label, coalesce(sqlc.narg('label')::VARCHAR, name) AS shown FROM authors;
