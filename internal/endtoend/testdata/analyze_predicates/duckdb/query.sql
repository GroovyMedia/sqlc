-- name: BooleanCombinations :many
SELECT id,
    NOT flag AS inactive,
    active AND flag AS both_,
    active OR flag AS either,
    NOT active AS not_active,
    NOT (bio = 'a') AS not_cmp,
    NOT EXISTS (SELECT 1 FROM authors a WHERE a.id > authors.id) AS is_last,
    NOT (bio IS NULL) AS has_bio,
    bio IS NULL AND active AS anonymous
FROM authors;

-- name: Membership :many
SELECT id,
    bio IN ('a', 'b') AS in_list,
    score IN (99, NULL) AS in_null_item,
    score IN (5, 7) AS in_scores,
    score IN (SELECT length(bio) FROM authors) AS in_lengths,
    score IN (SELECT score FROM authors) AS in_sub,
    bio = ANY(['a']) AS any_list,
    score > ALL(SELECT length(bio) FROM authors) AS above_all
FROM authors;

-- name: Ranges :many
SELECT id,
    born BETWEEN '2000-01-01' AND '2020-01-01' AS born_in_range,
    score BETWEEN 1 AND 10 AS score_in_range
FROM authors;
