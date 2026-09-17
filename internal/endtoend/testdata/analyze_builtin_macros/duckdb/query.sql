-- name: AppendReason :exec
UPDATE runs
SET retry_reasons = array_append(coalesce(retry_reasons, ARRAY[]::TEXT[]), $1)
WHERE id = $2;

-- name: ListMacros :many
SELECT
  list_append(scores, n) AS appended,
  array_prepend(reason, retry_reasons) AS prepended,
  array_to_string(scores, ',') AS joined,
  list_sum(scores) AS total,
  list_min(scores) AS lowest,
  list_count(scores) AS n_scores,
  list_avg(weights) AS mean_weight,
  split_part(reason, ':', 1) AS reason_head,
  fdiv(n, 2) AS halved,
  fmod(id, 3) AS remainder,
  date_add(started, INTERVAL 1 DAY) AS next_day,
  days_in_month(started) AS days
FROM runs;

-- name: FilterByMacros :many
SELECT id FROM runs
WHERE list_contains(array_append(retry_reasons, $reason), 'timeout')
  AND split_part(reason, $sep, 1) = 'timeout'
  AND fdiv(n, $divisor::DOUBLE) > 1;

-- name: AggregateMacros :one
SELECT json_group_array(reason) AS reasons, geomean(id) AS geo, wavg(n, id) AS weighted FROM runs;
