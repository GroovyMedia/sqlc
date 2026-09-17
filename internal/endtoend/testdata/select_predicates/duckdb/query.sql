-- name: Predicates :many
SELECT id,
    NOT flag AS inactive,
    active AND flag AS both_,
    active OR flag AS either,
    NOT active AS not_active,
    bio IN ('a', 'b') AS in_list,
    score IN (5, 7) AS in_scores,
    score IN (SELECT length(bio) FROM authors) AS in_lengths,
    born BETWEEN '2000-01-01' AND '2020-01-01' AS born_in_range,
    score BETWEEN 1 AND 10 AS score_in_range,
    bio = ANY(['a']) AS any_list,
    score > ALL(SELECT length(bio) FROM authors) AS above_all,
    NOT EXISTS (SELECT 1 FROM authors a WHERE a.id > authors.id) AS is_last,
    NOT (bio IS NULL) AS has_bio
FROM authors;
