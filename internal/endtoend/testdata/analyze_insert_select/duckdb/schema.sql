CREATE TABLE pages (
  id      BIGINT  NOT NULL,
  slug    TEXT    NOT NULL,
  hits    INTEGER,
  updated TIMESTAMPTZ
);
