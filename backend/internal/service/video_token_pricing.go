package service

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"maps"
	"strings"

	"github.com/shopspring/decimal"
)

// VideoTokenPricing is deliberately separate from image factors and per-second
// video prices. Geometry is explicit: a resolution label alone is not a size.
type VideoTokenPricing struct {
	Version         int                            `json:"version"`
	FPS             int                            `json:"fps"`
	Divisor         int                            `json:"divisor"`
	AdaptiveSeconds int                            `json:"adaptive_seconds"`
	MaxInputSeconds int                            `json:"max_input_seconds"`
	Tiers           map[string]VideoTokenPriceTier `json:"tiers"`
}

type VideoTokenPriceTier struct {
	WithoutVideo string                        `json:"without_video"`
	WithVideo    string                        `json:"with_video"`
	Sizes        map[string]VideoTokenGeometry `json:"sizes"`
}

type VideoTokenGeometry struct {
	Width  int `json:"width"`
	Height int `json:"height"`
}

type VideoTokenPricingInput struct {
	Tier          string `json:"tier"`
	Width         int    `json:"width"`
	Height        int    `json:"height"`
	OutputSeconds string `json:"output_seconds"`
	InputSeconds  string `json:"input_seconds"`
	HasVideo      bool   `json:"has_video"`
	Multiplier    string `json:"multiplier"`
}

type VideoTokenPricingSnapshot struct {
	RuleHash           string                 `json:"rule_hash"`
	Rule               *VideoTokenPricing     `json:"rule"`
	Input              VideoTokenPricingInput `json:"input"`
	BilledInputSeconds string                 `json:"billed_input_seconds"`
	Tokens             int64                  `json:"tokens"`
	TokenSource        string                 `json:"token_source"`
	PricePerMillion    string                 `json:"price_per_million"`
	TotalCost          string                 `json:"total_cost"`
	ActualCost         string                 `json:"actual_cost"`
	HoldID             string                 `json:"hold_id,omitempty"`
}

func CloneVideoTokenPricing(rule *VideoTokenPricing) *VideoTokenPricing {
	if rule == nil {
		return nil
	}
	cp := *rule
	cp.Tiers = maps.Clone(rule.Tiers)
	for k, tier := range cp.Tiers {
		tier.Sizes = maps.Clone(tier.Sizes)
		cp.Tiers[k] = tier
	}
	return &cp
}

func ValidateVideoTokenPricing(rule *VideoTokenPricing) error {
	if rule == nil || rule.Version != 1 || rule.FPS != 24 || rule.Divisor != 1024 {
		return fmt.Errorf("video token pricing requires version 1, fps 24 and divisor 1024")
	}
	if rule.AdaptiveSeconds != 15 || rule.MaxInputSeconds < 1 || rule.MaxInputSeconds > 3600 {
		return fmt.Errorf("adaptive_seconds must be 15; max_input_seconds must be 1..3600")
	}
	if len(rule.Tiers) == 0 || len(rule.Tiers) > 32 {
		return fmt.Errorf("video token pricing requires 1..32 tiers")
	}
	for name, tier := range rule.Tiers {
		if name == "" || len(name) > 48 || name != strings.ToLower(strings.TrimSpace(name)) {
			return fmt.Errorf("invalid video tier %q", name)
		}
		for _, price := range []string{tier.WithoutVideo, tier.WithVideo} {
			if _, err := parameterDecimal(price, true); err != nil {
				return fmt.Errorf("tier %s price: %w", name, err)
			}
		}
		if len(tier.Sizes) == 0 || len(tier.Sizes) > 32 {
			return fmt.Errorf("tier %s requires explicit aspect-ratio sizes", name)
		}
		for ratio, size := range tier.Sizes {
			var w, h int
			if _, err := fmt.Sscanf(ratio, "%d:%d", &w, &h); err != nil || w <= 0 || h <= 0 || fmt.Sprintf("%d:%d", w, h) != ratio {
				return fmt.Errorf("invalid ratio %q", ratio)
			}
			if size.Width < 16 || size.Height < 16 || size.Width > 8192 || size.Height > 8192 {
				return fmt.Errorf("tier %s size must be within 16..8192 pixels", name)
			}
		}
	}
	return nil
}

func CalculateVideoTokenPricing(rule *VideoTokenPricing, input VideoTokenPricingInput) (*VideoTokenPricingSnapshot, error) {
	return calculateVideoTokenPricing(rule, input, nil)
}

