package service

const (
	OpenAIEndpointCapabilityGeminiImages OpenAIEndpointCapability = "gemini_images"
	OpenAIEndpointCapabilityMiniMaxVideo OpenAIEndpointCapability = "minimax_video"
)

func supportsMiniMaxVideoAccount(account *Account) bool {
	return account != nil && account.Type == AccountTypeAPIKey &&
		((account.Platform == PlatformMiniMax && account.IsMultiProtocolAPIKey()) ||
			account.SupportsOpenAIEndpointCapability(OpenAIEndpointCapabilityMiniMaxVideo))
}
