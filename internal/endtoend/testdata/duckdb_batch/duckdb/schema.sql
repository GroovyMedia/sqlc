CREATE TYPE status AS ENUM ('live', 'paused');

CREATE TABLE daily (
  day      DATE NOT NULL,
  source   VARCHAR NOT NULL,
  clicks   BIGINT,
  revenue  DOUBLE,
  tags     VARCHAR[],
  meta     JSON,
  status   status,
  seen_at  TIMESTAMPTZ,
  PRIMARY KEY (day, source)
);

CREATE SEQUENCE keywords_id_seq;
CREATE TABLE keywords (
  id            BIGINT DEFAULT nextval('keywords_id_seq') NOT NULL PRIMARY KEY,
  keyword       VARCHAR NOT NULL,
  keyword_lower VARCHAR NOT NULL UNIQUE,
  first_seen    TIMESTAMPTZ NOT NULL,
  last_seen     TIMESTAMPTZ NOT NULL
);

CREATE TABLE seen (
  ad_id BIGINT NOT NULL,
  day   DATE NOT NULL,
  ct    INTEGER NOT NULL,
  PRIMARY KEY (ad_id, day)
);

CREATE SEQUENCE log_id_seq;
CREATE TABLE log (
  id   BIGINT DEFAULT nextval('log_id_seq') NOT NULL,
  msg  VARCHAR NOT NULL
);
