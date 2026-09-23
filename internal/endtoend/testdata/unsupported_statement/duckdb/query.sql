-- name: ListAuthors :many
SELECT id, name FROM authors;

-- name: CreateMacro :exec
CREATE MACRO add(a, b) AS a + b;

-- name: CallProcedure :exec
CALL dbgen(sf = 1);
