package service

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/logger"
)

var ErrImageParameterOutcomeUncertain = errors.New("image generation outcome is uncertain")
var ErrImageParameterHoldNotUncertain = errors.New("image hold is no longer uncertain")
var ErrImageParameterFrozenBalanceInconsistent = errors.New("image frozen balance is inconsistent")

type ImageParameterBalanceHold struct {
	ID               string
	UserID, APIKeyID int64
	AccountID        int64
	Quote            *ParameterPricingSnapshot
}

type ImageParameterSettlement struct {
	Command *UsageBillingCommand `json:"command"`
	Usage   *UsageLog            `json:"usage"`
}

// Optional capability keeps the new lifecycle separate from legacy image billing.
type ImageParameterBillingRepository interface {
	ReserveImageParameterBalance(context.Context, *ImageParameterBalanceHold) error
	MarkImageParameterBalance(context.Context, string, string) error
	ReleaseImageParameterBalance(context.Context, string) error
	StageImageParameterSettlement(context.Context, *ImageParameterSettlement) error
	ClaimImageParameterSettlements(context.Context) ([]*ImageParameterSettlement, error)
}

type ImageParameterUncertainHold struct {
	ID         string            `json:"id"`
	UserID     int64             `json:"user_id"`
	APIKeyID   int64             `json:"api_key_id"`
	AccountID  int64             `json:"account_id"`
	Amount     string            `json:"amount"`
	Dimensions map[string]string `json:"dimensions"`
	CreatedAt  time.Time         `json:"created_at"`
}

type ImageParameterHoldAdminRepository interface {
	ListUncertainImageParameterHolds(context.Context, int, int) ([]ImageParameterUncertainHold, int64, error)
	ReleaseUncertainImageParameterHold(context.Context, string, int64, string) (int64, error)
}

func (s *OpenAIGatewayService) imageParameterRepo() ImageParameterBillingRepository {
	repo, _ := s.usageBillingRepo.(ImageParameterBillingRepository)
	return repo
}

func (s *OpenAIGatewayService) ReserveImageParameterBalance(ctx context.Context, key *APIKey, account *Account, req *OpenAIImagesRequest) error {
	if req.ParameterQuote == nil {
		return nil
	}
	if err := s.ReserveImageParameterQuote(ctx, key, account, req.ParameterQuote); err != nil {
		return err
	}
	req.ParameterHoldUserID = key.User.ID
	req.ParameterHoldSent, req.ParameterHoldRejected = false, false
	return nil
}

// ReserveImageParameterQuote freezes a shared parameter-pricing quote before
// an upstream image request. Protocol handlers use this to share the same
// ledger lifecycle without depending on OpenAI request types.
func (s *OpenAIGatewayService) ReserveImageParameterQuote(ctx context.Context, key *APIKey, account *Account, quote *ParameterPricingSnapshot) error {
	if quote == nil {
		return nil
	}
	if key == nil || key.User == nil || key.Group == nil || s.imageParameterRepo() == nil {
		return ErrBillingServiceUnavailable
	}
	if key.Group.IsSubscriptionType() || (s.cfg != nil && s.cfg.RunMode == config.RunModeSimple) {
		return fmt.Errorf("图片（参数）数据库预扣需要余额计费分组")
	}
	hold := &ImageParameterBalanceHold{
		ID: generateRequestID(), UserID: key.User.ID, APIKeyID: key.ID, AccountID: account.ID,
		Quote: quote,
	}
	quote.HoldID = hold.ID
	if err := s.imageParameterRepo().ReserveImageParameterBalance(ctx, hold); err != nil {
		quote.HoldID = ""
		if errors.Is(err, ErrInsufficientBalance) {
			return err
		}
		return ErrBillingServiceUnavailable.WithCause(err)
	}
	s.invalidateVideoTokenBalance(ctx, key.User.ID)
	return nil
}

func (s *OpenAIGatewayService) finishImageParameterAttempt(ctx context.Context, req *OpenAIImagesRequest, result *OpenAIForwardResult, forwardErr error) error {
	if req.ParameterQuote == nil || req.ParameterQuote.HoldID == "" {
		return forwardErr
	}
	hasOutput := result != nil && result.ImageCount > 0
	err := s.FinishImageParameterQuoteAttempt(ctx, req.ParameterQuote, req.ParameterHoldUserID,
		req.ParameterHoldSent, req.ParameterHoldRejected, hasOutput, forwardErr)
	return err
}

