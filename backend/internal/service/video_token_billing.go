package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/logger"
	"github.com/shopspring/decimal"
	"github.com/tidwall/gjson"
)

type VideoTokenTaskSpec struct {
	AdaptiveDuration bool    `json:"adaptive_duration"`
	AdaptiveRatio    bool    `json:"adaptive_ratio"`
	Ratio            string  `json:"ratio"`
	Provider         string  `json:"provider,omitempty"`
	Group            *Group  `json:"group"`
	AccountType      string  `json:"account_type"`
	AccountRate      float64 `json:"account_rate"`
}

const (
	VideoTokenProviderSeedance = "seedance"
	VideoTokenProviderMiniMax  = "minimax"
)

type VideoTokenBillingJob struct {
	ID            string                     `json:"id"`
	TaskID        string                     `json:"task_id"`
	UserID        int64                      `json:"user_id"`
	APIKeyID      int64                      `json:"api_key_id"`
	AccountID     int64                      `json:"account_id"`
	GroupID       int64                      `json:"group_id"`
	Model         string                     `json:"model"`
	UpstreamModel string                     `json:"upstream_model"`
	State         string                     `json:"state"`
	Quote         *VideoTokenPricingSnapshot `json:"quote"`
	TaskQuote     *VideoTaskPricingSnapshot `json:"task_quote,omitempty"`
	Spec          VideoTokenTaskSpec         `json:"spec"`
	CreatedAt     time.Time                  `json:"created_at"`
}

// Optional capability on the monetary repository keeps the new job lifecycle
// out of legacy image/video pricing and their repository mocks.
type VideoTokenBillingRepository interface {
	ReserveVideoTokenJob(context.Context, *VideoTokenBillingJob) error
	BindVideoTokenJob(context.Context, string, string) error
	ReleaseVideoTokenJob(context.Context, string) error
	MarkVideoTokenJob(context.Context, string, string, string) error
	GetVideoTokenJob(context.Context, string, int64) (*VideoTokenBillingJob, error)
	ClaimVideoTokenJobs(context.Context) ([]*VideoTokenBillingJob, error)
}

func (s *OpenAIGatewayService) videoTokenRepo() VideoTokenBillingRepository {
	repo, _ := s.usageBillingRepo.(VideoTokenBillingRepository)
	return repo
}

func (s *OpenAIGatewayService) LoadVideoTokenBillingJob(ctx context.Context, taskID string, keyID int64) (*VideoTokenBillingJob, error) {
	if s.videoTokenRepo() == nil {
		return nil, nil
	}
	return s.videoTokenRepo().GetVideoTokenJob(ctx, taskID, keyID)
}

type videoTokenEndpointContextKey struct{}

func WithSeedanceEndpoint(ctx context.Context) context.Context {
	return WithVideoTokenEndpoint(ctx)
}

func WithVideoTokenEndpoint(ctx context.Context) context.Context {
	return context.WithValue(ctx, videoTokenEndpointContextKey{}, true)
}

func isVideoTokenEndpoint(ctx context.Context) bool {
	v, _ := ctx.Value(videoTokenEndpointContextKey{}).(bool)
	return v
}

