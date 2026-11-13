package service

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
)

// ModelPlazaReferencePricing is manually maintained display-only pricing.
// Values are USD per token except PerRequestPrice, which is USD per request.
type ModelPlazaReferencePricing struct {
	InputPrice        *float64 `json:"input_price,omitempty"`
	OutputPrice       *float64 `json:"output_price,omitempty"`
	CacheWritePrice   *float64 `json:"cache_write_price,omitempty"`
	CacheWrite1hPrice *float64 `json:"cache_write_1h_price,omitempty"`
	CacheReadPrice    *float64 `json:"cache_read_price,omitempty"`
	ImageInputPrice   *float64 `json:"image_input_price,omitempty"`
	ImageOutputPrice  *float64 `json:"image_output_price,omitempty"`
	PerRequestPrice   *float64 `json:"per_request_price,omitempty"`
}

type ModelPlazaReferencePrice struct {
	ID          int64                       `json:"id"`
	Platform    string                      `json:"platform"`
	ModelName   string                      `json:"model_name"`
	Pricing     ModelPlazaReferencePricing  `json:"pricing"`
	SourceLabel string                      `json:"source_label"`
	SourceURL   string                      `json:"source_url"`
	Notes       string                      `json:"notes"`
	UpdatedBy   *int64                      `json:"updated_by,omitempty"`
	CreatedAt   time.Time                   `json:"created_at"`
	UpdatedAt   time.Time                   `json:"updated_at"`
}

type ModelPlazaReferencePriceRepository interface {
	ListModelPlazaReferencePrices(ctx context.Context) ([]ModelPlazaReferencePrice, error)
	ListModelPlazaReferencePricesPage(ctx context.Context, params pagination.PaginationParams, search string) ([]ModelPlazaReferencePrice, *pagination.PaginationResult, error)
	UpsertModelPlazaReferencePrice(ctx context.Context, price *ModelPlazaReferencePrice) error
	DeleteModelPlazaReferencePrice(ctx context.Context, id int64) error
	DeleteModelPlazaReferencePrices(ctx context.Context, ids []int64) (int64, error)
}

var ErrInvalidModelPlazaReferencePrice = errors.New("invalid model plaza reference price")

func validateModelPlazaReferencePrice(price *ModelPlazaReferencePrice) error {
	if price == nil {
		return fmt.Errorf("%w: missing price", ErrInvalidModelPlazaReferencePrice)
	}
	price.Platform = strings.ToLower(strings.TrimSpace(price.Platform))
	price.ModelName = strings.TrimSpace(price.ModelName)
	price.SourceLabel = strings.TrimSpace(price.SourceLabel)
	price.SourceURL = strings.TrimSpace(price.SourceURL)
	price.Notes = strings.TrimSpace(price.Notes)
	if price.Platform == "" || len(price.Platform) > 50 {
		return fmt.Errorf("%w: invalid platform", ErrInvalidModelPlazaReferencePrice)
	}
	if price.ModelName == "" || len(price.ModelName) > 255 {
		return fmt.Errorf("%w: model name is required (max 255 characters)", ErrInvalidModelPlazaReferencePrice)
	}
	if price.SourceLabel == "" || len(price.SourceLabel) > 255 {
		return fmt.Errorf("%w: source label is required (max 255 characters)", ErrInvalidModelPlazaReferencePrice)
	}
	if len(price.SourceURL) > 2048 {
		return fmt.Errorf("%w: source URL is too long", ErrInvalidModelPlazaReferencePrice)
	}
	if len(price.Notes) > 2000 {
		return fmt.Errorf("%w: notes are too long", ErrInvalidModelPlazaReferencePrice)
	}
	if price.SourceURL != "" {
		parsed, err := url.ParseRequestURI(price.SourceURL)
		if err != nil || (parsed.Scheme != "https" && parsed.Scheme != "http") || parsed.Host == "" {
			return fmt.Errorf("%w: source URL must be HTTP or HTTPS", ErrInvalidModelPlazaReferencePrice)
		}
	}

	values := []*float64{
		price.Pricing.InputPrice, price.Pricing.OutputPrice,
		price.Pricing.CacheWritePrice, price.Pricing.CacheWrite1hPrice,
		price.Pricing.CacheReadPrice, price.Pricing.ImageInputPrice,
		price.Pricing.ImageOutputPrice, price.Pricing.PerRequestPrice,
	}
	hasPrice := false
	for _, value := range values {
		if value == nil {
			continue
		}
		hasPrice = true
		if *value < 0 {
			return fmt.Errorf("%w: prices must be non-negative", ErrInvalidModelPlazaReferencePrice)
		}
	}
	if !hasPrice {
		return fmt.Errorf("%w: at least one price is required", ErrInvalidModelPlazaReferencePrice)
	}
	return nil
}
