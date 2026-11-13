package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"math"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/Wei-Shaw/sub2api/internal/service"
)

type modelPlazaReferencePriceRepository struct {
	db *sql.DB
}

func NewModelPlazaReferencePriceRepository(db *sql.DB) service.ModelPlazaReferencePriceRepository {
	return &modelPlazaReferencePriceRepository{db: db}
}

func (r *modelPlazaReferencePriceRepository) ListModelPlazaReferencePrices(ctx context.Context) ([]service.ModelPlazaReferencePrice, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, platform, model_name, pricing, source_label, source_url, notes,
		       updated_by, created_at, updated_at
		FROM model_plaza_reference_prices
		ORDER BY LOWER(platform), LOWER(model_name), id
	`)
	if err != nil {
		return nil, fmt.Errorf("list model plaza reference prices: %w", err)
	}
	defer func() { _ = rows.Close() }()

	result := make([]service.ModelPlazaReferencePrice, 0)
	for rows.Next() {
		var price service.ModelPlazaReferencePrice
		var rawPricing []byte
		var updatedBy sql.NullInt64
		if err := rows.Scan(
			&price.ID, &price.Platform, &price.ModelName, &rawPricing,
			&price.SourceLabel, &price.SourceURL, &price.Notes, &updatedBy,
			&price.CreatedAt, &price.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan model plaza reference price: %w", err)
		}
		if err := json.Unmarshal(rawPricing, &price.Pricing); err != nil {
			return nil, fmt.Errorf("decode model plaza reference price %d: %w", price.ID, err)
		}
		if updatedBy.Valid {
			id := updatedBy.Int64
			price.UpdatedBy = &id
		}
		result = append(result, price)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate model plaza reference prices: %w", err)
	}
	return result, nil
}

func (r *modelPlazaReferencePriceRepository) ListModelPlazaReferencePricesPage(ctx context.Context, params pagination.PaginationParams, search string) ([]service.ModelPlazaReferencePrice, *pagination.PaginationResult, error) {
	params.PageSize = params.Limit()
	if params.Page < 1 {
		params.Page = 1
	}
	search = strings.TrimSpace(search)
	var total int64
	if err := r.db.QueryRowContext(ctx, `
		SELECT COUNT(*)
		FROM model_plaza_reference_prices
		WHERE $1 = '' OR platform ILIKE '%' || $1 || '%'
		   OR model_name ILIKE '%' || $1 || '%'
		   OR source_label ILIKE '%' || $1 || '%'
	`, search).Scan(&total); err != nil {
		return nil, nil, fmt.Errorf("count model plaza reference prices: %w", err)
	}

	rows, err := r.db.QueryContext(ctx, `
		SELECT id, platform, model_name, pricing, source_label, source_url, notes,
		       updated_by, created_at, updated_at
		FROM model_plaza_reference_prices
		WHERE $1 = '' OR platform ILIKE '%' || $1 || '%'
		   OR model_name ILIKE '%' || $1 || '%'
		   OR source_label ILIKE '%' || $1 || '%'
		ORDER BY LOWER(platform), LOWER(model_name), id
		LIMIT $2 OFFSET $3
	`, search, params.PageSize, params.Offset())
	if err != nil {
		return nil, nil, fmt.Errorf("list model plaza reference prices page: %w", err)
	}
	defer func() { _ = rows.Close() }()

	result := make([]service.ModelPlazaReferencePrice, 0)
	for rows.Next() {
		var price service.ModelPlazaReferencePrice
		var rawPricing []byte
		var updatedBy sql.NullInt64
		if err := rows.Scan(
			&price.ID, &price.Platform, &price.ModelName, &rawPricing,
			&price.SourceLabel, &price.SourceURL, &price.Notes, &updatedBy,
			&price.CreatedAt, &price.UpdatedAt,
		); err != nil {
			return nil, nil, fmt.Errorf("scan model plaza reference price: %w", err)
		}
		if err := json.Unmarshal(rawPricing, &price.Pricing); err != nil {
			return nil, nil, fmt.Errorf("decode model plaza reference price %d: %w", price.ID, err)
		}
		if updatedBy.Valid {
			id := updatedBy.Int64
			price.UpdatedBy = &id
		}
		result = append(result, price)
	}
	if err := rows.Err(); err != nil {
		return nil, nil, fmt.Errorf("iterate model plaza reference prices: %w", err)
	}
	pages := int(math.Ceil(float64(total) / float64(params.PageSize)))
	if pages < 1 {
		pages = 1
	}
	return result, &pagination.PaginationResult{
		Total: total, Page: params.Page, PageSize: params.PageSize, Pages: pages,
	}, nil
}

func (r *modelPlazaReferencePriceRepository) UpsertModelPlazaReferencePrice(ctx context.Context, price *service.ModelPlazaReferencePrice) error {
	rawPricing, err := json.Marshal(price.Pricing)
	if err != nil {
		return fmt.Errorf("encode model plaza reference pricing: %w", err)
	}
	err = r.db.QueryRowContext(ctx, `
		INSERT INTO model_plaza_reference_prices
			(platform, model_name, pricing, source_label, source_url, notes, updated_by)
		VALUES ($1, $2, $3::jsonb, $4, $5, $6, $7)
		ON CONFLICT (LOWER(platform), LOWER(model_name)) DO UPDATE SET
			pricing = EXCLUDED.pricing,
			source_label = EXCLUDED.source_label,
			source_url = EXCLUDED.source_url,
			notes = EXCLUDED.notes,
			updated_by = EXCLUDED.updated_by,
			updated_at = NOW()
		RETURNING id, created_at, updated_at
	`, price.Platform, price.ModelName, string(rawPricing), price.SourceLabel, price.SourceURL, price.Notes, price.UpdatedBy).
		Scan(&price.ID, &price.CreatedAt, &price.UpdatedAt)
	if err != nil {
		return fmt.Errorf("upsert model plaza reference price: %w", err)
	}
	return nil
}

func (r *modelPlazaReferencePriceRepository) DeleteModelPlazaReferencePrice(ctx context.Context, id int64) error {
	result, err := r.db.ExecContext(ctx, `DELETE FROM model_plaza_reference_prices WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("delete model plaza reference price: %w", err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("count deleted model plaza reference prices: %w", err)
	}
	if affected == 0 {
		return sql.ErrNoRows
	}
	return nil
}

func (r *modelPlazaReferencePriceRepository) DeleteModelPlazaReferencePrices(ctx context.Context, ids []int64) (int64, error) {
	if len(ids) == 0 {
		return 0, nil
	}
	placeholders := make([]string, len(ids))
	args := make([]any, len(ids))
	for i, id := range ids {
		placeholders[i] = fmt.Sprintf("$%d", i+1)
		args[i] = id
	}
	result, err := r.db.ExecContext(ctx,
		`DELETE FROM model_plaza_reference_prices WHERE id IN (`+strings.Join(placeholders, ",")+`)`,
		args...,
	)
	if err != nil {
		return 0, fmt.Errorf("delete model plaza reference prices: %w", err)
	}
	deleted, err := result.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("count deleted model plaza reference prices: %w", err)
	}
	return deleted, nil
}
