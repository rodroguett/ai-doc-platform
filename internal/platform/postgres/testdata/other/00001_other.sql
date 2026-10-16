-- +goose Up
CREATE TABLE other_probe (id integer PRIMARY KEY);

-- +goose Down
DROP TABLE other_probe;
