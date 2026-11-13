CREATE TABLE IF NOT EXISTS model_plaza_reference_prices (
    id BIGSERIAL PRIMARY KEY,
    platform VARCHAR(50) NOT NULL,
    model_name VARCHAR(255) NOT NULL,
    pricing JSONB NOT NULL,
    source_label VARCHAR(255) NOT NULL,
    source_url TEXT NOT NULL DEFAULT '',
    notes TEXT NOT NULL DEFAULT '',
    updated_by BIGINT REFERENCES users(id) ON DELETE SET NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_model_plaza_reference_prices_model
    ON model_plaza_reference_prices (LOWER(platform), LOWER(model_name));

CREATE INDEX IF NOT EXISTS idx_model_plaza_reference_prices_updated_at
    ON model_plaza_reference_prices (updated_at DESC, id DESC);
