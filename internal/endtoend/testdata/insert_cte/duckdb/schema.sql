CREATE TABLE codes (
    hash TEXT PRIMARY KEY,
    code TEXT,
    is_private BOOLEAN NOT NULL
);

CREATE TABLE test_codes (
    test_id INTEGER NOT NULL,
    code_hash TEXT NOT NULL,
    PRIMARY KEY (test_id, code_hash)
);
