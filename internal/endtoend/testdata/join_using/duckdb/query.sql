-- name: SelectJoinUsing :many
SELECT t1.fk, count(t2.id) AS n FROM t1 JOIN t2 USING (fk) GROUP BY fk;

-- name: LeftJoinUsing :many
SELECT fk, a, b FROM t1 LEFT JOIN t2 USING (fk);

-- name: RightJoinUsing :many
SELECT fk, a, b FROM t1 RIGHT JOIN t2 USING (fk);

-- name: FullJoinUsing :many
SELECT fk, a, b FROM t1 FULL JOIN t2 USING (fk);

-- name: StarJoinUsing :many
SELECT * FROM t1 LEFT JOIN t2 USING (fk);

-- name: BothSidesJoinUsing :many
SELECT t1.fk, t2.fk FROM t1 LEFT JOIN t2 USING (fk) WHERE fk = $1;

-- name: NaturalJoin :many
SELECT * FROM t1 NATURAL LEFT JOIN t2;
