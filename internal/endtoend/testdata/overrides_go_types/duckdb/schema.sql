CREATE TABLE things (
  id        INTEGER PRIMARY KEY,
  price     DECIMAL(10,2) NOT NULL,
  discount  DECIMAL(10,2),
  meta      JSON,
  text_meta JSON,
  ref       UUID NOT NULL,
  parent    UUID,
  prices    DECIMAL(10,2)[]
);
