package handler

import (
	"context"
	"mime"
	"net/http"
	"time"

	middleware "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

// SeedanceTasks exposes Ark's native asynchronous video task protocol.
func (h *OpenAIGatewayHandler) SeedanceTasks(c *gin.Context) {
	c.Request = c.Request.WithContext(service.WithSeedanceEndpoint(c.Request.Context()))
	if c.Request.Method == http.MethodPost && c.GetHeader("Content-Type") != "" {
		mediaType, _, err := mime.ParseMediaType(c.GetHeader("Content-Type"))
		if err != nil || mediaType != "application/json" {
			h.errorResponse(c, http.StatusUnsupportedMediaType, "invalid_request_error", "Seedance requires application/json")
			return
		}
	}
	key, ok := middleware.GetAPIKeyFromContext(c)
	if !ok || key.Group == nil || (key.Group.Platform != service.PlatformOpenAI && key.Group.Platform != service.PlatformComposite) {
		h.errorResponse(c, http.StatusForbidden, "permission_error", "Seedance requires an OpenAI or composite group")
		return
	}
	endpoint := service.SeedanceEndpointCreate
	setActualUpstreamEndpoint(c, EndpointSeedanceTasks)
	taskID := ""
	if c.Request.Method != http.MethodPost {
		taskID = service.SeedanceTaskKey(c.Param("task_id"))
		endpoint = service.SeedanceEndpointStatus
		if c.Request.Method == http.MethodDelete {
			endpoint = service.SeedanceEndpointDelete
		}
	}
	h.handleGrokMedia(c, endpoint, taskID)
}

// MiniMaxVideoTasks exposes MiniMax Video V2's native create/query contract.
func (h *OpenAIGatewayHandler) MiniMaxVideoTasks(c *gin.Context) {
	c.Request = c.Request.WithContext(service.WithVideoTokenEndpoint(c.Request.Context()))
	endpoint := service.MiniMaxVideoEndpointCreate
	taskID := service.MiniMaxVideoTaskKey("")
	if c.Request.Method == http.MethodGet {
		endpoint = service.MiniMaxVideoEndpointStatus
		rawTaskID := c.Param("task_id")
		if rawTaskID == "" {
			h.errorResponse(c, http.StatusBadRequest, "invalid_request_error", "task_id is required")
			return
		}
		taskID = service.MiniMaxVideoTaskKey(rawTaskID)
		setActualUpstreamEndpoint(c, "/v2/query/video_generation/{task_id}")
	} else if c.Request.Method == http.MethodPost {
		setActualUpstreamEndpoint(c, "/v2/video_generation")
	} else {
		h.errorResponse(c, http.StatusMethodNotAllowed, "invalid_request_error", "MiniMax Video V2 only supports POST create and GET query")
		return
	}
	h.handleGrokMedia(c, endpoint, taskID)
}

// Legacy Ark token pricing remains unchanged. Opt-in formula jobs have their
// own durable reservation/settlement and must not enter this legacy path.
func prepareSeedanceCompletionBilling(ctx context.Context, h *OpenAIGatewayHandler, key *service.APIKey, subject middleware.AuthSubject, taskID string, result *service.OpenAIForwardResult) *service.OpenAIForwardResult {
	if result == nil || result.VideoTokenManaged || result.Usage.OutputTokens <= 0 {
		return nil
	}
	pending, err := h.gatewayService.LoadGrokVideoPendingBilling(ctx, taskID, subject.UserID, key.ID)
	if err != nil || pending == nil {
		return nil
	}
	claimed, err := h.gatewayService.ClaimGrokVideoBilling(ctx, taskID, subject.UserID, key.ID)
	if err != nil || !claimed {
		return nil
	}
	merged := *result
	merged.Model = pending.Model
	merged.BillingModel = firstNonEmptyString(pending.BillingModel, pending.Model)
	merged.UpstreamModel = firstNonEmptyString(pending.UpstreamModel, result.UpstreamModel)
	merged.RequestID = service.StableGrokVideoBillingRequestID(taskID)
	merged.ResponseID = taskID
	merged.Duration = service.GrokVideoE2EDuration(pending.CreatedAt, time.Now())
	return &merged
}
