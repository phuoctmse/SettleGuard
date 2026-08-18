CREATE TABLE settlement_transactions (
    settlement_id  UUID NOT NULL REFERENCES settlements(id),
    transaction_id UUID NOT NULL REFERENCES transactions(id),
    PRIMARY KEY (settlement_id, transaction_id)
);
