ALTER TABLE channel_model_pricing
    ADD COLUMN IF NOT EXISTS image_edit_price NUMERIC(20, 10) CHECK (image_edit_price >= 0);

ALTER TABLE channel_account_stats_model_pricing
    ADD COLUMN IF NOT EXISTS image_edit_price NUMERIC(20, 10) CHECK (image_edit_price >= 0);
