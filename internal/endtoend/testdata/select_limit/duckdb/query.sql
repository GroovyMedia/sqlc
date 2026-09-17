-- name: FooLimit :many
SELECT a
FROM foo
LIMIT $1;

-- name: FooLimitOffset :many
SELECT a
FROM foo
LIMIT $1
OFFSET $2;

-- name: FooNamedLimit :many
SELECT a
FROM foo
LIMIT $page_size
OFFSET @skip;
