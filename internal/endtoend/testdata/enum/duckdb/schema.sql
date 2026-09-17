CREATE TYPE mood AS ENUM ('sad', 'ok', 'happy');

CREATE SCHEMA hr;
CREATE TYPE hr.level AS ENUM ('junior', 'senior');

CREATE TABLE people (
  id INTEGER PRIMARY KEY,
  name VARCHAR NOT NULL,
  current_mood mood NOT NULL,
  last_mood mood,
  lvl hr.level
);
