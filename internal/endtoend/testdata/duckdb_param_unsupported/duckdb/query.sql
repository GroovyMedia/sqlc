-- A star's REPLACE list is syntax sqlc has no node for, so a placeholder
-- inside it is one the analyzer never sees.

-- name: ReplaceNamed :many
SELECT id FROM authors WHERE EXISTS (SELECT * REPLACE (small + @n AS small) FROM authors);

-- name: ReplaceNumbered :many
SELECT id FROM authors WHERE name = $1 AND EXISTS (SELECT * REPLACE (small + $2 AS small) FROM authors);
