package handler

import (
	"bytes"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestPublicModelRouteRewritesBeforeAllowlisting(t *testing.T) {
	gin.SetMode(gin.TestMode)
	const groupID int64 = 9921
	handler := newGatewayModelsHandlerForTest(&gatewayModelsAccountRepoStub{
		byGroup: map[int64][]service.Account{groupID: {{
			ID: 1, Platform: service.PlatformOpenAI, Type: service.AccountTypeAPIKey,
			Status: service.StatusActive, Schedulable: true,
			Credentials: map[string]any{"model_mapping": map[string]any{"gpt-6.1-sol/hjwl": "gpt-6.1-sol"}},
		}}},
	})
	for _, allowed := range []string{"gpt-6.1-sol", "gpt-6.1-sol/hjwl"} {
		t.Run(allowed, func(t *testing.T) {
			router := gin.New()
			router.Use(func(c *gin.Context) {
				c.Set(string(middleware.ContextKeyAPIKey), &service.APIKey{
					GroupID: &groupID, Group: &service.Group{ID: groupID, Platform: service.PlatformComposite,
						ModelAllowlist: service.GroupModelAllowlist{Enabled: true, Models: []string{allowed}}},
				})
			})
			router.POST("/v1/chat/completions", func(c *gin.Context) {
				handler.ResolvePublicModelRoute(c)
			}, middleware.GroupModelAllowlist(), func(c *gin.Context) {
				body, err := io.ReadAll(c.Request.Body)
				require.NoError(t, err)
				require.Contains(t, string(body), `"model":"gpt-6.1-sol/hjwl"`)
				model, ok := service.RequestedPublicModelFromContext(c.Request.Context())
				require.True(t, ok)
				require.Equal(t, "gpt-6.1-sol/hjwl", model)
				c.Status(http.StatusNoContent)
			})
			rec := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions",
				strings.NewReader(`{"model":"gpt-6.1-sol","messages":[]}`))
			req.Header.Set("Content-Type", "application/json")
			router.ServeHTTP(rec, req)
			require.Equal(t, http.StatusNoContent, rec.Code, rec.Body.String())
		})
	}
}

func TestPublicModelRouteMultipartPreservesReferenceImage(t *testing.T) {
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	require.NoError(t, writer.WriteField("model", "gpt-image-2"))
	require.NoError(t, writer.WriteField("prompt", "edit this"))
	part, err := writer.CreateFormFile("image", "reference.png")
	require.NoError(t, err)
	image := []byte{0, 255, 1, 2, 3, 4}
	_, err = part.Write(image)
	require.NoError(t, err)
	require.NoError(t, writer.Close())
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/images/edits", nil)
	c.Request.Header.Set("Content-Type", writer.FormDataContentType())
	result, contentType, err := rewritePublicModelBody(c, body.Bytes(), "gpt-image-2/hjwl")
	require.NoError(t, err)
	request := httptest.NewRequest(http.MethodPost, "/v1/images/edits", bytes.NewReader(result))
	request.Header.Set("Content-Type", contentType)
	require.NoError(t, request.ParseMultipartForm(1<<20))
	defer request.MultipartForm.RemoveAll()
	require.Equal(t, "gpt-image-2/hjwl", request.FormValue("model"))
	require.Equal(t, "edit this", request.FormValue("prompt"))
	file, header, err := request.FormFile("image")
	require.NoError(t, err)
	defer file.Close()
	data, err := io.ReadAll(file)
	require.NoError(t, err)
	require.Equal(t, image, data)
	require.Equal(t, "reference.png", header.Filename)
}

func TestPublicModelListDeduplicatesChannels(t *testing.T) {
	c, rec := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodGet, "/v1/models", nil)
	writeAllowlistedModelsList(c, service.PlatformOpenAI, []string{"gpt-6.1-sol/a", "gpt-6.1-sol/b"})
	require.Equal(t, http.StatusOK, rec.Code)
	require.Equal(t, 1, strings.Count(rec.Body.String(), `"id":"gpt-6.1-sol"`))
	require.NotContains(t, rec.Body.String(), "/a")
}
