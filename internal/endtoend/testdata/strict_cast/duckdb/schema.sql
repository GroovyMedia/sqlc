CREATE TABLE users (
  id   BIGINT NOT NULL,
  name TEXT   NOT NULL,
  age  INTEGER,
  tags INTEGER[]
);

CREATE TABLE posts (
  id      INTEGER NOT NULL,
  user_id BIGINT  NOT NULL
);
