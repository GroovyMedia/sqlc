-- name: ListAggregates :one
SELECT array_agg(name) AS names, list(id) AS ids FROM events;

-- name: Lists :many
SELECT
  list_distinct(tags) AS distinct_tags,
  [id] AS id_list,
  ints[1] AS first_int,
  tags[1:2] AS first_tags,
  list_transform(ints, lambda x: x + 1) AS next_ints,
  len(tags) AS tag_count,
  list_contains(tags, 'x') AS has_x
FROM events;

-- name: Json :many
SELECT doc -> 'k' AS k, doc ->> 'k' AS k_text, json_extract_string(doc, '$.n') AS n_text FROM events;

-- name: Aggregates :one
SELECT
  count(*) AS total,
  count(n) AS with_n,
  count(*) FILTER (WHERE ts > $since) AS recent,
  sum(n) AS sum_n,
  string_agg(name, ',' ORDER BY name) AS joined
FROM events;

-- name: Scalars :many
SELECT
  hash(n) AS h,
  nullif(n, 0) AS nonzero,
  ts - INTERVAL 1 DAY AS yesterday,
  n / 2 AS half,
  row_number() OVER (PARTITION BY ok ORDER BY ts) AS rn
FROM events;

-- name: ParamsFromSiblings :many
SELECT id FROM events
WHERE coalesce(n, $fallback) > 1
  AND (CASE WHEN ok THEN $when_ok ELSE n END) = 2
  AND greatest(amount, $floor) > 1
  AND list_contains(ints, $needle);
