SELECT id FROM users WHERE name = sqlc.arg(name) AND age = sqlc.arg(age);
