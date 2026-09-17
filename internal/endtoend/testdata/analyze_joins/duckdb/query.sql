-- name: LeftJoin :many
SELECT u.id, u.name, p.id AS post_id, p.title, p.body
FROM users u LEFT JOIN posts p ON p.user_id = u.id;

-- name: RightJoin :many
SELECT u.id, u.name, p.id AS post_id, p.title
FROM users u RIGHT JOIN posts p ON p.user_id = u.id;

-- name: FullJoin :many
SELECT u.id, u.name, p.id AS post_id, p.title
FROM users u FULL JOIN posts p ON p.user_id = u.id;

-- name: InnerJoin :many
SELECT u.id, u.name, p.id AS post_id, p.title
FROM users u JOIN posts p ON p.user_id = u.id;

-- name: LeftJoinChain :many
SELECT u.id, p.id AS post_id, l.user_id AS liker
FROM users u
LEFT JOIN posts p ON p.user_id = u.id
LEFT JOIN likes l ON l.post_id = p.id;

-- name: RightJoinOverLeft :many
SELECT u.id, p.id AS post_id, l.user_id AS liker
FROM users u
LEFT JOIN posts p ON p.user_id = u.id
RIGHT JOIN likes l ON l.post_id = p.id;

-- name: LeftJoinStar :many
SELECT * FROM users u LEFT JOIN posts p ON p.user_id = u.id;

-- name: CoalesceJoinColumn :many
SELECT u.id, COALESCE(p.title, '') AS title, COALESCE(p.body, '') AS body, COALESCE(p.title, p.body) AS either
FROM users u LEFT JOIN posts p ON p.user_id = u.id;

-- name: ParamsAgainstJoinColumns :many
SELECT u.id, p.title
FROM users u LEFT JOIN posts p ON p.user_id = u.id AND p.title <> $1
WHERE p.title <> $2 OR p.title IS NULL;

-- name: LateralScope :many
SELECT u.id, u.name, latest.title IS NOT NULL AS has_post
FROM users u
LEFT JOIN LATERAL (
  SELECT recent.title
  FROM (SELECT p.id, p.title FROM posts p WHERE p.user_id = u.id ORDER BY p.id DESC LIMIT 5) recent
  LIMIT 1
) latest ON true;
