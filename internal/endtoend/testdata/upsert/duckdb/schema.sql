CREATE TABLE locations (
    id        INTEGER PRIMARY KEY,
    name      TEXT NOT NULL UNIQUE,
    address   TEXT NOT NULL,
    zip_code  INTEGER NOT NULL,
    note      TEXT,
    visits    INTEGER NOT NULL DEFAULT 0
);
