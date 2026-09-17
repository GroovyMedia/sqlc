-- name: Page :many
SELECT id, title FROM posts ORDER BY id LIMIT $1 OFFSET $2;

-- name: TopScored :many
SELECT id FROM posts WHERE score > $1 ORDER BY score DESC LIMIT $2;

-- name: PageNamed :many
SELECT id FROM posts ORDER BY id LIMIT $page_size OFFSET $skip;
