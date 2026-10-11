-- +goose Up
CREATE TABLE probe (id integer PRIMARY KEY);

-- +goose Down
DROP TABLE probe;
