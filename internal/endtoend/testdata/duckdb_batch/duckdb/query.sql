-- name: UpsertDaily :batchexec
INSERT INTO daily (day, source, clicks, revenue, tags, meta, status, seen_at)
VALUES (@day, @source, @clicks, @revenue, @tags, @meta, @status, @seen_at)
ON CONFLICT (day, source) DO UPDATE SET
    clicks  = EXCLUDED.clicks,
    revenue = EXCLUDED.revenue,
    tags    = EXCLUDED.tags,
    meta    = EXCLUDED.meta,
    status  = EXCLUDED.status,
    seen_at = EXCLUDED.seen_at;

-- name: UpsertKeywords :batchmany
INSERT INTO keywords (keyword, keyword_lower, first_seen, last_seen)
VALUES (@keyword, lower(@keyword), @seen, @seen)
ON CONFLICT (keyword_lower) DO UPDATE SET
    first_seen = LEAST(keywords.first_seen, EXCLUDED.first_seen),
    last_seen  = GREATEST(keywords.last_seen, EXCLUDED.last_seen)
RETURNING id;

-- name: UpsertKeyword :batchone
INSERT INTO keywords (keyword, keyword_lower, first_seen, last_seen)
VALUES (@keyword, lower(@keyword), @seen, @seen)
ON CONFLICT (keyword_lower) DO UPDATE SET last_seen = EXCLUDED.last_seen
RETURNING *;

-- name: AddSeen :batchexec
INSERT INTO seen (ad_id, day, ct) VALUES (@ad_id, @day, @ct)
ON CONFLICT (ad_id, day) DO UPDATE SET ct = seen.ct + EXCLUDED.ct;

-- name: MergeSeen :batchexec
MERGE INTO seen
USING (VALUES (@ad_id::BIGINT, @day::DATE, @ct::INTEGER)) AS s(ad_id, day, ct)
ON seen.ad_id = s.ad_id AND seen.day = s.day
WHEN MATCHED THEN UPDATE SET ct = s.ct
WHEN NOT MATCHED THEN INSERT (ad_id, day, ct) VALUES (s.ad_id, s.day, s.ct);

-- name: InsertLog :batchexec
INSERT INTO log (msg) VALUES (@msg);

-- name: InsertLogReturning :batchmany
INSERT INTO log (msg) VALUES (@msg) RETURNING id;

-- name: InsertLogLists :batchmany
INSERT INTO log (msg) SELECT unnest(@msgs::VARCHAR[]) RETURNING id;

-- name: BulkUpsertSeen :batchmany
INSERT INTO seen (ad_id, day, ct)
SELECT unnest(@ad_ids::BIGINT[]), unnest(@days::DATE[]), unnest(@cts::INTEGER[])
ON CONFLICT (ad_id, day) DO UPDATE SET ct = EXCLUDED.ct
RETURNING ad_id, ct;

-- name: BulkPauseDaily :batchexec
INSERT INTO daily (day, source, status, seen_at)
SELECT unnest(@days::DATE[]), unnest(@sources::VARCHAR[]), 'paused'::status, unnest(@seen::TIMESTAMPTZ[])
ON CONFLICT (day, source) DO UPDATE SET status = EXCLUDED.status, seen_at = EXCLUDED.seen_at;

-- name: BulkSetStatus :batchexec
INSERT INTO daily (day, source, status)
SELECT unnest(@days::DATE[]), unnest(@sources::VARCHAR[]), unnest(@statuses::TEXT[])::status
ON CONFLICT (day, source) DO UPDATE SET status = EXCLUDED.status;

-- name: InsertLogPayloads :batchmany
INSERT INTO log (msg, payload)
SELECT unnest(@msgs::VARCHAR[]), unnest(@payloads::JSON[])
RETURNING id, payload;

-- name: UpsertMetric :batchexec
INSERT INTO metrics (id, vals, meta) VALUES ($1, ($2::JSON)::DOUBLE[], $3)
ON CONFLICT (id) DO UPDATE SET vals = EXCLUDED.vals, meta = EXCLUDED.meta;

-- A float travels as text, so NaN and the infinities survive.
-- name: BulkSetRevenue :batchexec
INSERT INTO daily (day, source, revenue)
SELECT unnest(@days::DATE[]), unnest(@sources::VARCHAR[]), unnest(@revenues::DOUBLE[])
ON CONFLICT (day, source) DO UPDATE SET revenue = EXCLUDED.revenue;

-- A placeholder outside the VALUES row: runs row by row.
-- name: SetSeen :batchexec
INSERT INTO seen (ad_id, day, ct) VALUES (@ad_id, @day, @ct)
ON CONFLICT (ad_id, day) DO UPDATE SET ct = @ct;

-- ON compares expressions, not columns: runs row by row.
-- name: MergeKeywordLower :batchexec
MERGE INTO keywords t
USING (VALUES (@keyword::VARCHAR, @seen::TIMESTAMPTZ)) AS s(keyword, seen)
ON lower(t.keyword) = lower(s.keyword)
WHEN MATCHED THEN UPDATE SET last_seen = s.seen;

-- A nullable key cannot be read back by key: runs row by row.
-- name: HitTag :batchone
INSERT INTO tags (name, hits) VALUES (@name, 1)
ON CONFLICT (name) DO UPDATE SET hits = tags.hits + 1
RETURNING id;

-- A JSON[] would lose which elements are strings: runs row by row.
-- name: InsertLogPayloadList :batchexec
INSERT INTO log (msg, payloads) VALUES (@msg, @payloads);

-- WHEN NOT MATCHED BY SOURCE would undo earlier rounds: runs row by row.
-- name: MergeSeenPrune :batchexec
MERGE INTO seen
USING (VALUES (@ad_id::BIGINT, @day::DATE, @ct::INTEGER)) AS s(ad_id, day, ct)
ON seen.ad_id = s.ad_id AND seen.day = s.day
WHEN MATCHED THEN UPDATE SET ct = s.ct
WHEN NOT MATCHED BY SOURCE THEN DELETE;

-- Each row whose key is NULL gets a round of its own: one INSERT ... ON
-- CONFLICT keeps only one of several NULL-key rows.
-- name: CountTag :batchexec
INSERT INTO tags (name, hits) VALUES (@name, 1)
ON CONFLICT (name) DO UPDATE SET hits = tags.hits + 1;
