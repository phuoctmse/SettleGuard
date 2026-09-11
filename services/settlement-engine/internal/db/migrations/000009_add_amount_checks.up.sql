ALTER TABLE transactions ADD CONSTRAINT transactions_amount_positive CHECK (amount > 0);
ALTER TABLE settlements ADD CONSTRAINT settlements_total_amount_positive CHECK (total_amount > 0);
