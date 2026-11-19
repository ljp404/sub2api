//go:build unit

package service

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestImagePriceCurrencyResolution(t *testing.T) {
	price, edit, rate, tier := 0.15, 0.3, 6.6693, 0.4
	config := &ChannelModelPricing{
		BillingMode: BillingModeImage, PerRequestPrice: &price, ImageEditPrice: &edit,
		ImagePriceCurrency: "CNY", ImageCNYPerUSD: &rate,
		Intervals: []PricingInterval{{TierLabel: "2K", PerRequestPrice: &tier}},
	}
	require.NoError(t, checkBillingModeRequirements(*config))
	resolved := (&ModelPricingResolver{}).resolveConfiguredPricing(config, "qwen-image-2.1", PricingSourceGroup)
	require.InDelta(t, 0.15/6.6693, resolved.DefaultPerRequestPrice, 1e-14)
	require.InDelta(t, 0.3/6.6693, config.ImagePriceUSD(*config.ImageEditPrice), 1e-14)
	require.Equal(t, tier, *resolved.RequestTiers[0].PerRequestPrice)
	require.InDelta(t, 0.3/6.6693, *calculateStatsCost(config, UsageTokens{}, 2), 1e-14)
	display := config.ImagePricingUSD()
	require.InDelta(t, 0.15/6.6693, *display.PerRequestPrice, 1e-14)
	require.Equal(t, "USD", display.ImagePriceCurrency)
	require.Nil(t, display.ImageCNYPerUSD)
	require.Equal(t, 0.15, *config.PerRequestPrice, "billing and display must preserve the configured CNY amount")
	require.Equal(t, rate, *config.ImageCNYPerUSD)
}

func TestImagePriceCurrencyLegacyAndValidation(t *testing.T) {
	price, rate, zero := 0.15, 7.0, 0.0
	legacy := ChannelModelPricing{BillingMode: BillingModeImage, PerRequestPrice: &price}
	require.NoError(t, checkBillingModeRequirements(legacy))
	require.Equal(t, price, legacy.ImagePriceUSD(price))
	require.Equal(t, 0.0, legacy.ImagePriceUSD(0))
	for _, config := range []ChannelModelPricing{
		{BillingMode: BillingModeImage, ImagePriceCurrency: "CNY"},
		{BillingMode: BillingModeImage, ImagePriceCurrency: "CNY", ImageCNYPerUSD: &zero},
		{BillingMode: BillingModeImage, ImagePriceCurrency: "EUR"},
		{BillingMode: BillingModeToken, ImagePriceCurrency: "CNY", ImageCNYPerUSD: &rate},
	} {
		require.Error(t, checkBillingModeRequirements(config))
	}
	legacy.ImagePriceCurrency, legacy.ImageCNYPerUSD = "CNY", &rate
	legacy.ImageEditPrice = &zero
	require.Equal(t, 0.0, *legacy.ImagePricingUSD().ImageEditPrice)
}
