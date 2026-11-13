-- Durable holds for opt-in parameter-priced synchronous image requests.
CREATE TABLE IF NOT EXISTS image_parameter_billing_holds (
    id VARCHAR(64) PRIMARY KEY,
    user_id BIGINT NOT NULL,
    api_key_id BIGINT NOT NULL,
    account_id BIGINT NOT NULL,
    quote JSONB NOT NULL,
    hold_amount NUMERIC(20,8) NOT NULL CHECK (hold_amount >= 0),
    state TEXT NOT NULL DEFAULT 'reserved'
        CHECK (state IN ('reserved','submitted','settlement_required','settled','released','uncertain')),
    settlement JSONB,
    last_error TEXT,
    next_retry_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS image_parameter_holds_pending
    ON image_parameter_billing_holds(next_retry_at)
    WHERE state='settlement_required';
