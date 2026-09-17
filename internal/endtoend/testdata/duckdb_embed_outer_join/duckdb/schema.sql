CREATE TABLE authors (
  id   INTEGER PRIMARY KEY,
  name VARCHAR NOT NULL,
  bio  VARCHAR
);

CREATE TABLE books (
  id        INTEGER PRIMARY KEY,
  author_id INTEGER NOT NULL,
  title     VARCHAR NOT NULL
);
