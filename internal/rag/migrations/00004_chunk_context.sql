-- +goose Up

-- La introducción del ítem al que pertenece el fragmento, cuando la tiene.
-- Los fragmentos guardados antes de esta columna quedan sin ella hasta que
-- su documento se vuelva a ingerir.
ALTER TABLE chunks ADD COLUMN context text NOT NULL DEFAULT '';

-- +goose Down

ALTER TABLE chunks DROP COLUMN context;
