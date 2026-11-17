package service

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/tidwall/gjson"
)

func TestExtractOpenAIUsageFromJSONBytes_MergesHostedImageGenToolUsage(t *testing.T) {
	// SSE response.completed event with response.usage + response.tool_usage.image_gen
	body := []byte(`{
		"type": "response.completed",
		"response": {
			"usage": {
				"input_tokens": 43792,
				"output_tokens": 1005,
				"total_tokens": 44797
			},
			"tool_usage": {
				"image_gen": {
					"input_tokens": 7918,
					"input_tokens_details": {
						"image_tokens": 7620,
						"text_tokens": 298,
						"cached_tokens": 500,
						"cached_tokens_details": {"text_tokens": 200, "image_tokens": 300}
					},
					"output_tokens": 186,
					"output_tokens_details": {"image_tokens": 186, "text_tokens": 0},
					"total_tokens": 8104
				}
			}
		}
	}`)

	usage, ok := extractOpenAIUsageFromJSONBytes(body)
	assert.True(t, ok)
	assert.Equal(t, 43792, usage.InputTokens, "input_tokens from response.usage")
	assert.Equal(t, 1005, usage.OutputTokens, "output_tokens from response.usage")
	assert.Equal(t, 186, usage.ImageOutputTokens, "image output tokens merged from tool_usage.image_gen")
	assert.Equal(t, 7620, usage.ImageInputTokens, "image input tokens merged from tool_usage.image_gen")
	assert.Equal(t, 500, usage.CacheReadInputTokens, "cached input tokens merged from tool_usage.image_gen")
	assert.Equal(t, 300, usage.ImageCacheReadTokens, "cached image tokens merged from tool_usage.image_gen")
}

func TestExtractOpenAIUsageFromJSONBytes_NonStreamingMergesImageGen(t *testing.T) {
	// Non-streaming response with top-level usage + tool_usage
	body := []byte(`{
		"id": "resp_abc123",
		"object": "response",
		"usage": {
			"input_tokens": 5000,
			"output_tokens": 200
		},
		"tool_usage": {
			"image_gen": {
				"input_tokens": 3000,
				"input_tokens_details": {"image_tokens": 2800, "text_tokens": 200},
				"output_tokens": 150,
				"output_tokens_details": {"image_tokens": 150, "text_tokens": 0},
				"total_tokens": 3150
			}
		}
	}`)

	usage, ok := extractOpenAIUsageFromJSONBytes(body)
	assert.True(t, ok)
	assert.Equal(t, 5000, usage.InputTokens)
	assert.Equal(t, 200, usage.OutputTokens)
	assert.Equal(t, 150, usage.ImageOutputTokens, "image output tokens from tool_usage.image_gen")
	assert.Equal(t, 2800, usage.ImageInputTokens, "image input tokens from tool_usage.image_gen")
}

func TestExtractOpenAIUsageFromJSONBytes_NoToolUsageUnchanged(t *testing.T) {
	// Standard response without tool_usage — behavior should be unchanged
	body := []byte(`{
		"usage": {
			"input_tokens": 100,
			"output_tokens": 50
		}
	}`)

	usage, ok := extractOpenAIUsageFromJSONBytes(body)
	assert.True(t, ok)
	assert.Equal(t, 100, usage.InputTokens)
	assert.Equal(t, 50, usage.OutputTokens)
	assert.Equal(t, 0, usage.ImageOutputTokens, "no image tokens without tool_usage")
	assert.Equal(t, 0, usage.ImageInputTokens, "no image tokens without tool_usage")
}

