-- name: DaysBackBare :many
SELECT id FROM authors WHERE created_at >= now() - @days * INTERVAL '1 day';

-- name: ShiftedDate :many
SELECT id FROM authors WHERE created_at::DATE + @n > current_date;
