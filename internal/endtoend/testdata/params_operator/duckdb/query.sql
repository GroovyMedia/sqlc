-- name: Expired :many
SELECT id FROM authors WHERE created_at + @age < now();

-- name: Scaled :many
SELECT id, price * @factor::DECIMAL(10,2) AS scaled FROM authors;

-- name: Search :many
SELECT id FROM authors WHERE name LIKE '%' || @q || '%';

-- name: Shifted :many
SELECT id + @n AS shifted FROM authors;

-- name: DaysBack :many
SELECT id FROM authors WHERE created_at >= now() - @days::BIGINT * INTERVAL '1 day';
