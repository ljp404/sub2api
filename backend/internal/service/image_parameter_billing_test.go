//go:build unit

package service

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type imageParameterHoldRepoStub struct {
	UsageBillingRepository
	reserveErr error
	reserved   int
	released   int
	states     []string
}

func (r *imageParameterHoldRepoStub) ReserveImageParameterBalance(context.Context, *ImageParameterBalanceHold) error {
	r.reserved++
	return r.reserveErr
}
func (r *imageParameterHoldRepoStub) MarkImageParameterBalance(_ context.Context, _ string, state string) error {
	r.states = append(r.states, state)
	return nil
}
func (r *imageParameterHoldRepoStub) ReleaseImageParameterBalance(context.Context, string) error {
	r.released++
	return nil
}
func (r *imageParameterHoldRepoStub) StageImageParameterSettlement(context.Context, *ImageParameterSettlement) error {
	return nil
}
func (r *imageParameterHoldRepoStub) ClaimImageParameterSettlements(context.Context) ([]*ImageParameterSettlement, error) {
	return nil, nil
}

func TestImageParameterHoldLifecycle(t *testing.T) {
	for _, tc := range []struct {
		name                      string
		sent, rejected, hasOutput bool
		err                       error
		release, uncertain        bool
	}{
		{"local failure", false, false, false, errors.New("invalid URL"), true, false},
		{"rejected", true, true, false, errors.New("upstream 400"), true, false},
		{"transport timeout", true, false, false, errors.New("timeout"), false, true},
		{"upstream 500", true, false, false, errors.New("upstream 500"), false, true},
		{"empty success", true, false, false, nil, true, false},
		{"success", true, false, true, nil, false, false},
		{"partial success", true, false, true, errors.New("truncated stream"), false, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			repo := &imageParameterHoldRepoStub{}
			svc := &OpenAIGatewayService{usageBillingRepo: repo}
			req := &OpenAIImagesRequest{ParameterQuote: &ParameterPricingSnapshot{HoldID: "hold"},
				ParameterHoldSent: tc.sent, ParameterHoldRejected: tc.rejected}
			var result *OpenAIForwardResult
			if tc.hasOutput {
				result = &OpenAIForwardResult{ImageCount: 1}
			}
			err := svc.finishImageParameterAttempt(context.Background(), req, result, tc.err)
			require.Equal(t, tc.release, repo.released == 1)
			require.Equal(t, tc.uncertain, errors.Is(err, ErrImageParameterOutcomeUncertain))
			if tc.uncertain {
				require.Equal(t, []string{"uncertain"}, repo.states)
			}
		})
	}
}

func TestImageParameterHoldRequiresDatabaseAndBalanceGroup(t *testing.T) {
	req := &OpenAIImagesRequest{ParameterQuote: &ParameterPricingSnapshot{ActualCost: "0.4"}}
	key := &APIKey{ID: 7, User: &User{ID: 42}, Group: &Group{}}
	account := &Account{ID: 9}
	svc := &OpenAIGatewayService{}
	require.ErrorIs(t, svc.ReserveImageParameterBalance(context.Background(), key, account, req), ErrBillingServiceUnavailable)
	repo := &imageParameterHoldRepoStub{reserveErr: ErrInsufficientBalance}
	svc.usageBillingRepo = repo
	require.ErrorIs(t, svc.ReserveImageParameterBalance(context.Background(), key, account, req), ErrInsufficientBalance)
	require.Empty(t, req.ParameterQuote.HoldID)
	key.Group.SubscriptionType = SubscriptionTypeSubscription
	require.ErrorContains(t, svc.ReserveImageParameterBalance(context.Background(), key, account, req), "余额计费分组")
	require.Equal(t, 1, repo.reserved)
	key.Group.SubscriptionType = SubscriptionTypeStandard
	repo.reserveErr = nil
	require.NoError(t, svc.ReserveImageParameterBalance(context.Background(), key, account, req))
	require.NotEmpty(t, req.ParameterQuote.HoldID)
	require.Equal(t, int64(42), req.ParameterHoldUserID)
}

