SELECT id FROM users WHERE name = @FooBar OR bio = sqlc.arg(FooBar) OR name = @foobar;
