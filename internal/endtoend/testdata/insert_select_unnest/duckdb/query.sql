-- name: InsertAuditFromLists :exec
INSERT INTO audit (user_id, note, seen_at)
SELECT unnest($1), unnest($2), now();

-- name: InsertUsersFromLists :many
INSERT INTO users (id, name, age)
SELECT unnest($1), unnest($2), unnest($3)
RETURNING id;
