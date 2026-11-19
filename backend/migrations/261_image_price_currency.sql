ALTER TABLE channel_model_pricing
    ADD COLUMN IF NOT EXISTS image_price_currency TEXT NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS image_cny_per_usd NUMERIC(20, 10);

ALTER TABLE channel_account_stats_model_pricing
    ADD COLUMN IF NOT EXISTS image_price_currency TEXT NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS image_cny_per_usd NUMERIC(20, 10);
