-- name: AddChannels :exec
WITH input AS (SELECT DISTINCT unnest($1::INTEGER[]) AS channel)
INSERT INTO buckets (bucket, channel, is_in_use)
SELECT $2::TEXT, channel, false FROM input
ON CONFLICT (bucket, channel) DO NOTHING;

-- name: CopyBucket :many
WITH src AS (SELECT channel, last_free_at FROM buckets WHERE bucket = 'old')
INSERT INTO buckets (bucket, channel, is_in_use, last_free_at)
SELECT $1, channel, false, last_free_at FROM src
RETURNING bucket, channel, last_free_at;

-- name: MoveBucket :many
WITH moved AS (DELETE FROM buckets WHERE bucket = 'old' RETURNING channel, last_free_at)
INSERT INTO buckets (bucket, channel, is_in_use, last_free_at)
SELECT $1, channel, false, last_free_at FROM moved
RETURNING bucket, channel;