func TestExtractOpenAIUsageFromJSONBytes_ImageTokenCacheBreakdown(t *testing.T) {
	body := []byte(`{
		"usage": {
			"input_tokens": 10000,
			"input_tokens_details": {
				"text_tokens": 9000,
				"image_tokens": 1000,
				"cached_tokens": 1120,
				"cached_tokens_details": {
					"text_tokens": 1000,
					"image_tokens": 120
				}
			},
			"output_tokens": 18000,
			"output_tokens_details": {
				"text_tokens": 0,
				"image_tokens": 18000
			},
			"total_tokens": 28000
		}
	}`)

	usage, ok := extractOpenAIUsageFromJSONBytes(body)
	assert.True(t, ok)
	assert.Equal(t, 10000, usage.InputTokens)
	assert.Equal(t, 1000, usage.ImageInputTokens)
	assert.Equal(t, 1120, usage.CacheReadInputTokens)
	assert.Equal(t, 1000, usage.CacheReadInputTokens-usage.ImageCacheReadTokens)
	assert.Equal(t, 120, usage.ImageCacheReadTokens)
	assert.Equal(t, 18000, usage.OutputTokens)
	assert.Equal(t, 18000, usage.ImageOutputTokens)

	tokens := UsageTokens{
		InputTokens:           usage.InputTokens - usage.CacheReadInputTokens,
		ImageInputTokens:      usage.ImageInputTokens - usage.ImageCacheReadTokens,
		ImageCacheReadTokens:  usage.ImageCacheReadTokens,
		CacheReadTokens:       usage.CacheReadInputTokens,
		OutputTokens:          usage.OutputTokens,
		ImageOutputTokens:     usage.ImageOutputTokens,
	}
	pricing := &ModelPricing{
		InputPricePerToken:          5e-6,
		ImageInputPricePerToken:     8e-6,
		CacheReadPricePerToken:      1.25e-6,
		ImageCacheReadPricePerToken: 2e-6,
		ImageOutputPricePerToken:    30e-6,
	}
	cost := (&BillingService{}).computeTokenBreakdown(pricing, tokens, 1, "", false)
	assert.InDelta(t, 8000*5e-6, cost.InputCost, 1e-12, "uncached text input")
	assert.InDelta(t, 880*8e-6, cost.ImageInputCost, 1e-12, "uncached image input")
	assert.InDelta(t, 1000*1.25e-6+120*2e-6, cost.CacheReadCost, 1e-12, "cached text and image input")
	assert.InDelta(t, 18000*30e-6, cost.ImageOutputCost, 1e-12, "image output")
}

func TestExtractOpenAIUsageFromJSONBytes_BaseUsageHasImageTokensNoOverride(t *testing.T) {
	// If base usage already has image tokens (e.g. from output_tokens_details),
	// tool_usage should NOT override them.
	body := []byte(`{
		"usage": {
			"input_tokens": 100,
			"output_tokens": 50,
			"output_tokens_details": {"image_tokens": 30}
		},
		"tool_usage": {
			"image_gen": {
				"input_tokens": 200,
				"output_tokens": 100,
				"output_tokens_details": {"image_tokens": 100, "text_tokens": 0},
				"total_tokens": 300
			}
		}
	}`)

	usage, ok := extractOpenAIUsageFromJSONBytes(body)
	assert.True(t, ok)
	assert.Equal(t, 30, usage.ImageOutputTokens, "base usage image tokens preserved, not overridden")
}

func TestMergeHostedImageGenToolUsage_EmptyImageGen(t *testing.T) {
	tests := []struct {
		name string
		json string
	}{
		{"missing", `{}`},
		{"null", `{"image_gen": null}`},
		{"not object", `{"image_gen": 42}`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			usage := OpenAIUsage{InputTokens: 100, OutputTokens: 50}
			original := usage
			body := []byte(tt.json)
			mergeHostedImageGenToolUsage(gjson.GetBytes(body, "image_gen"), &usage)
			assert.Equal(t, original, usage, "usage should be unchanged")
		})
	}
}

func TestParseSSEUsageBytes_ResponseCompletedWithImageGen(t *testing.T) {
	svc := &OpenAIGatewayService{}
	data := []byte(`{
		"type": "response.completed",
		"response": {
			"usage": {
				"input_tokens": 10000,
				"output_tokens": 500
			},
			"tool_usage": {
				"image_gen": {
					"input_tokens": 4000,
					"input_tokens_details": {"image_tokens": 3800, "text_tokens": 200},
					"output_tokens": 186,
					"output_tokens_details": {"image_tokens": 186, "text_tokens": 0},
					"total_tokens": 4186
				}
			}
		}
	}`)

	usage := &OpenAIUsage{}
	svc.parseSSEUsageBytes(data, usage)

	assert.Equal(t, 10000, usage.InputTokens)
	assert.Equal(t, 500, usage.OutputTokens)
	assert.Equal(t, 186, usage.ImageOutputTokens, "image output tokens from SSE tool_usage")
	assert.Equal(t, 3800, usage.ImageInputTokens, "image input tokens from SSE tool_usage")
}
