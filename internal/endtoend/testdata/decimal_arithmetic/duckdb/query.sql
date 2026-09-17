-- name: MixedDecimal :one
SELECT
    price * 2 AS doubled,
    price + stock AS plus_big,
    2 * price AS twice,
    price - 1 AS less_one,
    price % 2 AS remainder,
    price / 2 AS half,
    price // 2 AS half_floor,
    price + weight AS plus_double
FROM items WHERE id = $1;
