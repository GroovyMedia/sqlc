CREATE TABLE users (
  user_id INTEGER NOT NULL,
  city_id INTEGER
);
CREATE TABLE cities (
  city_id  INTEGER NOT NULL,
  mayor_id INTEGER NOT NULL
);
CREATE TABLE mayors (
  mayor_id  INTEGER NOT NULL,
  full_name TEXT    NOT NULL
);

CREATE TABLE authors (
  id        INTEGER NOT NULL,
  name      TEXT    NOT NULL,
  parent_id INTEGER
);
