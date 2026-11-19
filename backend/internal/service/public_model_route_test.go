package service

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestPublicModelIDs(t *testing.T) {
	require.Equal(t, []string{"gpt-6.1-sol", "gemini-3-pro-image", "qwen/Qwen3", "models/gemini-pro"},
		PublicModelIDs([]string{
			"gpt-6.1-sol/hjwl", "gpt-6.1-sol/sdl", "gpt-6.1-sol",
			"gemini-3-pro-image/jczn", "qwen/Qwen3", "models/gemini-pro",
		}))
	require.Equal(t, "router/a/b", PublicModelID("router/a/b"))
	require.Equal(t, "name/not-a-channel-code", PublicModelID("name/not-a-channel-code"))
	require.Nil(t, PublicModelIDs(nil))
}

func publicModelTestPrice(input, output float64) *ChannelModelPricing {
	return &ChannelModelPricing{BillingMode: BillingModeToken, InputPrice: &input, OutputPrice: &output}
}

func TestChoosePublicModelCandidateCheapestAcrossEveryPrice(t *testing.T) {
	candidates := []publicModelCandidate{
		{model: "model/expensive", pricing: publicModelTestPrice(4, 12)},
		{model: "model/cheap", pricing: publicModelTestPrice(2, 6)},
		{model: "model/middle", pricing: publicModelTestPrice(3, 8)},
	}
	for i := 0; i < 25; i++ {
		require.Equal(t, "model/cheap", choosePublicModelCandidate(candidates).model)
	}
}

func TestPublicModelPricePreservesMissingPricesAndRuleStructure(t *testing.T) {
	a, ok := publicModelPriceVector(publicModelTestPrice(1, 2))
	require.True(t, ok)
	p := publicModelTestPrice(1, 2)
	p.OutputPrice = nil
	b, ok := publicModelPriceVector(p)
	require.True(t, ok)
	require.NotEqual(t, a.shape, b.shape)
	p = publicModelTestPrice(1, 2)
	p.BillingMode = BillingModePerRequest
	b, ok = publicModelPriceVector(p)
	require.True(t, ok)
	require.NotEqual(t, a.shape, b.shape)
	_, ok = publicModelPriceVector(nil)
	require.False(t, ok)
}

func TestPublicModelPriceIgnoresStorageIDsButKeepsTiers(t *testing.T) {
	a := publicModelTestPrice(1, 2)
	a.ID, a.ChannelID, a.Models = 1, 8, []string{"model/a"}
	b := publicModelTestPrice(2, 4)
	b.ID, b.ChannelID, b.Models = 2, 9, []string{"model/b"}
	av, ok := publicModelPriceVector(a)
	require.True(t, ok)
	bv, ok := publicModelPriceVector(b)
	require.True(t, ok)
	require.Equal(t, av.shape, bv.shape)
	b.Intervals = []PricingInterval{{MinTokens: 1000}}
	bv, ok = publicModelPriceVector(b)
	require.True(t, ok)
	require.NotEqual(t, av.shape, bv.shape)
}

func TestPublicModelParameterAndVideoPrices(t *testing.T) {
	t.Run("image parameters", func(t *testing.T) {
		candidates := []publicModelCandidate{
			{model: "image/a", pricing: &ChannelModelPricing{BillingMode: BillingModeParameter,
				ParameterPricing: &ParameterPricing{Version: 1, Unit: "image", BasePrice: "0.2",
					Factors: []ParameterPricingFactor{{Key: "resolution", Values: map[string]string{"1k": "1", "2k": "2"}}}}}},
			{model: "image/b", pricing: &ChannelModelPricing{BillingMode: BillingModeParameter,
				ParameterPricing: &ParameterPricing{Version: 1, Unit: "image", BasePrice: "0.1",
					Factors: []ParameterPricingFactor{{Key: "resolution", Values: map[string]string{"1k": "1", "2k": "2"}}}}}},
		}
		require.Equal(t, "image/b", choosePublicModelCandidate(candidates).model)
	})
	t.Run("video token", func(t *testing.T) {
		makeRule := func(price string) *ChannelModelPricing {
			return &ChannelModelPricing{BillingMode: BillingModeVideoToken,
				VideoTokenPricing: &VideoTokenPricing{Version: 1, FPS: 24, Divisor: 1024,
					Tiers: map[string]VideoTokenPriceTier{"720p": {WithoutVideo: price, WithVideo: price}}}}
		}
		require.Equal(t, "video/b", choosePublicModelCandidate([]publicModelCandidate{
			{model: "video/a", pricing: makeRule("7")}, {model: "video/b", pricing: makeRule("5")},
		}).model)
	})
}

func TestPublicModelRouteCapabilityAndAllowlist(t *testing.T) {
	account := &Account{Platform: PlatformOpenAI, Type: AccountTypeAPIKey,
		Credentials: map[string]any{"base_url": "https://relay.example"}}
	require.False(t, publicModelAccountSupportsPath(account, PlatformComposite, "/v1/messages"))
	account.Credentials["openai_capabilities"] = []any{"anthropic_messages", "gemini_images", "seedance"}
	require.True(t, publicModelAccountSupportsPath(account, PlatformComposite, "/v1/messages"))
	require.True(t, publicModelAccountSupportsPath(account, PlatformComposite, "/v1beta/models/gemini:generateContent"))
	require.True(t, publicModelAccountSupportsPath(account, PlatformComposite, "/api/v3/contents/generations/tasks"))
	require.False(t, publicModelAccountSupportsPath(account, PlatformGemini, "/v1/messages"))
	allowlist := GroupModelAllowlist{Enabled: true, Models: []string{"gpt-6.1-sol"}}
	require.True(t, allowlist.Allows("gpt-6.1-sol/hjwl"))
	require.Equal(t, []string{"gpt-6.1-sol"}, allowlist.FilterForListing([]string{"gpt-6.1-sol/hjwl"}))
	require.False(t, GroupModelAllowlist{Enabled: true, Models: []string{"gpt-6.1-sol/hjwl"}}.Allows("gpt-6.1-sol/sdl"))
}