// New-mode validation runs before reserving money or sending a billable create.
// Fixed requests use the administrator's explicit resolution/ratio size table.
func prepareVideoTokenQuote(rule *VideoTokenPricing, body []byte, multiplier string) (*VideoTokenPricingSnapshot, VideoTokenTaskSpec, error) {
	var spec VideoTokenTaskSpec
	if err := ValidateVideoTokenPricing(rule); err != nil {
		return nil, spec, err
	}
	seen := map[string]bool{}
	duplicate := false
	gjson.ParseBytes(body).ForEach(func(k, _ gjson.Result) bool {
		duplicate = duplicate || seen[k.String()]
		seen[k.String()] = true
		return true
	})
	if duplicate {
		return nil, spec, fmt.Errorf("duplicate request fields are not allowed for video token billing")
	}
	if gjson.GetBytes(body, "frames").Exists() {
		return nil, spec, fmt.Errorf("video token billing currently requires duration instead of frames")
	}
	for _, path := range []string{"fps", "frame_rate"} {
		if fps := gjson.GetBytes(body, path); fps.Exists() && (fps.Type != gjson.Number || fps.Float() != 24) {
			return nil, spec, fmt.Errorf("video token pricing requires 24 fps")
		}
	}
	resolution := gjson.GetBytes(body, "resolution")
	ratio := gjson.GetBytes(body, "ratio")
	duration := gjson.GetBytes(body, "duration")
	if resolution.Type != gjson.String || ratio.Type != gjson.String || duration.Type != gjson.Number {
		return nil, spec, fmt.Errorf("video token billing requires duration, resolution and ratio")
	}
	tierName := strings.ToLower(resolution.String())
	requestedTierName := tierName
	tier, ok := rule.Tiers[tierName]
	// Separate pricing cards may contain only fast:* tiers. No model-name
	// heuristic or client-controlled tariff discount is used.
	if !ok {
		tierName = "fast:" + tierName
		tier, ok = rule.Tiers[tierName]
	}
	if !ok {
		model := strings.TrimSpace(gjson.GetBytes(body, "model").String())
		tiers := make([]string, 0, len(rule.Tiers))
		for name := range rule.Tiers {
			tiers = append(tiers, name)
		}
		sort.Strings(tiers)
		configured := strings.Join(tiers, "、")
		modelHint := ""
		if model != "" {
			modelHint = fmt.Sprintf("模型 %s ", model)
		}
		return nil, spec, fmt.Errorf(
			"当前计费模式为：视频（Token），%s未配置请求分辨率 %q 的价格档位。已配置档位：%s。请补充 %s 档位，或将 resolution 改为已配置档位",
			modelHint, resolution.String(), configured, requestedTierName,
		)
	}
	spec.Ratio = ratio.String()
	spec.AdaptiveRatio = spec.Ratio == "adaptive"
	size, ok := tier.Sizes[spec.Ratio]
	if spec.AdaptiveRatio {
		for _, candidate := range tier.Sizes {
			if candidate.Width*candidate.Height > size.Width*size.Height {
				size = candidate
			}
		}
		ok = true
	}
	if !ok {
		return nil, spec, fmt.Errorf("ratio %q has no configured pixel dimensions", spec.Ratio)
	}
	seconds := duration.Raw
	if seconds == "-1" {
		spec.AdaptiveDuration = true
		seconds = strconv.Itoa(rule.AdaptiveSeconds)
	} else {
		n, err := parameterDecimal(seconds, true)
		if err != nil || n.GreaterThan(decimal.NewFromInt(3600)) {
			return nil, spec, fmt.Errorf("invalid video duration")
		}
	}
	hasVideo := false
	for _, content := range gjson.GetBytes(body, "content").Array() {
		switch content.Get("type").String() {
		case "video_url":
			hasVideo = true
		case "draft_task":
			return nil, spec, fmt.Errorf("draft_task requires upstream metering and is not supported by video token pricing")
		}
	}
	inputSeconds := "0"
	if hasVideo {
		// Reserve the configured total input ceiling; a URL/client duration is
		// not trusted as evidence of the actual referenced video's length.
		inputSeconds = strconv.Itoa(rule.MaxInputSeconds)
	}
	quote, err := CalculateVideoTokenPricing(rule, VideoTokenPricingInput{
		Tier: tierName, Width: size.Width, Height: size.Height, OutputSeconds: seconds,
		HasVideo: hasVideo, InputSeconds: inputSeconds, Multiplier: multiplier,
	})
	return quote, spec, err
}

func (s *OpenAIGatewayService) PrepareSeedanceVideoTokenJob(ctx context.Context, key *APIKey, account *Account, body []byte) (*VideoTokenBillingJob, error) {
	return s.PrepareVideoTokenJob(ctx, key, account, body, VideoTokenProviderSeedance)
}

