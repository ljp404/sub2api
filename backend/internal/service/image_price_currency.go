package service

import (
	"fmt"
	"math"

	"github.com/shopspring/decimal"
)

func validateImagePriceCurrency(p ChannelModelPricing) error {
	if p.ImagePriceCurrency == "" && p.ImageCNYPerUSD == nil {
		return nil
	}
	if p.BillingMode != BillingModeImage {
		return fmt.Errorf("image price currency requires image billing mode")
	}
	if p.ImagePriceCurrency != "" && p.ImagePriceCurrency != "USD" && p.ImagePriceCurrency != "CNY" {
		return fmt.Errorf("image price currency must be USD or CNY")
	}
	if p.ImagePriceCurrency == "CNY" && (p.ImageCNYPerUSD == nil ||
		math.IsNaN(*p.ImageCNYPerUSD) || math.IsInf(*p.ImageCNYPerUSD, 0) || *p.ImageCNYPerUSD <= 0) {
		return fmt.Errorf("人民币图片价格必须配置大于 0 的人民币 / 美元汇率")
	}
	return nil
}

// ImagePriceUSD converts default/edit prices for billing without changing the
// configured amount. Legacy entries and resolution tiers remain in USD.
func (p *ChannelModelPricing) ImagePriceUSD(price float64) float64 {
	if p != nil && p.BillingMode == BillingModeImage && p.ImagePriceCurrency == "CNY" &&
		p.ImageCNYPerUSD != nil && *p.ImageCNYPerUSD > 0 {
		value, _ := decimal.NewFromFloat(price).Div(decimal.NewFromFloat(*p.ImageCNYPerUSD)).Float64()
		return value
	}
	return price
}

// ImagePricingUSD provides a display copy for public APIs that quote USD.
func (p *ChannelModelPricing) ImagePricingUSD() *ChannelModelPricing {
	if p == nil || p.BillingMode != BillingModeImage || p.ImagePriceCurrency != "CNY" {
		return p
	}
	cp := p.Clone()
	if p.PerRequestPrice != nil {
		price := p.ImagePriceUSD(*p.PerRequestPrice)
		cp.PerRequestPrice = &price
	}
	if p.ImageEditPrice != nil {
		price := p.ImagePriceUSD(*p.ImageEditPrice)
		cp.ImageEditPrice = &price
	}
	cp.ImagePriceCurrency = "USD"
	cp.ImageCNYPerUSD = nil
	return &cp
}
