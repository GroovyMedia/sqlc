CREATE TABLE stock (
  item    VARCHAR NOT NULL,
  day     DATE NOT NULL,
  qty     INTEGER NOT NULL,
  note    VARCHAR,
  updated TIMESTAMPTZ,
  PRIMARY KEY (item, day)
);

CREATE TABLE incoming (
  item VARCHAR NOT NULL,
  day  DATE NOT NULL,
  qty  INTEGER NOT NULL
);
