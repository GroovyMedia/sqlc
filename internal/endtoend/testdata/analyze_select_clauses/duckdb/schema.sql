CREATE SEQUENCE users_id_seq START 1;

CREATE TABLE users (
  id   BIGINT NOT NULL,
  name TEXT   NOT NULL,
  age  INTEGER
);

CREATE INDEX users_name_idx ON users (name);

CREATE TABLE posts (
  id      BIGINT NOT NULL,
  user_id BIGINT NOT NULL,
  title   TEXT   NOT NULL,
  score   DOUBLE
);

ANALYZE;
