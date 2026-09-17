-- name: LambdaParamInBody :many
SELECT id FROM authors
WHERE list_has_any(list_transform(tags, lambda x: x || $suffix::VARCHAR), ['xa']) AND small = $small;

-- name: LambdaTwoParams :many
SELECT list_reduce(ints, lambda x, y: x + y + $acc::INTEGER) AS total FROM authors;

-- name: LambdaShadowsColumn :many
SELECT list_transform(tags, lambda name: name || '!') AS shouted, name FROM authors;

-- name: LambdaNested :many
SELECT list_filter(ints, lambda x: len(list_filter(ints, lambda y: y > x)) > 0) AS not_largest FROM authors;
