package service

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func h3VideoTaskPricing() *VideoTaskPricing {
	return &VideoTaskPricing{
		Version: 1, Currency: "CNY", CNYPerUSD: "7.2",
		Price480PPerSecond: "0.04", Price768PPerSecond: "0.10",
		Upgrade2KPerSecond: "0.15", PromptPolishPricePerCall: "0.05",
		PromptPolishDefaultEnabled: true,
	}
}

func TestCalculateVideoTaskPricingH3Components(t *testing.T) {
	snapshot, err := CalculateVideoTaskPricing(h3VideoTaskPricing(), VideoTaskPricingInput{
		Resolution: "2K", DurationSeconds: "5", PromptPolish: true, Multiplier: "1",
	})
	require.NoError(t, err)
	require.Equal(t, "0.06944444", snapshot.BasePrice)
	require.Equal(t, "0.10416667", snapshot.UpgradePrice)
	require.Equal(t, "0.00694444", snapshot.PromptPolishPrice)
	require.Equal(t, "0.18055556", snapshot.ActualCost)
	require.Equal(t, "CNY", snapshot.SourceCurrency)
	require.Equal(t, "USD", snapshot.Currency)
}

func TestCalculateVideoTaskPricingRespectsPromptPolishOffAndMultiplier(t *testing.T) {
	snapshot, err := CalculateVideoTaskPricing(h3VideoTaskPricing(), VideoTaskPricingInput{
		Resolution: "768p", DurationSeconds: "5", PromptPolish: false, Multiplier: "1.5",
	})
	require.NoError(t, err)
	require.Equal(t, "0.06944444", snapshot.TotalCost)
	require.Equal(t, "0.10416667", snapshot.ActualCost)
	require.Equal(t, "0.00000000", snapshot.PromptPolishPrice)
}

func TestNormalizeVideoTaskResolution(t *testing.T) {
	for input, expected := range map[string]string{
		"480P": "480p",
		"768":  "768p",
		"2KP":  "2k",
	} {
		actual, err := normalizeVideoTaskResolution(input)
		require.NoError(t, err)
		require.Equal(t, expected, actual)
	}
	_, err := normalizeVideoTaskResolution("1080p")
	require.Error(t, err)
}

func TestPrepareVideoTaskQuoteUsesServiceDefaultOnlyWhenPromptPolishOmitted(t *testing.T) {
	rule := h3VideoTaskPricing()
	body := []byte(`{"model":"MiniMax-H3","duration":5,"resolution":"768p","metadata":{"prompt_polish":false}}`)
	quote, _, err := prepareVideoTaskQuote(rule, body, "1")
	require.NoError(t, err)
	require.False(t, quote.Input.PromptPolish)

	body = []byte(`{"model":"MiniMax-H3","duration":5,"resolution":"768p"}`)
	quote, _, err = prepareVideoTaskQuote(rule, body, "1")
	require.NoError(t, err)
	require.True(t, quote.Input.PromptPolish)
}

func TestPrepareVideoTaskQuoteSupportsV2PromptPolishOverride(t *testing.T) {
	rule := h3VideoTaskPricing()
	body := []byte(`{"model":"MiniMax-H3","duration":5,"resolution":"480p","prompt_polish":false}`)
	quote, _, err := prepareVideoTaskQuote(rule, body, "1")
	require.NoError(t, err)
	require.False(t, quote.Input.PromptPolish)
}
