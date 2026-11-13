//go:build unit

package service

import (
	"bytes"
	"context"
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

func imageParameterTestRule() *ParameterPricing {
	return &ParameterPricing{
		Version: 1, Unit: "image", BasePrice: "0.052706",
		Factors: []ParameterPricingFactor{
			{Key: "quality", Values: map[string]string{"high": "1", "low": "0.111338"}},
			{Key: "resolution", Values: map[string]string{"1k": "1", "2k": "2.030577"}},
			{Key: "aspect_ratio", Values: map[string]string{"1:1": "1", "16:9": "0.479421"}},
		},
	}
}

func TestParameterPricingDecimalSettlement(t *testing.T) {
	rule := imageParameterTestRule()
	input := ParameterPricingInput{Dimensions: map[string]string{
		"quality": "HIGH", "resolution": "2k", "aspect_ratio": "16:9", "prompt": "never audit",
	}, Quantity: 2, Multiplier: "0.8"}
	snapshot, err := CalculateParameterPricing(rule, input)
	require.NoError(t, err)
	require.Equal(t, "0.08209497", snapshot.ActualCost)
	require.Equal(t, "0.10261871", snapshot.TotalCost)
	require.NotContains(t, snapshot.Input.Dimensions, "prompt")
	require.Equal(t, "high", snapshot.Input.Dimensions["quality"])
	rule.Factors[0].Values["high"] = "9"
	require.Equal(t, "1", snapshot.Rule.Factors[0].Values["high"])
	again, err := CalculateParameterPricing(snapshot.Rule, snapshot.Input)
	require.NoError(t, err)
	require.Equal(t, snapshot.RuleHash, again.RuleHash)

	input.Multiplier = "0"
	free, err := CalculateParameterPricing(snapshot.Rule, input)
	require.NoError(t, err)
	require.Equal(t, "0.00000000", free.ActualCost)
	input.Multiplier, input.Quantity = "1", 0
	none, err := CalculateParameterPricing(snapshot.Rule, input)
	require.NoError(t, err)
	require.Equal(t, "0.00000000", none.ActualCost)
}

func TestParameterPricingEmptyFactorsHaveStableSnapshot(t *testing.T) {
	rule := &ParameterPricing{Version: 1, Unit: "image", BasePrice: "0.052706"}
	input := ParameterPricingInput{Quantity: 1, Multiplier: "1"}
	quote, err := CalculateParameterPricing(rule, input)
	require.NoError(t, err)
	require.NotNil(t, quote.Rule.Factors)
	again, err := CalculateParameterPricing(quote.Rule, input)
	require.NoError(t, err)
	require.Equal(t, quote.RuleHash, again.RuleHash)
}

func TestParameterPricingReportsAllInvalidFieldsFromCurrentRule(t *testing.T) {
	rule := imageParameterTestRule()
	for _, test := range []struct {
		name       string
		dimensions map[string]string
		want       string
	}{
		{
			name:       "missing fields",
			dimensions: map[string]string{"quality": "low"},
			want:       "当前计费模式为：图片（参数），缺少必填参数 resolution（图片分辨率），可选值：1k、2k；缺少必填参数 aspect_ratio（画面比例），可选值：16:9、1:1",
		},
		{
			name:       "empty and whitespace",
			dimensions: map[string]string{"quality": "low", "resolution": "  ", "aspect_ratio": ""},
			want:       "当前计费模式为：图片（参数），缺少必填参数 resolution（图片分辨率），可选值：1k、2k；缺少必填参数 aspect_ratio（画面比例），可选值：16:9、1:1",
		},
		{
			name:       "unsupported value and missing field",
			dimensions: map[string]string{"quality": "low", "resolution": "8K"},
			want:       "当前计费模式为：图片（参数），参数 resolution（图片分辨率）的值 \"8k\" 不受支持，可选值：1k、2k；缺少必填参数 aspect_ratio（画面比例），可选值：16:9、1:1",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			quote, err := CalculateParameterPricing(rule, ParameterPricingInput{
				Dimensions: test.dimensions, Quantity: 1, Multiplier: "1",
			})
			require.EqualError(t, err, test.want)
			require.Nil(t, quote)
		})
	}
	t.Run("configured values and custom fields only", func(t *testing.T) {
		custom := &ParameterPricing{Version: 1, Unit: "image", BasePrice: "1",
			Factors: []ParameterPricingFactor{
				{Key: "quality", Default: "low", Values: map[string]string{"low": "1"}},
				{Key: "render_mode", Values: map[string]string{"vivid": "2", "natural": "1"}},
			}}
		quote, err := CalculateParameterPricing(custom, ParameterPricingInput{Quantity: 1, Multiplier: "1"})
		require.EqualError(t, err, "当前计费模式为：图片（参数），缺少必填参数 render_mode，可选值：natural、vivid")
		require.Nil(t, quote)
		require.NotContains(t, err.Error(), "resolution")
		require.NotContains(t, err.Error(), "quality")
		require.NotContains(t, err.Error(), "base_price")
		quote, err = CalculateParameterPricing(custom, ParameterPricingInput{
			Dimensions: map[string]string{"render_mode": "NATURAL"}, Quantity: 1, Multiplier: "1",
		})
		require.NoError(t, err)
		require.Equal(t, "1.00000000", quote.ActualCost)
		require.Equal(t, "low", quote.Input.Dimensions["quality"])
	})
}

