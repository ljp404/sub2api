//go:build unit

package service

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func nativeMessagesCapabilityAccount() *Account {
	return &Account{
		ID: 35, Platform: PlatformOpenAI, Type: AccountTypeAPIKey,
		Credentials: map[string]any{
			"base_url":            "https://upstream.example/v1",
			"api_key":             "upstream-native-key",
			"openai_capabilities": []string{"anthropic_messages"},
			"model_mapping":       map[string]any{"claude-opus-4-7/supplier": "claude-opus-4-7"},
		},
	}
}

func TestResolveMessagesEndpointCapabilityPreservesLegacyDispatch(t *testing.T) {
	groupID := int64(12)
	account := nativeMessagesCapabilityAccount()
	svc := &OpenAIGatewayService{accountRepo: schedulerTestOpenAIAccountRepo{accounts: []Account{*account}}}
	capability, err := svc.ResolveMessagesEndpointCapability(context.Background(), &groupID, "claude-opus-4-7/supplier", PlatformOpenAI)
	require.NoError(t, err)
	require.Equal(t, OpenAIEndpointCapabilityAnthropicMessages, capability)

	capability, err = svc.ResolveMessagesEndpointCapability(context.Background(), &groupID, "gpt-5.4", PlatformOpenAI)
	require.NoError(t, err)
	require.Equal(t, OpenAIEndpointCapabilityChatCompletions, capability)

	account.Credentials["openai_capabilities"] = []string{"chat_completions"}
	capability, err = svc.ResolveMessagesEndpointCapability(context.Background(), &groupID, "claude-opus-4-7/supplier", PlatformOpenAI)
	require.NoError(t, err)
	require.Equal(t, OpenAIEndpointCapabilityChatCompletions, capability)
	require.Equal(t, "gpt-5.4", (&Group{Platform: PlatformOpenAI}).ResolveMessagesDispatchModel("claude-opus-4-7"))
}

func TestBulkOpenAIAnthropicCapability(t *testing.T) {
	value, includeChat, err := normalizeBulkOpenAIEndpointCapabilities([]any{"anthropic_messages", "chat_completions", "anthropic_messages"})
	require.NoError(t, err)
	require.True(t, includeChat)
	require.Equal(t, []string{"chat_completions", "anthropic_messages"}, value)
}

func TestOpenAIAnthropicCapabilityForwardsNativeMessages(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, streaming := range []bool{false, true} {
		name := "buffered"
		responseBody := `{"id":"msg_native","type":"message","role":"assistant","model":"claude-opus-4-7","content":[{"type":"text","text":"OK"}],"stop_reason":"end_turn","usage":{"input_tokens":21,"output_tokens":6}}`
		contentType := "application/json"
		streamJSON := "false"
		if streaming {
			name = "streaming"
			streamJSON = "true"
			contentType = "text/event-stream"
			responseBody = "event: message_start\ndata: {\"type\":\"message_start\",\"message\":{\"id\":\"msg_native\",\"type\":\"message\",\"role\":\"assistant\",\"model\":\"claude-opus-4-7\",\"content\":[],\"usage\":{\"input_tokens\":21,\"output_tokens\":0}}}\n\n" +
				"event: content_block_start\ndata: {\"type\":\"content_block_start\",\"index\":0,\"content_block\":{\"type\":\"text\",\"text\":\"\"}}\n\n" +
				"event: content_block_delta\ndata: {\"type\":\"content_block_delta\",\"index\":0,\"delta\":{\"type\":\"text_delta\",\"text\":\"OK\"}}\n\n" +
				"event: content_block_stop\ndata: {\"type\":\"content_block_stop\",\"index\":0}\n\n" +
				"event: message_delta\ndata: {\"type\":\"message_delta\",\"delta\":{\"stop_reason\":\"end_turn\"},\"usage\":{\"output_tokens\":6}}\n\n" +
				"event: message_stop\ndata: {\"type\":\"message_stop\"}\n\n"
		}
		t.Run(name, func(t *testing.T) {
			body := []byte(`{"model":"claude-opus-4-7/supplier","max_tokens":32,"stream":` + streamJSON + `,"messages":[{"role":"user","content":"hello"}]}`)
			rec := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(rec)
			c.Request = httptest.NewRequest(http.MethodPost, "/v1/messages", bytes.NewReader(body))
			c.Request.Header.Set("Authorization", "Bearer downstream-secret")
			c.Request.Header.Set("x-api-key", "downstream-secret")
			upstream := &httpUpstreamRecorder{resp: &http.Response{
				StatusCode: http.StatusOK, Header: http.Header{"Content-Type": []string{contentType}},
				Body: io.NopCloser(strings.NewReader(responseBody)),
			}}
			svc := &OpenAIGatewayService{cfg: rawChatCompletionsTestConfig(), httpUpstream: upstream}
			result, err := svc.ForwardAsAnthropic(context.Background(), c, nativeMessagesCapabilityAccount(), body, "", "gpt-5.4")
			require.NoError(t, err)
			require.NotNil(t, result)
			require.Equal(t, "https://upstream.example/v1/messages", upstream.lastReq.URL.String())
			require.Equal(t, "upstream-native-key", upstream.lastReq.Header.Get("x-api-key"))
			require.Empty(t, upstream.lastReq.Header.Get("Authorization"))
			require.Equal(t, "2023-06-01", upstream.lastReq.Header.Get("anthropic-version"))
			require.Equal(t, "claude-opus-4-7", gjson.GetBytes(upstream.lastBody, "model").String())
			require.True(t, gjson.GetBytes(upstream.lastBody, "messages").IsArray())
			require.False(t, gjson.GetBytes(upstream.lastBody, "input").Exists())
			require.Equal(t, "/v1/messages", result.UpstreamEndpoint)
			require.Equal(t, 21, result.Usage.InputTokens)
			require.Equal(t, 6, result.Usage.OutputTokens)
			require.Equal(t, streaming, result.Stream)
			require.Contains(t, rec.Body.String(), "OK")
		})
	}
}

func TestOpenAIAnthropicCapabilityEndpointAndCountTokens(t *testing.T) {
	account := nativeMessagesCapabilityAccount()
	for _, base := range []string{"https://upstream.example", "https://upstream.example/v1/"} {
		account.Credentials["base_url"] = base
		svc := &OpenAIGatewayService{cfg: rawChatCompletionsTestConfig()}
		target, err := svc.nativeAnthropicTargetURL(account)
		require.NoError(t, err)
		require.Equal(t, "https://upstream.example/v1/messages", target)
	}
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	err := (&OpenAIGatewayService{}).ForwardCountTokensAsAnthropic(context.Background(), c, account,
		[]byte(`{"model":"claude-opus-4-7/supplier","messages":[{"role":"user","content":"hello"}]}`), "gpt-5.4")
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, rec.Code)
	require.Positive(t, gjson.GetBytes(rec.Body.Bytes(), "input_tokens").Int())
}
