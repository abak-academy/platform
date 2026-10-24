ALTER TABLE users DROP CONSTRAINT IF EXISTS users_role_check;
ALTER TABLE users ADD CONSTRAINT users_role_check
    CHECK (role IN ('student', 'admin_school', 'admin_exam', 'admin_store', 'super_admin', 'affiliator'));

CREATE TABLE IF NOT EXISTS affiliators (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id             UUID NOT NULL UNIQUE REFERENCES users (id),
    code                TEXT NOT NULL UNIQUE,
    status              TEXT NOT NULL DEFAULT 'pending_approval'
                        CHECK (status IN ('pending_approval', 'active', 'suspended', 'deactivated', 'rejected')),
    rejection_reason    TEXT,
    level               TEXT NOT NULL DEFAULT 'bronze' CHECK (level IN ('bronze', 'silver', 'gold')),
    level_locked_until  TIMESTAMPTZ,
    warn_quarters       INT NOT NULL DEFAULT 0,
    suspended_until     TIMESTAMPTZ,
    frozen_until        TIMESTAMPTZ,
    terms_version       TEXT NOT NULL,
    terms_accepted_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    bank_name           TEXT,
    bank_account_number TEXT,
    bank_account_holder TEXT,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_affiliators_status_level
    ON affiliators (status, level);

CREATE TABLE IF NOT EXISTS affiliate_bindings (
    id               UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    affiliator_id    UUID NOT NULL REFERENCES affiliators (id),
    student_id       UUID NOT NULL REFERENCES users (id),
    status           TEXT NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'ended')),
    source           TEXT NOT NULL CHECK (source IN ('link', 'manual_code', 'checkout_code')),
    started_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    ended_at         TIMESTAMPTZ,
    last_extended_at TIMESTAMPTZ,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at       TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX IF NOT EXISTS uq_affiliate_bindings_student_active
    ON affiliate_bindings (student_id)
    WHERE status = 'active';

CREATE INDEX IF NOT EXISTS idx_affiliate_bindings_affiliator
    ON affiliate_bindings (affiliator_id, status);

