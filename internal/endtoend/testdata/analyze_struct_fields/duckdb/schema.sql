CREATE TABLE things (
  id     INTEGER NOT NULL,
  point  STRUCT(a INTEGER, b VARCHAR) NOT NULL,
  nested STRUCT(deep STRUCT(x DOUBLE), tags VARCHAR[]),
  points STRUCT(a INTEGER)[]
);
