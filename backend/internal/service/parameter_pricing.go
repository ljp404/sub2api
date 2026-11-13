package service

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"maps"
	"regexp"
	"slices"
	"strings"

	"github.com/shopspring/decimal"
)

// ParameterPricing is an opt-in rule. Prices and factors remain decimal strings
// until the existing monetary ledger boundary.
type ParameterPricing struct {
	Version   int                      `json:"version"`
	Unit      string                   `json:"unit"`
	BasePrice string                   `json:"base_price"`
	Factors   []ParameterPricingFactor `json:"factors"`
}

type ParameterPricingFactor struct {
	Key     string            `json:"key"`
	Default string            `json:"default,omitempty"`
	Values  map[string]string `json:"values"`
}

type ParameterPricingInput struct {
	Dimensions map[string]string `json:"dimensions"`
	Quantity   int               `json:"quantity"`
	Multiplier string            `json:"multiplier"`
}

type ParameterPricingSnapshot struct {
	HoldID     string                `json:"hold_id,omitempty"`
	RuleHash   string                `json:"rule_hash"`
	Rule       *ParameterPricing     `json:"rule"`
	Input      ParameterPricingInput `json:"input"`
	Factors    map[string]string     `json:"matched_factors"`
	UnitPrice  string                `json:"unit_price"`
	TotalCost  string                `json:"total_cost"`
	ActualCost string                `json:"actual_cost"`
}

var parameterKeyPattern = regexp.MustCompile(`^[a-z][a-z0-9_]{0,47}$`)
var parameterDecimalPattern = regexp.MustCompile(`^(0|[1-9][0-9]{0,8})(\.[0-9]{1,12})?$`)

func parameterDecimal(value string, positive bool) (decimal.Decimal, error) {
	if !parameterDecimalPattern.MatchString(value) {
		return decimal.Zero, fmt.Errorf("invalid decimal %q (up to 12 decimal places)", value)
	}
	n, err := decimal.NewFromString(value)
	if err != nil || (positive && !n.IsPositive()) {
		return decimal.Zero, fmt.Errorf("decimal %q must be positive", value)
	}
	return n, nil
}

func allowedParameterKey(key string) bool {
	if !parameterKeyPattern.MatchString(key) {
		return false
	}
	switch key {
	case "prompt", "model", "n", "stream", "api_key", "authorization", "password",
		"token", "access_token", "secret", "image", "images", "mask":
		return false
	}
	return true
}

func normalizeParameterValue(value string) string { return strings.ToLower(strings.TrimSpace(value)) }

func parameterPricingFieldLabel(key string) string {
	switch key {
	case "resolution":
		return "resolution（图片分辨率）"
	case "aspect_ratio":
		return "aspect_ratio（画面比例）"
	case "quality":
		return "quality（图片质量）"
	case "variant":
		return "variant（图片版本）"
	default:
		return key
	}
}

func ValidateParameterPricing(rule *ParameterPricing) error {
	if rule == nil || rule.Version != 1 || rule.Unit != "image" {
		return fmt.Errorf("parameter pricing requires version 1 and image unit")
	}
	if _, err := parameterDecimal(rule.BasePrice, true); err != nil {
		return fmt.Errorf("base_price: %w", err)
	}
	if len(rule.Factors) > 16 {
		return fmt.Errorf("at most 16 parameter factors are supported")
	}
	seen := make(map[string]bool)
	for _, f := range rule.Factors {
		if !allowedParameterKey(f.Key) || seen[f.Key] {
			return fmt.Errorf("invalid or duplicate parameter key %q", f.Key)
		}
		seen[f.Key] = true
		if len(f.Values) == 0 || len(f.Values) > 64 {
			return fmt.Errorf("parameter %s requires 1 to 64 values", f.Key)
		}
		for key, value := range f.Values {
			if key == "" || len(key) > 64 || key != normalizeParameterValue(key) {
				return fmt.Errorf("parameter %s has invalid value %q (use lowercase)", f.Key, key)
			}
			if _, err := parameterDecimal(value, true); err != nil {
				return fmt.Errorf("parameter %s value %s: %w", f.Key, key, err)
			}
		}
		if f.Default != "" {
			if _, ok := f.Values[f.Default]; !ok {
				return fmt.Errorf("parameter %s default must be a configured value", f.Key)
			}
		}
	}
	return nil
}

