//go:build unit

package service

import (
	"context"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/stretchr/testify/require"
)

func TestImageEditPriceOnlyOverridesImageEditEndpoint(t *testing.T) {
	editPrice, free, tierPrice, groupPrice := 0.15, 0.0, 0.4, 0.7
	for _, tt := range []struct {
		name     string
		endpoint bool
		edit     bool
		price    *float64
		group    *float64
		want     float64
	}{
		{"generation keeps tier", true, false, &editPrice, nil, 0.8},
		{"edit uses separate price", true, true, &editPrice, nil, 0.3},
		{"zero means free editing", true, true, &free, nil, 0},
		{"empty edit price keeps tier", true, true, nil, nil, 0.8},
		{"other endpoint ignores edit price", false, true, &editPrice, nil, 0.8},
		{"empty edit price keeps group override", true, true, nil, &groupPrice, 1.4},
		{"explicit edit price overrides legacy group price", true, true, &editPrice, &groupPrice, 0.3},
	} {
		t.Run(tt.name, func(t *testing.T) {
			const groupID int64 = 126
			resolver := newOpenAIImageChannelPricingResolverForTest(t, groupID, "qwen-image-2.1", 0.25)
			cache := resolver.channelService.cache.Load().(*channelCache)
			pricing := cache.pricingByGroupModel[channelModelKey{groupID: groupID, model: "qwen-image-2.1"}]
			pricing.ImageEditPrice = tt.price
			pricing.Intervals = []PricingInterval{{TierLabel: "2K", PerRequestPrice: &tierPrice}}
			svc := &OpenAIGatewayService{
				billingService: NewBillingService(&config.Config{}, nil),
				resolver:       resolver,
			}
			cost := svc.calculateOpenAIImageCost(context.Background(), "qwen-image-2.1",
				&APIKey{GroupID: i64p(groupID), Group: &Group{ID: groupID, ImagePrice2K: tt.group}},
				&OpenAIForwardResult{ImageCount: 2, ImageSize: "2K", ImageEndpoint: tt.endpoint, ImageEdit: tt.edit}, 2)
			require.InDelta(t, tt.want, cost.TotalCost, 1e-12)
			require.InDelta(t, tt.want*2, cost.ActualCost, 1e-12)
			require.Len(t, pricing.Intervals, 1, "cached generation tiers must remain unchanged")
		})
	}
}

func TestAccountStatsImageEditPrice(t *testing.T) {
	generation, edit, free := 0.1, 0.2, 0.0
	channel := &Channel{AccountStatsPricingRules: []AccountStatsPricingRule{{
		AccountIDs: []int64{1},
		Pricing: []ChannelModelPricing{{
			Platform: PlatformOpenAI, Models: []string{"qwen-image-2.1"},
			BillingMode: BillingModeImage, PerRequestPrice: &generation, ImageEditPrice: &edit,
		}},
	}}}
	require.InDelta(t, 0.2, *tryCustomRequestRules(channel, 1, 2, PlatformOpenAI, "qwen-image-2.1", UsageTokens{}, 2, "", false), 1e-12)
	require.InDelta(t, 0.4, *tryCustomRequestRules(channel, 1, 2, PlatformOpenAI, "qwen-image-2.1", UsageTokens{}, 2, "", true), 1e-12)
	channel.AccountStatsPricingRules[0].Pricing[0].ImageEditPrice = &free
	require.Equal(t, 0.0, *tryCustomRequestRules(channel, 1, 2, PlatformOpenAI, "qwen-image-2.1", UsageTokens{}, 2, "", true))
}