func (s *OpenAIGatewayService) PrepareVideoTokenJob(ctx context.Context, key *APIKey, account *Account, body []byte, provider string) (*VideoTokenBillingJob, error) {
	if key == nil || key.GroupID == nil {
		return nil, nil
	}
	model := gjson.GetBytes(body, "model").String()
	mapping := s.ResolveChannelMapping(ctx, *key.GroupID, model)
	mapped := firstNonEmpty(mapping.MappedModel, model)
	upstream := account.GetMappedModel(mapped)
	billingModel := upstream
	switch mapping.BillingModelSource {
	case BillingModelSourceRequested:
		billingModel = model
	case BillingModelSourceChannelMapped:
		billingModel = mapped
	}
	resolved := s.resolveOpenAIChannelPricing(ctx, billingModel, key)
	if resolved != nil && resolved.Mode == BillingModeVideo && resolved.channelPricing != nil && resolved.channelPricing.VideoTaskPricing != nil {
		if s.videoTokenRepo() == nil || key.User == nil || key.Group == nil {
			return nil, fmt.Errorf("video task billing is unavailable")
		}
		if key.Group.IsSubscriptionType() || (s.cfg != nil && s.cfg.RunMode == config.RunModeSimple) {
			return nil, fmt.Errorf("video task billing requires a standard balance group")
		}
		taskQuote, taskSpec, err := prepareVideoTaskQuote(resolved.channelPricing.VideoTaskPricing, body,
			strconv.FormatFloat(resolveVideoRateMultiplier(key,
				s.ResolveUserGroupRateMultiplier(ctx, key.User.ID, *key.GroupID, key.Group.RateMultiplier)), 'f', -1, 64))
		if err != nil {
			return nil, err
		}
		taskSpec.Group = key.Group
		taskSpec.Provider = strings.TrimSpace(provider)
		taskSpec.AccountType, taskSpec.AccountRate = account.Type, account.BillingRateMultiplier()
		job := &VideoTokenBillingJob{
			ID: generateRequestID(), UserID: key.User.ID, APIKeyID: key.ID, GroupID: *key.GroupID,
			AccountID: account.ID, Model: model, UpstreamModel: upstream, TaskQuote: taskQuote, Spec: taskSpec,
		}
		taskQuote.HoldID = job.ID
		if err := s.videoTokenRepo().ReserveVideoTokenJob(ctx, job); err != nil {
			return nil, err
		}
		s.invalidateVideoTokenBalance(ctx, job.UserID)
		return job, nil
	}
	if resolved != nil && resolved.Mode == BillingModeParameter {
		return nil, fmt.Errorf("parameter pricing is only supported for image generation")
	}
	if resolved == nil || resolved.Mode != BillingModeVideoToken {
		return nil, nil
	}
	if resolved.channelPricing == nil || s.videoTokenRepo() == nil || key.User == nil || key.Group == nil {
		return nil, fmt.Errorf("video token billing is unavailable")
	}
	if key.Group.IsSubscriptionType() || (s.cfg != nil && s.cfg.RunMode == config.RunModeSimple) {
		return nil, fmt.Errorf("video token billing requires a standard balance group")
	}
	rate := s.ResolveUserGroupRateMultiplier(ctx, key.User.ID, *key.GroupID, key.Group.RateMultiplier)
	rate = resolveVideoRateMultiplier(key, rate)
	quote, spec, err := prepareVideoTokenQuote(resolved.channelPricing.VideoTokenPricing, body, strconv.FormatFloat(rate, 'f', -1, 64))
	if err != nil {
		return nil, err
	}
	spec.Group = key.Group
	spec.Provider = strings.TrimSpace(provider)
	spec.AccountType, spec.AccountRate = account.Type, account.BillingRateMultiplier()
	job := &VideoTokenBillingJob{
		ID: generateRequestID(), UserID: key.User.ID, APIKeyID: key.ID, GroupID: *key.GroupID,
		AccountID: account.ID, Model: model, UpstreamModel: upstream, Quote: quote, Spec: spec,
	}
	quote.HoldID = job.ID
	if err := s.videoTokenRepo().ReserveVideoTokenJob(ctx, job); err != nil {
		return nil, err
	}
	s.invalidateVideoTokenBalance(ctx, job.UserID)
	return job, nil
}

