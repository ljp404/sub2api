//go:build unit

package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestImageParameterDatabaseReserve(t *testing.T) {
	for _, sufficient := range []bool{true, false} {
		t.Run(map[bool]string{true: "sufficient", false: "insufficient"}[sufficient], func(t *testing.T) {
			db, mock, err := sqlmock.New()
			require.NoError(t, err)
			defer db.Close()
			hold := &service.ImageParameterBalanceHold{ID: "image-hold", UserID: 42, APIKeyID: 7, AccountID: 9,
				Quote: &service.ParameterPricingSnapshot{HoldID: "image-hold", ActualCost: "0.40000000"}}
			raw, _ := json.Marshal(hold.Quote)
			mock.ExpectBegin()
			mock.ExpectExec(`INSERT INTO image_parameter_billing_holds`).
				WithArgs(hold.ID, hold.UserID, hold.APIKeyID, hold.AccountID, string(raw), "0.40000000").
				WillReturnResult(sqlmock.NewResult(0, 1))
			balance := mock.ExpectQuery(`(?s)UPDATE users SET balance=balance-\$1.*balance >= \$1.*RETURNING balance`).
				WithArgs("0.40000000", int64(42))
			if sufficient {
				balance.WillReturnRows(sqlmock.NewRows([]string{"balance"}).AddRow(0.6))
				mock.ExpectCommit()
			} else {
				balance.WillReturnError(sql.ErrNoRows)
				mock.ExpectRollback()
			}
			err = (&usageBillingRepository{db: db}).ReserveImageParameterBalance(context.Background(), hold)
			if sufficient {
				require.NoError(t, err)
			} else {
				require.ErrorIs(t, err, service.ErrInsufficientBalance)
			}
			require.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

func TestImageParameterSettlementAtomicity(t *testing.T) {
	for _, scenario := range []string{"commit", "audit rollback", "duplicate", "wrong owner", "wrong dimensions", "wrong amount"} {
		t.Run(scenario, func(t *testing.T) {
			db, mock, err := sqlmock.New()
			require.NoError(t, err)
			defer db.Close()
			rule := &service.ParameterPricing{Version: 1, Unit: "image", BasePrice: "0.25",
				Factors: []service.ParameterPricingFactor{{Key: "quality", Values: map[string]string{"high": "2", "low": "1"}}}}
			held, err := service.CalculateParameterPricing(rule, service.ParameterPricingInput{
				Dimensions: map[string]string{"quality": "high"}, Quantity: 2, Multiplier: "0.8",
			})
			require.NoError(t, err)
			final, err := service.CalculateParameterPricing(rule, service.ParameterPricingInput{
				Dimensions: held.Input.Dimensions, Quantity: 1, Multiplier: "0.8",
			})
			require.NoError(t, err)
			held.HoldID, final.HoldID = "hold-1", "hold-1"
			if scenario == "wrong dimensions" {
				final.Input.Dimensions["quality"] = "low"
			}
			cmd := &service.UsageBillingCommand{RequestID: "image-1", APIKeyID: 7, UserID: 42,
				AccountID: 9, ImageCount: 1, BalanceCost: 0.4, ParameterPricing: final}
			if scenario == "wrong amount" {
				cmd.BalanceCost = 0.3
			}
			cmd.Normalize()
			mock.ExpectBegin()
			claim := mock.ExpectQuery(`INSERT INTO usage_billing_dedup`).
				WithArgs(cmd.RequestID, cmd.APIKeyID, cmd.RequestFingerprint)
			if scenario == "duplicate" {
				claim.WillReturnError(sql.ErrNoRows)
				mock.ExpectQuery(`(?s)SELECT request_fingerprint.*FROM usage_billing_dedup`).
					WithArgs(cmd.RequestID, cmd.APIKeyID).
					WillReturnRows(sqlmock.NewRows([]string{"request_fingerprint"}).AddRow(cmd.RequestFingerprint))
				mock.ExpectRollback()
			} else {
				claim.WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(1))
				mock.ExpectQuery(`(?s)SELECT request_fingerprint.*FROM usage_billing_dedup_archive`).
					WithArgs(cmd.RequestID, cmd.APIKeyID).WillReturnError(sql.ErrNoRows)
				owner := int64(42)
				if scenario == "wrong owner" {
					owner = 43
				}
				raw, _ := json.Marshal(held)
				mock.ExpectQuery(`(?s)SELECT hold_amount,quote,state.*FROM image_parameter_billing_holds.*FOR UPDATE`).
					WithArgs("hold-1").
					WillReturnRows(sqlmock.NewRows([]string{"hold", "quote", "state", "user", "key", "account"}).
						AddRow("0.80000000", raw, "settlement_required", owner, int64(7), int64(9)))
				if scenario == "wrong owner" || scenario == "wrong dimensions" || scenario == "wrong amount" {
					mock.ExpectRollback()
				} else {
					// Consume $0.4 from the $0.8 hold and refund the remaining $0.4.
					mock.ExpectQuery(`(?s)UPDATE users SET balance=balance\+\$1-\$2.*frozen_balance.*RETURNING balance`).
						WithArgs("0.80000000", "0.40000000", int64(42)).
						WillReturnRows(sqlmock.NewRows([]string{"balance"}).AddRow(9.6))
					mock.ExpectExec(`UPDATE image_parameter_billing_holds SET state='settled'`).
						WithArgs("hold-1").WillReturnResult(sqlmock.NewResult(0, 1))
					snapshot, _ := json.Marshal(final)
					audit := mock.ExpectExec(`INSERT INTO parameter_billing_snapshots`).
						WithArgs(cmd.RequestID, cmd.APIKeyID, string(snapshot))
					if scenario == "audit rollback" {
						audit.WillReturnError(sql.ErrConnDone)
						mock.ExpectRollback()
					} else {
						audit.WillReturnResult(sqlmock.NewResult(0, 1))
						mock.ExpectCommit()
					}
				}
			}
			result, err := (&usageBillingRepository{db: db}).Apply(context.Background(), cmd)
			if scenario != "commit" && scenario != "duplicate" {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				require.Equal(t, scenario == "commit", result.Applied)
				if result.Applied {
					require.Equal(t, 9.6, *result.NewBalance)
				}
			}
			require.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

func TestImageTokenHoldSettlementUsesActualImageCount(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	held := &service.ParameterPricingSnapshot{
		HoldID:       "token-image-hold",
		RuleHash:     "token-image-rule",
		HoldOnly:     true,
		HoldPerImage: "0.05000000",
		Input: service.ParameterPricingInput{
			Quantity:   2,
			Multiplier: "1",
		},
		TotalCost:  "0.10000000",
		ActualCost: "0.10000000",
	}
	final := *held
	final.HoldID = held.HoldID
	cmd := &service.UsageBillingCommand{
		RequestID:        "image-token-1",
		APIKeyID:         7,
		UserID:           42,
		AccountID:        9,
		ImageCount:       1, // The upstream returned one image although two were requested.
		BalanceCost:      0.08,
		ParameterPricing: &final,
	}
	cmd.Normalize()

	mock.ExpectBegin()
	mock.ExpectQuery(`INSERT INTO usage_billing_dedup`).
		WithArgs(cmd.RequestID, cmd.APIKeyID, cmd.RequestFingerprint).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(1))
	mock.ExpectQuery(`(?s)SELECT request_fingerprint.*FROM usage_billing_dedup_archive`).
		WithArgs(cmd.RequestID, cmd.APIKeyID).WillReturnError(sql.ErrNoRows)
	raw, err := json.Marshal(held)
	require.NoError(t, err)
	mock.ExpectQuery(`(?s)SELECT hold_amount,quote,state.*FROM image_parameter_billing_holds.*FOR UPDATE`).
		WithArgs(held.HoldID).
		WillReturnRows(sqlmock.NewRows([]string{"hold", "quote", "state", "user", "key", "account"}).
			AddRow("0.10000000", raw, "settlement_required", int64(42), int64(7), int64(9)))
	mock.ExpectQuery(`(?s)UPDATE users SET balance=balance\+\$1-\$2.*frozen_balance.*RETURNING balance`).
		WithArgs("0.10000000", "0.08", int64(42)).
		WillReturnRows(sqlmock.NewRows([]string{"balance"}).AddRow(9.92))
	mock.ExpectExec(`UPDATE image_parameter_billing_holds SET state='settled'`).
		WithArgs(held.HoldID).WillReturnResult(sqlmock.NewResult(0, 1))
	snapshot, err := json.Marshal(&final)
	require.NoError(t, err)
	mock.ExpectExec(`INSERT INTO parameter_billing_snapshots`).
		WithArgs(cmd.RequestID, cmd.APIKeyID, string(snapshot)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	result, err := (&usageBillingRepository{db: db}).Apply(context.Background(), cmd)
	require.NoError(t, err)
	require.True(t, result.Applied)
	require.Equal(t, 9.92, *result.NewBalance)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestImageParameterReleaseIdempotency(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()
	repo := &usageBillingRepository{db: db}
	mock.ExpectBegin()
	mock.ExpectQuery(`(?s)UPDATE image_parameter_billing_holds SET state='released'.*RETURNING user_id,hold_amount`).
		WithArgs("hold-1").WillReturnRows(sqlmock.NewRows([]string{"user", "amount"}).AddRow(int64(42), "0.40000000"))
	mock.ExpectExec(`(?s)UPDATE users SET balance=balance\+\$1.*frozen_balance`).
		WithArgs("0.40000000", int64(42)).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()
	require.NoError(t, repo.ReleaseImageParameterBalance(context.Background(), "hold-1"))
	mock.ExpectBegin()
	mock.ExpectQuery(`UPDATE image_parameter_billing_holds SET state='released'`).
		WithArgs("hold-1").WillReturnError(sql.ErrNoRows)
	mock.ExpectRollback()
	require.NoError(t, repo.ReleaseImageParameterBalance(context.Background(), "hold-1"))
	require.NoError(t, mock.ExpectationsWereMet())
}
