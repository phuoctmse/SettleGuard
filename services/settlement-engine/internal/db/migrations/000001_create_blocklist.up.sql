CREATE TABLE blocklist (
    id          UUID PRIMARY KEY,
    entity_type TEXT NOT NULL CHECK (entity_type IN ('account', 'client')),
    entity_id   UUID NOT NULL,
    reason      TEXT NOT NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (entity_type, entity_id)
);