func (s *OpenAIGatewayService) VideoBillingModeForRequest(
	ctx context.Context,
	key *APIKey,
	account *Account,
	model string,
) BillingMode {
	if key == nil || key.GroupID == nil || account == nil {
		return ""
	}
	mapping := s.ResolveChannelMapping(ctx, *key.GroupID, model)
	mapped := firstNonEmpty(mapping.MappedModel, model)
	billingModel := account.GetMappedModel(mapped)
	switch mapping.BillingModelSource {
	case BillingModelSourceRequested:
		billingModel = model
	case BillingModelSourceChannelMapped:
		billingModel = mapped
	}
	resolved := s.resolveOpenAIChannelPricing(ctx, billingModel, key)
	if resolved == nil {
		return ""
	}
	return resolved.Mode
}

func (s *OpenAIGatewayService) invalidateVideoTokenBalance(ctx context.Context, userID int64) {
	if s.billingCacheService != nil {
		if err := s.billingCacheService.InvalidateUserBalance(ctx, userID); err != nil {
			logger.LegacyPrintf("service.video_token", "balance cache invalidation failed user=%d: %v", userID, err)
		}
	}
}

// Upstream tokens include all billable video input and output for this task.
// Never use total_tokens, which may include unrelated text usage.
func measuredVideoTokens(body []byte) (*int64, error) {
	usagePath := "usage"
	if !gjson.GetBytes(body, usagePath).IsObject() && gjson.GetBytes(body, "task.usage").IsObject() {
		usagePath = "task.usage"
	}
	minimaxTaskUsage := usagePath == "task.usage"
	values := make(map[string]int64, 3)
	seen := map[string]bool{}
	var invalid error
	gjson.GetBytes(body, usagePath).ForEach(func(key, value gjson.Result) bool {
		name := key.String()
		if name != "completion_tokens" && name != "video_tokens" && !(minimaxTaskUsage && name == "total_tokens") {
			return true
		}
		tokens, err := strconv.ParseInt(value.Raw, 10, 64)
		if seen[name] || value.Type != gjson.Number || err != nil || tokens <= 0 || tokens > 100000000000 {
			invalid = fmt.Errorf("%s.%s must be an integer between 1 and 100000000000 without duplicate fields", usagePath, name)
			return false
		}
		seen[name] = true
		values[name] = tokens
		return true
	})
	if invalid != nil {
		return nil, invalid
	}
	if completion, ok := values["completion_tokens"]; ok {
		if videoTokens, exists := values["video_tokens"]; exists && videoTokens != completion {
			return nil, fmt.Errorf("usage.completion_tokens and usage.video_tokens disagree")
		}
	}
	priorities := []string{"video_tokens", "completion_tokens"}
	if minimaxTaskUsage {
		priorities = append([]string{"total_tokens"}, priorities...)
	}
	for _, name := range priorities {
		if tokens, ok := values[name]; ok {
			return &tokens, nil
		}
	}
	return nil, nil
}

func normalizeVideoTaskStatus(status string) string {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "success", "succeeded", "completed", "done":
		return "succeeded"
	case "failed", "failure", "error", "cancelled", "canceled", "expired":
		return "failed"
	default:
		return strings.ToLower(strings.TrimSpace(status))
	}
}

