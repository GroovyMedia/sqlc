-- name: UpsertBio :exec
INSERT INTO authors (id, name, score) VALUES ($1, $2, $3)
ON CONFLICT (id) DO UPDATE SET bio = $4, score = $5
WHERE score < $6;

-- name: UpsertExcluded :exec
INSERT INTO authors (id, name, score) VALUES ($1, $2, $3)
ON CONFLICT (id) DO UPDATE SET name = excluded.name, bio = $4
WHERE authors.score < excluded.score;
