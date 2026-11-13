//go:build unit

package service

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func videoTokenTestRule() *VideoTokenPricing {
	return &VideoTokenPricing{Version: 1, FPS: 24, Divisor: 1024, AdaptiveSeconds: 15, MaxInputSeconds: 15,
		Tiers: map[string]VideoTokenPriceTier{
			"1080p":     {WithoutVideo: "7.7", WithVideo: "4.7", Sizes: map[string]VideoTokenGeometry{"16:9": {1920, 1080}}},
			"fast:720p": {WithoutVideo: "5.6", WithVideo: "3.3", Sizes: map[string]VideoTokenGeometry{"16:9": {1280, 720}}},
		},
	}
}

func TestVideoTokenSupplierFormula(t *testing.T) {
	tests := []struct {
		name, seconds, input, rate, tier string
		video                            bool
		width, height                    int
		tokens                           int64
		cost                             string
	}{
		{"screenshot example", "5", "0", "1", "1080p", false, 1920, 1080, 243000, "1.87110000"},
		{"group multiplier", "5", "0", "0.7", "1080p", false, 1920, 1080, 243000, "1.30977000"},
		{"minimum two thirds", "5", "1", "1", "1080p", true, 1920, 1080, 405000, "1.90350000"},
		{"actual input larger", "5", "10", "1", "1080p", true, 1920, 1080, 729000, "3.42630000"},
		{"fast tariff", "5", "0", "1", "fast:720p", false, 1280, 720, 108000, "0.60480000"},
		{"floor fractional tokens", "1.2345", "0", "1", "1080p", false, 1919, 1079, 59909, "0.46129930"},
		{"free multiplier", "5", "0", "0", "1080p", false, 1920, 1080, 243000, "0.00000000"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rule := videoTokenTestRule()
			quote, err := CalculateVideoTokenPricing(rule, VideoTokenPricingInput{
				Tier: tt.tier, Width: tt.width, Height: tt.height, OutputSeconds: tt.seconds,
				InputSeconds: tt.input, HasVideo: tt.video, Multiplier: tt.rate,
			})
			require.NoError(t, err)
			require.Equal(t, tt.tokens, quote.Tokens)
			require.Equal(t, tt.cost, quote.ActualCost)
			frozen := quote.Rule.Tiers[tt.tier]
			frozen.Sizes["16:9"] = VideoTokenGeometry{16, 16}
			require.NotEqual(t, frozen.Sizes["16:9"], rule.Tiers[tt.tier].Sizes["16:9"])
		})
	}
}

func TestVideoTokenFastAcceptsResolutionWithoutPrefix(t *testing.T) {
	rule := videoTokenTestRule()
	delete(rule.Tiers, "1080p")
	body := []byte(`{"model":"seedance-2.0-fast","content":[{"type":"text","text":"example"}],"duration":5,"resolution":"720p","ratio":"16:9"}`)
	quote, _, err := prepareVideoTokenQuote(rule, body, "1")
	require.NoError(t, err)
	require.Equal(t, "fast:720p", quote.Input.Tier)
	require.Equal(t, "5.6", quote.PricePerMillion)
	require.Equal(t, int64(108000), quote.Tokens)
	require.Equal(t, "720p", gjson.GetBytes(body, "resolution").String())
}

func TestVideoTokenMissingResolutionTierExplainsConfiguration(t *testing.T) {
	rule := videoTokenTestRule()
	body := []byte(`{"model":"seedance-2.0","content":[{"type":"text","text":"example"}],"duration":10,"resolution":"2K","ratio":"16:9"}`)

	quote, _, err := prepareVideoTokenQuote(rule, body, "1")

	require.Nil(t, quote)
	require.EqualError(t, err, "当前计费模式为：视频（Token），模型 seedance-2.0 未配置请求分辨率 \"2K\" 的价格档位。已配置档位：1080p、fast:720p。请补充 2k 档位，或将 resolution 改为已配置档位")
}

