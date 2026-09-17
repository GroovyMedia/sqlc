CREATE TABLE authors (
  id     INTEGER NOT NULL,
  name   TEXT    NOT NULL,
  bio    TEXT,
  born   DATE,
  active BOOLEAN NOT NULL,
  extra  JSON    NOT NULL,
  handle UNION(num INTEGER, str VARCHAR) NOT NULL
);
