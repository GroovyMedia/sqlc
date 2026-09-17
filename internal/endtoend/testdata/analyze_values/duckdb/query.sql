-- name: ValuesWithNull :many
VALUES (1, 'a'), (NULL, NULL);

-- name: ValuesAll :many
VALUES (1, 'a'), (2, 'b');

-- name: ValuesDerived :many
SELECT v.a, v.b FROM (VALUES (1, 'a'), (NULL, NULL)) v(a, b);
