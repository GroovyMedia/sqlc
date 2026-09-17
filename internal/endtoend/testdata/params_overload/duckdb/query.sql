-- name: DatePart :many
SELECT id, date_part(@part, seen) AS part FROM events;

-- name: Strptime :many
SELECT id, strptime(label, @format) AS parsed FROM events;

-- name: Quantile :one
SELECT quantile(score, @q) AS q FROM events;

-- name: RegexpGroup :many
SELECT id, regexp_extract(label, '([a-z]+)', @grp::INTEGER) AS word FROM events;

-- name: JsonArrow :many
SELECT id FROM events WHERE meta->>@key::VARCHAR = @value;
