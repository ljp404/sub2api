package service

import (
	"context"
	"encoding/json"
	"math"
	"math/rand/v2"
	"reflect"
	"sort"
	"strconv"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/domain"
	"github.com/shopspring/decimal"
)

// PublicModelID removes the channel code added by the account model importer.
// Provider namespaces and multi-component upstream resource IDs stay intact.
func PublicModelID(model string) string {
	model = strings.TrimSpace(model)
	base, code, ok := strings.Cut(model, "/")
	if !ok || base == "" || code == "" || len(code) > 32 {
		return model
	}
	switch strings.ToLower(base) {
	case "models", "openai", "anthropic", "google", "google-ai-studio", "meta-llama",
		"qwen", "deepseek-ai", "mistralai", "black-forest-labs", "stabilityai":
		return model
	}
	for _, ch := range code {
		if (ch < 'a' || ch > 'z') && (ch < 'A' || ch > 'Z') && (ch < '0' || ch > '9') {
			return model
		}
	}
	return base
}

func PublicModelIDs(models []string) []string {
	if models == nil {
		return nil
	}
	out := make([]string, 0, len(models))
	seen := make(map[string]bool, len(models))
	for _, model := range models {
		model = PublicModelID(model)
		key := strings.ToLower(model)
		if model != "" && !seen[key] {
			seen[key] = true
			out = append(out, model)
		}
	}
	return out
}

type publicModelCandidate struct {
	model    string
	platform string
	pricing  *ChannelModelPricing
}

// ResolvePublicModelRoute chooses a configured channel alias before the usual
// protocol dispatch, account scheduling and billing. Explicit suffixed requests
// retain their existing behavior.
func (s *GatewayService) ResolvePublicModelRoute(ctx context.Context, group *Group, model, path string) (string, string, error) {
	if s == nil || s.accountRepo == nil || group == nil || PublicModelID(model) != model {
		return model, "", nil
	}
	// Ordinary requests without a channel alias keep their original hot path.
	platforms := []string{group.Platform}
	if group.Platform == PlatformComposite {
		platforms = domain.ConcretePlatformIDs()
	}
	hasConfiguredAlias := false
	for _, platform := range platforms {
		for _, configured := range s.GetAvailableModels(ctx, &group.ID, platform) {
			if configured != model && strings.EqualFold(PublicModelID(configured), model) {
				hasConfiguredAlias = true
				break
			}
		}
		if hasConfiguredAlias {
			break
		}
	}
	if !hasConfiguredAlias {
		return model, "", nil
	}
	if group.Platform == PlatformComposite && s.compositeResolver != nil && s.compositeResolver.repo != nil {
		routes, err := s.compositeResolver.repo.ListByGroup(ctx, group.ID, false)
		if err != nil {
			return "", "", err
		}
		if _, matched := matchCompositeRoute(routes, model, publicModelEndpoint(path)); matched {
			return model, "", nil
		}
	}
	accounts, err := s.accountRepo.ListSchedulableByGroupID(ctx, group.ID)
	if err != nil {
		return "", "", err
	}
	byKey := make(map[string]publicModelCandidate)
	hasAlias := false
	for i := range accounts {
		account := &accounts[i]
		if !account.IsSchedulable() || !publicModelAccountSupportsPath(account, group.Platform, path) {
			continue
		}
		for configured, upstream := range account.GetModelMapping() {
			if strings.Contains(configured, "*") || strings.TrimSpace(upstream) == "" ||
				!strings.EqualFold(PublicModelID(configured), model) ||
				!group.ModelAllowlist.Allows(configured) ||
				!account.IsSchedulableForModelWithContext(ctx, account.GetMappedModel(configured)) {
				continue
			}
			hasAlias = hasAlias || configured != model
			candidate := publicModelCandidate{model: configured, platform: account.Platform}
			pricingCtx := WithResolvedTargetPlatform(ctx, account.Platform)
			mapping := ChannelMappingResult{MappedModel: configured, BillingModelSource: BillingModelSourceChannelMapped}
			if s.channelService != nil {
				mapping = s.channelService.ResolveChannelMapping(pricingCtx, group.ID, configured)
			}
			billingModel := mapping.MappedModel
			switch mapping.BillingModelSource {
			case BillingModelSourceRequested:
				billingModel = configured
			case BillingModelSourceUpstream:
				billingModel = account.GetMappedModel(mapping.MappedModel)
			case BillingModelSourceResponse:
				billingModel = ""
			}
			if billingModel != "" {
				candidate.pricing = matchGroupModelPricing(group, billingModel)
				groupPrice := candidate.pricing != nil
				if candidate.pricing == nil && s.channelService != nil {
					candidate.pricing = s.channelService.GetChannelModelPricing(pricingCtx, group.ID, billingModel)
				}
				if groupPrice &&
					(candidate.pricing.BillingMode == "" || candidate.pricing.BillingMode == BillingModeToken) {
					candidate.pricing.Intervals = nil
				}
			}
			key := account.Platform + "\x00" + configured
			if previous, exists := byKey[key]; exists && !samePublicModelPrice(previous.pricing, candidate.pricing) {
				// One alias can be scheduled to several accounts. If their
				// upstream-based prices differ, its price is not predictable.
				candidate.pricing = nil
			}
			byKey[key] = candidate
		}
	}
	if !hasAlias {
		return model, "", nil
	}
	candidates := make([]publicModelCandidate, 0, len(byKey))
	for _, candidate := range byKey {
		candidates = append(candidates, candidate)
	}
	sort.Slice(candidates, func(i, j int) bool {
		if candidates[i].model != candidates[j].model {
			return candidates[i].model < candidates[j].model
		}
		return candidates[i].platform < candidates[j].platform
	})
	selected := choosePublicModelCandidate(candidates)
	return selected.model, selected.platform, nil
}

