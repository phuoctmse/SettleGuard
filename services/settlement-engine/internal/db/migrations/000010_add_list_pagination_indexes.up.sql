CREATE INDEX idx_transactions_status_scored ON transactions (status, scored_at DESC, id DESC);
CREATE INDEX idx_settlements_created ON settlements (created_at DESC, id DESC);
