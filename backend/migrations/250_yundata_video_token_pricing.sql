ALTER TABLE channel_model_pricing ADD COLUMN IF NOT EXISTS video_token_pricing JSONB;

-- No TTL: holds must survive Redis eviction, restarts, and a client that never polls.
CREATE TABLE IF NOT EXISTS video_token_billing_jobs (
    id VARCHAR(64) PRIMARY KEY,
    task_id VARCHAR(255) UNIQUE,
    user_id BIGINT NOT NULL,
    api_key_id BIGINT NOT NULL,
    account_id BIGINT NOT NULL,
    group_id BIGINT NOT NULL,
    model TEXT NOT NULL,
    upstream_model TEXT NOT NULL,
    quote JSONB NOT NULL,
    spec JSONB NOT NULL,
    hold_amount NUMERIC(20,8) NOT NULL CHECK (hold_amount >= 0),
    state TEXT NOT NULL DEFAULT 'reserved',
    last_error TEXT,
    next_poll_at TIMESTAMPTZ NOT NULL DEFAULT NOW() + INTERVAL '30 seconds',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CHECK (state IN ('reserved', 'submitted', 'settled', 'released', 'uncertain', 'measurement_required'))
);
CREATE INDEX IF NOT EXISTS video_token_jobs_pending ON video_token_billing_jobs(next_poll_at)
    WHERE state IN ('submitted', 'measurement_required');
CREATE TABLE IF NOT EXISTS video_token_billing_snapshots (
    request_id VARCHAR(255) NOT NULL,
    api_key_id BIGINT NOT NULL,
    snapshot JSONB NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (request_id, api_key_id)
);
