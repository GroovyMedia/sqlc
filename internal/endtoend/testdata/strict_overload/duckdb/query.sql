-- name: JsonArrowBare :many
SELECT id FROM events WHERE meta->>@key = 'x';

-- name: RegexpGroupBare :many
SELECT id, regexp_extract(label, '([a-z]+)', @grp) AS word FROM events;
