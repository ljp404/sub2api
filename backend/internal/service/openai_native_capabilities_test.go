//go:build unit

package service

import (
	"context"
	"net/http"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/stretchr/testify/require"
)

func TestOpenAINativeCapabilitiesRequireExplicitOptIn(t *testing.T) {
	for _, capability := range []OpenAIEndpointCapability{OpenAIEndpointCapabilityGeminiImages, OpenAIEndpointCapabilityMiniMaxVideo} {
		t.Run(string(capability), func(t *testing.T) {
			account := &Account{Platform: PlatformOpenAI, Type: AccountTypeAPIKey,
				Credentials: map[string]any{"base_url": "https://upstream.example/v1"}}
			require.False(t, account.SupportsOpenAIEndpointCapability(capability))
			account.Credentials["openai_capabilities"] = []string{string(capability)}
			require.True(t, account.SupportsOpenAIEndpointCapability(capability))
			account.Platform = PlatformGemini
			require.False(t, account.SupportsOpenAIEndpointCapability(capability))
			account.Platform = PlatformOpenAI
			account.Type = AccountTypeOAuth
			require.False(t, account.SupportsOpenAIEndpointCapability(capability))
			account.Type = AccountTypeAPIKey
			account.Credentials["base_url"] = ""
			require.False(t, account.SupportsOpenAIEndpointCapability(capability))
		})
	}
}

func TestBulkOpenAINativeCapabilitiesPreservesTwoNativeOptions(t *testing.T) {
	value, _, err := normalizeBulkOpenAIEndpointCapabilities([]any{"minimax_video", "gemini_images", "gemini_images"})
	require.NoError(t, err)
	require.Equal(t, []string{"gemini_images", "minimax_video"}, value)
	value, _, err = normalizeBulkOpenAIEndpointCapabilities([]any{"chat_completions", "embeddings", "seedance"})
	require.NoError(t, err)
	require.Equal(t, []string{"chat_completions", "embeddings", "seedance"}, value)
}

func TestOpenAIGeminiNativeForwardUsesExistingProtocol(t *testing.T) {
	svc, upstream := newGeminiVertexAPIKeyTestService()
	account := &Account{ID: 17, Platform: PlatformOpenAI, Type: AccountTypeAPIKey,
		Credentials: map[string]any{
			"base_url": "https://upstream.example/v1", "api_key": "native-key",
			"openai_capabilities": []string{"gemini_images"},
			"model_mapping": map[string]any{"gemini-public-image": "gemini-3-pro-image"},
		}}
	c, _ := newGeminiNativeTestContext(t)
	result, err := svc.ForwardNative(context.Background(), c, account, "gemini-public-image", "generateContent", false,
		[]byte(`{"contents":[{"parts":[{"text":"draw snow"}]}],"generationConfig":{"responseModalities":["IMAGE"]}}`))
	require.NoError(t, err)
	require.NotNil(t, result)
	require.Equal(t, "https://upstream.example/v1beta/models/gemini-3-pro-image:generateContent", upstream.lastReq.URL.String())
	require.Equal(t, "native-key", upstream.lastReq.Header.Get("x-goog-api-key"))
	require.Empty(t, upstream.lastReq.Header.Get("Authorization"))

	for _, test := range []struct {
		name  string
		model string
		caps  []string
	}{
		{"not opted in", "gemini-3-pro-image", []string{"chat_completions"}},
		{"text request", "gemini-3-flash", []string{"gemini_images"}},
	} {
		t.Run(test.name, func(t *testing.T) {
			blockedSvc, blockedUpstream := newGeminiVertexAPIKeyTestService()
			account.Credentials["openai_capabilities"] = test.caps
			ctx, _ := newGeminiNativeTestContext(t)
			_, err := blockedSvc.ForwardNative(context.Background(), ctx, account, test.model, "generateContent", false,
				[]byte(`{"contents":[{"parts":[{"text":"hello"}]}]}`))
			require.Error(t, err)
			require.Zero(t, blockedUpstream.calls)
		})
	}
}

func TestOpenAIMiniMaxNativeForwardUsesExistingProtocol(t *testing.T) {
	upstream := &grokMediaContentUpstreamStub{response: grokMediaContentStatusResponse(`{"task_id":"native-task"}`)}
	svc := &OpenAIGatewayService{cfg: &config.Config{}, httpUpstream: upstream}
	account := &Account{ID: 17, Platform: PlatformOpenAI, Type: AccountTypeAPIKey,
		Credentials: map[string]any{
			"base_url": "https://upstream.example/v1", "api_key": "native-key",
			"openai_capabilities": []string{"minimax_video"},
		}}
	c, _ := grokMediaContentTestContext(http.MethodPost, "/v2/video_generation", nil)
	result, err := svc.ForwardMiniMaxVideoPriced(context.Background(), c, account, MiniMaxVideoEndpointCreate, "",
		[]byte(`{"model":"MiniMax-H3","prompt":"snow","duration":5,"resolution":"2K"}`), nil, nil)
	require.NoError(t, err)
	require.Equal(t, "minimax:native-task", result.ResponseID)
	require.Equal(t, "https://upstream.example/v2/video_generation", upstream.request.URL.String())
	require.Equal(t, "Bearer native-key", upstream.request.Header.Get("Authorization"))
	account.Credentials["openai_capabilities"] = []string{"gemini_images"}
	require.False(t, supportsMiniMaxVideoAccount(account))
}
