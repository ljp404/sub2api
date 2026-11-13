package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/shopspring/decimal"
)

var _ service.ImageParameterBillingRepository = (*usageBillingRepository)(nil)
var _ service.ImageParameterHoldAdminRepository = (*usageBillingRepository)(nil)

func (r *usageBillingRepository) ListUncertainImageParameterHolds(ctx context.Context, limit, offset int) ([]service.ImageParameterUncertainHold, int64, error) {
	var total int64
	if err := r.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM image_parameter_billing_holds WHERE state='uncertain'`).Scan(&total); err != nil {
		return nil, 0, err
	}
	rows, err := r.db.QueryContext(ctx, `SELECT id,user_id,api_key_id,account_id,hold_amount::text,
		COALESCE(quote->'input'->'dimensions','{}'::jsonb),created_at
		FROM image_parameter_billing_holds WHERE state='uncertain'
		ORDER BY created_at DESC,id LIMIT $1 OFFSET $2`, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	holds := make([]service.ImageParameterUncertainHold, 0)
	for rows.Next() {
		var hold service.ImageParameterUncertainHold
		var dimensions []byte
		if err := rows.Scan(&hold.ID, &hold.UserID, &hold.APIKeyID, &hold.AccountID, &hold.Amount, &dimensions, &hold.CreatedAt); err != nil {
			return nil, 0, err
		}
		if err := json.Unmarshal(dimensions, &hold.Dimensions); err != nil {
			return nil, 0, err
		}
		holds = append(holds, hold)
	}
	return holds, total, rows.Err()
}

func (r *usageBillingRepository) ReleaseUncertainImageParameterHold(ctx context.Context, id string, operatorID int64, reason string) (int64, error) {
	id, reason = strings.TrimSpace(id), strings.TrimSpace(reason)
	if id == "" || operatorID <= 0 || reason == "" {
		return 0, fmt.Errorf("hold id, operator, and release reason are required")
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()
	var userID int64
	var amount string
	err = tx.QueryRowContext(ctx, `UPDATE image_parameter_billing_holds
		SET state='released',released_by=$2,release_reason=$3,released_at=NOW(),updated_at=NOW()
		WHERE id=$1 AND state='uncertain'
		RETURNING user_id,hold_amount::text`, id, operatorID, reason).Scan(&userID, &amount)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, service.ErrImageParameterHoldNotUncertain
	}
	if err != nil {
		return 0, err
	}
	result, err := tx.ExecContext(ctx, `UPDATE users SET balance=balance+$1,
		frozen_balance=COALESCE(frozen_balance,0)-$1,updated_at=NOW()
		WHERE id=$2 AND COALESCE(frozen_balance,0)>=$1`, amount, userID)
	if err != nil {
		return 0, err
	}
	if n, _ := result.RowsAffected(); n != 1 {
		return 0, service.ErrImageParameterFrozenBalanceInconsistent
	}
	if err := tx.Commit(); err != nil {
		return 0, err
	}
	return userID, nil
}

func (r *usageBillingRepository) ReserveImageParameterBalance(ctx context.Context, hold *service.ImageParameterBalanceHold) error {
	raw, err := json.Marshal(hold.Quote)
	if err != nil {
		return err
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err = tx.ExecContext(ctx, `INSERT INTO image_parameter_billing_holds
		(id,user_id,api_key_id,account_id,quote,hold_amount) VALUES ($1,$2,$3,$4,$5,$6)`,
		hold.ID, hold.UserID, hold.APIKeyID, hold.AccountID, string(raw), hold.Quote.ActualCost); err != nil {
		return err
	}
	var balance float64
	err = tx.QueryRowContext(ctx, `UPDATE users SET balance=balance-$1,
		frozen_balance=COALESCE(frozen_balance,0)+$1,updated_at=NOW()
		WHERE id=$2 AND deleted_at IS NULL AND balance >= $1 AND balance >= 0 RETURNING balance`,
		hold.Quote.ActualCost, hold.UserID).Scan(&balance)
	if errors.Is(err, sql.ErrNoRows) {
		return service.ErrInsufficientBalance
	}
	if err != nil {
		return err
	}
	return tx.Commit()
}

func (r *usageBillingRepository) MarkImageParameterBalance(ctx context.Context, id, state string) error {
	if state != "submitted" && state != "uncertain" {
		return fmt.Errorf("invalid image hold state")
	}
	result, err := r.db.ExecContext(ctx, `UPDATE image_parameter_billing_holds SET state=$2,updated_at=NOW()
		WHERE id=$1 AND state IN ('reserved','submitted')`, id, state)
	if err != nil {
		return err
	}
	if n, _ := result.RowsAffected(); n != 1 {
		return fmt.Errorf("image hold is not active")
	}
	return nil
}

func (r *usageBillingRepository) ReleaseImageParameterBalance(ctx context.Context, id string) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var userID int64
	var amount string
	err = tx.QueryRowContext(ctx, `UPDATE image_parameter_billing_holds SET state='released',updated_at=NOW()
		WHERE id=$1 AND state IN ('reserved','submitted')
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
		return fmt.Errorf("image frozen balance is inconsistent")
	}
	return tx.Commit()
}

