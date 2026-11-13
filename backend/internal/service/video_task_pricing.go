package service

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/shopspring/decimal"
	"github.com/tidwall/gjson"
)

// VideoTaskPricing is an opt-in per-second tariff with additive task charges.
// Prices are entered in Currency; the frozen snapshot is converted to USD.
type VideoTaskPricing struct {
	Version                    int    `json:"version"`
	Currency                   string `json:"currency"`
	CNYPerUSD                  string `json:"cny_per_usd,omitempty"`
	Price480PPerSecond         string `json:"price_480p_per_second"`
	Price768PPerSecond         string `json:"price_768p_per_second"`
	Upgrade2KPerSecond         string `json:"upgrade_2k_per_second"`
	PromptPolishPricePerCall   string `json:"prompt_polish_price_per_call"`
	PromptPolishDefaultEnabled bool   `json:"prompt_polish_default_enabled"`
}

type VideoTaskPricingInput struct {
	Resolution       string `json:"resolution"`
	DurationSeconds  string `json:"duration_seconds"`
	PromptPolish     bool   `json:"prompt_polish"`
	Multiplier       string `json:"multiplier"`
}

type VideoTaskPricingSnapshot struct {
	RuleHash          string               `json:"rule_hash"`
	Rule              *VideoTaskPricing    `json:"rule"`
	Input             VideoTaskPricingInput `json:"input"`
	BasePrice         string               `json:"base_price"`
	UpgradePrice      string               `json:"upgrade_price"`
	PromptPolishPrice string               `json:"prompt_polish_price"`
	Currency          string               `json:"currency"`
	SourceCurrency    string               `json:"source_currency"`
	FXRate            string               `json:"fx_rate"`
	TotalCost         string               `json:"total_cost"`
	ActualCost        string               `json:"actual_cost"`
	HoldID            string               `json:"hold_id,omitempty"`
}

func CloneVideoTaskPricing(rule *VideoTaskPricing) *VideoTaskPricing {
	if rule == nil {
		return nil
	}
	cp := *rule
	return &cp
}

func ValidateVideoTaskPricing(rule *VideoTaskPricing) error {
	if rule == nil || rule.Version != 1 {
		return fmt.Errorf("video task pricing requires version 1")
	}
	currency := strings.ToUpper(strings.TrimSpace(rule.Currency))
	if currency != "USD" && currency != "CNY" {
		return fmt.Errorf("video task pricing currency must be USD or CNY")
	}
	for name, value := range map[string]string{
		"480p price": rule.Price480PPerSecond,
		"768p price": rule.Price768PPerSecond,
		"2k upgrade price": rule.Upgrade2KPerSecond,
		"prompt polish price": rule.PromptPolishPricePerCall,
	} {
		if _, err := parameterDecimal(value, true); err != nil {
			return fmt.Errorf("%s: %w", name, err)
		}
	}
	if currency == "CNY" {
		fx, err := parameterDecimal(rule.CNYPerUSD, true)
		if err != nil || !fx.IsPositive() {
			return fmt.Errorf("CNY pricing requires a positive CNY-per-USD exchange rate")
		}
	}
	return nil
}

func normalizeVideoTaskResolution(value string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "480", "480p":
		return "480p", nil
	case "768", "768p":
		return "768p", nil
	case "2k", "2kp":
		return "2k", nil
	default:
		return "", fmt.Errorf("unsupported video resolution %q", value)
	}
}

