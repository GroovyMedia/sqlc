-- name: UnsupportedExpression :one
SELECT id, (#2)::TEXT AS second FROM users WHERE id = $1;

-- name: UnknownFunction :many
SELECT frobnicate(name)::TEXT AS f FROM users;

-- name: UnknownFunctionParam :many
SELECT id FROM users WHERE frobnicate(name) = $1::TEXT;

-- name: BareParam :one
SELECT $1::INTEGER AS p;

-- name: NullColumn :one
SELECT NULL::TEXT AS n FROM users;

-- name: ParamUsedTwice :many
SELECT p.id FROM posts p JOIN users u ON u.id = p.user_id WHERE p.id = $1::BIGINT AND u.id = $1;

-- name: ParamUsedTwiceAlike :many
SELECT p.id FROM posts p JOIN users u ON u.id = p.user_id WHERE p.user_id = $1 AND u.id = $1;
