-- name: AtSignSpellings :many
SELECT id FROM authors WHERE name = @FooBar OR bio = sqlc.arg(FooBar) OR name = @foobar;

-- name: DollarNameSpellings :many
SELECT id FROM authors WHERE name = $FooBar OR name = $foobar;
