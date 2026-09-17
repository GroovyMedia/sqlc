-- name: UnsupportedExpression :one
SELECT id, tags[1] AS first_tag FROM users WHERE id = $1;

-- name: StarExclude :many
SELECT * EXCLUDE (tags) FROM users;

-- name: UnknownFunction :many
SELECT frobnicate(name) AS f FROM users;

-- name: UnknownFunctionParam :many
SELECT id FROM users WHERE frobnicate(name) = $1;

-- name: BareParam :one
SELECT $1 AS p;

-- name: NullColumn :one
SELECT NULL AS n FROM users;

-- name: ParamUsedTwice :many
SELECT p.id FROM posts p JOIN users u ON u.id = p.user_id WHERE p.id = $1 AND u.id = $1;

-- name: UntypedDerivedColumn :many
SELECT s.t FROM (SELECT tags[1] AS t FROM users) s;

-- name: UntypedCteColumn :many
WITH t(y) AS (SELECT tags[1] AS x FROM users) SELECT y FROM t;
