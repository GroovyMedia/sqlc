SELECT id FROM users WHERE a = sqlc.arg(x) OR b = @x OR c = sqlc.arg(y);
