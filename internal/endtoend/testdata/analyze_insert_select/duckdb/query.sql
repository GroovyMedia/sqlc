-- name: InsertPageIfMissing :exec
INSERT INTO pages (id, slug, hits, updated)
SELECT $1, $2, $3, now()
WHERE NOT EXISTS (SELECT 1 FROM pages WHERE slug = $2);

-- name: CopyPages :exec
INSERT INTO pages (id, slug, hits, updated)
SELECT id + 1000, slug, $hits, updated FROM pages WHERE id < $max_id;
