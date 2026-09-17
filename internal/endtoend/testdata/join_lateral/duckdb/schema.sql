CREATE TABLE users (
  id   BIGINT NOT NULL,
  name TEXT   NOT NULL
);

CREATE TABLE posts (
  id      BIGINT NOT NULL,
  user_id BIGINT NOT NULL,
  title   TEXT   NOT NULL,
  body    TEXT
);
