-- name: First :many
SELECT id FROM authors;

-- name: Second :many
SELECT id FROM authors;

-- name: DuplicateCTE :many
WITH x AS (SELECT 1), x AS (SELECT 2) SELECT * FROM x;