func TestVideoTokenAdmissionAndCompletion(t *testing.T) {
	rule := videoTokenTestRule()
	body := []byte(`{"model":"seedance","content":[{"type":"text","text":"example"}],"duration":5,"resolution":"1080p","ratio":"16:9"}`)
	quote, spec, err := prepareVideoTokenQuote(rule, body, "1")
	require.NoError(t, err)
	job := &VideoTokenBillingJob{ID: "hold-1", Quote: quote, Spec: spec}
	final, err := FinalVideoTokenQuote(job, []byte(`{"status":"succeeded"}`))
	require.NoError(t, err)
	require.Equal(t, int64(243000), final.Tokens, "fixed request works without usage or output metadata")
	require.Equal(t, "hold-1", final.HoldID)

	adaptive := []byte(`{"model":"seedance","content":[{"type":"video_url","video_url":{"url":"https://example.test/video.mp4"}}],"duration":-1,"resolution":"1080p","ratio":"adaptive"}`)
	quote, spec, err = prepareVideoTokenQuote(rule, adaptive, "0.8")
	require.NoError(t, err)
	require.Equal(t, "15", quote.Input.OutputSeconds)
	require.Equal(t, "15", quote.Input.InputSeconds)
	job.Quote, job.Spec = quote, spec
	_, err = FinalVideoTokenQuote(job, []byte(`{"status":"succeeded"}`))
	require.Error(t, err)
	_, err = FinalVideoTokenQuote(job, []byte(`{"status":"succeeded","duration":5,"width":1920,"height":1080}`))
	require.ErrorContains(t, err, "input_video_duration")
	final, err = FinalVideoTokenQuote(job, []byte(`{"status":"succeeded","duration":5,"width":1920,"height":1080,"input_video_duration":1}`))
	require.NoError(t, err)
	require.Equal(t, "1.52280000", final.ActualCost)
	require.Equal(t, int64(405000), final.Tokens)
}

func videoTokenMeasuredTestJob(t *testing.T, hasVideo bool) *VideoTokenBillingJob {
	t.Helper()
	rule := videoTokenTestRule()
	rule.Tiers["480p"] = VideoTokenPriceTier{WithoutVideo: "7", WithVideo: "4.3",
		Sizes: map[string]VideoTokenGeometry{"16:9": {854, 480}}}
	input := VideoTokenPricingInput{Tier: "480p", Width: 854, Height: 480,
		OutputSeconds: "5", InputSeconds: "0", HasVideo: hasVideo, Multiplier: "1"}
	if hasVideo {
		input.InputSeconds = "15"
	}
	quote, err := CalculateVideoTokenPricing(rule, input)
	require.NoError(t, err)
	return &VideoTokenBillingJob{ID: "measured-hold", Quote: quote, Spec: VideoTokenTaskSpec{Ratio: "16:9"}}
}

func TestVideoTokenTrustedMeasurementPriority(t *testing.T) {
	for _, test := range []struct {
		name, body, source, cost string
		tokens                   int64
		width, height            int
	}{
		{"actual dimensions fallback", `{"width":864,"height":496,"duration":5,"fps":24}`, "formula", "0.35154000", 50220, 864, 496},
		{"actual tokens override formula", `{"width":864,"height":496,"duration":5,"fps":24,"usage":{"completion_tokens":60000}}`, "upstream", "0.42000000", 60000, 864, 496},
		{"video tokens alias", `{"usage":{"video_tokens":50220}}`, "upstream", "0.35154000", 50220, 854, 480},
		{"consistent aliases", `{"usage":{"completion_tokens":50220,"video_tokens":50220}}`, "upstream", "0.35154000", 50220, 854, 480},
		{"total tokens ignored", `{"usage":{"total_tokens":999999}}`, "formula", "0.33625900", 48037, 854, 480},
		{"no metadata legacy fallback", `{}`, "formula", "0.33625900", 48037, 854, 480},
	} {
		t.Run(test.name, func(t *testing.T) {
			job := videoTokenMeasuredTestJob(t, false)
			before, err := json.Marshal(job)
			require.NoError(t, err)
			quote, err := FinalVideoTokenQuote(job, []byte(test.body))
			require.NoError(t, err)
			require.Equal(t, test.tokens, quote.Tokens)
			require.Equal(t, test.source, quote.TokenSource)
			require.Equal(t, test.cost, quote.ActualCost)
			require.Equal(t, job.Quote.RuleHash, quote.RuleHash)
			require.Equal(t, job.ID, quote.HoldID)
			require.Equal(t, test.width, quote.Input.Width)
			require.Equal(t, test.height, quote.Input.Height)
			after, err := json.Marshal(job)
			require.NoError(t, err)
			require.JSONEq(t, string(before), string(after), "never mutate reservation")
		})
	}
}

func TestVideoTokenRejectInvalidUpstreamMeasurements(t *testing.T) {
	for _, body := range []string{
		`{"usage":{"completion_tokens":0}}`, `{"usage":{"completion_tokens":-1}}`,
		`{"usage":{"completion_tokens":"50220"}}`, `{"usage":{"completion_tokens":null}}`,
		`{"usage":{"completion_tokens":false}}`, `{"usage":{"completion_tokens":1.5}}`,
		`{"usage":{"completion_tokens":100000000001}}`,
		`{"usage":{"completion_tokens":9223372036854775808}}`,
		`{"usage":{"completion_tokens":50220,"video_tokens":50221}}`,
		`{"usage":{"completion_tokens":50220,"completion_tokens":50220}}`,
		`{"width":864,"usage":{"completion_tokens":50220}}`,
		`{"fps":30,"usage":{"completion_tokens":50220}}`,
	} {
		t.Run(body, func(t *testing.T) {
			quote, err := FinalVideoTokenQuote(videoTokenMeasuredTestJob(t, false), []byte(body))
			require.Error(t, err)
			require.Nil(t, quote, "invalid measurements must not silently fall back")
		})
	}
}

