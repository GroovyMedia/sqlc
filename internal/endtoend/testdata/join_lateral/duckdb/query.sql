-- name: LatestPostPerUser :many
SELECT u.id, u.name, latest.title, latest.body
FROM users u, LATERAL (
  SELECT p.title, p.body
  FROM posts p
  WHERE p.user_id = u.id
  ORDER BY p.id DESC
  LIMIT 1
) latest;

-- name: LatestPostOrNone :many
SELECT u.id, u.name, latest.title
FROM users u
LEFT JOIN LATERAL (
  SELECT recent.title
  FROM (
    SELECT p.id, p.title
    FROM posts p
    WHERE p.user_id = u.id AND p.title <> $1
    ORDER BY p.id DESC
    LIMIT 5
  ) recent
  LIMIT 1
) latest ON true;
