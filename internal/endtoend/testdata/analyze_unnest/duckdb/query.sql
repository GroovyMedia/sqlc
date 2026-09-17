-- name: UnnestColumns :many
SELECT id, unnest(tags) AS tag, unnest(ints) AS n FROM posts;

-- name: UnnestInFrom :many
SELECT p.id, t.tag, i.unnest, i.ordinality
FROM posts p, unnest(p.tags) AS t(tag), unnest(p.ints) WITH ORDINALITY AS i;

-- name: PruneMissing :exec
DELETE FROM posts WHERE NOT EXISTS (SELECT 1 FROM unnest($1::BIGINT[]) AS keep(id) WHERE keep.id = posts.id);

-- name: FilterByAny :many
SELECT id FROM posts WHERE id = ANY($ids);

-- name: FilterByUnnested :many
SELECT id FROM posts WHERE id IN (SELECT unnest($1::BIGINT[]));

-- name: MatchesAnyTag :many
SELECT id FROM posts WHERE $tag = ANY(tags);
