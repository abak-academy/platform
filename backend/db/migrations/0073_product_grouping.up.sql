CREATE TABLE IF NOT EXISTS product_grouping (
    id             UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name           TEXT NOT NULL,
    description    TEXT,
    status         TEXT NOT NULL DEFAULT 'draft' CHECK (status IN ('draft', 'published')),
    display_order  INT NOT NULL DEFAULT 0,
    created_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at     TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_product_grouping_status
    ON product_grouping (status);

ALTER TABLE product ADD COLUMN IF NOT EXISTS grouping_id UUID REFERENCES product_grouping (id) ON DELETE SET NULL;
ALTER TABLE product ADD COLUMN IF NOT EXISTS grouping_order INT NOT NULL DEFAULT 0;

CREATE INDEX IF NOT EXISTS idx_product_grouping_id
    ON product (grouping_id) WHERE grouping_id IS NOT NULL;
