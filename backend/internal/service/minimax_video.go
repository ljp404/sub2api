package service

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/logger"
	"github.com/gin-gonic/gin"
	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
)

const (
	MiniMaxVideoEndpointCreate GrokMediaEndpoint = "minimax_video_create"
	MiniMaxVideoEndpointStatus GrokMediaEndpoint = "minimax_video_status"
)

func (e GrokMediaEndpoint) IsMiniMaxVideo() bool {
	return e == MiniMaxVideoEndpointCreate || e == MiniMaxVideoEndpointStatus
}

func MiniMaxVideoTaskKey(id string) string {
	id = strings.TrimSpace(id)
	if strings.HasPrefix(id, "minimax:") {
		return id
	}
	return "minimax:" + id
}

func ParseMiniMaxVideoRequest(body []byte) (GrokMediaRequestInfo, error) {
	var info GrokMediaRequestInfo
	if !gjson.ValidBytes(body) || !gjson.ParseBytes(body).IsObject() {
		return info, fmt.Errorf("request body must be a JSON object")
	}
	model := gjson.GetBytes(body, "model")
	if model.Type != gjson.String || strings.TrimSpace(model.String()) == "" {
		return info, fmt.Errorf("model is required")
	}
	info.Model = strings.TrimSpace(model.String())
	info.DurationSeconds = int(gjson.GetBytes(body, "duration").Int())
	info.Resolution = strings.TrimSpace(gjson.GetBytes(body, "resolution").String())
	info.AspectRatio = strings.TrimSpace(gjson.GetBytes(body, "ratio").String())
	return info, nil
}

func ParseMiniMaxVideoBillingStatus(body []byte, taskID string) *OpenAIForwardResult {
	if !gjson.ValidBytes(body) || !gjson.ParseBytes(body).IsObject() {
		return nil
	}
	read := func(paths ...string) gjson.Result {
		for _, path := range paths {
			value := gjson.GetBytes(body, path)
			if value.Exists() {
				return value
			}
		}
		return gjson.Result{}
	}
	status := normalizeVideoTaskStatus(read("task.status", "status").String())
	if status != "succeeded" {
		return nil
	}

	result := &OpenAIForwardResult{
		ResponseID: MiniMaxVideoTaskKey(taskID),
		Model:      strings.TrimSpace(read("task.model", "model").String()),
		VideoCount: 1,
	}
	result.BillingModel = result.Model
	result.UpstreamModel = result.Model
	result.VideoResolution = strings.TrimSpace(read("task.resolution", "resolution").String())
	if duration := read("task.duration", "duration", "task.video.duration", "video.duration"); duration.Type == gjson.Number {
		seconds := duration.Float()
		if seconds > 0 && seconds <= 3600 {
			result.VideoDurationSeconds = int(seconds)
		}
	}
	if tokens, err := measuredVideoTokens(body); err == nil && tokens != nil && *tokens <= int64(^uint(0)>>1) {
		result.Usage.OutputTokens = int(*tokens)
	}
	return result
}

func BuildMiniMaxVideoURL(base string, endpoint GrokMediaEndpoint, taskID string) (string, error) {
	base = strings.TrimRight(strings.TrimSpace(base), "/")
	if strings.HasSuffix(base, "/v1") {
		base = strings.TrimRight(strings.TrimSuffix(base, "/v1"), "/")
	}
	if endpoint == MiniMaxVideoEndpointCreate {
		return base + "/v2/video_generation", nil
	}
	if endpoint != MiniMaxVideoEndpointStatus {
		return "", fmt.Errorf("unsupported MiniMax video endpoint")
	}
	taskID = strings.TrimPrefix(strings.TrimSpace(taskID), "minimax:")
	if err := validateUpstreamPathSegment("MiniMax task ID", taskID); err != nil || taskID == "" {
		return "", fmt.Errorf("invalid MiniMax task ID")
	}
	return base + "/v2/query/video_generation/" + taskID, nil
}

