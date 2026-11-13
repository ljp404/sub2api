ALTER TABLE channel_model_pricing ADD COLUMN IF NOT EXISTS video_task_pricing JSONB;

CREATE TABLE IF NOT EXISTS video_task_billing_snapshots (
    request_id VARCHAR(255) NOT NULL,
    api_key_id BIGINT NOT NULL,
    snapshot JSONB NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (request_id, api_key_id)
);
