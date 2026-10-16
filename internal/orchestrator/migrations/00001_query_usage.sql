-- +goose Up
-- Una fila por consulta atendida (ADR-0013). No guarda la pregunta, la
-- respuesta ni los pasajes.
CREATE TABLE query_usage (
    id            bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    occurred_at   timestamptz NOT NULL,
    trace_id      text        NOT NULL,
    collection    text        NOT NULL,
    provider      text        NOT NULL,
    model         text        NOT NULL,
    prompt_tokens integer     NOT NULL CHECK (prompt_tokens >= 0),
    output_tokens integer     NOT NULL CHECK (output_tokens >= 0),
    cache         text        NOT NULL CHECK (cache IN ('hit', 'miss', 'bypass')),
    outcome       text        NOT NULL CHECK (outcome IN ('answered', 'not_answered', 'degraded', 'no_sources')),
    -- Nulo si el modelo no tiene tarifa conocida: desconocido, no cero.
    estimated_cost_usd double precision CHECK (estimated_cost_usd >= 0)
);

-- El consumo se consulta por periodo.
CREATE INDEX query_usage_occurred_at ON query_usage (occurred_at);

-- +goose Down
DROP TABLE query_usage;
