package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/shopspring/decimal"
)

var _ service.VideoTokenBillingRepository = (*usageBillingRepository)(nil)

func (r *usageBillingRepository) ReserveVideoTokenJob(ctx context.Context, job *service.VideoTokenBillingJob) error {
	var pricingQuote any
	var holdAmount string
	if job.TaskQuote != nil && job.Quote == nil {
		pricingQuote = job.TaskQuote
		holdAmount = job.TaskQuote.ActualCost
	} else if job.Quote != nil && job.TaskQuote == nil {
		pricingQuote = job.Quote
		holdAmount = job.Quote.ActualCost
	} else {
		return fmt.Errorf("video reservation requires exactly one pricing quote")
	}
	quote, err := json.Marshal(pricingQuote)
	if err != nil {
		return err
	}
	spec, err := json.Marshal(job.Spec)
	if err != nil {
		return err
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	_, err = tx.ExecContext(ctx, `INSERT INTO video_token_billing_jobs
		(id,user_id,api_key_id,account_id,group_id,model,upstream_model,quote,spec,hold_amount)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)`,
		job.ID, job.UserID, job.APIKeyID, job.AccountID, job.GroupID, job.Model,
		job.UpstreamModel, string(quote), string(spec), holdAmount)
	if err != nil {
		return err
	}
	var balance float64
	err = tx.QueryRowContext(ctx, `UPDATE users SET balance=balance-$1,
		frozen_balance=COALESCE(frozen_balance,0)+$1,updated_at=NOW()
		WHERE id=$2 AND deleted_at IS NULL AND balance >= $1 AND balance >= 0 RETURNING balance`,
		holdAmount, job.UserID).Scan(&balance)
	if errors.Is(err, sql.ErrNoRows) {
		return fmt.Errorf("insufficient balance for video reservation")
	}
	if err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	job.State, job.CreatedAt = "reserved", time.Now()
	return nil
}

func (r *usageBillingRepository) BindVideoTokenJob(ctx context.Context, id, taskID string) error {
	result, err := r.db.ExecContext(ctx, `UPDATE video_token_billing_jobs SET task_id=$2,state='submitted',
		updated_at=NOW(),next_poll_at=NOW()+INTERVAL '15 seconds' WHERE id=$1 AND state='reserved'`, id, taskID)
	if err != nil {
		return err
	}
	count, _ := result.RowsAffected()
	if count != 1 {
		return fmt.Errorf("video reservation is not bindable")
	}
	return nil
}

func (r *usageBillingRepository) ReleaseVideoTokenJob(ctx context.Context, id string) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var userID int64
	var amount string
	err = tx.QueryRowContext(ctx, `UPDATE video_token_billing_jobs SET state='released',updated_at=NOW()
		WHERE id=$1 AND state IN ('reserved','submitted','measurement_required')
		RETURNING user_id,hold_amount`, id).Scan(&userID, &amount)
	if errors.Is(err, sql.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}
	result, err := tx.ExecContext(ctx, `UPDATE users SET balance=balance+$1,
		frozen_balance=COALESCE(frozen_balance,0)-$1,updated_at=NOW()
		WHERE id=$2 AND COALESCE(frozen_balance,0)>=$1`, amount, userID)
	if err != nil {
		return err
	}
	if n, _ := result.RowsAffected(); n != 1 {
		return fmt.Errorf("video frozen balance is inconsistent")
	}
	return tx.Commit()
}

func (r *usageBillingRepository) MarkVideoTokenJob(ctx context.Context, id, state, message string) error {
	if state != "uncertain" && state != "measurement_required" {
		return fmt.Errorf("invalid video billing state")
	}
	_, err := r.db.ExecContext(ctx, `UPDATE video_token_billing_jobs SET state=$2,last_error=$3,updated_at=NOW()
		WHERE id=$1 AND state IN ('reserved','submitted','measurement_required')`, id, state, message)
	return err
}

const videoTokenJobColumns = `id,COALESCE(task_id,''),user_id,api_key_id,account_id,group_id,
	model,upstream_model,state,quote,spec,created_at`

func scanVideoTokenJob(row interface{ Scan(...any) error }) (*service.VideoTokenBillingJob, error) {
	job := &service.VideoTokenBillingJob{}
	var quote, spec []byte
	if err := row.Scan(&job.ID, &job.TaskID, &job.UserID, &job.APIKeyID, &job.AccountID, &job.GroupID,
		&job.Model, &job.UpstreamModel, &job.State, &quote, &spec, &job.CreatedAt); err != nil {
		return nil, err
	}
	if err := json.Unmarshal(quote, &job.Quote); err != nil {
		return nil, err
	}
	var quoteShape struct {
		Rule struct {
			Currency string `json:"currency"`
		} `json:"rule"`
	}
	if err := json.Unmarshal(quote, &quoteShape); err != nil {
		return nil, err
	}
	if quoteShape.Rule.Currency != "" {
		job.Quote = nil
		if err := json.Unmarshal(quote, &job.TaskQuote); err != nil {
			return nil, err
		}
	}
	if err := json.Unmarshal(spec, &job.Spec); err != nil {
		return nil, err
	}
	return job, nil
}

