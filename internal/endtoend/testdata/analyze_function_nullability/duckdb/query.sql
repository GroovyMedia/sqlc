-- name: GreatestLeast :many
SELECT
  id,
  greatest(bio, bio) AS g,
  least(born) AS l,
  concat_ws(bio, name, name) AS c
FROM authors;

-- name: WindowFrames :many
SELECT
  id,
  first_value(name) OVER (ORDER BY id ROWS BETWEEN 1 FOLLOWING AND 2 FOLLOWING) AS fv,
  last_value(name) OVER (ORDER BY id ROWS BETWEEN 1 FOLLOWING AND 2 FOLLOWING) AS lv,
  lag(name) OVER (ORDER BY id) AS previous
FROM authors;

-- name: Conversions :many
SELECT
  id,
  json_transform(extra, '"INTEGER"')::INTEGER AS n,
  from_json(extra, '"INTEGER"')::INTEGER AS m,
  union_extract(handle, 'str') AS str
FROM authors;
