-- name: Now :one
SELECT current_timestamp AS now, current_time AS t, localtimestamp AS lts, localtime AS lt, current_schema AS s, current_catalog AS c;

-- name: Today :many
SELECT id FROM runs WHERE started < CURRENT_DATE - INTERVAL 1 DAY;

-- name: ColumnWins :many
SELECT current_date FROM runs;
