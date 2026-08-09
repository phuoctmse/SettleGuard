CREATE TABLE accounts (
    id UUID PRIMARY KEY,
    client_id UUID NOT NULL REFERENCES client_businesses(id),
    external_ref TEXT,
    status TEXT NOT NULL CHECK (status IN ('active', 'suspended', 'closed')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_accounts_client_id ON accounts (client_id);
