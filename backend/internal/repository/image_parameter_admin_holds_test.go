package repository

import (
	"context"
	"database/sql"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/require"
)

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
