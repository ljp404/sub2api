//go:build unit

package service

import (
	"context"
	"io"
	"net/http"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func TestBuildMiniMaxVideoURL(t *testing.T) {
	tests := []struct {
		base     string
		endpoint GrokMediaEndpoint
		taskID   string
		want     string
		wantErr  bool
	}{
		{"https://api.minimax.cn", MiniMaxVideoEndpointCreate, "", "https://api.minimax.cn/v2/video_generation", false},
		{"https://api.minimaxi.com/v1/", MiniMaxVideoEndpointCreate, "", "https://api.minimaxi.com/v2/video_generation", false},
		{"https://upstream.example/v1", MiniMaxVideoEndpointStatus, "minimax:task-123", "https://upstream.example/v2/query/video_generation/task-123", false},
		{"https://upstream.example", MiniMaxVideoEndpointStatus, "minimax:a/b", "", true},
	}
	for _, test := range tests {
		got, err := BuildMiniMaxVideoURL(test.base, test.endpoint, test.taskID)
		if test.wantErr {
			require.Error(t, err)
			continue
		}
		require.NoError(t, err)
		require.Equal(t, test.want, got)
	}
}

func TestParseMiniMaxVideoRequest(t *testing.T) {
	info, err := ParseMiniMaxVideoRequest([]byte(`{"model":"video-model","duration":5,"resolution":"2K","ratio":"16:9"}`))
	require.NoError(t, err)
	require.Equal(t, "video-model", info.Model)
	require.Equal(t, 5, info.DurationSeconds)
	require.Equal(t, "2K", info.Resolution)
	require.Equal(t, "16:9", info.AspectRatio)
	_, err = ParseMiniMaxVideoRequest([]byte(`{"model":" "}`))
	require.Error(t, err)
}

func TestParseMiniMaxVideoBillingStatus(t *testing.T) {
	t.Run("parses successful nested task billing fields", func(t *testing.T) {
		result := ParseMiniMaxVideoBillingStatus([]byte(`{
			"task":{"id":"task-123","status":"Success","model":"video-upstream","resolution":"720p","duration":5,
				"usage":{"total_tokens":260390}}
		}`), "minimax:task-123")
		require.NotNil(t, result)
		require.Equal(t, "minimax:task-123", result.ResponseID)
		require.Equal(t, "video-upstream", result.Model)
		require.Equal(t, "720p", result.VideoResolution)
		require.Equal(t, 5, result.VideoDurationSeconds)
		require.Equal(t, 1, result.VideoCount)
		require.Equal(t, 260390, result.Usage.OutputTokens)
	})

	t.Run("accepts successful status aliases case insensitively", func(t *testing.T) {
		for _, status := range []string{"succeeded", "COMPLETED", "done"} {
			result := ParseMiniMaxVideoBillingStatus([]byte(`{"task":{"status":"`+status+`","model":"video-model","duration":5}}`), "task-123")
			require.NotNil(t, result, status)
		}
	})

	t.Run("ignores tasks that have not succeeded", func(t *testing.T) {
		require.Nil(t, ParseMiniMaxVideoBillingStatus([]byte(`{"task":{"status":"Processing"}}`), "task-123"))
	})
}

func TestNormalizeVideoTaskStatus(t *testing.T) {
	for input, want := range map[string]string{
		" Success ":  "succeeded",
		"COMPLETED":  "succeeded",
		"Failure":    "failed",
		"Canceled":   "failed",
		"PROCESSING": "processing",
	} {
		require.Equal(t, want, normalizeVideoTaskStatus(input))
	}
}

func TestForwardMiniMaxVideoV2(t *testing.T) {
	t.Run("creates a task using account credentials and mapped model", func(t *testing.T) {
		upstream := &grokMediaContentUpstreamStub{response: grokMediaContentStatusResponse(`{"task_id":"task-123"}`)}
		svc := &OpenAIGatewayService{cfg: &config.Config{}, httpUpstream: upstream}
		account := &Account{
			ID: 10, Platform: PlatformMiniMax, Type: AccountTypeAPIKey,
			Credentials: map[string]any{
				"api_key": "secret", "base_url": "https://upstream.example/v1",
				"model_mapping": map[string]any{"video-public": "video-upstream"},
			},
		}
		body := []byte(`{"model":"video-public","prompt":"waves","duration":5,"resolution":"2K","ratio":"16:9"}`)
		c, recorder := grokMediaContentTestContext(http.MethodPost, "/v2/video_generation", nil)

		result, err := svc.ForwardMiniMaxVideoPriced(context.Background(), c, account, MiniMaxVideoEndpointCreate, "", body, nil, nil)

		require.NoError(t, err)
		require.Equal(t, http.StatusOK, recorder.Code)
		require.Equal(t, "minimax:task-123", result.ResponseID)
		require.Equal(t, "2K", result.VideoResolution)
		require.Equal(t, 5, result.VideoDurationSeconds)
		require.Equal(t, "https://upstream.example/v2/video_generation", upstream.request.URL.String())
		require.Equal(t, http.MethodPost, upstream.request.Method)
		require.Equal(t, "Bearer secret", upstream.request.Header.Get("Authorization"))
		forwarded, err := io.ReadAll(upstream.request.Body)
		require.NoError(t, err)
		require.Equal(t, "video-upstream", gjson.GetBytes(forwarded, "model").String())
		require.Equal(t, "waves", gjson.GetBytes(forwarded, "prompt").String())
	})
	t.Run("queries the prefixed task id using MiniMax route", func(t *testing.T) {
		response := grokMediaContentStatusResponse(`{"task":{"id":"task-123","status":"running"}}`)
		upstream := &grokMediaContentUpstreamStub{response: response}
		svc := &OpenAIGatewayService{cfg: &config.Config{}, httpUpstream: upstream}
		account := &Account{
			ID: 10, Platform: PlatformMiniMax, Type: AccountTypeAPIKey,
			Credentials: map[string]any{"api_key": "secret", "base_url": "https://upstream.example"},
		}
		c, recorder := grokMediaContentTestContext(http.MethodGet, "/v2/query/video_generation/task-123", nil)

		result, err := svc.ForwardMiniMaxVideoPriced(context.Background(), c, account, MiniMaxVideoEndpointStatus, "minimax:task-123", nil, nil, nil)

		require.NoError(t, err)
		require.Equal(t, http.StatusOK, recorder.Code)
		require.Equal(t, "minimax:task-123", result.ResponseID)
		require.Equal(t, "https://upstream.example/v2/query/video_generation/task-123", upstream.request.URL.String())
		require.Equal(t, http.MethodGet, upstream.request.Method)
		require.Equal(t, "Bearer secret", upstream.request.Header.Get("Authorization"))
		require.JSONEq(t, `{"task":{"id":"task-123","status":"running"}}`, recorder.Body.String())
	})
}

func TestMiniMaxVideoTokenMeasurementAndFinalQuote(t *testing.T) {
	t.Run("uses MiniMax nested total tokens", func(t *testing.T) {
		tokens, err := measuredVideoTokens([]byte(`{"task":{"usage":{"total_tokens":260390,"prompt_tokens":13500,"completion_tokens":260390}}}`))
		require.NoError(t, err)
		require.NotNil(t, tokens)
		require.EqualValues(t, 260390, *tokens)
	})
	t.Run("rejects actual resolution without configured billing tier", func(t *testing.T) {
		quote, err := CalculateVideoTokenPricing(videoTokenTestRule(), VideoTokenPricingInput{
			Tier: "1080p", Width: 1920, Height: 1080, OutputSeconds: "15",
			InputSeconds: "0", Multiplier: "1",
		})
		require.NoError(t, err)
		job := &VideoTokenBillingJob{
			ID: "hold", Quote: quote,
			Spec: VideoTokenTaskSpec{AdaptiveRatio: true, AdaptiveDuration: true},
		}
		_, err = FinalVideoTokenQuote(job, []byte(`{"task":{"status":"succeeded","resolution":"4K","duration":5,"ratio":"16:9","usage":{"total_tokens":260390}}}`))
		require.ErrorContains(t, err, `actual video resolution "4k" has no configured price tier`)
	})
	t.Run("uses returned result to settle an adaptive request", func(t *testing.T) {
		rule := videoTokenTestRule()
		rule.Tiers["2k"] = VideoTokenPriceTier{
			WithoutVideo: "8", WithVideo: "5",
			Sizes: map[string]VideoTokenGeometry{"16:9": {Width: 1920, Height: 1080}},
		}
		quote, err := CalculateVideoTokenPricing(rule, VideoTokenPricingInput{
			Tier: "1080p", Width: 1920, Height: 1080, OutputSeconds: "15",
			InputSeconds: "0", Multiplier: "1",
		})
		require.NoError(t, err)
		job := &VideoTokenBillingJob{
			ID: "hold", Quote: quote,
			Spec: VideoTokenTaskSpec{AdaptiveRatio: true, AdaptiveDuration: true},
		}
		final, err := FinalVideoTokenQuote(job, []byte(`{"task":{"status":"succeeded","resolution":"2K","duration":5,"ratio":"16:9","usage":{"total_tokens":260390,"prompt_tokens":13500,"completion_tokens":260390}}}`))
		require.NoError(t, err)
		require.EqualValues(t, 260390, final.Tokens)
		require.Equal(t, "upstream", final.TokenSource)
		require.Equal(t, "5", final.Input.OutputSeconds)
		require.Equal(t, 1920, final.Input.Width)
		require.Equal(t, 1080, final.Input.Height)
		require.Equal(t, "2k", final.Input.Tier)
	})
}
