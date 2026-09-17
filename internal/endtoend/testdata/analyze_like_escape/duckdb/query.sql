-- name: LikeEscape :many
SELECT id FROM authors WHERE name LIKE $1 ESCAPE '\';

-- name: NotILikeEscape :many
SELECT id FROM authors WHERE name NOT ILIKE $1 ESCAPE '\';

-- name: LikeEscapeColumn :many
SELECT id, bio LIKE '%\%%' ESCAPE '\' AS has_percent, name ILIKE $1 ESCAPE '\' AS matches
FROM authors;
