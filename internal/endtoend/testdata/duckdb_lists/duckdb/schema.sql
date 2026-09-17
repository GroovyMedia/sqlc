CREATE TABLE things (
  id     INTEGER PRIMARY KEY,
  li     INTEGER[] NOT NULL,
  ls     VARCHAR[]
);

CREATE TABLE parts (
  id       INTEGER PRIMARY KEY,
  thing_id INTEGER NOT NULL,
  name     VARCHAR
);
