package handler

import (
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

func canFallbackToOpenAIMiniMaxVideo(c *gin.Context, key *service.APIKey) bool {
	if key == nil || key.Group == nil || key.Group.Platform != service.PlatformComposite {
		return false
	}
	source, _ := service.CompositeRouteSourceFromContext(c.Request.Context())
	return source == "" || source == service.CompositeRouteSourceDetector
}