// FinalVideoTokenQuote never accepts an input video duration from the client.
// For adaptive outputs the trusted task response must carry the actual values.
func FinalVideoTokenQuote(job *VideoTokenBillingJob, body []byte) (*VideoTokenPricingSnapshot, error) {
	measured, err := measuredVideoTokens(body)
	if err != nil {
		return nil, err
	}
	input := job.Quote.Input
	read := func(paths ...string) gjson.Result {
		for _, path := range paths {
			if v := gjson.GetBytes(body, path); v.Exists() {
				return v
			}
		}
		return gjson.Result{}
	}
	if resolution := strings.ToLower(strings.TrimSpace(read("resolution", "task.resolution").String())); resolution != "" {
		tierName := resolution
		if strings.HasPrefix(input.Tier, "fast:") {
			tierName = "fast:" + resolution
		}
		if _, ok := job.Quote.Rule.Tiers[tierName]; !ok {
			return nil, fmt.Errorf("actual video resolution %q has no configured price tier", resolution)
		}
		input.Tier = tierName
	}
	width, height := read("width", "video.width", "task.width", "task.video.width"), read("height", "video.height", "task.height", "task.video.height")
	if width.Exists() || height.Exists() {
		if width.Type != gjson.Number || height.Type != gjson.Number || width.Float() != float64(width.Int()) || height.Float() != float64(height.Int()) {
			return nil, fmt.Errorf("invalid actual video dimensions")
		}
		input.Width, input.Height = int(width.Int()), int(height.Int())
	} else {
		// MiniMax returns the actual resolution and ratio in the task object.
		// Use those values when present so adaptive and provider-adjusted outputs
		// are settled against the generated geometry, not the create-time quote.
		tier := job.Quote.Rule.Tiers[input.Tier]
		ratio := read("ratio", "video.ratio", "task.ratio", "task.video.ratio").String()
		if ratio != "" {
			size, ok := tier.Sizes[ratio]
			if !ok {
				return nil, fmt.Errorf("actual ratio %q has no configured pixel dimensions", ratio)
			}
			input.Width, input.Height = size.Width, size.Height
		} else if job.Spec.AdaptiveRatio {
			return nil, fmt.Errorf("adaptive output requires actual width/height or configured actual ratio")
		}
	}
	duration := read("duration", "video.duration", "task.duration", "task.video.duration")
	if duration.Exists() {
		if duration.Type != gjson.Number {
			return nil, fmt.Errorf("invalid actual output duration")
		}
		input.OutputSeconds = duration.Raw
	} else if job.Spec.AdaptiveDuration {
		return nil, fmt.Errorf("adaptive output requires actual duration")
	}
	if fps := read("fps", "video.fps"); fps.Exists() && (fps.Type != gjson.Number || fps.Float() != float64(job.Quote.Rule.FPS)) {
		return nil, fmt.Errorf("actual fps does not match configured video formula")
	}
	if input.HasVideo {
		duration := read("input_video_duration", "usage.input_video_duration", "task.usage.input_seconds")
		if duration.Type != gjson.Number {
			return nil, fmt.Errorf("input video requires trusted total input_video_duration")
		}
		input.InputSeconds = duration.Raw
	}
	quote, err := calculateVideoTokenPricing(job.Quote.Rule, input, measured)
	if err == nil {
		quote.HoldID = job.ID
	}
	return quote, err
}

