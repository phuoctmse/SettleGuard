CREATE TABLE processed_ledger_transactions (
    transaction_id UUID PRIMARY KEY,
    processed_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);