func CloneParameterPricing(rule *ParameterPricing) *ParameterPricing {
	if rule == nil {
		return nil
	}
	cp := *rule
	cp.Factors = slices.Clone(rule.Factors)
	if cp.Factors == nil {
		cp.Factors = []ParameterPricingFactor{}
	}
	for i := range cp.Factors {
		cp.Factors[i].Values = maps.Clone(rule.Factors[i].Values)
	}
	return &cp
}

func channelHasParameterPricing(channel *Channel) bool {
	if channel == nil {
		return false
	}
	for _, pricing := range channel.ModelPricing {
		if pricing.BillingMode == BillingModeParameter {
			return true
		}
	}
	return false
}

// CalculateParameterPricing is shared by preview, reservation and settlement.
// A configured default only applies to an absent value, never an unknown value.
func CalculateParameterPricing(rule *ParameterPricing, input ParameterPricingInput) (*ParameterPricingSnapshot, error) {
	if err := ValidateParameterPricing(rule); err != nil {
		return nil, err
	}
	if input.Quantity < 0 || input.Quantity > 10000 {
		return nil, fmt.Errorf("quantity must be between 0 and 10000")
	}
	rate, err := parameterDecimal(input.Multiplier, false)
	if err != nil {
		return nil, fmt.Errorf("multiplier: %w", err)
	}
	unit, _ := decimal.NewFromString(rule.BasePrice)
	dimensions := make(map[string]string, len(rule.Factors))
	factors := make(map[string]string, len(rule.Factors))
	var problems []string
	for _, f := range rule.Factors {
		value := normalizeParameterValue(input.Dimensions[f.Key])
		if value == "" {
			value = f.Default
			if value == "" && f.Key == "variant" {
				value = "flare"
			}
		}
		factor, ok := f.Values[value]
		if !ok {
			options := strings.Join(slices.Sorted(maps.Keys(f.Values)), "、")
			label := parameterPricingFieldLabel(f.Key)
			if value == "" {
				problems = append(problems, fmt.Sprintf("缺少必填参数 %s，可选值：%s", label, options))
			} else {
				problems = append(problems, fmt.Sprintf("参数 %s的值 %q 不受支持，可选值：%s", label, value, options))
			}
			continue
		}
		n, _ := decimal.NewFromString(factor)
		unit = unit.Mul(n)
		dimensions[f.Key], factors[f.Key] = value, factor
	}
	if len(problems) > 0 {
		return nil, fmt.Errorf("当前计费模式为：图片（参数），%s", strings.Join(problems, "；"))
	}
	total := unit.Mul(decimal.NewFromInt(int64(input.Quantity))).Round(UsageBillingMonetaryScale)
	actual := unit.Mul(decimal.NewFromInt(int64(input.Quantity))).Mul(rate).Round(UsageBillingMonetaryScale)
	limit := decimal.NewFromInt(100000000000)
	if total.GreaterThan(limit) || actual.GreaterThan(limit) || (input.Quantity > 0 && rate.IsPositive() && unit.Mul(rate).Round(UsageBillingMonetaryScale).IsZero()) {
		return nil, fmt.Errorf("parameter price exceeds ledger range or rounds to zero")
	}
	for _, amount := range []decimal.Decimal{total, actual} {
		value, _ := amount.Float64()
		if !decimal.NewFromFloat(value).Equal(amount) {
			return nil, fmt.Errorf("parameter amount exceeds ledger precision")
		}
	}
	frozenRule := CloneParameterPricing(rule)
	data, _ := json.Marshal(frozenRule)
	hash := sha256.Sum256(data)
	return &ParameterPricingSnapshot{
		RuleHash: hex.EncodeToString(hash[:]), Rule: frozenRule,
		Input:   ParameterPricingInput{Dimensions: dimensions, Quantity: input.Quantity, Multiplier: rate.String()},
		Factors: factors, UnitPrice: unit.String(), TotalCost: total.StringFixed(UsageBillingMonetaryScale),
		ActualCost: actual.StringFixed(UsageBillingMonetaryScale),
	}, nil
}

func parameterPricingCost(snapshot *ParameterPricingSnapshot) *CostBreakdown {
	total, _ := decimal.RequireFromString(snapshot.TotalCost).Float64()
	actual, _ := decimal.RequireFromString(snapshot.ActualCost).Float64()
	return &CostBreakdown{
		TotalCost: total, ActualCost: actual, ImageOutputCost: total,
		BillingMode: string(BillingModeParameter), ParameterPricing: snapshot,
	}
}
