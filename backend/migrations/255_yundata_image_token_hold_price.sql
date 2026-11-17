ALTER TABLE channel_model_pricing
    ADD COLUMN IF NOT EXISTS image_token_hold_price NUMERIC(20,8) NOT NULL DEFAULT 0.05;

COMMENT ON COLUMN channel_model_pricing.image_token_hold_price IS
    'Estimated per-image balance reservation for token-billed image generation, in USD';
