-- name: GetThing :one
SELECT * FROM things WHERE id = $1;

-- name: CreateThing :exec
INSERT INTO things (id, li, ls, lg, lu, ll) VALUES ($1, $2, $3, $4, $5, $6);

-- name: ThingsByRef :many
SELECT id FROM things WHERE lu[1] = ANY($1::UUID[]) ORDER BY id;

-- name: SetPrices :exec
UPDATE things SET ld = $1 WHERE id = $2;

-- name: ThingPrices :one
SELECT ld FROM things WHERE id = $1;

-- A list built in SQL may hold NULL elements: array_agg over the empty
-- side of a LEFT JOIN gives [NULL]. Scanning one into []int32 is an
-- error, the way a NULL scans into an int32 column.
-- name: ThingPartIDs :many
SELECT t.id, array_agg(p.id) AS part_ids
FROM things t
LEFT JOIN parts p ON p.thing_id = t.id
GROUP BY t.id
ORDER BY t.id;
