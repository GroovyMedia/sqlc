-- name: GetFirst :many
SELECT * FROM second_table;

-- name: GetCopied :many
SELECT val, n FROM second_table;

-- name: GetThroughView :many
SELECT val, n FROM foo_view;
