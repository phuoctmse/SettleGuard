CREATE TABLE settlements (
    id                UUID PRIMARY KEY,
    transaction_count INT NOT NULL,
    total_amount      BIGINT NOT NULL,
    created_at        TIMESTAMPTZ NOT NULL DEFAULT now()
);
