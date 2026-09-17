-- name: Expired :many
SELECT id FROM authors WHERE created_at + $1 < now();

-- name: Search :many
SELECT id FROM authors WHERE name LIKE '%' || $1 || '%';

-- name: DaysBack :many
SELECT id FROM authors WHERE created_at >= now() - $1::BIGINT * INTERVAL '1 day';
