-- name: WindowFrame :many
SELECT id, sum(small) OVER (ORDER BY id ROWS BETWEEN @n PRECEDING AND CURRENT ROW) AS s
FROM authors;

-- name: WindowFrameBothBounds :many
SELECT id, sum(small) OVER (ORDER BY id ROWS BETWEEN 1 PRECEDING AND @m FOLLOWING) AS s
FROM authors
WHERE small = @n;

-- name: WindowNamed :many
SELECT id, sum(small) OVER w AS s
FROM authors
WINDOW w AS (ORDER BY id ROWS BETWEEN @n PRECEDING AND CURRENT ROW);

-- name: WindowGroups :many
SELECT id, sum(small) OVER (ORDER BY small GROUPS BETWEEN @n PRECEDING AND CURRENT ROW) AS s
FROM authors;

-- name: WindowRange :many
SELECT id, sum(small) OVER (ORDER BY created_at RANGE BETWEEN @iv::INTERVAL PRECEDING AND CURRENT ROW) AS s
FROM authors;

-- name: WindowRangeNumber :many
SELECT id, sum(small) OVER (ORDER BY small RANGE BETWEEN @lo::INTEGER PRECEDING AND @hi::INTEGER FOLLOWING) AS s
FROM authors;

-- name: WindowDefaultFrame :many
SELECT id, sum(small) OVER (ORDER BY id) AS s
FROM authors;
