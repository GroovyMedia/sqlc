-- name: GetAuthor :one
SELECT * FROM authors WHERE id = $1;

-- name: ListAuthors :many
SELECT * FROM authors ORDER BY name LIMIT $limit OFFSET $offset;

-- name: CreateAuthor :one
INSERT INTO authors (name, bio, email, born, rating, tags, meta, external_id)
VALUES (@name, sqlc.narg('bio'), @email, @born, @rating, @tags, @meta, @external_id)
RETURNING *;

-- name: UpsertAuthor :one
INSERT INTO authors (id, name, bio, email)
VALUES ($1, $2, $3, $4)
ON CONFLICT (id) DO UPDATE SET name = EXCLUDED.name, bio = EXCLUDED.bio, updated_at = now()
RETURNING id, name, updated_at;

-- name: UpdateAuthorBio :execrows
UPDATE authors SET bio = sqlc.narg('bio'), updated_at = now() WHERE id = sqlc.arg('id');

-- name: DeleteAuthor :execresult
DELETE FROM authors WHERE id = ?;

-- name: DeleteInactiveAuthors :exec
DELETE FROM authors WHERE NOT active AND created_at < current_date - INTERVAL 1 YEAR;

-- name: CountBooks :one
SELECT count(*) FROM books WHERE author_id = $author_id;

-- name: AuthorsWithBookCount :many
SELECT a.id, a.name, count(b.id) AS book_count, sum(b.price) AS total_price, max(b.published) AS latest
FROM authors a
LEFT JOIN books b ON b.author_id = a.id
GROUP BY a.id, a.name
ORDER BY book_count DESC;

-- name: BooksWithAuthor :many
SELECT sqlc.embed(books), sqlc.embed(authors)
FROM books
JOIN authors ON authors.id = books.author_id
WHERE books.genre = $1;

-- name: BooksWithOptionalAuthor :many
SELECT sqlc.embed(books), authors.name AS author_name
FROM books
LEFT JOIN authors ON authors.id = books.author_id;

-- name: BooksByIDs :many
SELECT * FROM books WHERE id = ANY(sqlc.arg(ids));

-- name: AuthorsTagged :many
SELECT id, name FROM authors WHERE list_contains(tags, @tag);

-- name: AuthorTags :many
SELECT a.id, t.tag FROM authors a, unnest(a.tags) AS t(tag);

-- name: InsertBooksBulk :exec
INSERT INTO books (author_id, title, genre, price, isbn, extra)
SELECT unnest($1), unnest($2), unnest($3), unnest($4), unnest($5), unnest($6);

-- name: MonthlyRevenue :many
WITH monthly AS (
  SELECT date_trunc('month', published) AS month, genre, sum(price) AS revenue, count(*) AS n
  FROM books
  WHERE published IS NOT NULL
  GROUP BY 1, 2
)
SELECT month, genre, revenue, n,
       sum(revenue) OVER (PARTITION BY genre ORDER BY month) AS running,
       row_number() OVER (ORDER BY revenue DESC) AS rank
FROM monthly
ORDER BY month, genre;

-- name: RecentBooks :many
SELECT id, title, created_at FROM books WHERE created_at > now() - INTERVAL 7 DAY ORDER BY created_at DESC;

-- name: BookMeta :one
SELECT extra->>'publisher' AS publisher, extra->'dims' AS dims, list_contains(scores, 5) AS has_five, len(scores) AS n_scores
FROM books WHERE id = $1;

-- name: SearchBooks :many
SELECT id, title, price FROM books
WHERE (sqlc.narg('genre')::genre IS NULL OR genre = sqlc.narg('genre'))
  AND price BETWEEN sqlc.arg(min_price) AND sqlc.arg(max_price)
  AND title ILIKE '%' || @q || '%';

-- name: GetSample :one
SELECT * FROM samples WHERE id = $1;

-- name: ListSamples :many
SELECT * FROM samples;

-- name: CreateSample :exec
INSERT INTO samples VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19, $20, $21, $22, $23, $24, $25, $26, $27, $28, $29, $30, $31, $32, $33, $34, $35, $36, $37, $38, $39, $40, $41, $42, $43, $44, $45, $46, $47, $48, $49, $50, $51, $52, $53, $54, $55, $56, $57, $58, $59, $60, $61, $62, $63, $64, $65, $66, $67);
