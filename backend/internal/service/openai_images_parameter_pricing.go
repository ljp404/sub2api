package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/shopspring/decimal"
	"github.com/tidwall/gjson"
)

// Only short, top-level scalar parameters are kept. Prompts, files and secrets
// never enter a price snapshot.
func captureImagePriceParameter(req *OpenAIImagesRequest, key, value string) {
	if !allowedParameterKey(key) {
		return
	}
	if req.PriceDimensions == nil {
		req.PriceDimensions = make(map[string]string)
	}
	value = normalizeParameterValue(value)
	if len(value) > 64 {
		if req.InvalidPriceDimensions == nil {
			req.InvalidPriceDimensions = make(map[string]bool)
		}
		req.InvalidPriceDimensions[key] = true
		value = ""
	}
	req.PriceDimensions[key] = value
}

func captureImageJSONPriceParameters(req *OpenAIImagesRequest, body []byte) {
	gjson.ParseBytes(body).ForEach(func(key, value gjson.Result) bool {
		if !allowedParameterKey(key.String()) {
			return true
		}
		_, duplicate := req.PriceDimensions[key.String()]
		if value.Type != gjson.JSON && value.Type != gjson.Null {
			captureImagePriceParameter(req, key.String(), value.String())
		} else if value.Type == gjson.Null && key.String() == "variant" {
			captureImagePriceParameter(req, key.String(), "")
		}
		if duplicate || value.Type == gjson.JSON || (value.Type == gjson.Null && key.String() != "variant") ||
			(key.String() == "size" && value.Type != gjson.String) {
			if req.InvalidPriceDimensions == nil {
				req.InvalidPriceDimensions = make(map[string]bool)
			}
			req.InvalidPriceDimensions[key.String()] = true
		}
		return true
	})
}

func imagePriceDimensions(req *OpenAIImagesRequest) map[string]string {
	dimensions := make(map[string]string, len(req.PriceDimensions)+2)
	for k, v := range req.PriceDimensions {
		dimensions[k] = v
	}
	// size is an alias only when the explicit dimension is absent. Pixel sizes
	// provide an exact aspect ratio; do not guess a tier for "auto" or missing size.
	size := normalizeParameterValue(req.Size)
	if dimensions["resolution"] == "" {
		switch size {
		case "1k", "2k", "4k":
			dimensions["resolution"] = size
		}
		if dimensions["image_size"] != "" {
			dimensions["resolution"] = dimensions["image_size"]
		}
	}
	if dimensions["aspect_ratio"] == "" {
		if size == "auto" {
			dimensions["aspect_ratio"] = "auto"
		} else if parts := strings.Split(size, "x"); len(parts) == 2 {
			w, e1 := strconv.Atoi(parts[0])
			h, e2 := strconv.Atoi(parts[1])
			if e1 == nil && e2 == nil && w > 0 && h > 0 && w <= 65536 && h <= 65536 {
				a, b := w, h
				for b != 0 {
					a, b = b, a%b
				}
				dimensions["aspect_ratio"] = fmt.Sprintf("%d:%d", w/a, h/a)
			}
		}
	}
	return dimensions
}

// PrepareImageParameterPricing runs before the upstream call and freezes the
// matching rule for this attempt. Group model rules precede channel rules;
// explicit channel rules still precede legacy group image defaults.
func (s *OpenAIGatewayService) PrepareImageParameterPricing(ctx context.Context, key *APIKey, req *OpenAIImagesRequest, account *Account, mapping ChannelMappingResult, requested string) error {
	req.ParameterQuote = nil
	req.SizeTier = normalizeOpenAIImageSizeTier(req.Size)
	if key == nil || s.resolver == nil {
		return nil
	}
	mapped := firstNonEmpty(mapping.MappedModel, req.Model)
	upstream := account.GetMappedModel(mapped)
	model := upstream
	switch mapping.BillingModelSource {
	case BillingModelSourceRequested:
		model = requested
	case BillingModelSourceChannelMapped:
		model = mapped
	}
	candidates := usageBillingModelCandidates(model, mapped, requested, upstream, req.Model)
	for _, candidate := range candidates {
		resolved := s.resolveOpenAIChannelPricing(ctx, candidate, key)
		if resolved == nil {
			continue
		}
		if resolved.Mode == BillingModeToken {
			if key.Group != nil && (key.Group.IsSubscriptionType() ||
				(s.cfg != nil && s.cfg.RunMode == config.RunModeSimple)) {
				return nil
			}
			holdPrice := serviceImageTokenHoldPrice(resolved.channelPricing)
			quote, err := s.calculateImageTokenHoldQuote(ctx, key, req.N, holdPrice)
			if err != nil {
				return err
			}
			req.ParameterQuote = quote
			return nil
		}
		if resolved.Mode != BillingModeParameter {
			return nil
		}
		if account.Type != AccountTypeAPIKey {
			return fmt.Errorf("parameter pricing requires an API-key Images account")
		}
		if resolved.channelPricing == nil {
			return fmt.Errorf("parameter pricing rule is missing")
		}
		rule := resolved.channelPricing.ParameterPricing
		if err := ValidateParameterPricing(rule); err != nil {
			return err
		}
		for _, factor := range rule.Factors {
			if req.InvalidPriceDimensions[factor.Key] {
				return fmt.Errorf("invalid or duplicate billing parameter %s", factor.Key)
			}
		}
		dimensions, err := imageSizeMappedDimensions(req, rule)
		if err != nil {
			return err
		}
		quote, err := s.calculateParameterQuote(ctx, key, rule, dimensions, req.N)
		if err != nil {
			return err
		}
		req.ParameterQuote = quote
		if resolution := quote.Input.Dimensions["resolution"]; resolution != "" {
			req.SizeTier = normalizeOpenAIImageSizeTier(resolution)
		}
		return nil
	}
	return nil
}

