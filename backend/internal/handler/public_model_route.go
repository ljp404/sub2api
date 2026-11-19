package handler

import (
	"bytes"
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/pkg/requestmodel"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
)

// ResolvePublicModelRoute runs after authentication and before allowlisting and
// composite dispatch. All later handlers see the selected configured alias,
// including the existing reservation and settlement code.
func (h *GatewayHandler) ResolvePublicModelRoute(c *gin.Context) {
	if h == nil || h.gatewayService == nil || c.Request.Method != http.MethodPost {
		return
	}
	apiKey, ok := middleware.GetAPIKeyFromContext(c)
	if !ok || apiKey == nil || apiKey.Group == nil {
		return
	}
	model := groupRouteModelFromParams(c)
	var body []byte
	if model == "" {
		models, done := middleware.ReadPublicModelRouteBody(c)
		if !done {
			return
		}
		body = models.Body
		if len(models.Models) != 1 {
			return
		}
		model = models.Models[0]
	}
	group := apiKey.Group
	if forced, exists := middleware.GetForcePlatformFromContext(c); exists && forced != "" {
		copy := *group
		copy.Platform = forced
		group = &copy
	}
	selected, platform, err := h.gatewayService.ResolvePublicModelRoute(c.Request.Context(), group, model, c.Request.URL.Path)
	if err != nil {
		c.AbortWithStatusJSON(http.StatusServiceUnavailable, gin.H{"error": gin.H{
			"type": "model_routing_error", "message": "Unable to resolve model channels",
		}})
		return
	}
	if platform == "" {
		return
	}
	if len(body) > 0 {
		rewritten, contentType, err := rewritePublicModelBody(c, body, selected)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": gin.H{
				"type": "invalid_request_error", "message": "Unable to rewrite request model",
			}})
			return
		}
		requestmodel.ResetRequestBody(c.Request, rewritten)
		c.Request.Header.Set("Content-Type", contentType)
	}
	// Keep the internal alias in the account-ownership context. Using the base
	// name here would exclude the accounts that explicitly expose the alias.
	c.Request = c.Request.WithContext(service.WithCompositeRouteDecision(c.Request.Context(), service.CompositeRouteDecision{
		Matched: true, Source: service.CompositeRouteSourceAccount, PublicModel: selected,
		UpstreamModel: selected, TargetPlatform: platform,
	}))
}

func groupRouteModelFromParams(c *gin.Context) string {
	if model := c.Param("model"); model != "" {
		return model
	}
	if action := strings.TrimPrefix(c.Param("modelAction"), "/"); action != "" {
		if colon := strings.LastIndex(action, ":"); colon >= 0 {
			return action[:colon]
		}
	}
	return ""
}

func rewritePublicModelBody(c *gin.Context, body []byte, selected string) ([]byte, string, error) {
	contentType := c.GetHeader("Content-Type")
	if gjson.ValidBytes(body) {
		_, path := requestmodel.JSONModelPathForRoute(c.FullPath(), body)
		if path == "" {
			return nil, "", fmt.Errorf("missing model")
		}
		out, err := sjson.SetBytes(body, path, selected)
		return out, contentType, err
	}
	mediaType, params, err := mime.ParseMediaType(contentType)
	if err != nil || mediaType != "multipart/form-data" {
		return nil, "", fmt.Errorf("unsupported body")
	}
	reader := multipart.NewReader(bytes.NewReader(body), params["boundary"])
	var out bytes.Buffer
	writer := multipart.NewWriter(&out)
	for {
		part, err := reader.NextPart()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, "", err
		}
		target, err := writer.CreatePart(part.Header)
		if err != nil {
			return nil, "", err
		}
		if part.FormName() == "model" && part.FileName() == "" {
			_, err = io.WriteString(target, selected)
		} else {
			_, err = io.Copy(target, part)
		}
		if err != nil {
			return nil, "", err
		}
	}
	if err := writer.Close(); err != nil {
		return nil, "", err
	}
	return out.Bytes(), writer.FormDataContentType(), nil
}
