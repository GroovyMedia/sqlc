SELECT sqlc.embed(users), sqlc.embed(orders) FROM users JOIN orders ON orders.user_id = users.id;
