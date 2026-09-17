-- name: IdentOnNonNullable :many
SELECT bar FROM foo WHERE bar = sqlc.narg(bar);

-- name: IdentOnNullable :many
SELECT maybe_bar FROM foo WHERE maybe_bar = sqlc.narg(maybe_bar);

-- name: StringOnNonNullable :many
SELECT bar FROM foo WHERE bar = sqlc.narg('bar');

-- name: StringOnNullable :many
SELECT maybe_bar FROM foo WHERE maybe_bar = sqlc.narg('maybe_bar');

-- name: Projected :many
SELECT bar, sqlc.narg('label')::TEXT AS label FROM foo;

-- name: ProjectedInExpr :many
SELECT bar, (sqlc.narg('label')::TEXT || bar) AS labelled FROM foo;