func (s *OpenAIGatewayService) MarkImageParameterQuoteSubmitted(ctx context.Context, quote *ParameterPricingSnapshot) error {
	if quote == nil || quote.HoldID == "" {
		return nil
	}
	persistCtx, cancel := detachedBillingContext(ctx)
	defer cancel()
	if repo := s.imageParameterRepo(); repo != nil {
		if err := repo.MarkImageParameterBalance(persistCtx, quote.HoldID, "submitted"); err != nil {
			return ErrBillingServiceUnavailable.WithCause(err)
		}
		return nil
	}
	return ErrBillingServiceUnavailable
}

// FinishImageParameterQuoteAttempt shares OpenAI's release/uncertain rules
// with other image protocols using parameter pricing.
func (s *OpenAIGatewayService) FinishImageParameterQuoteAttempt(ctx context.Context, quote *ParameterPricingSnapshot, userID int64, sent, rejected, hasOutput bool, forwardErr error) error {
	if quote == nil || quote.HoldID == "" || hasOutput {
		return forwardErr
	}
	persistCtx, cancel := detachedBillingContext(ctx)
	defer cancel()
	repo := s.imageParameterRepo()
	uncertain := sent && !rejected && forwardErr != nil
	var err error
	if uncertain {
		err = repo.MarkImageParameterBalance(persistCtx, quote.HoldID, "uncertain")
	} else {
		err = repo.ReleaseImageParameterBalance(persistCtx, quote.HoldID)
	}
	if err != nil {
		logger.LegacyPrintf("service.image_parameter", "hold finalization failed hold=%s: %v", quote.HoldID, err)
		return errors.Join(forwardErr, ErrBillingServiceUnavailable.WithCause(err))
	}
	if userID != 0 {
		s.invalidateVideoTokenBalance(persistCtx, userID)
	}
	if uncertain {
		return errors.Join(forwardErr, ErrImageParameterOutcomeUncertain)
	}
	quote.HoldID = ""
	return forwardErr
}

func (s *OpenAIGatewayService) reconcileImageParameterBilling(ctx context.Context, keys *APIKeyService) {
	repo := s.imageParameterRepo()
	if repo == nil {
		return
	}
	pending, err := repo.ClaimImageParameterSettlements(ctx)
	if err != nil {
		logger.LegacyPrintf("service.image_parameter", "settlement claim failed: %v", err)
		return
	}
	for _, item := range pending {
		func() {
			billingCtx, cancel := context.WithTimeout(ctx, 20*time.Second)
			defer cancel()
			result, err := s.usageBillingRepo.Apply(billingCtx, item.Command)
			if err != nil {
				logger.LegacyPrintf("service.image_parameter", "settlement pending hold=%s: %v", item.Command.ParameterPricing.HoldID, err)
				return
			}
			s.invalidateVideoTokenBalance(billingCtx, item.Command.UserID)
			if keys != nil {
				if key, err := keys.GetByID(billingCtx, item.Command.APIKeyID); err == nil {
					keys.InvalidateAuthCacheByKey(billingCtx, key.Key)
					if result != nil && result.Applied && key.User != nil && s.accountRepo != nil {
						if account, err := s.accountRepo.GetByID(billingCtx, item.Command.AccountID); err == nil {
							rate := 1.0
							if item.Usage.AccountRateMultiplier != nil {
								rate = *item.Usage.AccountRateMultiplier
							}
							finalizePostUsageBilling(billingCtx, &postUsageBillingParams{
								Cost: parameterPricingCost(item.Command.ParameterPricing),
								User: key.User, APIKey: key, Account: account,
								AccountRateMultiplier: rate, APIKeyService: keys, Platform: PlatformOpenAI,
							}, s.billingDeps(), result)
						}
					}
				}
			}
			if s.billingCacheService != nil {
				_ = s.billingCacheService.InvalidateAPIKeyRateLimit(billingCtx, item.Command.APIKeyID)
			}
			writeUsageLogBestEffort(billingCtx, s.usageLogRepo, item.Usage, "service.image_parameter")
		}()
	}
}
