-- name: UpsertLocation :exec
INSERT INTO locations (id, name, address, zip_code)
VALUES (@id, @name, @address, @zip_code)
ON CONFLICT (id) DO UPDATE SET
  name = excluded.name,
  address = excluded.address,
  zip_code = excluded.zip_code;

-- name: UpsertLocationNote :exec
INSERT INTO locations (id, name, address, zip_code)
VALUES (@id, @name, @address, @zip_code)
ON CONFLICT (id) DO UPDATE SET note = sqlc.narg(note), visits = locations.visits + @step
WHERE visits < @max_visits;

-- name: UpsertLocationIfNearer :one
INSERT INTO locations (id, name, address, zip_code)
VALUES (@id, @name, @address, @zip_code)
ON CONFLICT (id) DO UPDATE SET address = excluded.address, note = @note
WHERE locations.zip_code <> excluded.zip_code
RETURNING id, note, visits;
