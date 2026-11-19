package service

import "context"

const (
	OpenAIEndpointCapabilityGeminiImages      OpenAIEndpointCapability = "gemini_images"
	OpenAIEndpointCapabilityMiniMaxVideo      OpenAIEndpointCapability = "minimax_video"
	OpenAIEndpointCapabilityAnthropicMessages OpenAIEndpointCapability = "anthropic_messages"
)

// ResolveMessagesEndpointCapability keeps explicitly enabled native Messages
// accounts separate from the existing Messages-to-GPT dispatch pool.
func (s *OpenAIGatewayService) ResolveMessagesEndpointCapability(ctx context.Context, groupID *int64, model, platform string) (OpenAIEndpointCapability, error) {
	if NormalizeOpenAICompatiblePlatform(platform) != PlatformOpenAI {
		return OpenAIEndpointCapabilityChatCompletions, nil
	}
	accounts, err := s.listSchedulableAccounts(ctx, groupID, platform)
	if err != nil {
		return "", err
	}
	for i := range accounts {
		account := &accounts[i]
		if !account.SupportsOpenAIEndpointCapability(OpenAIEndpointCapabilityAnthropicMessages) ||
			!supportsOpenAIRequestModel(account, model, false) {
			continue
		}
		if source, ok := CompositeRouteSourceFromContext(ctx); ok && source == CompositeRouteSourceAccount {
			if publicModel, found := RequestedPublicModelFromContext(ctx); found && !explicitModelMappingClaims(*account, publicModel) {
				continue
			}
		}
		return OpenAIEndpointCapabilityAnthropicMessages, nil
	}
	return OpenAIEndpointCapabilityChatCompletions, nil
}

func supportsMiniMaxVideoAccount(account *Account) bool {
	return account != nil && account.Type == AccountTypeAPIKey &&
		((account.Platform == PlatformMiniMax && account.IsMultiProtocolAPIKey()) ||
			account.SupportsOpenAIEndpointCapability(OpenAIEndpointCapabilityMiniMaxVideo))
}
