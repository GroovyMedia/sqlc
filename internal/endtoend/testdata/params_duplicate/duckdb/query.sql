/* name: SelectUserByID :many */
SELECT first_name from
users where (sqlc.arg(id) = id OR sqlc.arg(id) = 0);

/* name: SelectUserByName :many */
SELECT first_name
FROM users
WHERE first_name = sqlc.arg(name)
   OR last_name = @name;

/* name: SelectUserByDollarName :many */
SELECT first_name
FROM users
WHERE first_name = $name
   OR last_name = $name;

/* name: SelectUserQuestion :many */
SELECT first_name from
users where ($1 = id OR  $1 = 0);

/* name: SelectUserMixed :many */
SELECT first_name from
users where (? = id OR sqlc.arg(name) = first_name OR ? = 0);
