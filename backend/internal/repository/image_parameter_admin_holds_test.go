package repository

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/require"
)

func TestListImageParameterHoldsIncludesSubmittedAndUncertain(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()
	repo := &usageBillingRepository{db: db}
	createdAt := time.Date(2026, 11, 16, 16, 23, 51, 0, time.FixedZone("CST", 8*60*60))

	mock.ExpectQuery(`(?s)SELECT COUNT\(\*\) FROM image_parameter_billing_holds WHERE state IN \('submitted','uncertain'\)`).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(2))
	mock.ExpectQuery(`(?s)SELECT id,user_id,api_key_id,account_id,hold_amount::text,state,.*FROM image_parameter_billing_holds WHERE state IN \('submitted','uncertain'\).*ORDER BY created_at DESC,id LIMIT \$1 OFFSET \$2`).
		WithArgs(20, 0).
		WillReturnRows(sqlmock.NewRows([]string{"id", "user_id", "api_key_id", "account_id", "amount", "state", "dimensions", "created_at"}).
			AddRow("submitted-hold", int64(42), int64(7), int64(9), "0.05000000", "submitted", []byte(`{}`), createdAt).
			AddRow("uncertain-hold", int64(43), int64(8), int64(10), "0.10000000", "uncertain", []byte(`{"resolution":"2k"}`), createdAt.Add(-time.Minute)))

	holds, total, err := repo.ListImageParameterHolds(context.Background(), 20, 0)
	require.NoError(t, err)
	require.Equal(t, int64(2), total)
	require.Len(t, holds, 2)
	require.Equal(t, "submitted", holds[0].State)
	require.Equal(t, "uncertain", holds[1].State)
	require.Equal(t, "2k", holds[1].Dimensions["resolution"])
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestReleaseUncertainImageParameterHold(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()
	repo := &usageBillingRepository{db: db}

	mock.ExpectBegin()
	mock.ExpectQuery(`(?s)UPDATE image_parameter_billing_holds SET state='released'.*WHERE id=\$1 AND state='uncertain'.*RETURNING user_id,hold_amount`).
		WithArgs("hold-1", int64(99), "confirmed no upstream task").
		WillReturnRows(sqlmock.NewRows([]string{"user_id", "hold_amount"}).AddRow(42, "0.10702359"))
	mock.ExpectExec(`UPDATE users SET balance=balance\+\$1.*frozen_balance.*WHERE id=\$2.*frozen_balance`).
		WithArgs("0.10702359", int64(42)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	userID, err := repo.ReleaseUncertainImageParameterHold(context.Background(), "hold-1", 99, "confirmed no upstream task")
	require.NoError(t, err)
	require.Equal(t, int64(42), userID)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestReleaseUncertainImageParameterHoldRejectsAlreadyProcessed(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()
	repo := &usageBillingRepository{db: db}

	mock.ExpectBegin()
	mock.ExpectQuery(`(?s)UPDATE image_parameter_billing_holds SET state='released'.*WHERE id=\$1 AND state='uncertain'`).
		WithArgs("hold-1", int64(99), "reviewed").
		WillReturnError(sql.ErrNoRows)
	mock.ExpectRollback()

	_, err = repo.ReleaseUncertainImageParameterHold(context.Background(), "hold-1", 99, "reviewed")
	require.ErrorContains(t, err, "no longer uncertain")
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestReleaseUncertainImageParameterHoldRollsBackOnFrozenBalanceMismatch(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()
	repo := &usageBillingRepository{db: db}

	mock.ExpectBegin()
	mock.ExpectQuery(`(?s)UPDATE image_parameter_billing_holds SET state='released'.*WHERE id=\$1 AND state='uncertain'`).
		WithArgs("hold-1", int64(99), "reviewed").
		WillReturnRows(sqlmock.NewRows([]string{"user_id", "hold_amount"}).AddRow(42, "0.10702359"))
	mock.ExpectExec(`UPDATE users SET balance=balance\+\$1.*frozen_balance`).
		WithArgs("0.10702359", int64(42)).
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectRollback()

	_, err = repo.ReleaseUncertainImageParameterHold(context.Background(), "hold-1", 99, "reviewed")
	require.ErrorContains(t, err, "frozen balance is inconsistent")
	require.NoError(t, mock.ExpectationsWereMet())
}
