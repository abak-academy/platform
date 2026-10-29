ALTER TABLE users DROP CONSTRAINT IF EXISTS users_role_check;
ALTER TABLE users ADD CONSTRAINT users_role_check
    CHECK (role IN ('student', 'admin_school', 'admin_exam', 'admin_store', 'super_admin', 'affiliator'));

ALTER TABLE orders ADD COLUMN IF NOT EXISTS affiliate_code TEXT;

ALTER TABLE product ADD COLUMN IF NOT EXISTS affiliate_commission JSONB;

CREATE TABLE IF NOT EXISTS affiliators (
    user_id                UUID PRIMARY KEY REFERENCES users (id),
    referral_code          TEXT UNIQUE,
    program_status         TEXT NOT NULL DEFAULT 'pending_approval'
                           CHECK (program_status IN ('pending_approval', 'active', 'suspended', 'deactivated', 'rejected')),
    rejection_reason       TEXT,
    level                  TEXT NOT NULL DEFAULT 'bronze' CHECK (level IN ('bronze', 'silver', 'gold')),
    missed_quarters        INT NOT NULL DEFAULT 0,
    last_evaluated_quarter TEXT,
    terms_version          TEXT NOT NULL,
    bank_account           JSONB
);

CREATE INDEX IF NOT EXISTS idx_affiliators_status_level
    ON affiliators (program_status, level);

CREATE TABLE IF NOT EXISTS affiliate_bindings (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    affiliator_id UUID NOT NULL REFERENCES affiliators (user_id),
    student_id    UUID NOT NULL REFERENCES users (id),
    status        TEXT NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'ended')),
    source        TEXT NOT NULL CHECK (source IN ('link', 'manual_code', 'checkout_code')),
    started_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    expires_at    TIMESTAMPTZ,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX IF NOT EXISTS uq_affiliate_bindings_student_active
    ON affiliate_bindings (student_id)
    WHERE status = 'active';

CREATE INDEX IF NOT EXISTS idx_affiliate_bindings_affiliator
    ON affiliate_bindings (affiliator_id, status);

-- The money tables below (commissions, payouts, adjustments, ledger entries)
-- are replaced by affiliate_transactions in the same migration file.
CREATE TABLE IF NOT EXISTS affiliate_commissions (
    id               UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    affiliator_id    UUID NOT NULL REFERENCES affiliators (user_id),
    student_id       UUID NOT NULL REFERENCES users (id),
    order_id         UUID NOT NULL REFERENCES orders (id),
    order_item_id    UUID NOT NULL UNIQUE REFERENCES order_item (id),
    product_type     TEXT NOT NULL,
    quantity         INT NOT NULL DEFAULT 1,
    base_amount      NUMERIC(12, 2) NOT NULL,
    commission_type  TEXT NOT NULL DEFAULT 'percent' CHECK (commission_type IN ('percent', 'fixed')),
    commission_value NUMERIC(12, 2) NOT NULL,
    level_snapshot   TEXT NOT NULL,
    amount           NUMERIC(12, 2) NOT NULL,
    status           TEXT NOT NULL DEFAULT 'pending'
                     CHECK (status IN ('pending', 'available', 'cancelled')),
    wait_until       TIMESTAMPTZ,
    paid_at          TIMESTAMPTZ,
    received_at      TIMESTAMPTZ,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at       TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_affiliate_commissions_affiliator
    ON affiliate_commissions (affiliator_id, status);

CREATE INDEX IF NOT EXISTS idx_affiliate_commissions_wait
    ON affiliate_commissions (status, wait_until)
    WHERE status = 'pending';

CREATE INDEX IF NOT EXISTS idx_affiliate_commissions_order
    ON affiliate_commissions (order_id);

CREATE TABLE IF NOT EXISTS affiliate_payouts (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    affiliator_id       UUID NOT NULL REFERENCES affiliators (user_id),
    amount              NUMERIC(12, 2) NOT NULL,
    status              TEXT NOT NULL DEFAULT 'requested'
                        CHECK (status IN ('requested', 'processing', 'paid', 'rejected')),
    bank_name           TEXT NOT NULL,
    bank_account_number TEXT NOT NULL,
    bank_account_holder TEXT NOT NULL,
    proof_url           TEXT,
    reject_reason       TEXT,
    processed_by        UUID REFERENCES users (id),
    processed_at        TIMESTAMPTZ,
    requested_at        TIMESTAMPTZ NOT NULL DEFAULT now(),
    created_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_affiliate_payouts_affiliator
    ON affiliate_payouts (affiliator_id, status);

CREATE TABLE IF NOT EXISTS affiliate_adjustments (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    affiliator_id   UUID NOT NULL REFERENCES affiliators (user_id),
    category        TEXT NOT NULL CHECK (category IN ('correction', 'offline', 'bonus', 'other')),
    amount          NUMERIC(12, 2) NOT NULL,
    reason          TEXT NOT NULL,
    note            TEXT,
    approval_status TEXT CHECK (approval_status IN ('pending', 'approved')),
    approver_id     UUID REFERENCES users (id),
    approved_at     TIMESTAMPTZ,
    created_by      UUID NOT NULL REFERENCES users (id),
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_affiliate_adjustments_affiliator
    ON affiliate_adjustments (affiliator_id, created_at);

CREATE TABLE IF NOT EXISTS affiliate_ledger_entries (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    affiliator_id UUID NOT NULL REFERENCES affiliators (user_id),
    entry_type    TEXT NOT NULL
                  CHECK (entry_type IN ('commission', 'refund_deduction', 'adjustment', 'payout', 'payout_reversal')),
    amount        NUMERIC(12, 2) NOT NULL,
    commission_id UUID REFERENCES affiliate_commissions (id),
    payout_id     UUID REFERENCES affiliate_payouts (id),
    adjustment_id UUID REFERENCES affiliate_adjustments (id),
    note          TEXT,
    actor_id      UUID REFERENCES users (id),
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_affiliate_ledger_affiliator
    ON affiliate_ledger_entries (affiliator_id, created_at);

CREATE UNIQUE INDEX IF NOT EXISTS uq_affiliate_ledger_commission
    ON affiliate_ledger_entries (commission_id, entry_type)
    WHERE commission_id IS NOT NULL;

CREATE FUNCTION affiliate_ledger_append_only() RETURNS trigger AS $$
BEGIN
    RAISE EXCEPTION 'affiliate_ledger_entries is append-only';
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER trg_affiliate_ledger_append_only
    BEFORE UPDATE OR DELETE ON affiliate_ledger_entries
    FOR EACH ROW EXECUTE FUNCTION affiliate_ledger_append_only();

CREATE TABLE IF NOT EXISTS affiliate_settings (
    id                     INT PRIMARY KEY DEFAULT 1 CHECK (id = 1),
    cookie_days            INT NOT NULL DEFAULT 30,
    binding_months         INT NOT NULL DEFAULT 3,
    min_extend_amount      NUMERIC(12, 2) NOT NULL DEFAULT 0,
    digital_wait_days      INT NOT NULL DEFAULT 7,
    physical_wait_days     INT NOT NULL DEFAULT 14,
    min_payout             NUMERIC(12, 2) NOT NULL DEFAULT 100000,
    payout_day_of_month    INT,
    launch_date            DATE,
    eval_day               INT NOT NULL DEFAULT 8,
    daily_registration_cap INT,
    levels                 JSONB NOT NULL DEFAULT '{}',
    updated_at             TIMESTAMPTZ NOT NULL DEFAULT now()
);