// Measured tokens come only from a trusted task status, never a client preview.
func calculateVideoTokenPricing(rule *VideoTokenPricing, input VideoTokenPricingInput, measuredTokens *int64) (*VideoTokenPricingSnapshot, error) {
	if err := ValidateVideoTokenPricing(rule); err != nil {
		return nil, err
	}
	tier, ok := rule.Tiers[input.Tier]
	if !ok {
		return nil, fmt.Errorf("unsupported video tier %q", input.Tier)
	}
	if input.Width < 16 || input.Height < 16 || input.Width > 8192 || input.Height > 8192 {
		return nil, fmt.Errorf("actual width and height must be within 16..8192")
	}
	output, err := parameterDecimal(input.OutputSeconds, true)
	if err != nil || output.GreaterThan(decimal.NewFromInt(3600)) {
		return nil, fmt.Errorf("output_seconds must be positive and <= 3600")
	}
	in, err := parameterDecimal(input.InputSeconds, false)
	if err != nil || in.GreaterThan(decimal.NewFromInt(int64(rule.MaxInputSeconds))) || (!input.HasVideo && !in.IsZero()) {
		return nil, fmt.Errorf("invalid input_seconds")
	}
	rate, err := parameterDecimal(input.Multiplier, false)
	if err != nil {
		return nil, fmt.Errorf("invalid multiplier: %w", err)
	}
	price := tier.WithoutVideo
	// Keep the 2/3 bound rational until the final integer floor. Rounding 2/3
	// before multiplying pixels can change the integer token count.
	numerator := output
	billedInput := decimal.Zero
	if input.HasVideo {
		price = tier.WithVideo
		if in.Mul(decimal.NewFromInt(3)).LessThan(output.Mul(decimal.NewFromInt(2))) {
			numerator = output.Mul(decimal.NewFromInt(5))
			billedInput = output.Mul(decimal.NewFromInt(2)).Div(decimal.NewFromInt(3))
		} else {
			numerator = output.Add(in).Mul(decimal.NewFromInt(3))
			billedInput = in
		}
	} else {
		numerator = output.Mul(decimal.NewFromInt(3))
	}
	pixelsFPS := int64(input.Width) * int64(input.Height) * int64(rule.FPS)
	q, _ := numerator.Mul(decimal.NewFromInt(pixelsFPS)).QuoRem(decimal.NewFromInt(int64(rule.Divisor)*3), 0)
	tokens := q.IntPart()
	tokenSource := "formula"
	if measuredTokens != nil {
		if *measuredTokens <= 0 || *measuredTokens > 100000000000 {
			return nil, fmt.Errorf("measured video token count must be between 1 and 100000000000")
		}
		tokens = *measuredTokens
		tokenSource = "upstream"
	}
	if tokens <= 0 {
		return nil, fmt.Errorf("video token count rounds to zero")
	}
	p, _ := decimal.NewFromString(price)
	total := decimal.NewFromInt(tokens).Mul(p).Div(decimal.NewFromInt(1000000))
	actual := total.Mul(rate).Round(UsageBillingMonetaryScale)
	if total.GreaterThan(decimal.NewFromInt(100000000000)) || actual.GreaterThan(decimal.NewFromInt(100000000000)) || (rate.IsPositive() && actual.IsZero()) {
		return nil, fmt.Errorf("video cost exceeds ledger range or rounds to zero")
	}
	for _, amount := range []decimal.Decimal{total.Round(UsageBillingMonetaryScale), actual} {
		value, _ := amount.Float64()
		if !decimal.NewFromFloat(value).Equal(amount) {
			return nil, fmt.Errorf("video cost exceeds ledger precision")
		}
	}
	frozen := CloneVideoTokenPricing(rule)
	data, _ := json.Marshal(frozen)
	hash := sha256.Sum256(data)
	return &VideoTokenPricingSnapshot{
		RuleHash: hex.EncodeToString(hash[:]), Rule: frozen, Input: input,
		BilledInputSeconds: billedInput.String(), Tokens: tokens, TokenSource: tokenSource, PricePerMillion: price,
		TotalCost:  total.Round(UsageBillingMonetaryScale).StringFixed(UsageBillingMonetaryScale),
		ActualCost: actual.StringFixed(UsageBillingMonetaryScale),
	}, nil
}

func videoTokenPricingCost(snapshot *VideoTokenPricingSnapshot) *CostBreakdown {
	total, _ := decimal.RequireFromString(snapshot.TotalCost).Float64()
	actual, _ := decimal.RequireFromString(snapshot.ActualCost).Float64()
	return &CostBreakdown{TotalCost: total, ActualCost: actual, OutputCost: total,
		BillingMode: string(BillingModeVideoToken), VideoTokenPricing: snapshot}
}
