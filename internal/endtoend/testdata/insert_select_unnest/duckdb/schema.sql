CREATE TABLE users (
  id   BIGINT NOT NULL,
  name TEXT   NOT NULL,
  age  INTEGER
);

CREATE TABLE audit (
  user_id BIGINT    NOT NULL,
  note    TEXT      NOT NULL,
  seen_at TIMESTAMP NOT NULL
);
