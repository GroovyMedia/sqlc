-- name: GetThing :one
SELECT * FROM things WHERE id = $1;

-- name: ListThings :many
SELECT * FROM things;

-- name: CreateThing :exec
INSERT INTO things (id, b, nb, i1, ni1, i2, ni2, i4, ni4, i8, ni8, u1, nu1, u2, nu2, u4, nu4, u8, nu8, h, nh, uh, bn, f4, nf4, f8, nf8, d, nd, n, s, ns, t, bl, nbl, dt, ndt, tm, ntm, ts, nts, tstz, ntstz, tsns, iv, niv, u, nu, j, nj, e, ne, li, nli, ls, lu, lj, fixed, st, nst, m, nm)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19, $20, $21, $22, $23, $24, $25, $26, $27, $28, $29, $30, $31, $32, $33, $34, $35, $36, $37, $38, $39, $40, $41, $42, $43, $44, $45, $46, $47, $48, $49, $50, $51, $52, $53, $54, $55, $56, $57, $58, $59, $60, $61, $62);

-- name: FindThings :many
SELECT id, j, d, li FROM things
WHERE li = $1 AND j = $2 AND d = $3 AND u = $4 AND nj = $5;

-- name: SumThings :one
SELECT sum(i4) AS total, count(*) AS n, max(d) AS max_d, list(s) AS names FROM things;

-- name: CastThings :one
SELECT CAST($1 AS JSON) AS j, $2::INTEGER[] AS li, $3::DECIMAL(10,2) AS d, $4::UUID AS u, $5::HUGEINT AS h, $6::VARCHAR[] AS ls;

-- name: OneList :one
SELECT li FROM things WHERE id = $1;

-- name: OneJSON :one
SELECT nj FROM things WHERE id = $1;

-- name: OneDecimal :one
SELECT nd FROM things WHERE id = $1;