func TestParameterPricingAutoSizeReportsUnconfiguredAuto(t *testing.T) {
	gin.SetMode(gin.TestMode)
	svc := &OpenAIGatewayService{}
	body := []byte(`{"model":"gpt-image-2.5-sunburst","prompt":"test","size":"auto","quality":"low"}`)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest("POST", "/v1/images/generations", bytes.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	req, err := svc.ParseOpenAIImagesRequest(c, body)
	require.NoError(t, err)
	dimensions, err := imageSizeMappedDimensions(req, imageParameterTestRule())
	require.Nil(t, dimensions)
	require.ErrorContains(t, err, "aspect_ratio=auto")
}

func TestParameterPricingRejectsInvalidRulesAndDimensions(t *testing.T) {
	for name, modify := range map[string]func(*ParameterPricing){
		"zero base":           func(r *ParameterPricing) { r.BasePrice = "0" },
		"nan":                 func(r *ParameterPricing) { r.BasePrice = "NaN" },
		"negative":            func(r *ParameterPricing) { r.Factors[0].Values["high"] = "-1" },
		"zero factor":         func(r *ParameterPricing) { r.Factors[0].Values["high"] = "0" },
		"unsupported meter":   func(r *ParameterPricing) { r.Unit = "video" },
		"unsupported version": func(r *ParameterPricing) { r.Version = 2 },
		"duplicate":           func(r *ParameterPricing) { r.Factors = append(r.Factors, r.Factors[0]) },
		"secret":              func(r *ParameterPricing) { r.Factors[0].Key = "api_key" },
		"bad default":         func(r *ParameterPricing) { r.Factors[0].Default = "auto" },
		"uppercase":           func(r *ParameterPricing) { r.Factors[0].Values["High"] = "1" },
	} {
		t.Run(name, func(t *testing.T) {
			rule := imageParameterTestRule()
			modify(rule)
			require.Error(t, ValidateParameterPricing(rule))
		})
	}
	rule := &ParameterPricing{Version: 1, Unit: "image", BasePrice: "1",
		Factors: []ParameterPricingFactor{{Key: "quality", Default: "high", Values: map[string]string{"high": "2"}}}}
	out, err := CalculateParameterPricing(rule, ParameterPricingInput{Quantity: 1, Multiplier: "1"})
	require.NoError(t, err)
	require.Equal(t, "2.00000000", out.ActualCost)
	for _, input := range []ParameterPricingInput{
		{Dimensions: map[string]string{"quality": "unknown"}, Quantity: 1, Multiplier: "1"},
		{Quantity: -1, Multiplier: "1"}, {Quantity: 1, Multiplier: "-1"}, {Quantity: 1, Multiplier: "NaN"},
	} {
		_, err := CalculateParameterPricing(rule, input)
		require.Error(t, err)
	}
	rule.BasePrice = "0.000000000001"
	_, err = CalculateParameterPricing(rule, ParameterPricingInput{Quantity: 1, Multiplier: "1"})
	require.ErrorContains(t, err, "rounds to zero")
}

func TestParameterPricingJSONAndMultipartDimensions(t *testing.T) {
	gin.SetMode(gin.TestMode)
	svc := &OpenAIGatewayService{}
	for _, encoding := range []string{"json", "multipart"} {
		t.Run(encoding, func(t *testing.T) {
			body := []byte(`{"model":"gpt-image-2","prompt":"private","quality":"high","size":"1920x1080","resolution":"2k","variant":"sunburst"}`)
			contentType := "application/json"
			path := "/v1/images/generations"
			if encoding == "multipart" {
				var buf bytes.Buffer
				writer := multipart.NewWriter(&buf)
				for k, v := range map[string]string{"model": "gpt-image-2", "prompt": "private", "quality": "high", "size": "1920x1080", "resolution": "2k", "variant": "sunburst"} {
					require.NoError(t, writer.WriteField(k, v))
				}
				part, err := writer.CreateFormFile("image", "input.png")
				require.NoError(t, err)
				_, err = part.Write([]byte("test image"))
				require.NoError(t, err)
				require.NoError(t, writer.Close())
				body, contentType, path = buf.Bytes(), writer.FormDataContentType(), "/v1/images/edits"
			}
			c, _ := gin.CreateTestContext(httptest.NewRecorder())
			c.Request = httptest.NewRequest("POST", path, bytes.NewReader(body))
			c.Request.Header.Set("Content-Type", contentType)
			parsed, err := svc.ParseOpenAIImagesRequest(c, body)
			require.NoError(t, err)
			dimensions := imagePriceDimensions(parsed)
			require.Equal(t, "16:9", dimensions["aspect_ratio"])
			require.Equal(t, "2k", dimensions["resolution"])
			require.Equal(t, "sunburst", dimensions["variant"])
			require.NotContains(t, dimensions, "prompt")
			require.NotContains(t, dimensions, "image")
		})
	}
}

func TestParameterPricingForwardUsesActualOutputsAndFrozenRule(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, test := range []struct {
		name, contentType, body string
		count                   int
	}{
		{"partial batch", "application/json", `{"data":[{"b64_json":"aGVsbG8="}]}`, 1},
		{"empty batch", "application/json", `{"data":[]}`, 0},
		{"stream", "text/event-stream", "data: {\"type\":\"image_generation.partial_image\",\"b64_json\":\"aGVsbG8=\"}\n\ndata: {\"type\":\"image_generation.completed\",\"b64_json\":\"aGVsbG8=\"}\n\ndata: [DONE]\n\n", 1},
	} {
		t.Run(test.name, func(t *testing.T) {
			request := []byte(`{"model":"gpt-image-2","prompt":"test","n":3,"quality":"high","resolution":"2k","aspect_ratio":"16:9","stream":true}`)
			svc := &OpenAIGatewayService{cfg: &config.Config{}, httpUpstream: &httpUpstreamRecorder{
				resp: &http.Response{StatusCode: 200, Header: http.Header{"Content-Type": []string{test.contentType}},
					Body: io.NopCloser(strings.NewReader(test.body))},
			}}
			c, _ := gin.CreateTestContext(httptest.NewRecorder())
			c.Request = httptest.NewRequest("POST", "/v1/images/generations", bytes.NewReader(request))
			c.Request.Header.Set("Content-Type", "application/json")
			parsed, err := svc.ParseOpenAIImagesRequest(c, request)
			require.NoError(t, err)
			rule := imageParameterTestRule()
			parsed.ParameterQuote, err = CalculateParameterPricing(rule, ParameterPricingInput{
				Dimensions: imagePriceDimensions(parsed), Quantity: parsed.N, Multiplier: "0.8",
			})
			require.NoError(t, err)
			rule.BasePrice = "99"
			result, err := svc.ForwardImages(context.Background(), c, &Account{ID: 1, Platform: PlatformOpenAI, Type: AccountTypeAPIKey,
				Credentials: map[string]any{"api_key": "test", "base_url": "https://image-upstream.example/v1"}}, request, parsed, "")
			require.NoError(t, err)
			require.Equal(t, test.count, result.ImageCount)
			cost, err := svc.calculateOpenAIRecordUsageCost(context.Background(), result, nil, nil,
				999, 0.8, 999, 999, UsageTokens{InputTokens: 100000, ImageOutputTokens: 100000}, "priority", nil, false, time.Now())
			require.NoError(t, err)
			require.Equal(t, "parameter", cost.BillingMode)
			require.Equal(t, test.count, cost.ParameterPricing.Input.Quantity)
			require.Equal(t, "0.052706", cost.ParameterPricing.Rule.BasePrice)
			if test.count == 1 {
				require.Equal(t, "0.04104749", cost.ParameterPricing.ActualCost)
			} else {
				require.Zero(t, cost.ActualCost)
			}
		})
	}
}

func TestParameterPricingPreflightAndOldModes(t *testing.T) {
	ctx := context.Background()
	rule := imageParameterTestRule()
	channel := Channel{ID: 1, Status: StatusActive, GroupIDs: []int64{1}, ModelPricing: []ChannelModelPricing{
		{Platform: PlatformOpenAI, Models: []string{"gpt-image-2"}, BillingMode: BillingModeParameter, ParameterPricing: rule},
	}}
	channelService := newTestChannelService(makeStandardRepo(channel, map[int64]string{1: PlatformOpenAI}))
	require.True(t, channelService.IsModelRestricted(ctx, 1, "gpt-image-2"), "other endpoints must not dispatch an image parameter rule")
	require.False(t, channelService.IsModelRestricted(WithOpenAIImagesEndpoint(ctx), 1, "gpt-image-2"))
	billing := newTestBillingServiceForResolver()
	svc := &OpenAIGatewayService{resolver: NewModelPricingResolver(channelService, billing)}
	gid := int64(1)
	key := &APIKey{GroupID: &gid, Group: &Group{ID: 1, ModelPricing: []ChannelModelPricing{
		{Models: []string{"gpt-image-2"}, BillingMode: BillingModeImage, PerRequestPrice: testPtrFloat64(99)},
	}}}
	account := &Account{Type: AccountTypeAPIKey}
	req := &OpenAIImagesRequest{Model: "gpt-image-2", N: 1, Size: "2048x2048", PriceDimensions: map[string]string{"quality": "high", "resolution": "2k", "aspect_ratio": "1:1"}}
	mapping := ChannelMappingResult{MappedModel: req.Model, BillingModelSource: BillingModelSourceChannelMapped}
	require.NoError(t, svc.PrepareImageParameterPricing(ctx, key, req, account, mapping, req.Model))
	require.Nil(t, req.ParameterQuote, "explicit group model pricing must override the channel parameter rule")
	key.Group.ModelPricing = nil
	key.Group.ImagePrice1K = testPtrFloat64(99)
	require.NoError(t, svc.PrepareImageParameterPricing(ctx, key, req, account, mapping, req.Model))
	require.NotNil(t, req.ParameterQuote, "explicit channel parameter rule must precede group legacy image defaults")
	req.PriceDimensions["quality"] = "unknown"
	require.Error(t, svc.PrepareImageParameterPricing(ctx, key, req, account, mapping, req.Model))
	require.Nil(t, req.ParameterQuote)
	req.PriceDimensions["quality"] = "high"
	req.InvalidPriceDimensions = map[string]bool{"quality": true}
	require.ErrorContains(t, svc.PrepareImageParameterPricing(ctx, key, req, account, mapping, req.Model), "duplicate")
	req.InvalidPriceDimensions = nil
	account.Type = AccountTypeOAuth
	require.ErrorContains(t, svc.PrepareImageParameterPricing(ctx, key, req, account, mapping, req.Model), "API-key")
	resolved := svc.resolver.Resolve(ctx, PricingInput{Model: req.Model, GroupID: &gid, Group: key.Group})
	_, err := billing.CalculateCostUnified(CostInput{Resolver: svc.resolver, Resolved: resolved})
	require.ErrorContains(t, err, "validated image")
	resolved.Mode = BillingModeImage
	resolved.DefaultPerRequestPrice = 0.2
	cost, err := billing.CalculateCostUnified(CostInput{Resolver: svc.resolver, Resolved: resolved, RequestCount: 2, RateMultiplier: 0.7})
	require.NoError(t, err)
	require.InDelta(t, 0.28, cost.ActualCost, 1e-8)
}

func TestPrepareParameterPricingForGeminiModels(t *testing.T) {
	ctx := context.Background()
	rule := &ParameterPricing{
		Version: 1, Unit: "image", BasePrice: "0.05",
		Factors: []ParameterPricingFactor{
			{Key: "resolution", Values: map[string]string{"1k": "1", "2k": "2"}},
			{Key: "aspect_ratio", Values: map[string]string{"1:1": "1", "16:9": "0.5"}},
		},
	}
	channel := Channel{ID: 1, Status: StatusActive, GroupIDs: []int64{1}, ModelPricing: []ChannelModelPricing{
		{Platform: PlatformGemini, Models: []string{"gemini-3-pro-image"}, BillingMode: BillingModeParameter, ParameterPricing: rule},
	}}
	channelService := newTestChannelService(makeStandardRepo(channel, map[int64]string{1: PlatformGemini}))
	svc := &OpenAIGatewayService{resolver: NewModelPricingResolver(channelService, newTestBillingServiceForResolver())}
	gid := int64(1)
	key := &APIKey{GroupID: &gid, Group: &Group{ID: gid, Platform: PlatformGemini, RateMultiplier: 1}}

	quote, err := svc.PrepareParameterPricing(
		ctx,
		key,
		[]string{"gemini-3-pro-image"},
		GeminiImagePricingDimensions([]byte(`{"generationConfig":{"imageConfig":{"imageSize":"2K","aspectRatio":"16:9"}}}`)),
		1,
	)
	require.NoError(t, err)
	require.NotNil(t, quote)
	require.Equal(t, "2k", quote.Input.Dimensions["resolution"])
	require.Equal(t, "16:9", quote.Input.Dimensions["aspect_ratio"])
	require.Equal(t, "0.05000000", quote.ActualCost)

	cost := (&GatewayService{billingService: newTestBillingServiceForResolver()}).calculateImageCost(
		ctx,
		&ForwardResult{ImageCount: 1, ParameterQuote: quote},
		key,
		"gemini-3-pro-image",
		99,
	)
	require.Equal(t, string(BillingModeParameter), cost.BillingMode)
	require.Equal(t, "0.05000000", cost.ParameterPricing.ActualCost)
}

func TestParameterPricingSeparateModelsWithoutVariant(t *testing.T) {
	gin.SetMode(gin.TestMode)
	ctx := WithOpenAIImagesEndpoint(context.Background())
	flare, sunburst := imageParameterTestRule(), imageParameterTestRule()
	flare.BasePrice, sunburst.BasePrice = "1", "2"
	channel := Channel{ID: 1, Status: StatusActive, GroupIDs: []int64{1}, ModelPricing: []ChannelModelPricing{
		{Platform: PlatformOpenAI, Models: []string{"gpt-image-2.5-flare"}, BillingMode: BillingModeParameter, ParameterPricing: flare},
		{Platform: PlatformOpenAI, Models: []string{"gpt-image-2.5-sunburst"}, BillingMode: BillingModeParameter, ParameterPricing: sunburst},
	}}
	channelService := newTestChannelService(makeStandardRepo(channel, map[int64]string{1: PlatformOpenAI}))
	for _, test := range []struct{ model, amount string }{
		{"gpt-image-2.5-flare", "1.00000000"},
		{"gpt-image-2.5-sunburst", "2.00000000"},
	} {
		t.Run(test.model, func(t *testing.T) {
			upstream := &httpUpstreamRecorder{resp: &http.Response{StatusCode: 200,
				Header: http.Header{"Content-Type": []string{"application/json"}},
				Body:   io.NopCloser(strings.NewReader(`{"data":[{"b64_json":"aGVsbG8="}]}`))}}
			svc := &OpenAIGatewayService{cfg: &config.Config{}, httpUpstream: upstream,
				resolver: NewModelPricingResolver(channelService, newTestBillingServiceForResolver())}
			body := []byte(`{"model":"` + test.model + `","prompt":"test","quality":"high","size":"1024x1024"}`)
			c, _ := gin.CreateTestContext(httptest.NewRecorder())
			c.Request = httptest.NewRequest("POST", "/v1/images/generations", bytes.NewReader(body))
			c.Request.Header.Set("Content-Type", "application/json")
			req, err := svc.ParseOpenAIImagesRequest(c, body)
			require.NoError(t, err)
			gid := int64(1)
			key := &APIKey{GroupID: &gid, Group: &Group{ID: gid, RateMultiplier: 1}}
			account := &Account{ID: 1, Type: AccountTypeAPIKey, Platform: PlatformOpenAI,
				Credentials: map[string]any{"api_key": "test", "base_url": "https://image-upstream.example/v1"}}
			mapping := ChannelMappingResult{MappedModel: test.model, BillingModelSource: BillingModelSourceRequested}
			require.NoError(t, svc.PrepareImageParameterPricing(ctx, key, req, account, mapping, test.model))
			require.NotNil(t, req.ParameterQuote)
			require.Equal(t, test.amount, req.ParameterQuote.ActualCost)
			require.NotContains(t, req.ParameterQuote.Input.Dimensions, "variant")
			_, err = svc.ForwardImages(ctx, c, account, body, req, "")
			require.NoError(t, err)
			require.Equal(t, test.model, gjson.GetBytes(upstream.lastBody, "model").String())
			require.False(t, gjson.GetBytes(upstream.lastBody, "variant").Exists())
		})
	}
}

func TestParameterPricingVariantDefault(t *testing.T) {
	for _, test := range []struct {
		name, variant, fallback, want string
		fail                          bool
	}{
		{name: "missing", want: "flare"},
		{name: "empty", variant: "", want: "flare"},
		{name: "whitespace", variant: " \t ", want: "flare"},
		{name: "explicit", variant: "SUNBURST", want: "sunburst"},
		{name: "configured default", fallback: "sunburst", want: "sunburst"},
		{name: "unknown", variant: "unknown", fail: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			rule := &ParameterPricing{Version: 1, Unit: "image", BasePrice: "1",
				Factors: []ParameterPricingFactor{{Key: "variant", Default: test.fallback,
					Values: map[string]string{"flare": "1", "sunburst": "2"}}}}
			quote, err := CalculateParameterPricing(rule, ParameterPricingInput{
				Dimensions: map[string]string{"variant": test.variant}, Quantity: 1, Multiplier: "1",
			})
			if test.fail {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			require.Equal(t, test.want, quote.Input.Dimensions["variant"])
			require.Equal(t, test.fallback, rule.Factors[0].Default, "do not mutate saved rules")
		})
	}
}

func TestParameterPricingVariantForwardDefault(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, encoding := range []string{"json", "multipart"} {
		for _, test := range []struct {
			name, field, want string
			legacy            bool
		}{
			{name: "missing", want: "flare"},
			{name: "null", field: `,"variant":null`, want: "flare"},
			{name: "empty", field: `,"variant":""`, want: "flare"},
			{name: "whitespace", field: `,"variant":"  "`, want: "flare"},
			{name: "explicit", field: `,"variant":"sunburst"`, want: "sunburst"},
			{name: "legacy unchanged", legacy: true},
		} {
			t.Run(encoding+"/"+test.name, func(t *testing.T) {
				body := []byte(`{"model":"gpt-image-2","prompt":"test","n":1` + test.field + `}`)
				contentType, endpoint := "application/json", "/v1/images/generations"
				if encoding == "multipart" {
					var buf bytes.Buffer
					writer := multipart.NewWriter(&buf)
					gjson.ParseBytes(body).ForEach(func(key, value gjson.Result) bool {
						require.NoError(t, writer.WriteField(key.String(), value.String()))
						return true
					})
					file, err := writer.CreateFormFile("image", "input.png")
					require.NoError(t, err)
					_, err = file.Write([]byte("original image"))
					require.NoError(t, err)
					require.NoError(t, writer.Close())
					body, contentType, endpoint = buf.Bytes(), writer.FormDataContentType(), "/v1/images/edits"
				}
				upstream := &httpUpstreamRecorder{resp: &http.Response{StatusCode: 200,
					Header: http.Header{"Content-Type": []string{"application/json"}},
					Body:   io.NopCloser(strings.NewReader(`{"data":[{"b64_json":"aGVsbG8="}]}`))}}
				svc := &OpenAIGatewayService{cfg: &config.Config{}, httpUpstream: upstream}
				c, _ := gin.CreateTestContext(httptest.NewRecorder())
				c.Request = httptest.NewRequest("POST", endpoint, bytes.NewReader(body))
				c.Request.Header.Set("Content-Type", contentType)
				parsed, err := svc.ParseOpenAIImagesRequest(c, body)
				require.NoError(t, err)
				require.False(t, parsed.InvalidPriceDimensions["variant"])
				if !test.legacy {
					rule := &ParameterPricing{Version: 1, Unit: "image", BasePrice: "1",
						Factors: []ParameterPricingFactor{{Key: "variant", Values: map[string]string{"flare": "1", "sunburst": "2"}}}}
					parsed.ParameterQuote, err = CalculateParameterPricing(rule, ParameterPricingInput{
						Dimensions: imagePriceDimensions(parsed), Quantity: 1, Multiplier: "1",
					})
					require.NoError(t, err)
					require.Equal(t, test.want, parsed.ParameterQuote.Input.Dimensions["variant"])
				}
				_, err = svc.ForwardImages(context.Background(), c, &Account{ID: 1, Platform: PlatformOpenAI, Type: AccountTypeAPIKey,
					Credentials: map[string]any{"api_key": "test", "base_url": "https://image-upstream.example/v1"}}, body, parsed, "")
				require.NoError(t, err)
				if encoding == "json" {
					require.Equal(t, test.want, gjson.GetBytes(upstream.lastBody, "variant").String())
				} else {
					_, params, err := mime.ParseMediaType(upstream.lastReq.Header.Get("Content-Type"))
					require.NoError(t, err)
					form, err := multipart.NewReader(bytes.NewReader(upstream.lastBody), params["boundary"]).ReadForm(1 << 20)
					require.NoError(t, err)
					defer form.RemoveAll()
					if test.legacy {
						require.NotContains(t, form.Value, "variant")
					} else {
						require.Equal(t, []string{test.want}, form.Value["variant"])
					}
					file, err := form.File["image"][0].Open()
					require.NoError(t, err)
					data, err := io.ReadAll(file)
					require.NoError(t, err)
					require.NoError(t, file.Close())
					require.Equal(t, "original image", string(data))
				}
			})
		}
	}
	for _, body := range []string{
		`{"model":"gpt-image-2","prompt":"test","variant":null,"variant":"flare"}`,
		`{"model":"gpt-image-2","prompt":"test","variant":{} }`,
	} {
		req := &OpenAIImagesRequest{}
		captureImageJSONPriceParameters(req, []byte(body))
		require.True(t, req.InvalidPriceDimensions["variant"])
	}
}
