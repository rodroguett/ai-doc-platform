-- +goose Up

-- Mientras una pasada procesa un trabajo, lo reserva hasta esta hora. Si la
-- pasada muere sin liberarlo, la reserva vence sola y otra puede tomarlo.
ALTER TABLE jobs ADD COLUMN locked_until timestamptz;

-- +goose Down

ALTER TABLE jobs DROP COLUMN locked_until;
