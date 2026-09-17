-- name: Strptime :many
SELECT id FROM events WHERE strptime(label, $1) < seen;

-- name: JsonPath :many
SELECT id FROM events WHERE json_extract_string(meta, $1::VARCHAR) = 'x';

-- name: JsonArrow :many
SELECT id FROM events WHERE meta->>$1::VARCHAR = 'x';
