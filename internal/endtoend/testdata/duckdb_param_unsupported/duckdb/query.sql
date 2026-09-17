-- name: TryNamed :many
SELECT id FROM authors WHERE TRY(@x::INTEGER) = small;

-- name: TryNumbered :many
SELECT id FROM authors WHERE name = $1 AND small = TRY(CAST($2 AS INTEGER));
