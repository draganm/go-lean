CREATE TABLE IF NOT EXISTS blog (
    id INTEGER NOT NULL PRIMARY KEY,
    time DATETIME NOT NULL,
    description TEXT
);

INSERT INTO
    blog
values
    (1, DATE('now'), "foo");

INSERT INTO
    blog
values
    (2, DATE('now'), "bar");