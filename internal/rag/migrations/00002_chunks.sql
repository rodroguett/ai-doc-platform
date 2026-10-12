-- +goose Up

-- Un fragmento de un documento con su ubicación en el original, que es lo
-- que permite a una cita señalar el pasaje exacto.
CREATE TABLE chunks (
    id          uuid PRIMARY KEY,
    document_id uuid NOT NULL REFERENCES documents (id) ON DELETE CASCADE,
    position    integer NOT NULL CHECK (position >= 0),
    section     text NOT NULL DEFAULT '',
    heading     text NOT NULL DEFAULT '',
    page_start  integer NOT NULL CHECK (page_start >= 1),
    page_end    integer NOT NULL CHECK (page_end >= page_start),
    content     text NOT NULL,

    -- Nulo hasta que se generan los embeddings. La dimensión la fija el
    -- modelo (ADR-0008); cambiarla exige una migración y recalcular todo.
    embedding   public.vector(768),

    -- El modelo que produjo el vector: dos generaciones de vectores no son
    -- comparables y no deben mezclarse en una búsqueda.
    embedding_model text,

    created_at  timestamptz NOT NULL DEFAULT now(),

    CONSTRAINT chunks_document_position_key UNIQUE (document_id, position),
    CONSTRAINT chunks_embedding_has_model
        CHECK ((embedding IS NULL) = (embedding_model IS NULL))
);

-- Los fragmentos pendientes de un documento, en orden: es lo que recorre la
-- generación de embeddings y lo que permite reanudarla.
CREATE INDEX chunks_pending_idx ON chunks (document_id, position)
    WHERE embedding IS NULL;

-- +goose Down

DROP TABLE chunks;
