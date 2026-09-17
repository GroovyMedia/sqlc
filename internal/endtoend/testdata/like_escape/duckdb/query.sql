-- name: LikeEscape :many
SELECT id FROM authors WHERE name LIKE @pat ESCAPE '\';

-- name: LikeEscapeParam :many
SELECT id FROM authors WHERE name LIKE @pat ESCAPE @esc;

-- name: NotILikeEscapeParam :many
SELECT id FROM authors WHERE name NOT ILIKE @pat ESCAPE @esc;

-- name: LikeEscapeColumn :many
SELECT id, bio LIKE @pat ESCAPE @esc AS matches FROM authors;