func (s *OpenAIGatewayService) ForwardMiniMaxVideoPriced(
	ctx context.Context,
	c *gin.Context,
	account *Account,
	endpoint GrokMediaEndpoint,
	taskID string,
	body []byte,
	job *VideoTokenBillingJob,
	keys *APIKeyService,
) (*OpenAIForwardResult, error) {
	sent, bound, rejected := false, false, false
	if endpoint == MiniMaxVideoEndpointCreate && job != nil {
		defer func() {
			if bound {
				return
			}
			cleanupCtx, done := context.WithTimeout(context.WithoutCancel(ctx), 10*time.Second)
			defer done()
			var err error
			if !sent || rejected {
				err = s.videoTokenRepo().ReleaseVideoTokenJob(cleanupCtx, job.ID)
				s.invalidateVideoTokenBalance(cleanupCtx, job.UserID)
			} else {
				err = s.videoTokenRepo().MarkVideoTokenJob(cleanupCtx, job.ID, "uncertain", "create outcome unknown; reconcile upstream before releasing funds")
			}
			if err != nil {
				logger.LegacyPrintf("service.minimax_video", "create hold cleanup failed job=%s: %v", job.ID, err)
			}
		}()
	}
	if account == nil || account.Platform != PlatformMiniMax || !account.IsMultiProtocolAPIKey() || account.Type != AccountTypeAPIKey {
		return nil, fmt.Errorf("MiniMax video requires a MiniMax API key account")
	}
	if endpoint != MiniMaxVideoEndpointCreate && endpoint != MiniMaxVideoEndpointStatus {
		return nil, fmt.Errorf("unsupported MiniMax video endpoint")
	}
	base, err := s.validateUpstreamBaseURL(account.GetOpenAIBaseURL())
	if err != nil {
		return nil, fmt.Errorf("MiniMax video upstream base URL is invalid")
	}
	target, err := BuildMiniMaxVideoURL(base, endpoint, taskID)
	if err != nil {
		return nil, err
	}
	method := http.MethodGet
	model, upstreamModel := "", ""
	videoResolution, videoDurationSeconds := "", 0
	if endpoint == MiniMaxVideoEndpointCreate {
		info, parseErr := ParseMiniMaxVideoRequest(body)
		if parseErr != nil {
			return nil, parseErr
		}
		model = info.Model
		videoResolution = strings.TrimSpace(info.Resolution)
		videoDurationSeconds = info.DurationSeconds
		upstreamModel = account.GetMappedModel(model)
		if job != nil {
			upstreamModel = job.UpstreamModel
		}
		body, err = sjson.SetBytes(body, "model", upstreamModel)
		if err != nil {
			return nil, err
		}
		method = http.MethodPost
	}
	token := strings.TrimSpace(account.GetCredential("api_key"))
	if token == "" {
		return nil, fmt.Errorf("MiniMax account is missing api_key")
	}
	upstreamCtx, releaseUpstreamCtx := detachUpstreamContext(ctx)
	defer releaseUpstreamCtx()
	req, err := http.NewRequestWithContext(upstreamCtx, method, target, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/json")
	if method == http.MethodPost {
		req.Header.Set("Content-Type", "application/json")
	}
	account.ApplyHeaderOverrides(req.Header)
	proxy := ""
	if account.ProxyID != nil && account.Proxy != nil {
		proxy = account.Proxy.URL()
	}
	started := time.Now()
	sent = method == http.MethodPost
	resp, err := s.httpUpstream.Do(req, proxy, account.ID, account.Concurrency)
	SetOpsLatencyMs(c, OpsUpstreamLatencyMsKey, time.Since(started).Milliseconds())
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	responseBody, err := ReadUpstreamResponseBody(resp.Body, s.cfg, c, openAITooLargeError)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode >= 300 {
		rejected = resp.StatusCode >= 400 && resp.StatusCode < 500 && resp.StatusCode != http.StatusRequestTimeout
		writeGrokMediaResponse(c, resp, responseBody, s.responseHeaderFilter)
		return nil, fmt.Errorf("MiniMax video upstream status %d", resp.StatusCode)
	}
	result := &OpenAIForwardResult{
		Model: model, BillingModel: model, UpstreamModel: upstreamModel,
		Duration: time.Since(started), ResponseHeaders: resp.Header.Clone(),
		VideoResolution: videoResolution, VideoDurationSeconds: videoDurationSeconds,
	}
	if endpoint == MiniMaxVideoEndpointCreate {
		id := strings.TrimSpace(gjson.GetBytes(responseBody, "task_id").String())
		if id == "" {
			return nil, fmt.Errorf("MiniMax video create response is missing task_id")
		}
		result.ResponseID = MiniMaxVideoTaskKey(id)
		if job != nil {
			persistCtx, done := context.WithTimeout(context.WithoutCancel(ctx), 10*time.Second)
			defer done()
			if err := s.videoTokenRepo().BindVideoTokenJob(persistCtx, job.ID, result.ResponseID); err != nil {
				logger.LegacyPrintf("service.minimax_video", "task binding failed job=%s task=%s: %v", job.ID, result.ResponseID, err)
				return nil, fmt.Errorf("video billing task binding failed")
			}
			bound = true
		}
	} else {
		result.ResponseID = taskID
		result.UpstreamModel = strings.TrimSpace(gjson.GetBytes(responseBody, "task.model").String())
		if billingResult := ParseMiniMaxVideoBillingStatus(responseBody, taskID); billingResult != nil {
			result.Model = billingResult.Model
			result.BillingModel = billingResult.BillingModel
			result.VideoCount = billingResult.VideoCount
			result.VideoResolution = billingResult.VideoResolution
			result.VideoDurationSeconds = billingResult.VideoDurationSeconds
			result.Usage = billingResult.Usage
		}
		if job != nil {
			if keys != nil {
				if err := s.ProcessVideoTokenStatus(ctx, keys, job, responseBody); err != nil {
					logger.LegacyPrintf("service.minimax_video", "task settlement pending job=%s: %v", job.ID, err)
				}
			}
		}
	}
	writeGrokMediaResponse(c, resp, responseBody, s.responseHeaderFilter)
	return result, nil
}
