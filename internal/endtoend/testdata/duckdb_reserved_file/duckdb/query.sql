-- name: GetThing :one
SELECT * FROM things WHERE id = $1;