func (r *usageBillingRepository) GetVideoTokenJob(ctx context.Context, taskID string, apiKeyID int64) (*service.VideoTokenBillingJob, error) {
	job, err := scanVideoTokenJob(r.db.QueryRowContext(ctx, `SELECT `+videoTokenJobColumns+`
		FROM video_token_billing_jobs WHERE task_id=$1 AND api_key_id=$2`, taskID, apiKeyID))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	return job, err
}

func (r *usageBillingRepository) ClaimVideoTokenJobs(ctx context.Context) ([]*service.VideoTokenBillingJob, error) {
	rows, err := r.db.QueryContext(ctx, `UPDATE video_token_billing_jobs SET next_poll_at=NOW()+INTERVAL '5 minutes'
		WHERE id IN (SELECT id FROM video_token_billing_jobs
			WHERE state IN ('submitted','measurement_required') AND next_poll_at <= NOW()
			ORDER BY next_poll_at LIMIT 8 FOR UPDATE SKIP LOCKED)
		RETURNING `+videoTokenJobColumns)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var jobs []*service.VideoTokenBillingJob
	for rows.Next() {
		job, err := scanVideoTokenJob(rows)
		if err != nil {
			return nil, err
		}
		jobs = append(jobs, job)
	}
	return jobs, rows.Err()
}

// Capture and dedup are committed in the same transaction as all other ledger
// effects. A final cost larger than the hold records debt instead of losing it.
func captureVideoTokenJob(ctx context.Context, tx *sql.Tx, cmd *service.UsageBillingCommand, result *service.UsageBillingApplyResult) error {
	var amount, hash, state string
	var userID, keyID, accountID int64
	err := tx.QueryRowContext(ctx, `SELECT hold_amount,quote->>'rule_hash',state,user_id,api_key_id,account_id
		FROM video_token_billing_jobs WHERE id=$1 FOR UPDATE`, cmd.VideoTokenPricing.HoldID).
		Scan(&amount, &hash, &state, &userID, &keyID, &accountID)
	if err != nil {
		return err
	}
	if (state != "submitted" && state != "measurement_required") || hash != cmd.VideoTokenPricing.RuleHash ||
		userID != cmd.UserID || keyID != cmd.APIKeyID || accountID != cmd.AccountID || cmd.SubscriptionCost != 0 {
		return fmt.Errorf("video reservation settlement conflict")
	}
	expected, _ := decimal.NewFromString(cmd.VideoTokenPricing.ActualCost)
	if !expected.Equal(decimal.NewFromFloat(cmd.BalanceCost)) {
		return fmt.Errorf("video settlement amount mismatch")
	}
	var balance float64
	err = tx.QueryRowContext(ctx, `UPDATE users SET balance=balance+$1-$2,
		frozen_balance=COALESCE(frozen_balance,0)-$1,updated_at=NOW()
		WHERE id=$3 AND COALESCE(frozen_balance,0)>=$1 RETURNING balance`,
		amount, cmd.VideoTokenPricing.ActualCost, userID).Scan(&balance)
	if err != nil {
		return err
	}
	result.NewBalance, result.BalanceOverdrafted = &balance, balance < 0
	_, err = tx.ExecContext(ctx, `UPDATE video_token_billing_jobs SET state='settled',last_error=NULL,updated_at=NOW() WHERE id=$1`, cmd.VideoTokenPricing.HoldID)
	return err
}

func captureVideoTaskJob(ctx context.Context, tx *sql.Tx, cmd *service.UsageBillingCommand, result *service.UsageBillingApplyResult) error {
	quote := cmd.VideoTaskPricing
	if quote == nil || quote.HoldID == "" {
		return fmt.Errorf("video task reservation snapshot is missing")
	}
	var amount, hash, state string
	var userID, keyID, accountID int64
	err := tx.QueryRowContext(ctx, `SELECT hold_amount,quote->>'rule_hash',state,user_id,api_key_id,account_id
		FROM video_token_billing_jobs WHERE id=$1 FOR UPDATE`, quote.HoldID).
		Scan(&amount, &hash, &state, &userID, &keyID, &accountID)
	if err != nil {
		return err
	}
	if (state != "submitted" && state != "measurement_required") || hash != quote.RuleHash ||
		userID != cmd.UserID || keyID != cmd.APIKeyID || accountID != cmd.AccountID || cmd.SubscriptionCost != 0 {
		return fmt.Errorf("video task reservation settlement conflict")
	}
	expected, _ := decimal.NewFromString(quote.ActualCost)
	if !expected.Equal(decimal.NewFromFloat(cmd.BalanceCost)) {
		return fmt.Errorf("video task settlement amount mismatch")
	}
	var balance float64
	err = tx.QueryRowContext(ctx, `UPDATE users SET balance=balance+$1-$2,
		frozen_balance=COALESCE(frozen_balance,0)-$1,updated_at=NOW()
		WHERE id=$3 AND COALESCE(frozen_balance,0)>=$1 RETURNING balance`,
		amount, quote.ActualCost, userID).Scan(&balance)
	if err != nil {
		return err
	}
	result.NewBalance, result.BalanceOverdrafted = &balance, balance < 0
	_, err = tx.ExecContext(ctx, `UPDATE video_token_billing_jobs SET state='settled',last_error=NULL,updated_at=NOW() WHERE id=$1`, quote.HoldID)
	return err
}
