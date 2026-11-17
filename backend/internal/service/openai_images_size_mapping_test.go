//go:build unit

package service

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func imageSizeMappingTestRule() *ParameterPricing {
	return &ParameterPricing{Version: 1, Unit: "image", BasePrice: "1",
		Factors: []ParameterPricingFactor{
			{Key: "resolution", Values: map[string]string{"1k": "1", "2k": "2", "4k": "4"}},
			{Key: "aspect_ratio", Values: map[string]string{
				"auto": "1", "1:1": "1", "4:3": "1", "3:4": "1",
				"3:2": "1", "2:3": "1", "16:9": "1", "9:16": "1", "21:9": "1",
			}},
		}}
}

func TestParameterImageSizeMapping(t *testing.T) {
	for _, tc := range []struct{ size, resolution, ratio string }{
		{"auto", "2k", "auto"}, {" AUTO ", "2k", "auto"},
		{"1024x1024", "1k", "1:1"}, {"1024x768", "1k", "4:3"},
		{"768x1024", "1k", "3:4"}, {"1500x1000", "1k", "3:2"},
		{"1536x1024", "2k", "3:2"}, {"1920x1080", "2k", "16:9"},
		{"2000x1000", "2k", "16:9"}, {"3840x2160", "4k", "16:9"},
		{"1080x1920", "2k", "9:16"}, {"3072x1728", "4k", "16:9"},
		{"1920X1080", "2k", "16:9"},
	} {
		t.Run(tc.size, func(t *testing.T) {
			req := &OpenAIImagesRequest{Size: tc.size, PriceDimensions: map[string]string{
				"quality": "high", "resolution": "1k", "aspect_ratio": "auto", "image_size": "4k",
			}}
			got, err := imageSizeMappedDimensions(req, imageSizeMappingTestRule())
			require.NoError(t, err)
			require.Equal(t, tc.resolution, got["resolution"])
			require.Equal(t, tc.ratio, got["aspect_ratio"])
			require.Equal(t, "high", got["quality"])
			require.NotContains(t, got, "image_size")
			require.Equal(t, "1k", req.PriceDimensions["resolution"], "never mutate client inputs")
		})
	}
}

func TestParameterImageSizeMappingNilRule(t *testing.T) {
	require.False(t, imageSizeMappingEnabled(nil))
	dimensions, err := imageSizeMappedDimensions(&OpenAIImagesRequest{
		Size: "auto",
		PriceDimensions: map[string]string{
			"quality": "high",
		},
	}, nil)
	require.NoError(t, err)
	require.Equal(t, map[string]string{"quality": "high", "aspect_ratio": "auto"}, dimensions)
}

func TestParameterImageSizeMappingUsesOnlyConfiguredOptions(t *testing.T) {
	rule := imageSizeMappingTestRule()
	delete(rule.Factors[0].Values, "2k")
	dimensions, err := imageSizeMappedDimensions(&OpenAIImagesRequest{Size: "1920x1080"}, rule)
	require.NoError(t, err)
	require.Equal(t, "1k", dimensions["resolution"])
	_, err = imageSizeMappedDimensions(&OpenAIImagesRequest{Size: "auto"}, rule)
	require.ErrorContains(t, err, "resolution=2k")

	rule = imageSizeMappingTestRule()
	delete(rule.Factors[1].Values, "auto")
	_, err = imageSizeMappedDimensions(&OpenAIImagesRequest{Size: "auto"}, rule)
	require.ErrorContains(t, err, "aspect_ratio=auto")

	rule.Factors[1].Values = map[string]string{"auto": "1", "3:4": "1"}
	_, err = imageSizeMappedDimensions(&OpenAIImagesRequest{Size: "1920x1080"}, rule)
	require.ErrorContains(t, err, "同方向")
}

func TestParameterImageSizeMappingRejectsInvalidSizes(t *testing.T) {
	for _, size := range []string{"", " ", "1k", "1024", "0x1024", "-1x100", "1.5x100",
		"1024x", "1024x1024x1", "+1024x768", "1024 x 768", "65537x1", "999999999999999999999x1"} {
		t.Run(size, func(t *testing.T) {
			_, err := imageSizeMappedDimensions(&OpenAIImagesRequest{Size: size}, imageSizeMappingTestRule())
			require.Error(t, err)
		})
	}
	for _, body := range []string{
		`{"size":1024}`, `{"size":true}`, `{"size":["auto"]}`, `{"size":{"value":"auto"}}`,
		`{"size":null}`, `{"size":"auto","size":"1024x1024"}`,
	} {
		req := &OpenAIImagesRequest{}
		require.NoError(t, parseOpenAIImagesJSONRequest([]byte(body), req))
		_, err := imageSizeMappedDimensions(req, imageSizeMappingTestRule())
		require.Error(t, err)
	}
}

func TestParameterImageSizeMappingDeterministicRatioTies(t *testing.T) {
	for i := 0; i < 100; i++ {
		ratio, err := nearestImageAspectRatio(2, 1, map[string]string{"4:1": "1", "1:1": "1", "16:16": "1", "auto": "1"})
		// A square is not a landscape candidate; do not rotate or collapse orientation.
		require.NoError(t, err)
		require.Equal(t, "4:1", ratio)
		ratio, err = nearestImageAspectRatio(4, 1, map[string]string{"8:1": "1", "2:1": "1"})
		require.NoError(t, err)
		require.Equal(t, "2:1", ratio, "equal relative distances choose the smaller ratio")
		ratio, err = nearestImageAspectRatio(1920, 1080, map[string]string{"32:18": "1", "16:9": "2"})
		require.NoError(t, err)
		require.Equal(t, "16:9", ratio, "equivalent ratio aliases have a stable lexical tie break")
	}
}

