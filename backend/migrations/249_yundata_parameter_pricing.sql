ALTER TABLE channel_model_pricing ADD COLUMN IF NOT EXISTS parameter_pricing JSONB;

-- Kept in the same transaction as usage_billing_dedup and balance settlement.
CREATE TABLE IF NOT EXISTS parameter_billing_snapshots (
    request_id VARCHAR(255) NOT NULL,
    api_key_id BIGINT NOT NULL,
    snapshot JSONB NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (request_id, api_key_id)
);
