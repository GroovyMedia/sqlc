-- name: LeftJoinChain :many
SELECT u.id, p.title, l.user_id AS liker
FROM users u
LEFT JOIN posts p ON p.user_id = u.id
LEFT JOIN likes l ON l.post_id = p.id;

-- name: LeftJoinNested :many
SELECT u.id, p.title, l.user_id AS liker
FROM users u
LEFT JOIN (posts p JOIN likes l ON l.post_id = p.id) ON p.user_id = u.id;

-- name: RightJoinOverLeft :many
SELECT u.id, p.title, l.user_id AS liker
FROM users u
LEFT JOIN posts p ON p.user_id = u.id
RIGHT JOIN likes l ON l.post_id = p.id;

-- name: FullJoinOverLeft :many
SELECT u.id, p.title, l.user_id AS liker
FROM users u
LEFT JOIN posts p ON p.user_id = u.id
FULL JOIN likes l ON l.post_id = p.id;

-- name: InnerJoinOverLeft :many
SELECT u.id, p.title, l.user_id AS liker
FROM users u
LEFT JOIN posts p ON p.user_id = u.id
JOIN likes l ON l.post_id = p.id;
