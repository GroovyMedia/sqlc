-- name: Fields :many
SELECT point.a, t.point.b AS b, nested.deep.x AS x, nested.tags AS tags, point['a'] AS a2, struct_extract(point, 'b') AS b2, points[1].a AS first_a
FROM things t;
