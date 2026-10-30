ALTER TABLE users DROP CONSTRAINT IF EXISTS users_role_check;
ALTER TABLE users ADD CONSTRAINT users_role_check
    CHECK (role IN ('student', 'admin_school', 'admin_exam', 'admin_store', 'super_admin', 'affiliator'));

ALTER TABLE orders ADD COLUMN IF NOT EXISTS affiliate_code TEXT;

ALTER TABLE product ADD COLUMN IF NOT EXISTS affiliate_commission JSONB;

ALTER TABLE promo_code ADD COLUMN IF NOT EXISTS affiliator_only BOOLEAN NOT NULL DEFAULT false;

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

CREATE TABLE IF NOT EXISTS affiliate_transactions (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    affiliator_id UUID NOT NULL REFERENCES affiliators (user_id),
    type          TEXT NOT NULL
                  CHECK (type IN ('commission', 'refund_deduction', 'payout', 'payout_reversal', 'adjustment')),
    status        TEXT NOT NULL,
    amount        NUMERIC(12, 2) NOT NULL,
    effective_at  TIMESTAMPTZ,
    parent_id     UUID REFERENCES affiliate_transactions (id),
    actor_id      UUID REFERENCES users (id),
    note          TEXT,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    student_id    UUID REFERENCES users (id),
    order_id      UUID REFERENCES orders (id),
    order_item_id UUID REFERENCES order_item (id),
    product_type  TEXT,
    quantity      INT,
    base_amount   NUMERIC(12, 2),
    release_at    TIMESTAMPTZ,
    order_paid_at TIMESTAMPTZ,
    detail        JSONB NOT NULL DEFAULT '{}',
    CONSTRAINT affiliate_transactions_status_check CHECK (
        (type = 'commission' AND status IN ('pending', 'available', 'cancelled'))
        OR (type = 'payout' AND status IN ('requested', 'processing', 'paid', 'rejected'))
        OR (type IN ('refund_deduction', 'payout_reversal', 'adjustment') AND status = 'posted')
    ),
    CONSTRAINT affiliate_transactions_amount_check CHECK (
        (type = 'commission' AND amount > 0)
        OR (type IN ('refund_deduction', 'payout') AND amount < 0)
        OR (type = 'payout_reversal' AND amount > 0)
        OR (type = 'adjustment' AND amount <> 0)
    ),
    CONSTRAINT affiliate_transactions_commission_check CHECK (
        type <> 'commission' OR (
            student_id IS NOT NULL AND order_id IS NOT NULL AND order_item_id IS NOT NULL
            AND product_type IS NOT NULL AND quantity IS NOT NULL AND base_amount IS NOT NULL
        )
    ),
    CONSTRAINT affiliate_transactions_parent_check CHECK (
        type NOT IN ('refund_deduction', 'payout_reversal') OR parent_id IS NOT NULL
    ),
    CONSTRAINT affiliate_transactions_effective_check CHECK (
        (type NOT IN ('refund_deduction', 'payout', 'payout_reversal', 'adjustment') OR effective_at IS NOT NULL)
        AND (type <> 'commission' OR status <> 'available' OR effective_at IS NOT NULL)
    )
);

CREATE UNIQUE INDEX IF NOT EXISTS uq_affiliate_transactions_commission_item
    ON affiliate_transactions (order_item_id)
    WHERE type = 'commission';

CREATE UNIQUE INDEX IF NOT EXISTS uq_affiliate_transactions_refund_parent
    ON affiliate_transactions (parent_id)
    WHERE type = 'refund_deduction';

CREATE UNIQUE INDEX IF NOT EXISTS uq_affiliate_transactions_reversal_parent
    ON affiliate_transactions (parent_id)
    WHERE type = 'payout_reversal';

CREATE UNIQUE INDEX IF NOT EXISTS uq_affiliate_transactions_payout_in_flight
    ON affiliate_transactions (affiliator_id)
    WHERE type = 'payout' AND status IN ('requested', 'processing');

CREATE INDEX IF NOT EXISTS idx_affiliate_transactions_ledger
    ON affiliate_transactions (affiliator_id, effective_at)
    WHERE effective_at IS NOT NULL;

CREATE INDEX IF NOT EXISTS idx_affiliate_transactions_affiliator
    ON affiliate_transactions (affiliator_id, type, status);

CREATE INDEX IF NOT EXISTS idx_affiliate_transactions_release
    ON affiliate_transactions (release_at)
    WHERE type = 'commission' AND status = 'pending';

CREATE INDEX IF NOT EXISTS idx_affiliate_transactions_order
    ON affiliate_transactions (order_id)
    WHERE order_id IS NOT NULL;

-- Money fields never change after insert; only status moves, along the allowed
-- transitions, so the balance of any past date stays reproducible.
CREATE FUNCTION affiliate_transactions_guard() RETURNS trigger AS $$
BEGIN
    IF TG_OP = 'DELETE' THEN
        RAISE EXCEPTION 'affiliate_transactions rows cannot be deleted';
    END IF;

    IF NEW.id <> OLD.id
       OR NEW.affiliator_id <> OLD.affiliator_id
       OR NEW.type <> OLD.type
       OR NEW.amount <> OLD.amount
       OR NEW.created_at <> OLD.created_at
       OR NEW.parent_id IS DISTINCT FROM OLD.parent_id
       OR NEW.student_id IS DISTINCT FROM OLD.student_id
       OR NEW.order_id IS DISTINCT FROM OLD.order_id
       OR NEW.order_item_id IS DISTINCT FROM OLD.order_item_id
       OR NEW.product_type IS DISTINCT FROM OLD.product_type
       OR NEW.quantity IS DISTINCT FROM OLD.quantity
       OR NEW.base_amount IS DISTINCT FROM OLD.base_amount
       OR NEW.order_paid_at IS DISTINCT FROM OLD.order_paid_at THEN
        RAISE EXCEPTION 'affiliate_transactions money fields are immutable';
    END IF;

    IF OLD.effective_at IS NOT NULL AND NEW.effective_at IS DISTINCT FROM OLD.effective_at THEN
        RAISE EXCEPTION 'affiliate_transactions.effective_at is set once';
    END IF;
    IF OLD.release_at IS NOT NULL AND NEW.release_at IS DISTINCT FROM OLD.release_at THEN
        RAISE EXCEPTION 'affiliate_transactions.release_at is set once';
    END IF;

    IF NEW.status <> OLD.status AND NOT (
        (OLD.type = 'commission' AND OLD.status = 'pending' AND NEW.status IN ('available', 'cancelled'))
        OR (OLD.type = 'commission' AND OLD.status = 'available' AND NEW.status = 'cancelled')
        OR (OLD.type = 'payout' AND OLD.status = 'requested' AND NEW.status IN ('processing', 'paid', 'rejected'))
        OR (OLD.type = 'payout' AND OLD.status = 'processing' AND NEW.status IN ('paid', 'rejected'))
    ) THEN
        RAISE EXCEPTION 'affiliate_transactions status cannot move from % to %', OLD.status, NEW.status;
    END IF;

    IF NEW.detail IS DISTINCT FROM OLD.detail
       AND (OLD.type <> 'payout' OR NEW.status = OLD.status) THEN
        RAISE EXCEPTION 'affiliate_transactions.detail is immutable';
    END IF;

    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER trg_affiliate_transactions_guard
    BEFORE UPDATE OR DELETE ON affiliate_transactions
    FOR EACH ROW EXECUTE FUNCTION affiliate_transactions_guard();

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
