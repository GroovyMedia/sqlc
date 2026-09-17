-- name: GetThing :one
SELECT * FROM things WHERE id = $1;

-- name: CreateThing :exec
INSERT INTO things (id, price, discount, meta, text_meta, ref, parent)
VALUES ($1, $2, $3, $4, $5, $6, $7);