func publicModelEndpoint(path string) string {
	switch {
	case strings.Contains(path, "/messages/count_tokens"):
		return CompositeRouteEndpointCountTokens
	case strings.Contains(path, "/messages"):
		return CompositeRouteEndpointMessages
	case strings.Contains(path, "/chat/completions"):
		return CompositeRouteEndpointChatCompletions
	case strings.Contains(path, "/responses"):
		return CompositeRouteEndpointResponses
	case strings.Contains(path, "/embeddings"):
		return CompositeRouteEndpointEmbeddings
	case strings.Contains(path, "/images/"):
		return CompositeRouteEndpointImages
	case strings.Contains(path, "/v1beta/"):
		return CompositeRouteEndpointGemini
	default:
		return CompositeRouteEndpointAny
	}
}

func publicModelAccountSupportsPath(account *Account, groupPlatform, path string) bool {
	if groupPlatform != PlatformComposite && account.Platform != groupPlatform &&
		!mixedListingAccountAllowed(groupPlatform, account) {
		return false
	}
	switch {
	case strings.Contains(path, "/contents/generations/tasks"):
		return account.Platform == PlatformOpenAI &&
			account.SupportsOpenAIEndpointCapability(OpenAIEndpointCapabilitySeedance)
	case strings.Contains(path, "video_generation"):
		return supportsMiniMaxVideoAccount(account)
	case strings.Contains(path, "/videos"):
		return account.Platform == PlatformGrok
	case strings.Contains(path, "/v1beta/"):
		return account.Platform == PlatformGemini || account.Platform == PlatformAntigravity ||
			account.SupportsOpenAIEndpointCapability(OpenAIEndpointCapabilityGeminiImages)
	case strings.Contains(path, "/messages"):
		return account.Platform == PlatformAnthropic || account.Platform == PlatformAntigravity ||
			account.SupportsOpenAIEndpointCapability(OpenAIEndpointCapabilityAnthropicMessages)
	case strings.Contains(path, "/images/"):
		return account.Platform == PlatformOpenAI || account.Platform == PlatformMiniMax || account.Platform == PlatformGrok
	case strings.Contains(path, "/chat/completions"):
		return account.SupportsOpenAIEndpointCapability(OpenAIEndpointCapabilityChatCompletions)
	case strings.Contains(path, "/embeddings"):
		return account.SupportsOpenAIEndpointCapability(OpenAIEndpointCapabilityEmbeddings)
	case strings.Contains(path, "/responses"):
		return account.SupportsOpenAIEndpointCapability(OpenAIEndpointCapabilityResponses)
	default:
		return false
	}
}

