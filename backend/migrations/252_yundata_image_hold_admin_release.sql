ALTER TABLE image_parameter_billing_holds
    ADD COLUMN IF NOT EXISTS released_by BIGINT,
    ADD COLUMN IF NOT EXISTS release_reason TEXT,
    ADD COLUMN IF NOT EXISTS released_at TIMESTAMPTZ;
