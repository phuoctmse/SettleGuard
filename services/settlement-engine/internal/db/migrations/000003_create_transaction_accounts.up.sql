CREATE TABLE transaction_accounts (
    transaction_id UUID NOT NULL REFERENCES transactions(id),
    account_id     UUID NOT NULL,
    created_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (transaction_id, account_id)
);

CREATE INDEX idx_transaction_accounts_account_created ON transaction_accounts (account_id, created_at);
