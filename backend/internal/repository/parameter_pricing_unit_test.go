//go:build unit

package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestParameterPricingRepositoryRoundTrip(t *testing.T) {
	repo, mock := newChannelModelPricingTimePricingRepo(t)
	rule := &service.ParameterPricing{Version: 1, Unit: "image", BasePrice: "0.052706", Factors: []service.ParameterPricingFactor{
		{Key: "quality", Default: "high", Values: map[string]string{"high": "1", "low": "0.111338"}},
	}}
	stored := &capturedReasoningMultipliersJSON{}
	mock.ExpectQuery(`INSERT INTO channel_model_pricing .*parameter_pricing`).
		WithArgs(int64(7), "openai", []byte(`["gpt-image-2"]`), service.BillingModeParameter,
			nil, nil, nil, nil, nil, nil, nil, "{}", nil, nil, nil, nil, stored, nil).
		WillReturnRows(sqlmock.NewRows([]string{"id", "created_at", "updated_at"}).AddRow(int64(11), time.Time{}, time.Time{}))
	require.NoError(t, repo.CreateModelPricing(context.Background(), &service.ChannelModelPricing{
		ChannelID: 7, Platform: "openai", Models: []string{"gpt-image-2"}, BillingMode: service.BillingModeParameter, ParameterPricing: rule,
	}))
	mock.ExpectQuery(`SELECT .*parameter_pricing`).WithArgs(int64(7)).
		WillReturnRows(sqlmock.NewRows(channelModelPricingTimePricingColumns).AddRow(
			int64(11), int64(7), "openai", `["gpt-image-2"]`, service.BillingModeParameter,
			nil, nil, nil, nil, nil, nil, nil, "{}", nil, nil, nil, nil, time.Time{}, time.Time{}, stored.value, nil))
	expectEmptyModelPricingIntervals(mock)
	loaded, err := repo.ListModelPricing(context.Background(), 7)
	require.NoError(t, err)
	require.Equal(t, rule, loaded[0].ParameterPricing)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestParameterBillingAuditAtomicityAndDedup(t *testing.T) {
	for _, scenario := range []string{"commit", "audit error", "duplicate"} {
		t.Run(scenario, func(t *testing.T) {
			db, mock, err := sqlmock.New()
			require.NoError(t, err)
			defer db.Close()
			cmd := &service.UsageBillingCommand{
				RequestID: "parameter-image", APIKeyID: 7, UserID: 42, BalanceCost: 0.052706,
				ParameterPricing: &service.ParameterPricingSnapshot{RuleHash: "frozen-rule", ActualCost: "0.05270600"},
			}
			cmd.Normalize()
			mock.ExpectBegin()
			claim := mock.ExpectQuery(`INSERT INTO usage_billing_dedup`).
				WithArgs(cmd.RequestID, cmd.APIKeyID, cmd.RequestFingerprint)
			if scenario == "duplicate" {
				claim.WillReturnError(sql.ErrNoRows)
				mock.ExpectQuery(`(?s)SELECT request_fingerprint.*FROM usage_billing_dedup`).
					WithArgs(cmd.RequestID, cmd.APIKeyID).WillReturnRows(sqlmock.NewRows([]string{"request_fingerprint"}).AddRow(cmd.RequestFingerprint))
				mock.ExpectRollback()
			} else {
				claim.WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(1))
				mock.ExpectQuery(`(?s)SELECT request_fingerprint.*FROM usage_billing_dedup_archive`).
					WithArgs(cmd.RequestID, cmd.APIKeyID).WillReturnError(sql.ErrNoRows)
				mock.ExpectQuery(conditionalBalanceDeductSQL).
					WithArgs(cmd.BalanceCost, cmd.UserID).WillReturnRows(sqlmock.NewRows([]string{"balance"}).AddRow(9.947294))
				raw, _ := json.Marshal(cmd.ParameterPricing)
				audit := mock.ExpectExec(`INSERT INTO parameter_billing_snapshots`).WithArgs(cmd.RequestID, cmd.APIKeyID, string(raw))
				if scenario == "audit error" {
					audit.WillReturnError(sql.ErrConnDone)
					mock.ExpectRollback()
				} else {
					audit.WillReturnResult(sqlmock.NewResult(0, 1))
					mock.ExpectCommit()
				}
			}
			result, err := (&usageBillingRepository{db: db}).Apply(context.Background(), cmd)
			if scenario == "audit error" {
				require.ErrorIs(t, err, sql.ErrConnDone)
			} else {
				require.NoError(t, err)
				require.Equal(t, scenario == "commit", result.Applied)
			}
			require.NoError(t, mock.ExpectationsWereMet())
		})
	}
}
