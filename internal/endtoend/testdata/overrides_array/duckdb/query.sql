-- An override on the element type gives a []int64 for an INTEGER[]
-- column. The list is bound through a Valuer, so DuckDB casts the
-- BIGINT[] the driver infers to INTEGER[] instead of the driver asserting
-- int32 on each element.
-- name: CreateThing :exec
INSERT INTO things (id, tags) VALUES ($1, $2);

-- name: CountTagged :one
SELECT count(*) FROM things WHERE tags = $1;

-- name: Tags :many
SELECT unnest($1::INTEGER[]) AS tag;
