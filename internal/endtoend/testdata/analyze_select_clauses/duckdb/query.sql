-- name: LatestPostPerUser :many
SELECT id, user_id, title, row_number() OVER (PARTITION BY user_id ORDER BY id DESC) AS rn
FROM posts
QUALIFY rn <= $1;

-- name: TopPostOfUser :many
SELECT id, title FROM posts
QUALIFY row_number() OVER (PARTITION BY user_id ORDER BY score DESC) = 1 AND user_id = $1;

-- name: PostsPerUser :many
SELECT user_id, count(*) AS n FROM posts WHERE user_id > $1 GROUP BY ALL ORDER BY ALL;

-- name: SampleUsers :many
SELECT id, name FROM users USING SAMPLE 10%;

-- name: SamplePosts :many
SELECT p.id FROM posts p TABLESAMPLE reservoir(50 ROWS) REPEATABLE (42) WHERE p.user_id = $1;

-- name: SampleJoin :many
SELECT u.name, p.title FROM users u JOIN posts p ON p.user_id = u.id USING SAMPLE 5;
