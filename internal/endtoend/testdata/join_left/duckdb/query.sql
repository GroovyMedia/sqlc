-- name: GetMayors :many
SELECT
    user_id,
    mayors.full_name
FROM users
LEFT JOIN cities USING (city_id)
INNER JOIN mayors USING (mayor_id);

-- name: GetMayorsOptional :many
SELECT
    user_id,
    mayors.full_name
FROM users
LEFT JOIN cities USING (city_id)
LEFT JOIN mayors USING (mayor_id);

-- name: AllAuthors :many
SELECT  *
FROM    authors a
        LEFT JOIN authors p
            ON a.parent_id = p.id;

-- name: AllAuthorsAliases :many
SELECT  a.*, p.*
FROM    authors a
        LEFT JOIN authors p
            ON a.parent_id = p.id;

-- name: AuthorsWithParent :many
SELECT  a.id, a.name, p.id AS parent_id, p.name AS parent_name
FROM    authors a
        LEFT JOIN authors p
            ON a.parent_id = p.id
WHERE   p.name = $1;
