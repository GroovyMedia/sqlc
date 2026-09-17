CREATE TABLE authors (
  id    BIGINT    NOT NULL,
  name  VARCHAR   NOT NULL,
  tags  VARCHAR[] NOT NULL,
  ints  INTEGER[] NOT NULL,
  small INTEGER   NOT NULL
);
