-- name: GreatestLeast :many
SELECT
  id,
  greatest(bio, bio) AS g,
  least(born) AS l,
  concat_ws(bio, name, name) AS c
FROM authors;
