-- name: AuthorsWithFake :many
WITH q AS (
  SELECT authors.id, authors.name, fake.id AS fake_id, fake.bio AS fake_bio
  FROM authors
  LEFT JOIN fake ON authors.name = fake.name
)
SELECT *
FROM q AS c1
WHERE c1.name = $1 AND c1.fake_id = $2;

-- name: AuthorsWithFakeTwice :many
WITH q AS (
  SELECT authors.id, authors.name, fake.id AS fake_id
  FROM authors
  LEFT JOIN fake ON authors.name = fake.name
), r AS (
  SELECT q.id, q.fake_id, fake.bio
  FROM q
  LEFT JOIN fake ON fake.id = q.fake_id
)
SELECT id, fake_id, bio FROM r;