func (s *OpenAIGatewayService) ProcessVideoTokenStatus(ctx context.Context, keys *APIKeyService, job *VideoTokenBillingJob, body []byte) error {
	if job.State == "settled" || job.State == "released" {
		return nil
	}
	status := normalizeVideoTaskStatus(firstNonEmpty(gjson.GetBytes(body, "status").String(), gjson.GetBytes(body, "task.status").String()))
	switch status {
	case "failed":
		if err := s.videoTokenRepo().ReleaseVideoTokenJob(ctx, job.ID); err != nil {
			return err
		}
		s.invalidateVideoTokenBalance(ctx, job.UserID)
		return nil
	case "succeeded":
		break
	default:
		return nil
	}
	var quote *VideoTokenPricingSnapshot
	var taskQuote *VideoTaskPricingSnapshot
	var videoResolution string
	var videoDurationSeconds int
	if job.TaskQuote != nil {
		input := job.TaskQuote.Input
		if resolution := strings.TrimSpace(firstNonEmpty(
			gjson.GetBytes(body, "task.resolution").String(),
			gjson.GetBytes(body, "resolution").String(),
		)); resolution != "" {
			normalized, normalizeErr := normalizeVideoTaskResolution(resolution)
			if normalizeErr != nil {
				err := fmt.Errorf("actual video resolution %q is not configured for task pricing", resolution)
				_ = s.videoTokenRepo().MarkVideoTokenJob(ctx, job.ID, "measurement_required", err.Error())
				return err
			}
			input.Resolution = normalized
		}
		if duration := gjson.GetBytes(body, "task.duration"); duration.Type == gjson.Number && duration.Float() > 0 {
			input.DurationSeconds = strconv.FormatFloat(duration.Float(), 'f', -1, 64)
		} else if duration := gjson.GetBytes(body, "duration"); duration.Type == gjson.Number && duration.Float() > 0 {
			input.DurationSeconds = strconv.FormatFloat(duration.Float(), 'f', -1, 64)
		}
		var err error
		taskQuote, err = CalculateVideoTaskPricing(job.TaskQuote.Rule, input)
		if err == nil {
			taskQuote.HoldID = job.ID
			videoResolution = input.Resolution
			seconds, _ := strconv.ParseFloat(input.DurationSeconds, 64)
			videoDurationSeconds = int(seconds)
		}
		if err != nil {
			_ = s.videoTokenRepo().MarkVideoTokenJob(ctx, job.ID, "measurement_required", err.Error())
			return err
		}
	} else {
		var err error
		quote, err = FinalVideoTokenQuote(job, body)
		if err != nil {
			_ = s.videoTokenRepo().MarkVideoTokenJob(ctx, job.ID, "measurement_required", err.Error())
			return err
		}
		videoResolution = strings.TrimPrefix(quote.Input.Tier, "fast:")
		videoDurationSeconds = int(decimal.RequireFromString(quote.Input.OutputSeconds).Ceil().IntPart())
	}
	key, err := keys.GetByID(ctx, job.APIKeyID)
	if errors.Is(err, ErrAPIKeyNotFound) {
		// Deleting a key must not orphan a previously accepted billable task.
		// The repository already skips counters for deleted keys.
		key, err = &APIKey{ID: job.APIKeyID, UserID: job.UserID}, nil
	}
	if err != nil {
		return err
	}
	if key.UserID != job.UserID {
		return fmt.Errorf("video billing owner mismatch")
	}
	user, err := s.userRepo.GetByID(ctx, job.UserID)
	if err != nil {
		user, err = s.userRepo.GetByIDIncludeDeleted(ctx, job.UserID)
		if err != nil {
			return err
		}
	}
	account, err := s.accountRepo.GetByID(ctx, job.AccountID)
	if errors.Is(err, ErrAccountNotFound) {
		platform := PlatformOpenAI
		if job.Spec.Provider == VideoTokenProviderMiniMax {
			platform = PlatformMiniMax
		}
		account, err = &Account{ID: job.AccountID, Type: job.Spec.AccountType, Platform: platform, RateMultiplier: &job.Spec.AccountRate}, nil
	}
	if err != nil {
		return err
	}
	copyAccount := *account
	copyAccount.RateMultiplier = &job.Spec.AccountRate
	copyKey := *key
	copyKey.User, copyKey.GroupID, copyKey.Group = user, &job.GroupID, job.Spec.Group
	result := &OpenAIForwardResult{
		Model: job.Model, BillingModel: job.Model, UpstreamModel: job.UpstreamModel,
		ResponseID: job.TaskID, RequestID: StableGrokVideoBillingRequestID(job.TaskID),
		VideoCount: 1, VideoResolution: videoResolution,
		VideoTokenQuote: quote, VideoTaskQuote: taskQuote, Duration: time.Since(job.CreatedAt),
		VideoDurationSeconds: videoDurationSeconds,
	}
	if quote != nil {
		result.Usage.OutputTokens = int(quote.Tokens)
	}
	inboundEndpoint, upstreamEndpoint := "/api/v3/contents/generations/tasks", "/api/v3/contents/generations/tasks"
	if job.Spec.Provider == VideoTokenProviderMiniMax {
		inboundEndpoint, upstreamEndpoint = "/v2/video_generation", "/v2/video_generation"
	}
	return s.RecordUsage(ctx, &OpenAIRecordUsageInput{
		Result: result, APIKey: &copyKey, User: user, Account: &copyAccount, APIKeyService: keys,
		InboundEndpoint: inboundEndpoint, UpstreamEndpoint: upstreamEndpoint,
		RequestPayloadHash: HashUsageRequestPayload([]byte(job.TaskID)),
	})
}

