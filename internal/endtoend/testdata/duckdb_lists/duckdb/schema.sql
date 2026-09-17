CREATE TABLE things (
  id     INTEGER PRIMARY KEY,
  li     INTEGER[] NOT NULL,
  ls     VARCHAR[],
  ld     DECIMAL(10,2)[]
);

CREATE TABLE parts (
  id       INTEGER PRIMARY KEY,
  thing_id INTEGER NOT NULL,
  name     VARCHAR
);
