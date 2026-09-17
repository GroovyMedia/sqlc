-- name: InsertUserIfMissing :exec
INSERT INTO users (id, name, age)
SELECT $1, $2, $3
WHERE NOT EXISTS (SELECT 1 FROM users WHERE id = $1);

-- name: InsertAuditRow :exec
INSERT INTO audit (user_id, note, seen_at)
SELECT $1, $2, now();

-- name: InsertUsersFromValues :exec
INSERT INTO users (id, name, age)
SELECT * FROM (VALUES ($1, $2, $3), ($4, $5, $6));

-- name: InsertUsersFromNamedValues :exec
INSERT INTO users (id, name)
SELECT * FROM (VALUES ($1, $2)) AS v(id, name);

-- name: InsertAuditFromLists :exec
INSERT INTO audit (user_id, note, seen_at)
SELECT unnest($1::BIGINT[]), unnest($2::TEXT[]), now();
