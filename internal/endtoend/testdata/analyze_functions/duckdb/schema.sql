CREATE TABLE events (
  id     BIGINT      NOT NULL,
  name   TEXT        NOT NULL,
  tags   TEXT[],
  ints   INTEGER[]   NOT NULL,
  ts     TIMESTAMPTZ NOT NULL,
  doc    JSON,
  n      INTEGER,
  amount DECIMAL(10,2),
  ok     BOOLEAN     NOT NULL
);
