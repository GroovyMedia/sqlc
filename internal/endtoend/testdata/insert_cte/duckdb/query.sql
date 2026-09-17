-- name: InsertCode :one
WITH cc AS (
    INSERT INTO codes (hash, code, is_private)
    VALUES (@hash, @code, false)
    RETURNING hash
)
INSERT INTO test_codes (test_id, code_hash)
SELECT @test_id, hash FROM cc
RETURNING *;

-- name: AddTestCodes :exec
WITH input AS (SELECT DISTINCT unnest(@hashes::TEXT[]) AS hash)
INSERT INTO test_codes (test_id, code_hash)
SELECT @test_id, hash FROM input
ON CONFLICT (test_id, code_hash) DO NOTHING;

-- name: MoveTestCodes :many
WITH moved AS (
    DELETE FROM test_codes WHERE test_id = @old_test_id RETURNING code_hash
)
INSERT INTO test_codes (test_id, code_hash)
SELECT @new_test_id, code_hash FROM moved
RETURNING test_id, code_hash;
