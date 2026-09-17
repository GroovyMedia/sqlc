CREATE TABLE users (
    id integer PRIMARY KEY,
    name text NOT NULL,
    age integer
);

CREATE TABLE posts (
    id integer PRIMARY KEY,
    user_id integer NOT NULL
);

CREATE TABLE baz.users (
    id integer PRIMARY KEY,
    name text NOT NULL
);

CREATE VIEW adults AS
SELECT id, name, age FROM users WHERE age >= 18;
