CREATE TABLE authors (
    id INTEGER PRIMARY KEY,
    name VARCHAR NOT NULL,
    bio TEXT
);

CREATE TABLE editors (
    id INTEGER PRIMARY KEY,
    name VARCHAR NOT NULL,
    bio TEXT NOT NULL
);
