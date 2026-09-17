-- name: GetThing :one
SELECT * FROM things WHERE id = $1;

-- name: OneInterval :one
SELECT iv FROM things WHERE id = $1;

-- name: SetInterval :exec
UPDATE things SET iv = $1, m = $2, h = $3, u = $4, f4 = $5 WHERE id = $6;
