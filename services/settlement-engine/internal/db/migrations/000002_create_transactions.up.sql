CREATE TABLE transactions (
    id              UUID PRIMARY KEY,
    amount          BIGINT NOT NULL,
    score           INT NOT NULL,
    decision        TEXT NOT NULL CHECK (decision IN ('pass', 'hold')),
    status          TEXT NOT NULL CHECK (status IN ('pending_settlement', 'held', 'settled')),
    triggered_rules TEXT[] NOT NULL DEFAULT '{}',
    scored_at       TIMESTAMPTZ NOT NULL
);

CREATE INDEX idx_transactions_pending_settlement ON transactions (id) WHERE status = 'pending_settlement';
