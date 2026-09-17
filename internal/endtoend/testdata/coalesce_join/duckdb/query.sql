-- name: GetBar :many
SELECT foo.*, COALESCE(bar.id, 0) AS bar_id, COALESCE(bar.note, '') AS note, COALESCE(bar.note, NULL) AS maybe_note
FROM foo
LEFT JOIN bar ON foo.id = bar.id;
