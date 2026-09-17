-- name: Rollup :many
SELECT region, count(*) AS c FROM authors GROUP BY ROLLUP (region);

-- name: Cube :many
SELECT region, name, count(*) AS c FROM authors GROUP BY CUBE (region, name);

-- name: GroupingSets :many
SELECT region, count(*) AS c FROM authors GROUP BY GROUPING SETS ((region), ());

-- name: GroupingSetsShared :many
SELECT region, name, count(*) AS c FROM authors GROUP BY GROUPING SETS ((region, name), (region));

-- name: PlainAndRollup :many
SELECT region, name, count(*) AS c FROM authors GROUP BY region, ROLLUP (name);

-- name: RollupExpression :many
SELECT upper(region) AS r, coalesce(upper(region), 'all') AS r_or_all, count(*) AS c
FROM authors GROUP BY ROLLUP (upper(region));

-- name: RollupAlias :many
SELECT region AS r, count(*) AS c FROM authors GROUP BY ROLLUP (r);

-- name: RollupHaving :many
SELECT region, count(*) AS c FROM authors GROUP BY ROLLUP (region) HAVING region = $1 OR region IS NULL;

-- name: PlainGroupBy :many
SELECT region, count(*) AS c FROM authors GROUP BY region;
