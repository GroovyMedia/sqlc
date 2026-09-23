-- name: UpsertStock :exec
MERGE INTO stock
USING (SELECT @item::VARCHAR AS item, @day::DATE AS day) AS s
ON stock.item = s.item AND stock.day = s.day
WHEN MATCHED THEN UPDATE SET qty = @qty, note = sqlc.narg(note), updated = now()
WHEN NOT MATCHED THEN INSERT (item, day, qty, note) VALUES (s.item, s.day, @qty, sqlc.narg(note));

-- name: ApplyIncoming :many
MERGE INTO stock
USING incoming AS s
USING (item, day)
WHEN MATCHED AND s.qty = 0 THEN DELETE
WHEN MATCHED THEN UPDATE SET qty = stock.qty + s.qty
WHEN NOT MATCHED THEN INSERT BY NAME
WHEN NOT MATCHED BY SOURCE AND stock.day >= @since THEN DELETE
RETURNING merge_action, item, qty;

-- name: ReplaceDay :many
WITH src AS (SELECT item, day, qty FROM incoming WHERE day = $1)
MERGE INTO stock
USING src
USING (item, day)
WHEN MATCHED THEN UPDATE BY NAME
WHEN NOT MATCHED THEN INSERT BY NAME
RETURNING *;

-- name: SetNote :one
MERGE INTO stock
USING (VALUES (@item::VARCHAR, @day::DATE)) AS s(item, day)
ON stock.item = s.item AND stock.day = s.day
WHEN MATCHED THEN UPDATE SET note = @note
WHEN NOT MATCHED THEN ERROR 'no such row'
RETURNING merge_action, note;
