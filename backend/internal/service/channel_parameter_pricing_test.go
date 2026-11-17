//go:build unit

package service

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestValidatePricingEntries_ParameterPricingPlatforms(t *testing.T) {
	rule := &ParameterPricing{Version: 1, Unit: "image", BasePrice: "0.05"}

	for _, platform := range []string{PlatformOpenAI, PlatformGemini} {
		t.Run(platform, func(t *testing.T) {
			err := validatePricingEntries([]ChannelModelPricing{{
				Platform:         platform,
				Models:           []string{"image-model"},
				BillingMode:      BillingModeParameter,
				ParameterPricing: rule,
			}})
			require.NoError(t, err)
		})
	}

	t.Run("other platform remains unsupported", func(t *testing.T) {
		err := validatePricingEntries([]ChannelModelPricing{{
			Platform:         PlatformAnthropic,
			Models:           []string{"image-model"},
			BillingMode:      BillingModeParameter,
			ParameterPricing: rule,
		}})
		require.ErrorContains(t, err, "parameter pricing requires OpenAI or Gemini")
	})
}

func TestValidatePricingEntries_VideoTokenStillRequiresOpenAI(t *testing.T) {
	err := validatePricingEntries([]ChannelModelPricing{{
		Platform:         PlatformGemini,
		Models:           []string{"video-model"},
		BillingMode:      BillingModeVideoToken,
		VideoTokenPricing: &VideoTokenPricing{
			Version: 1,
		},
	}})
	require.ErrorContains(t, err, "video token pricing requires OpenAI")
}