func TestParameterImageSizeMappingPreservesRulesWithoutSizeFactors(t *testing.T) {
	rule := &ParameterPricing{Version: 1, Unit: "image", BasePrice: "1",
		Factors: []ParameterPricingFactor{{Key: "variant", Values: map[string]string{"flare": "1"}}}}
	dimensions, err := imageSizeMappedDimensions(&OpenAIImagesRequest{}, rule)
	require.NoError(t, err)
	require.Empty(t, dimensions)
}

func TestParameterImageSizeMappingForwardAndSettlement(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, encoding := range []string{"json", "multipart"} {
		for _, size := range []string{"auto", "1920x1080", "1536x1024"} {
			t.Run(encoding+"/"+size, func(t *testing.T) {
				rule := imageSizeMappingTestRule()
				upstream := &httpUpstreamRecorder{resp: &http.Response{
					StatusCode: 200, Header: http.Header{"Content-Type": []string{"application/json"}},
					Body: io.NopCloser(strings.NewReader(`{"data":[{"b64_json":"aGVsbG8="}]}`)),
				}}
				svc := &OpenAIGatewayService{cfg: &config.Config{}, httpUpstream: upstream,
					resolver: NewModelPricingResolver(nil, newTestBillingServiceForResolver())}
				payload := map[string]string{
					"model": "gpt-image-2", "prompt": "test", "size": size,
					"resolution": "1k", "aspect_ratio": "auto", "image_size": "1k",
				}
				body, err := json.Marshal(payload)
				require.NoError(t, err)
				contentType, endpoint := "application/json", "/v1/images/generations"
				if encoding == "multipart" {
					var buffer bytes.Buffer
					writer := multipart.NewWriter(&buffer)
					for k, v := range payload {
						require.NoError(t, writer.WriteField(k, v))
					}
					file, err := writer.CreateFormFile("image", "input.png")
					require.NoError(t, err)
					_, err = file.Write([]byte("unchanged upload bytes"))
					require.NoError(t, err)
					require.NoError(t, writer.Close())
					body, contentType, endpoint = buffer.Bytes(), writer.FormDataContentType(), "/v1/images/edits"
				}
				c, _ := gin.CreateTestContext(httptest.NewRecorder())
				c.Request = httptest.NewRequest("POST", endpoint, bytes.NewReader(body))
				c.Request.Header.Set("Content-Type", contentType)
				req, err := svc.ParseOpenAIImagesRequest(c, body)
				require.NoError(t, err)
				gid := int64(1)
				key := &APIKey{GroupID: &gid, Group: &Group{ID: gid, Platform: PlatformComposite, RateMultiplier: 1,
					ModelPricing: []ChannelModelPricing{{Platform: PlatformOpenAI, Models: []string{"gpt-image-2"},
						BillingMode: BillingModeParameter, ParameterPricing: rule}},
				}}
				account := &Account{ID: 1, Type: AccountTypeAPIKey, Platform: PlatformOpenAI,
					Credentials: map[string]any{"api_key": "test", "base_url": "https://image-upstream.example/v1"}}
				require.NoError(t, svc.PrepareImageParameterPricing(context.Background(), key, req, account,
					ChannelMappingResult{MappedModel: req.Model}, req.Model))
				require.Equal(t, "2k", req.ParameterQuote.Input.Dimensions["resolution"])
				expectedRatio := map[string]string{"auto": "auto", "1920x1080": "16:9", "1536x1024": "3:2"}[size]
				require.Equal(t, expectedRatio, req.ParameterQuote.Input.Dimensions["aspect_ratio"])
				require.Equal(t, "2K", req.SizeTier)
				// Both config and client input may change after preflight; the quote is authoritative.
				rule.BasePrice = "99"
				req.PriceDimensions["resolution"] = "4k"
				result, err := svc.ForwardImages(context.Background(), c, account, body, req, "")
				require.NoError(t, err)
				if encoding == "json" {
					require.False(t, gjson.GetBytes(upstream.lastBody, "size").Exists())
					require.False(t, gjson.GetBytes(upstream.lastBody, "image_size").Exists())
					require.Equal(t, "2k", gjson.GetBytes(upstream.lastBody, "resolution").String())
					require.Equal(t, expectedRatio, gjson.GetBytes(upstream.lastBody, "aspect_ratio").String())
				} else {
					_, params, err := mime.ParseMediaType(upstream.lastReq.Header.Get("Content-Type"))
					require.NoError(t, err)
					form, err := multipart.NewReader(bytes.NewReader(upstream.lastBody), params["boundary"]).ReadForm(1 << 20)
					require.NoError(t, err)
					defer form.RemoveAll()
					require.NotContains(t, form.Value, "size")
					require.NotContains(t, form.Value, "image_size")
					require.Equal(t, []string{"2k"}, form.Value["resolution"])
					require.Equal(t, []string{expectedRatio}, form.Value["aspect_ratio"])
					file, err := form.File["image"][0].Open()
					require.NoError(t, err)
					data, err := io.ReadAll(file)
					require.NoError(t, err)
					require.NoError(t, file.Close())
					require.Equal(t, "unchanged upload bytes", string(data))
				}
				require.Equal(t, size, result.ImageInputSize)
				cost, err := svc.calculateOpenAIRecordUsageCost(context.Background(), result, key, nil,
					999, 999, 999, 999, UsageTokens{}, "", nil, false, time.Now())
				require.NoError(t, err)
				require.Equal(t, "2.00000000", cost.ParameterPricing.ActualCost)
				require.Equal(t, expectedRatio, cost.ParameterPricing.Input.Dimensions["aspect_ratio"])
				require.Equal(t, "1", cost.ParameterPricing.Rule.BasePrice)
			})
		}
	}
}
