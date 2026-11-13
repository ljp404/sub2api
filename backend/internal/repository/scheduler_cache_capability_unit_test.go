//go:build unit

package repository

import (
	"context"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestSchedulerCacheSnapshotPreservesOpenAIEndpointCapabilities(t *testing.T) {
	for _, tt := range []struct {
		name         string
		capabilities []string
		seedance     bool
		chat         bool
		embeddings   bool
	}{
		{name: "seedance only", capabilities: []string{"seedance"}, seedance: true},
		{name: "embeddings only", capabilities: []string{"embeddings"}, embeddings: true},
		{name: "chat and seedance", capabilities: []string{"chat_completions", "seedance"}, seedance: true, chat: true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			cache := newSchedulerCacheUnit(t)
			account := service.Account{
				ID:          18,
				Platform:    service.PlatformOpenAI,
				Type:        service.AccountTypeAPIKey,
				Status:      service.StatusActive,
				Schedulable: true,
				GroupIDs:    []int64{11},
				Credentials: map[string]any{
					"base_url":            "https://ark-compatible.example",
					"openai_capabilities": tt.capabilities,
				},
			}
			bucket := service.SchedulerBucket{GroupID: 11, Platform: service.PlatformOpenAI, Mode: service.SchedulerModeSingle}
			token, err := cache.CaptureBucketWriteToken(ctx, bucket)
			require.NoError(t, err)
			require.NoError(t, cache.SetSnapshot(ctx, bucket, token, []service.Account{account}))

			snapshot, hit, err := cache.GetSnapshot(ctx, bucket)
			require.NoError(t, err)
			require.True(t, hit)
			require.Len(t, snapshot, 1)
			full, err := cache.GetAccount(ctx, account.ID)
			require.NoError(t, err)
			require.NotNil(t, full)
			for _, candidate := range []*service.Account{full, snapshot[0]} {
				require.Equal(t, tt.seedance, candidate.SupportsOpenAIEndpointCapability(service.OpenAIEndpointCapabilitySeedance))
				require.Equal(t, tt.chat, candidate.SupportsOpenAIEndpointCapability(service.OpenAIEndpointCapabilityChatCompletions))
				require.Equal(t, tt.embeddings, candidate.SupportsOpenAIEndpointCapability(service.OpenAIEndpointCapabilityEmbeddings))
			}
		})
	}
}
