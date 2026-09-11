CREATE TABLE api_keys (
    id          UUID PRIMARY KEY,
    client_id   UUID NOT NULL,
    key_hash    TEXT NOT NULL UNIQUE,
    label       TEXT NOT NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    revoked_at  TIMESTAMPTZ
);

CREATE INDEX idx_api_keys_client_id ON api_keys(client_id);
