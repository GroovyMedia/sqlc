-- name: InsertPageIfMissing :exec
INSERT INTO pages (id, slug, hits, updated)
SELECT $1, $2, $3, now()
WHERE NOT EXISTS (SELECT 1 FROM pages WHERE slug = $2);

-- name: CopyPages :exec
INSERT INTO pages (id, slug, hits, updated)
SELECT id + 1000, slug, $hits, updated FROM pages WHERE id < $max_id;

-- name: InsertUserIfMissing :exec
INSERT INTO users (id, name, age)
SELECT $1, $2, $3
WHERE NOT EXISTS (SELECT 1 FROM users WHERE id = $1);

-- name: InsertAuditRow :exec
INSERT INTO audit (user_id, note, seen_at)
SELECT $1, $2, now();

-- name: InsertAuditFromLists :exec
INSERT INTO audit (user_id, note, seen_at)
SELECT unnest($1::BIGINT[]), unnest($2::TEXT[]), now();
