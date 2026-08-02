-- Refunding does not move money: PaymentClient has no refund method, so the
-- admin transfers manually and the button only records that it happened. This
-- column is the evidence that it did — the same shape as payment_proof_url,
-- for the same reason (invariant 6: a settlement recorded by a human must
-- never exist without its proof).
--
-- Nullable with no default: rows refunded before this shipped genuinely have
-- no proof, and backfilling a placeholder would fabricate evidence.
ALTER TABLE orders ADD COLUMN refund_proof_url TEXT;
