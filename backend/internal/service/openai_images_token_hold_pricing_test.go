package service

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCalculateImageTokenHoldQuoteUsesPerImageDefaultAndMultiplier(t *testing.T) {
	groupID := int64(7)
	key := &APIKey{
		User:    &User{ID: 42},
		GroupID: &groupID,
		Group:   &Group{RateMultiplier: 1.5},
	}

	quote, err := (&OpenAIGatewayService{}).calculateImageTokenHoldQuote(
		context.Background(), key, 2, DefaultImageTokenHoldPrice,
	)

	require.NoError(t, err)
	require.True(t, quote.HoldOnly)
	require.Equal(t, "0.05000000", quote.HoldPerImage)
	require.Equal(t, "1.5", quote.Input.Multiplier)
	require.Equal(t, "0.15000000", quote.ActualCost)
	require.Equal(t, "0.15000000", quote.TotalCost)
}
