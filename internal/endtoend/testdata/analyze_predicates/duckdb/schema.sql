CREATE TABLE authors (
    id INTEGER PRIMARY KEY,
    bio TEXT,
    flag BOOLEAN,
    active BOOLEAN NOT NULL,
    born DATE,
    score INTEGER NOT NULL
);