func TestVideoTokenMeasuredIncludesInputWithoutAddingFormulaTokens(t *testing.T) {
	job := videoTokenMeasuredTestJob(t, true)
	quote, err := FinalVideoTokenQuote(job, []byte(`{"width":864,"height":496,"duration":5,"fps":24,"input_video_duration":1,"usage":{"completion_tokens":90000}}`))
	require.NoError(t, err)
	require.Equal(t, "upstream", quote.TokenSource)
	require.Equal(t, int64(90000), quote.Tokens)
	require.Equal(t, "4.3", quote.PricePerMillion)
	require.Equal(t, "0.38700000", quote.ActualCost)
	_, err = FinalVideoTokenQuote(job, []byte(`{"usage":{"completion_tokens":90000}}`))
	require.ErrorContains(t, err, "input_video_duration")
}

func TestVideoTokenMeasuredSurvivesUsageRecalculation(t *testing.T) {
	job := videoTokenMeasuredTestJob(t, false)
	job.Quote.Input.Multiplier = "0.7"
	quote, err := FinalVideoTokenQuote(job, []byte(`{"width":864,"height":496,"duration":5,"usage":{"completion_tokens":60000}}`))
	require.NoError(t, err)
	raw, err := json.Marshal(quote)
	require.NoError(t, err)
	var restored VideoTokenPricingSnapshot
	require.NoError(t, json.Unmarshal(raw, &restored))
	restored.ActualCost = "999.00000000"
	svc := &OpenAIGatewayService{}
	cost, err := svc.calculateOpenAIRecordUsageCost(context.Background(),
		&OpenAIForwardResult{VideoTokenQuote: &restored}, nil, nil,
		1, 99, 99, 99, UsageTokens{}, "", nil, false, time.Now())
	require.NoError(t, err)
	require.Equal(t, "upstream", cost.VideoTokenPricing.TokenSource)
	require.Equal(t, int64(60000), cost.VideoTokenPricing.Tokens)
	require.Equal(t, "0.29400000", cost.VideoTokenPricing.ActualCost)
	require.Equal(t, job.ID, cost.VideoTokenPricing.HoldID)
	require.Equal(t, quote.RuleHash, cost.VideoTokenPricing.RuleHash)
	restored.TokenSource = "invalid"
	_, err = svc.calculateOpenAIRecordUsageCost(context.Background(),
		&OpenAIForwardResult{VideoTokenQuote: &restored}, nil, nil,
		1, 1, 1, 1, UsageTokens{}, "", nil, false, time.Now())
	require.ErrorContains(t, err, "unsupported video token source")
}

func TestVideoTokenRejectAmbiguousParameters(t *testing.T) {
	for _, body := range []string{
		`{"content":[],"duration":5,"resolution":"1080p"}`,
		`{"content":[],"duration":5,"duration":10,"resolution":"1080p","ratio":"16:9"}`,
		`{"content":[],"duration":"5","resolution":"1080p","ratio":"16:9"}`,
		`{"content":[],"duration":5,"frames":120,"resolution":"1080p","ratio":"16:9"}`,
		`{"content":[],"duration":0,"resolution":"1080p","ratio":"16:9"}`,
		`{"content":[],"duration":5,"resolution":"1080p","ratio":"4:3"}`,
		`{"content":[{"type":"draft_task"}],"duration":5,"resolution":"1080p","ratio":"16:9"}`,
	} {
		_, _, err := prepareVideoTokenQuote(videoTokenTestRule(), []byte(body), "1")
		require.Error(t, err, body)
	}
}

func TestVideoTokenFrozenRuleAndDeterminism(t *testing.T) {
	rule := videoTokenTestRule()
	input := VideoTokenPricingInput{Tier: "1080p", Width: 1920, Height: 1080, OutputSeconds: "5", InputSeconds: "0", Multiplier: "0.7"}
	one, err := CalculateVideoTokenPricing(rule, input)
	require.NoError(t, err)
	rule.Tiers["1080p"] = VideoTokenPriceTier{WithoutVideo: "77", WithVideo: "47", Sizes: map[string]VideoTokenGeometry{"16:9": {1920, 1080}}}
	data, _ := json.Marshal(one)
	var reloaded VideoTokenPricingSnapshot
	require.NoError(t, json.Unmarshal(data, &reloaded))
	two, err := CalculateVideoTokenPricing(reloaded.Rule, reloaded.Input)
	require.NoError(t, err)
	require.Equal(t, one.RuleHash, two.RuleHash)
	require.Equal(t, one.ActualCost, two.ActualCost)
}
