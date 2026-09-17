-- name: GetPerson :one
SELECT * FROM people WHERE id = $1;

-- name: ListByMood :many
SELECT id, name, current_mood FROM people WHERE current_mood = $1;

-- name: ListHappy :many
SELECT id, name FROM people WHERE current_mood = 'happy';

-- name: ListByLevel :many
SELECT id, name FROM people WHERE lvl = $1;

-- name: CreatePerson :exec
INSERT INTO people (id, name, current_mood, last_mood, lvl)
VALUES ($1, $2, $3, $4, $5);

-- name: CreateHappyPerson :exec
INSERT INTO people (id, name, current_mood) VALUES ($1, $2, 'happy');

-- name: SetLastMood :exec
UPDATE people SET last_mood = $2 WHERE id = $1;

-- name: CastMood :one
SELECT $1::mood AS m, $2::hr.level AS l;
