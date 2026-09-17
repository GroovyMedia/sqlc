-- name: ListAuthors :many
SELECT id, name FROM authors;

-- name: CreateMacro :exec
CREATE MACRO add(a, b) AS a + b;

-- name: CallProcedure :exec
CALL dbgen(sf = 1);

-- name: MergeAuthors :exec
MERGE INTO authors USING authors AS src ON authors.id = src.id
WHEN MATCHED THEN UPDATE SET name = src.name;