func TestSharedImageParameterQuoteHoldLifecycle(t *testing.T) {
	repo := &imageParameterHoldRepoStub{}
	svc := &OpenAIGatewayService{usageBillingRepo: repo}
	key := &APIKey{ID: 7, User: &User{ID: 42}, Group: &Group{}}
	account := &Account{ID: 9}
	quote := &ParameterPricingSnapshot{ActualCost: "0.4"}

	require.NoError(t, svc.ReserveImageParameterQuote(context.Background(), key, account, quote))
	require.NotEmpty(t, quote.HoldID)
	require.Equal(t, 1, repo.reserved)
	require.NoError(t, svc.MarkImageParameterQuoteSubmitted(context.Background(), quote))
	require.Equal(t, []string{"submitted"}, repo.states)

	err := svc.FinishImageParameterQuoteAttempt(
		context.Background(), quote, key.User.ID, true, false, false, errors.New("timeout"),
	)
	require.ErrorIs(t, err, ErrImageParameterOutcomeUncertain)
	require.Equal(t, []string{"submitted", "uncertain"}, repo.states)
	require.Zero(t, repo.released)
	require.NotEmpty(t, quote.HoldID)

	quote = &ParameterPricingSnapshot{HoldID: "rejected"}
	err = svc.FinishImageParameterQuoteAttempt(
		context.Background(), quote, key.User.ID, true, true, false, errors.New("upstream rejected"),
	)
	require.ErrorContains(t, err, "upstream rejected")
	require.Empty(t, quote.HoldID)
	require.Equal(t, 1, repo.released)
}

func TestImageParameterStreamFailureAndPartialSettlement(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, partial := range []bool{false, true} {
		t.Run(map[bool]string{true: "partial output", false: "rejection"}[partial], func(t *testing.T) {
			stream := ""
			if partial {
				stream = "data: {\"type\":\"image_generation.completed\",\"b64_json\":\"aGVsbG8=\"}\n\n"
			}
			stream += "data: {\"type\":\"error\",\"error\":{\"type\":\"invalid_request_error\",\"message\":\"rejected\"}}\n\n"
			repo := &imageParameterHoldRepoStub{}
			svc := &OpenAIGatewayService{cfg: &config.Config{}, usageBillingRepo: repo,
				httpUpstream: &httpUpstreamRecorder{resp: &http.Response{
					StatusCode: 200, Header: http.Header{"Content-Type": []string{"text/event-stream"}},
					Body: io.NopCloser(strings.NewReader(stream)),
				}}}
			c, _ := gin.CreateTestContext(httptest.NewRecorder())
			c.Request = httptest.NewRequest("POST", "/v1/images/generations", strings.NewReader("{}"))
			req := &OpenAIImagesRequest{Model: "gpt-image-2", N: 1, Stream: true,
				ContentType: "application/json", Endpoint: "/v1/images/generations",
				ParameterQuote: &ParameterPricingSnapshot{HoldID: "hold",
					Rule: &ParameterPricing{Version: 1, Unit: "image", BasePrice: "0.4"}}}
			account := &Account{ID: 9, Platform: PlatformOpenAI, Type: AccountTypeAPIKey,
				Credentials: map[string]any{"api_key": "test", "base_url": "https://upstream.example/v1"}}
			result, err := svc.ForwardImages(context.Background(), c, account,
				[]byte(`{"model":"gpt-image-2","prompt":"test","stream":true}`), req, "")
			require.Error(t, err)
			require.False(t, errors.Is(err, ErrImageParameterOutcomeUncertain))
			if partial {
				require.Equal(t, 1, result.ImageCount)
				require.Equal(t, "hold", result.ParameterQuote.HoldID)
				require.Zero(t, repo.released)
			} else {
				require.Nil(t, result)
				require.Equal(t, 1, repo.released)
			}
		})
	}
}
