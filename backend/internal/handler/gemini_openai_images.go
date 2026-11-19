package handler

import (
	"errors"

	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

func canFallbackToOpenAIGeminiImages(c *gin.Context, key *service.APIKey, imageIntent bool) bool {
	if !imageIntent || middleware.HasForcePlatform(c) || key == nil || key.Group == nil ||
		key.Group.Platform != service.PlatformComposite || effectiveAPIKeyPlatform(c, key) != service.PlatformGemini {
		return false
	}
	source, _ := service.CompositeRouteSourceFromContext(c.Request.Context())
	// Explicit routes and account ownership remain authoritative.
	return source == "" || source == service.CompositeRouteSourceDetector
}

func (h *GatewayHandler) selectGeminiNativeAccount(c *gin.Context, key *service.APIKey, sessionKey, model string, excluded map[int64]struct{}, imageIntent bool) (*service.AccountSelectionResult, error) {
	selectOpenAI := func() (*service.AccountSelectionResult, error) {
		if h.openAIGatewayService == nil {
			return nil, service.ErrNoAvailableAccounts
		}
		selection, _, err := h.openAIGatewayService.SelectAccountWithSchedulerForCapability(
			c.Request.Context(), key.GroupID, "", sessionKey, model, excluded,
			service.OpenAIUpstreamTransportHTTPSSE, service.OpenAIEndpointCapabilityGeminiImages,
			false, false, false, service.PlatformOpenAI,
		)
		return selection, err
	}
	if !middleware.HasForcePlatform(c) && effectiveAPIKeyPlatform(c, key) == service.PlatformOpenAI {
		return selectOpenAI()
	}
	selection, err := h.gatewayService.SelectAccountWithLoadAwareness(c.Request.Context(), key.GroupID, sessionKey, model, excluded, "", 0)
	if errors.Is(err, service.ErrNoAvailableAccounts) && canFallbackToOpenAIGeminiImages(c, key, imageIntent) {
		return selectOpenAI()
	}
	return selection, err
}