func CalculateVideoTaskPricing(rule *VideoTaskPricing, input VideoTaskPricingInput) (*VideoTaskPricingSnapshot, error) {
	if err := ValidateVideoTaskPricing(rule); err != nil {
		return nil, err
	}
	resolution, err := normalizeVideoTaskResolution(input.Resolution)
	if err != nil {
		return nil, err
	}
	duration, err := parameterDecimal(input.DurationSeconds, true)
	if err != nil || duration.GreaterThan(decimal.NewFromInt(3600)) {
		return nil, fmt.Errorf("duration_seconds must be positive and <= 3600")
	}
	rate, err := parameterDecimal(input.Multiplier, false)
	if err != nil {
		return nil, fmt.Errorf("invalid multiplier: %w", err)
	}
	basePerSecond := rule.Price480PPerSecond
	if resolution == "768p" || resolution == "2k" {
		basePerSecond = rule.Price768PPerSecond
	}
	base, _ := decimal.NewFromString(basePerSecond)
	base = base.Mul(duration)
	upgrade := decimal.Zero
	if resolution == "2k" {
		upgradeRate, _ := decimal.NewFromString(rule.Upgrade2KPerSecond)
		upgrade = upgradeRate.Mul(duration)
	}
	polish := decimal.Zero
	if input.PromptPolish {
		polish, _ = decimal.NewFromString(rule.PromptPolishPricePerCall)
	}
	fx := decimal.NewFromInt(1)
	if strings.EqualFold(strings.TrimSpace(rule.Currency), "CNY") {
		fx, _ = decimal.NewFromString(rule.CNYPerUSD)
	}
	total := base.Add(upgrade).Add(polish).Div(fx)
	actual := total.Mul(rate).Round(UsageBillingMonetaryScale)

	frozen := CloneVideoTaskPricing(rule)
	data, _ := json.Marshal(frozen)
	hash := sha256.Sum256(data)
	return &VideoTaskPricingSnapshot{
		RuleHash: hex.EncodeToString(hash[:]), Rule: frozen,
		Input: VideoTaskPricingInput{
			Resolution: resolution, DurationSeconds: duration.String(),
			PromptPolish: input.PromptPolish, Multiplier: rate.String(),
		},
		BasePrice: base.Div(fx).StringFixed(UsageBillingMonetaryScale),
		UpgradePrice: upgrade.Div(fx).StringFixed(UsageBillingMonetaryScale),
		PromptPolishPrice: polish.Div(fx).StringFixed(UsageBillingMonetaryScale),
		Currency: "USD", SourceCurrency: strings.ToUpper(strings.TrimSpace(rule.Currency)), FXRate: fx.String(),
		TotalCost: total.Round(UsageBillingMonetaryScale).StringFixed(UsageBillingMonetaryScale),
		ActualCost: actual.StringFixed(UsageBillingMonetaryScale),
	}, nil
}

func videoTaskPricingCost(snapshot *VideoTaskPricingSnapshot) *CostBreakdown {
	total, _ := decimal.RequireFromString(snapshot.TotalCost).Float64()
	actual, _ := decimal.RequireFromString(snapshot.ActualCost).Float64()
	return &CostBreakdown{
		TotalCost: total, ActualCost: actual, OutputCost: total,
		BillingMode: string(BillingModeVideo), VideoTaskPricing: snapshot,
	}
}

func prepareVideoTaskQuote(rule *VideoTaskPricing, body []byte, multiplier string) (*VideoTaskPricingSnapshot, VideoTokenTaskSpec, error) {
	var spec VideoTokenTaskSpec
	if err := ValidateVideoTaskPricing(rule); err != nil {
		return nil, spec, err
	}
	resolution := gjson.GetBytes(body, "resolution")
	duration := gjson.GetBytes(body, "duration")
	if resolution.Type != gjson.String || duration.Type != gjson.Number {
		return nil, spec, fmt.Errorf("video task pricing requires resolution and duration")
	}
	promptPolish := rule.PromptPolishDefaultEnabled
	requestPromptPolish := gjson.GetBytes(body, "prompt_polish")
	if !requestPromptPolish.Exists() {
		requestPromptPolish = gjson.GetBytes(body, "metadata.prompt_polish")
	}
	if requestPromptPolish.Exists() {
		if requestPromptPolish.Type != gjson.True && requestPromptPolish.Type != gjson.False {
			return nil, spec, fmt.Errorf("prompt_polish must be a boolean")
		}
		promptPolish = requestPromptPolish.Bool()
	}
	quote, err := CalculateVideoTaskPricing(rule, VideoTaskPricingInput{
		Resolution: resolution.String(), DurationSeconds: duration.Raw,
		PromptPolish: promptPolish, Multiplier: multiplier,
	})
	if err != nil {
		return nil, spec, err
	}
	spec.Ratio = strings.TrimSpace(gjson.GetBytes(body, "ratio").String())
	return quote, spec, nil
}
