-- name: Rollup :many
SELECT region, count(*) AS c FROM authors GROUP BY ROLLUP (region);

-- name: Cube :many
SELECT region, name, count(*) AS c FROM authors GROUP BY CUBE (region, name);

-- name: GroupingSets :many
SELECT region, name, count(*) AS c FROM authors GROUP BY GROUPING SETS ((region, name), (region), ());

-- name: PlainGroupBy :many
SELECT region, count(*) AS c FROM authors GROUP BY region;
