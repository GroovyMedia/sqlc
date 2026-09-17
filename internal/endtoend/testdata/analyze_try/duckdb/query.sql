-- name: TryCastNotNull :many
SELECT id, TRY_CAST(name AS INTEGER) AS n FROM authors;

-- name: TryCastNullable :many
SELECT id, TRY_CAST(bio AS INTEGER) AS b FROM authors;

-- name: CastNotNull :many
SELECT id, CAST(score AS BIGINT) AS s FROM authors;

-- name: Try :many
SELECT id, TRY(score // 0) AS z FROM authors;

-- name: CastOverTry :many
SELECT id, TRY(score // 0)::BIGINT AS z FROM authors;