// A durable reconciliation loop settles jobs even if the customer stops polling.
// Multiple application replicas lease rows; money remains protected by DB dedup.
func (s *OpenAIGatewayService) StartVideoTokenBilling(keys *APIKeyService) {
	if s == nil || keys == nil || s.videoTokenRepo() == nil {
		return
	}
	s.videoTokenWorkerOnce.Do(func() {
		ctx, cancel := context.WithCancel(context.Background())
		s.videoTokenWorkerCancel = cancel
		go func() {
			ticker := time.NewTicker(15 * time.Second)
			defer ticker.Stop()
			for {
				select {
				case <-ctx.Done():
					return
				case <-ticker.C:
					s.reconcileImageParameterBilling(ctx, keys)
					jobs, err := s.videoTokenRepo().ClaimVideoTokenJobs(ctx)
					if err != nil {
						logger.LegacyPrintf("service.video_token", "reconcile claim failed: %v", err)
						continue
					}
					for _, job := range jobs {
						pollCtx, done := context.WithTimeout(ctx, 25*time.Second)
						err := s.pollVideoTokenJob(pollCtx, keys, job)
						done()
						if err != nil {
							logger.LegacyPrintf("service.video_token", "reconcile job=%s failed: %v", job.ID, err)
						}
					}
				}
			}
		}()
	})
}

func (s *OpenAIGatewayService) pollVideoTokenJob(ctx context.Context, keys *APIKeyService, job *VideoTokenBillingJob) error {
	account, err := s.accountRepo.GetByID(ctx, job.AccountID)
	if err != nil {
		return err
	}
	base, err := s.validateUpstreamBaseURL(account.GetOpenAIBaseURL())
	if err != nil {
		return fmt.Errorf("video upstream base URL is invalid")
	}
	var target string
	if job.Spec.Provider == VideoTokenProviderMiniMax {
		target, err = BuildMiniMaxVideoURL(base, MiniMaxVideoEndpointStatus, job.TaskID)
	} else {
		target, err = buildSeedanceURL(base, SeedanceEndpointStatus, strings.TrimPrefix(job.TaskID, "seedance:"))
	}
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, target, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+account.GetCredential("api_key"))
	req.Header.Set("Accept", "application/json")
	account.ApplyHeaderOverrides(req.Header)
	proxy := ""
	if account.ProxyID != nil && account.Proxy != nil {
		proxy = account.Proxy.URL()
	}
	resp, err := s.httpUpstream.Do(req, proxy, account.ID, account.Concurrency)
	if err != nil {
		return fmt.Errorf("video status transport failed")
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("video status returned HTTP %d", resp.StatusCode)
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, 1024*1024+1))
	if err != nil || len(data) > 1024*1024 || !json.Valid(data) {
		return fmt.Errorf("invalid video status response")
	}
	return s.ProcessVideoTokenStatus(ctx, keys, job, data)
}