CREATE TABLE IF NOT EXISTS affiliate_commissions (
    id             UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    affiliator_id  UUID NOT NULL REFERENCES affiliators (id),
    student_id     UUID NOT NULL REFERENCES users (id),
    order_id       UUID NOT NULL REFERENCES orders (id),
    order_item_id  UUID NOT NULL UNIQUE REFERENCES order_item (id),
    product_type   TEXT NOT NULL,
    base_amount    NUMERIC(12, 2) NOT NULL,
    percentage     NUMERIC(5, 2) NOT NULL,
    level_snapshot TEXT NOT NULL,
    amount         NUMERIC(12, 2) NOT NULL,
    status         TEXT NOT NULL DEFAULT 'pending'
                   CHECK (status IN ('pending', 'available', 'paid', 'cancelled', 'held')),
    wait_until     TIMESTAMPTZ,
    paid_at        TIMESTAMPTZ,
    received_at    TIMESTAMPTZ,
    created_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at     TIMESTAMPTZ NOT NULL DEFAULT now()
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
    affiliator_id       UUID NOT NULL REFERENCES affiliators (id),
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
    affiliator_id   UUID NOT NULL REFERENCES affiliators (id),
    category        TEXT NOT NULL CHECK (category IN ('correction', 'offline', 'bonus', 'violation', 'other')),
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
    affiliator_id UUID NOT NULL REFERENCES affiliators (id),
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

CREATE TABLE IF NOT EXISTS affiliate_violations (
    id                UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    affiliator_id     UUID NOT NULL REFERENCES affiliators (id),
    category          TEXT NOT NULL,
    description       TEXT NOT NULL,
    sanction          TEXT NOT NULL
                      CHECK (sanction IN ('warning', 'commission_cancel', 'balance_freeze', 'level_change', 'suspend', 'deactivate')),
    suspension_until  TIMESTAMPTZ,
    status            TEXT NOT NULL DEFAULT 'open' CHECK (status IN ('open', 'resolved')),
    decided_by        UUID NOT NULL REFERENCES users (id),
    decided_at        TIMESTAMPTZ NOT NULL DEFAULT now(),
    appeal_text       TEXT,
    appeal_status     TEXT NOT NULL DEFAULT 'none' CHECK (appeal_status IN ('none', 'pending', 'upheld', 'rejected')),
    appeal_decided_by UUID REFERENCES users (id),
    appeal_decided_at TIMESTAMPTZ,
    created_at        TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_affiliate_violations_affiliator
    ON affiliate_violations (affiliator_id, created_at);

CREATE TABLE IF NOT EXISTS affiliate_level_history (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    affiliator_id UUID NOT NULL REFERENCES affiliators (id),
    quarter       TEXT NOT NULL,
    from_level    TEXT,
    to_level      TEXT NOT NULL,
    net_revenue   NUMERIC(12, 2) NOT NULL DEFAULT 0,
    source        TEXT NOT NULL CHECK (source IN ('auto', 'manual')),
    reason        TEXT,
    created_by    UUID REFERENCES users (id),
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_affiliate_level_history_affiliator
    ON affiliate_level_history (affiliator_id, created_at);

CREATE TABLE IF NOT EXISTS affiliate_settings (
    id                           INT PRIMARY KEY DEFAULT 1 CHECK (id = 1),
    cookie_days                  INT NOT NULL DEFAULT 30,
    binding_months               INT NOT NULL DEFAULT 3,
    min_extend_amount            NUMERIC(12, 2) NOT NULL DEFAULT 0,
    digital_wait_days            INT NOT NULL DEFAULT 7,
    physical_received_days       INT NOT NULL DEFAULT 7,
    physical_wait_days           INT NOT NULL DEFAULT 7,
    min_payout                   NUMERIC(12, 2) NOT NULL DEFAULT 100000,
    payout_day_of_month          INT,
    adjustment_threshold         NUMERIC(12, 2) NOT NULL DEFAULT 1000000,
    adjustment_threshold_enabled BOOLEAN NOT NULL DEFAULT false,
    freeze_max_days              INT NOT NULL DEFAULT 14,
    soon_end_days                INT NOT NULL DEFAULT 30,
    launch_date                  DATE,
    eval_day                     INT NOT NULL DEFAULT 8,
    silver_threshold             NUMERIC(12, 2) NOT NULL DEFAULT 5000000,
    gold_threshold               NUMERIC(12, 2) NOT NULL DEFAULT 15000000,
    hold_thresholds              JSONB NOT NULL DEFAULT '{}',
    daily_registration_cap       INT,
    updated_by                   UUID REFERENCES users (id),
    updated_at                   TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS affiliate_level_settings (
    level                  TEXT PRIMARY KEY CHECK (level IN ('bronze', 'silver', 'gold')),
    commission_pct         JSONB,
    digital_wait_days      INT,
    physical_received_days INT,
    physical_wait_days     INT,
    min_payout             NUMERIC(12, 2),
    payout_day_of_month    INT,
    rise_threshold         NUMERIC(12, 2),
    hold_threshold         NUMERIC(12, 2)
);

CREATE TABLE IF NOT EXISTS affiliate_product_overrides (
    product_id  UUID PRIMARY KEY REFERENCES product (id),
    percentage  NUMERIC(5, 2) NOT NULL CHECK (percentage >= 0 AND percentage <= 100),
    updated_by  UUID REFERENCES users (id),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS affiliate_clicks (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    affiliator_id UUID NOT NULL REFERENCES affiliators (id),
    code          TEXT NOT NULL,
    ip_hash       TEXT,
    user_agent    TEXT,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_affiliate_clicks_affiliator_day
    ON affiliate_clicks (affiliator_id, created_at);

CREATE TABLE IF NOT EXISTS affiliate_flags (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    affiliator_id UUID NOT NULL REFERENCES affiliators (id),
    rule          TEXT NOT NULL
                  CHECK (rule IN ('self_purchase', 'registration_spike', 'small_repeat_purchase', 'high_refund_ratio', 'similar_contacts')),
    payload       JSONB NOT NULL DEFAULT '{}',
    status        TEXT NOT NULL DEFAULT 'open' CHECK (status IN ('open', 'reviewed', 'valid', 'ignored')),
    note          TEXT,
    reviewed_by   UUID REFERENCES users (id),
    reviewed_at   TIMESTAMPTZ,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_affiliate_flags_status
    ON affiliate_flags (status, created_at);
