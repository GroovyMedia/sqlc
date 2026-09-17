CREATE TABLE buckets (
    bucket TEXT NOT NULL,
    channel INTEGER NOT NULL,
    is_in_use BOOLEAN NOT NULL,
    last_free_at TIMESTAMP,
    PRIMARY KEY (bucket, channel)
);