func (r *usageBillingRepository) StageImageParameterSettlement(ctx context.Context, item *service.ImageParameterSettlement) error {
	cmd := item.Command
	raw, err := json.Marshal(item)
	if err != nil {
		return err
	}
	result, err := r.db.ExecContext(ctx, `UPDATE image_parameter_billing_holds
		SET state=CASE WHEN state='settled' THEN state ELSE 'settlement_required' END,
		settlement=CASE WHEN state='submitted' THEN $2::jsonb ELSE settlement END,
		next_retry_at=NOW()+INTERVAL '30 seconds',updated_at=NOW()
		WHERE id=$1 AND user_id=$3 AND api_key_id=$4 AND account_id=$5
		AND (state='submitted' OR (state IN ('settlement_required','settled')
			AND settlement->'command'=($2::jsonb)->'command'))`,
		cmd.ParameterPricing.HoldID, string(raw), cmd.UserID, cmd.APIKeyID, cmd.AccountID)
	if err != nil {
		return err
	}
	if n, _ := result.RowsAffected(); n != 1 {
		return fmt.Errorf("image settlement cannot be staged")
	}
	return nil
}

func (r *usageBillingRepository) ClaimImageParameterSettlements(ctx context.Context) ([]*service.ImageParameterSettlement, error) {
	rows, err := r.db.QueryContext(ctx, `UPDATE image_parameter_billing_holds
		SET next_retry_at=NOW()+INTERVAL '5 minutes'
		WHERE id IN (SELECT id FROM image_parameter_billing_holds
			WHERE state='settlement_required' AND next_retry_at<=NOW()
			ORDER BY next_retry_at LIMIT 8 FOR UPDATE SKIP LOCKED)
		RETURNING settlement`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []*service.ImageParameterSettlement
	for rows.Next() {
		var raw []byte
		if err := rows.Scan(&raw); err != nil {
			return nil, err
		}
		var item service.ImageParameterSettlement
		if err := json.Unmarshal(raw, &item); err != nil {
			return nil, err
		}
		if item.Command == nil || item.Command.ParameterPricing == nil || item.Usage == nil {
			return nil, fmt.Errorf("invalid stored image settlement")
		}
		items = append(items, &item)
	}
	return items, rows.Err()
}

func captureImageParameterBalance(ctx context.Context, tx *sql.Tx, cmd *service.UsageBillingCommand, result *service.UsageBillingApplyResult) error {
	var amount, state string
	var userID, keyID, accountID int64
	var raw []byte
	err := tx.QueryRowContext(ctx, `SELECT hold_amount,quote,state,user_id,api_key_id,account_id
		FROM image_parameter_billing_holds WHERE id=$1 FOR UPDATE`, cmd.ParameterPricing.HoldID).
		Scan(&amount, &raw, &state, &userID, &keyID, &accountID)
	if err != nil {
		return err
	}
	var held service.ParameterPricingSnapshot
	if err := json.Unmarshal(raw, &held); err != nil {
		return err
	}
	if state != "settlement_required" || userID != cmd.UserID || keyID != cmd.APIKeyID ||
		accountID != cmd.AccountID || cmd.SubscriptionCost != 0 || held.RuleHash != cmd.ParameterPricing.RuleHash ||
		held.Input.Multiplier != cmd.ParameterPricing.Input.Multiplier ||
		!reflect.DeepEqual(held.Input.Dimensions, cmd.ParameterPricing.Input.Dimensions) {
		return fmt.Errorf("image reservation settlement conflict")
	}
	expected, err := service.CalculateParameterPricing(held.Rule, service.ParameterPricingInput{
		Dimensions: held.Input.Dimensions, Multiplier: held.Input.Multiplier, Quantity: cmd.ImageCount,
	})
	if err != nil {
		return err
	}
	if expected.ActualCost != cmd.ParameterPricing.ActualCost || expected.TotalCost != cmd.ParameterPricing.TotalCost ||
		cmd.ParameterPricing.Input.Quantity != cmd.ImageCount ||
		!decimal.RequireFromString(expected.ActualCost).Equal(decimal.NewFromFloat(cmd.BalanceCost)) {
		return fmt.Errorf("image settlement amount mismatch")
	}
	var balance float64
	err = tx.QueryRowContext(ctx, `UPDATE users SET balance=balance+$1-$2,
		frozen_balance=COALESCE(frozen_balance,0)-$1,updated_at=NOW()
		WHERE id=$3 AND COALESCE(frozen_balance,0)>=$1 RETURNING balance`,
		amount, expected.ActualCost, userID).Scan(&balance)
	if err != nil {
		return err
	}
	result.NewBalance, result.BalanceOverdrafted = &balance, balance < 0
	_, err = tx.ExecContext(ctx, `UPDATE image_parameter_billing_holds SET state='settled',last_error=NULL,updated_at=NOW() WHERE id=$1`,
		cmd.ParameterPricing.HoldID)
	return err
}