func serviceImageTokenHoldPrice(pricing *ChannelModelPricing) float64 {
	if pricing != nil && pricing.ImageTokenHoldPrice != nil {
		return *pricing.ImageTokenHoldPrice
	}
	return DefaultImageTokenHoldPrice
}

func (s *OpenAIGatewayService) calculateImageTokenHoldQuote(
	ctx context.Context,
	key *APIKey,
	quantity int,
	unitPrice float64,
) (*ParameterPricingSnapshot, error) {
	if quantity < 1 || unitPrice < 0 {
		return nil, fmt.Errorf("invalid image token reservation amount")
	}
	rate := 1.0
	if key.Group != nil {
		rate = key.Group.RateMultiplier
		if key.GroupID != nil && key.User != nil {
			rate = s.ResolveUserGroupRateMultiplier(ctx, key.User.ID, *key.GroupID, rate)
		}
	}
	rate, _ = computePeakAwareMultipliers(key, rate, time.Now())
	multiplier := decimal.NewFromFloat(rate)
	perImage := decimal.NewFromFloat(unitPrice)
	amount := perImage.Mul(decimal.NewFromInt(int64(quantity))).Mul(multiplier).Round(8)
	normalizedUnitPrice := perImage.StringFixed(8)
	normalizedAmount := amount.StringFixed(8)
	ruleHashBytes := sha256.Sum256([]byte(fmt.Sprintf("%s|%d|%s", normalizedUnitPrice, quantity, multiplier.String())))
	return &ParameterPricingSnapshot{
		RuleHash:     hex.EncodeToString(ruleHashBytes[:]),
		HoldOnly:     true,
		HoldPerImage: normalizedUnitPrice,
		Input: ParameterPricingInput{
			Quantity:   quantity,
			Multiplier: multiplier.String(),
		},
		UnitPrice:  normalizedUnitPrice,
		TotalCost:  normalizedAmount,
		ActualCost: normalizedAmount,
	}, nil
}

// PrepareParameterPricing adapts protocol-specific image parameters to the
// existing shared parameter-pricing calculator. A nil quote means the model
// does not use parameter pricing.
func (s *OpenAIGatewayService) PrepareParameterPricing(
	ctx context.Context,
	key *APIKey,
	models []string,
	dimensions map[string]string,
	quantity int,
) (*ParameterPricingSnapshot, error) {
	if s == nil || key == nil || s.resolver == nil {
		return nil, nil
	}
	for _, model := range models {
		model = strings.TrimSpace(model)
		if model == "" {
			continue
		}
		resolved := s.resolveOpenAIChannelPricing(ctx, model, key)
		if resolved == nil {
			continue
		}
		if resolved.Mode != BillingModeParameter {
			return nil, nil
		}
		if resolved.channelPricing == nil {
			return nil, fmt.Errorf("parameter pricing rule is missing")
		}
		rule := resolved.channelPricing.ParameterPricing
		if err := ValidateParameterPricing(rule); err != nil {
			return nil, err
		}
		return s.calculateParameterQuote(ctx, key, rule, dimensions, quantity)
	}
	return nil, nil
}

func (s *OpenAIGatewayService) calculateParameterQuote(
	ctx context.Context,
	key *APIKey,
	rule *ParameterPricing,
	dimensions map[string]string,
	quantity int,
) (*ParameterPricingSnapshot, error) {
	baseRate := 1.0
	if s.cfg != nil {
		baseRate = s.cfg.Default.RateMultiplier
	}
	if key.GroupID != nil && key.Group != nil {
		baseRate = key.Group.RateMultiplier
		if key.User != nil {
			baseRate = s.ResolveUserGroupRateMultiplier(ctx, key.User.ID, *key.GroupID, baseRate)
		}
	}
	_, imageRate := computePeakAwareMultipliers(key, baseRate, time.Now())
	return CalculateParameterPricing(rule, ParameterPricingInput{
		Dimensions: dimensions, Quantity: quantity, Multiplier: strconv.FormatFloat(imageRate, 'f', -1, 64),
	})
}