func samePublicModelPrice(a, b *ChannelModelPricing) bool {
	return reflect.DeepEqual(a, b)
}

type publicModelPrice struct {
	shape  string
	values map[string]decimal.Decimal
}

// A missing price is not zero. Compare every configured price dimension and
// retain the entire remaining rule as its comparability signature.
func publicModelPriceVector(pricing *ChannelModelPricing) (publicModelPrice, bool) {
	if pricing == nil {
		return publicModelPrice{}, false
	}
	raw, err := json.Marshal(pricing)
	if err != nil {
		return publicModelPrice{}, false
	}
	var rule map[string]any
	if json.Unmarshal(raw, &rule) != nil {
		return publicModelPrice{}, false
	}
	for _, key := range []string{"id", "channel_id", "platform", "models", "created_at", "updated_at", "image_token_hold_price"} {
		delete(rule, key)
	}
	if rule["billing_mode"] == "" {
		rule["billing_mode"] = string(BillingModeToken)
	}
	if intervals, ok := rule["intervals"].([]any); ok {
		for _, interval := range intervals {
			if row, ok := interval.(map[string]any); ok {
				for _, key := range []string{"id", "pricing_id", "created_at", "updated_at"} {
					delete(row, key)
				}
			}
		}
	}
	values := make(map[string]decimal.Decimal)
	valid := true
	var visit func(any, string)
	visit = func(node any, path string) {
		switch value := node.(type) {
		case map[string]any:
			for key, child := range value {
				childPath := path + "/" + key
				isPrice := strings.HasSuffix(key, "_price") || strings.HasSuffix(key, "_per_second") ||
					strings.HasSuffix(key, "_per_call") ||
					key == "without_video" || key == "with_video" ||
					strings.Contains(path, "/factors/") && strings.HasSuffix(path, "/values")
				if isPrice && child != nil {
					var amount decimal.Decimal
					var err error
					switch number := child.(type) {
					case float64:
						if math.IsNaN(number) || math.IsInf(number, 0) {
							valid = false
							continue
						}
						amount = decimal.NewFromFloat(number)
					case string:
						amount, err = decimal.NewFromString(number)
					default:
						valid = false
						continue
					}
					if err != nil || amount.IsNegative() {
						valid = false
						continue
					}
					values[childPath] = amount
					value[key] = "configured-price"
				} else {
					visit(child, childPath)
				}
			}
		case []any:
			for i, child := range value {
				visit(child, path+"/"+strconv.Itoa(i))
			}
		}
	}
	visit(rule, "")
	shape, err := json.Marshal(rule)
	return publicModelPrice{shape: string(shape), values: values}, valid && err == nil && len(values) > 0
}

func choosePublicModelCandidate(candidates []publicModelCandidate) publicModelCandidate {
	if len(candidates) == 0 {
		return publicModelCandidate{}
	}
	prices := make([]publicModelPrice, len(candidates))
	for i := range candidates {
		var ok bool
		prices[i], ok = publicModelPriceVector(candidates[i].pricing)
		if !ok || i > 0 && prices[i].shape != prices[0].shape {
			return candidates[rand.IntN(len(candidates))]
		}
	}
	cheapest := make([]int, 0)
	for i := range prices {
		dominates := true
		for j := range prices {
			for key, amount := range prices[i].values {
				if amount.GreaterThan(prices[j].values[key]) {
					dominates = false
					break
				}
			}
			if !dominates {
				break
			}
		}
		if dominates {
			cheapest = append(cheapest, i)
		}
	}
	if len(cheapest) > 0 {
		return candidates[cheapest[rand.IntN(len(cheapest))]]
	}
	return candidates[rand.IntN(len(candidates))]
}
