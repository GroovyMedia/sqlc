CREATE TABLE events (
    id BIGINT PRIMARY KEY,
    meta JSON,
    seen TIMESTAMP NOT NULL,
    label TEXT NOT NULL,
    score DOUBLE NOT NULL
);
