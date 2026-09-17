-- name: InsertUsersFromValues :exec
INSERT INTO users (id, name, age)
SELECT * FROM (VALUES ($1, $2, $3), ($4, $5, $6));

-- name: InsertUsersFromNamedValues :exec
INSERT INTO users (id, name)
SELECT * FROM (VALUES ($1, $2)) AS v(id, name);
