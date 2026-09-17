-- name: GetAd :one
SELECT * FROM ads WHERE id = $1;

-- The driver decodes a JSON column, so the message a query returns is
-- re-encoded and not byte for byte what was stored. The stored text comes
-- back as a string through a cast.
-- name: GetAdText :one
SELECT id, meta::VARCHAR AS meta_text FROM ads WHERE id = $1;

-- name: CreateAd :exec
INSERT INTO ads (id, meta) VALUES ($1, $2);
