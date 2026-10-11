-- +goose Up

-- La extensión se instala en public de forma explícita para que no dependa
-- del search_path de quien migra.
CREATE EXTENSION IF NOT EXISTS vector WITH SCHEMA public;

CREATE TABLE documents (
    id             uuid PRIMARY KEY,
    source         text NOT NULL,
    collection     text NOT NULL,
    status         text NOT NULL DEFAULT 'pending'
                   CHECK (status IN ('pending', 'processing', 'ready', 'failed')),
    chunk_count    integer NOT NULL DEFAULT 0 CHECK (chunk_count >= 0),
    metadata       jsonb NOT NULL DEFAULT '{}',
    content_sha256 text NOT NULL,
    created_at     timestamptz NOT NULL DEFAULT now(),

    -- El mismo contenido no se ingiere dos veces en una colección.
    CONSTRAINT documents_collection_content_key UNIQUE (collection, content_sha256)
);

-- Sostiene el listado paginado por cursor, del más reciente al más antiguo.
CREATE INDEX documents_collection_created_idx
    ON documents (collection, created_at DESC, id DESC);

CREATE TABLE jobs (
    id          uuid PRIMARY KEY,
    kind        text NOT NULL CHECK (kind IN ('ingest')),
    status      text NOT NULL DEFAULT 'queued'
                CHECK (status IN ('queued', 'running', 'succeeded', 'failed')),
    progress    integer NOT NULL DEFAULT 0 CHECK (progress BETWEEN 0 AND 100),
    document_id uuid NOT NULL REFERENCES documents (id) ON DELETE CASCADE,
    error       text,
    created_at  timestamptz NOT NULL DEFAULT now(),
    updated_at  timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX jobs_document_idx ON jobs (document_id);

-- +goose Down

DROP TABLE jobs;
DROP TABLE documents;
-- La extensión no se elimina: otras tablas pueden depender de ella.
