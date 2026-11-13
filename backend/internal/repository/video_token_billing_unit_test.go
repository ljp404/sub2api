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

func TestVideoTokenSettlementAtomicity(t *testing.T) {
	for _, scenario := range []string{"commit", "audit rollback", "duplicate", "wrong owner"} {
		t.Run(scenario, func(t *testing.T) {
			db, mock, err := sqlmock.New()
			require.NoError(t, err)
			defer db.Close()
			cmd := &service.UsageBillingCommand{RequestID: "grok-video:seedance:task1", APIKeyID: 7,
				UserID: 42, AccountID: 9, BalanceCost: 3.5,
				VideoTokenPricing: &service.VideoTokenPricingSnapshot{RuleHash: "frozen", HoldID: "hold-1", ActualCost: "3.50000000"}}
			cmd.Normalize()
			mock.ExpectBegin()
			claim := mock.ExpectQuery(`INSERT INTO usage_billing_dedup`).WithArgs(cmd.RequestID, cmd.APIKeyID, cmd.RequestFingerprint)
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
				mock.ExpectQuery(`(?s)SELECT hold_amount.*FROM video_token_billing_jobs.*FOR UPDATE`).
					WithArgs("hold-1").WillReturnRows(sqlmock.NewRows([]string{"hold_amount", "hash", "state", "user", "key", "account"}).
					AddRow("3.00000000", "frozen", "submitted", owner, int64(7), int64(9)))
				if scenario == "wrong owner" {
					mock.ExpectRollback()
				} else {
					// The reserved $3 is consumed and final $3.5 records a $0.5 debt.
					mock.ExpectQuery(`(?s)UPDATE users SET balance=balance\+\$1-\$2.*RETURNING balance`).
						WithArgs("3.00000000", "3.50000000", int64(42)).
						WillReturnRows(sqlmock.NewRows([]string{"balance"}).AddRow(-0.5))
					mock.ExpectExec(`UPDATE video_token_billing_jobs SET state='settled'`).
						WithArgs("hold-1").WillReturnResult(sqlmock.NewResult(0, 1))
					raw, _ := json.Marshal(cmd.VideoTokenPricing)
					audit := mock.ExpectExec(`INSERT INTO video_token_billing_snapshots`).WithArgs(cmd.RequestID, cmd.APIKeyID, string(raw))
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
			if scenario == "audit rollback" || scenario == "wrong owner" {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				require.Equal(t, scenario == "commit", result.Applied)
				if result.Applied {
					require.True(t, result.BalanceOverdrafted)
					require.Equal(t, -0.5, *result.NewBalance)
				}
			}
			require.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

func TestVideoTokenReleaseIdempotency(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()
	repo := &usageBillingRepository{db: db}
	mock.ExpectBegin()
	mock.ExpectQuery(`(?s)UPDATE video_token_billing_jobs SET state='released'.*RETURNING user_id,hold_amount`).
		WithArgs("hold-1").WillReturnRows(sqlmock.NewRows([]string{"user_id", "hold_amount"}).AddRow(int64(42), "3.00000000"))
	mock.ExpectExec(`(?s)UPDATE users SET balance=balance\+\$1.*frozen_balance`).
		WithArgs("3.00000000", int64(42)).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()
	require.NoError(t, repo.ReleaseVideoTokenJob(context.Background(), "hold-1"))
	mock.ExpectBegin()
	mock.ExpectQuery(`UPDATE video_token_billing_jobs SET state='released'`).
		WithArgs("hold-1").WillReturnError(sql.ErrNoRows)
	mock.ExpectRollback()
	require.NoError(t, repo.ReleaseVideoTokenJob(context.Background(), "hold-1"))
	require.NoError(t, mock.ExpectationsWereMet())
}
