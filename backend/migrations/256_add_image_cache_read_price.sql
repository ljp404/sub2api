ALTER TABLE channel_model_pricing
    ADD COLUMN IF NOT EXISTS image_cache_read_price NUMERIC(20,12);

COMMENT ON COLUMN channel_model_pricing.image_cache_read_price IS
    'Cached image input token price in USD per token; NULL means use the model catalog price';
