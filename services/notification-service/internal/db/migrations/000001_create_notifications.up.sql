CREATE TABLE notifications (
    id UUID PRIMARY KEY,
    type TEXT NOT NULL CHECK (type IN ('risk_hold', 'settlement_finalized')),
    subject_id UUID NOT NULL,
    payload JSONB NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (type, subject_id)
);
