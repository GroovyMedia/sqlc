CREATE TABLE authors (
  id    INTEGER PRIMARY KEY,
  name  VARCHAR NOT NULL,
  score INTEGER NOT NULL
);

-- A schema statement sqlc has no node for is ignored.
CREATE MACRO twice(a) AS a * 2;
