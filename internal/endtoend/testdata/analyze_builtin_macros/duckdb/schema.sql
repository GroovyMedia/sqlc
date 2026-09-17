CREATE TABLE runs (
  id            BIGINT    NOT NULL,
  reason        TEXT      NOT NULL,
  retry_reasons TEXT[],
  scores        INTEGER[] NOT NULL,
  weights       DOUBLE[],
  started       TIMESTAMP NOT NULL,
  n             INTEGER,
  ok            BOOLEAN   NOT NULL
);
