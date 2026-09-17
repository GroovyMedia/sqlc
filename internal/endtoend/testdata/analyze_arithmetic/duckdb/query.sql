-- name: MixedIntegers :many
SELECT
    qty / 2 AS half,
    qty // 2 AS half_floor,
    qty + stock AS plus_big,
    weight * qty AS weighted,
    small + flags AS widened,
    qty ** 2 AS squared,
    small & qty AS masked,
    price / 2 AS half_price,
    price + weight AS heavier
FROM items;
