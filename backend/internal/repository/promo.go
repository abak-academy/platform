package repository

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"akademi-bimbel/internal/model"
)

func (r *Repository) GetPromoByCode(ctx context.Context, code string) (model.PromoCode, error) {
	p := model.PromoCode{}
	err := r.pool.QueryRow(ctx,
		`SELECT id, code, discount_percent, discount_amount, min_order_amount, max_discount_amount, max_uses, used_count, expires_at, created_at, is_public, starts_at, min_qty, affiliator_only, product_ids::text[]
		FROM promo_code
		WHERE code = $1`,
		code,
	).Scan(
		&p.ID, &p.Code, &p.DiscountPercent, &p.DiscountAmount, &p.MinOrderAmount, &p.MaxDiscountAmount, &p.MaxUses, &p.UsedCount, &p.ExpiresAt, &p.CreatedAt, &p.IsPublic, &p.StartsAt, &p.MinQty, &p.AffiliatorOnly, &p.ProductIDs,
	)
	if err != nil {
		if isNotFound(err) {
			return model.PromoCode{}, nil
		}
		return model.PromoCode{}, err
	}
	return p, nil
}

func (r *Repository) CreatePromoCode(ctx context.Context, p model.PromoCode) (model.PromoCode, error) {
	err := r.pool.QueryRow(ctx,
		`INSERT INTO promo_code (code, discount_percent, discount_amount, min_order_amount, max_discount_amount, max_uses, used_count, expires_at, is_public, starts_at, min_qty, affiliator_only, product_ids)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13::uuid[])
		RETURNING id, code, discount_percent, discount_amount, min_order_amount, max_discount_amount, max_uses, used_count, expires_at, created_at, is_public, starts_at, min_qty, affiliator_only, product_ids::text[]`,
		p.Code, p.DiscountPercent, p.DiscountAmount, p.MinOrderAmount, p.MaxDiscountAmount, p.MaxUses, p.UsedCount, p.ExpiresAt, p.IsPublic, p.StartsAt, p.MinQty, p.AffiliatorOnly, nonNilStrings(p.ProductIDs),
	).Scan(
		&p.ID, &p.Code, &p.DiscountPercent, &p.DiscountAmount, &p.MinOrderAmount, &p.MaxDiscountAmount, &p.MaxUses, &p.UsedCount, &p.ExpiresAt, &p.CreatedAt, &p.IsPublic, &p.StartsAt, &p.MinQty, &p.AffiliatorOnly, &p.ProductIDs,
	)
	return p, err
}

// UpdatePromoCode replaces every editable field; the code and used_count never change.
func (r *Repository) UpdatePromoCode(ctx context.Context, id uuid.UUID, p model.PromoCode) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE promo_code SET discount_percent = $1, discount_amount = $2, min_order_amount = $3, max_discount_amount = $4,
			max_uses = $5, expires_at = $6, is_public = $7, starts_at = $8, min_qty = $9, affiliator_only = $10, product_ids = $11::uuid[]
		WHERE id = $12`,
		p.DiscountPercent, p.DiscountAmount, p.MinOrderAmount, p.MaxDiscountAmount,
		p.MaxUses, p.ExpiresAt, p.IsPublic, p.StartsAt, p.MinQty, p.AffiliatorOnly, nonNilStrings(p.ProductIDs), id,
	)
	return err
}

func nonNilStrings(in []string) []string {
	if in == nil {
		return []string{}
	}
	return in
}

func (r *Repository) DeletePromoCode(ctx context.Context, id uuid.UUID) error {
	_, err := r.pool.Exec(ctx,
		`DELETE FROM promo_code WHERE id = $1`,
		id,
	)
	return err
}

func (r *Repository) ListPromoCodes(ctx context.Context) ([]model.PromoCode, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT id, code, discount_percent, discount_amount, min_order_amount, max_discount_amount, max_uses, used_count, expires_at, created_at, is_public, starts_at, min_qty, affiliator_only, product_ids::text[]
		FROM promo_code
		ORDER BY created_at DESC`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	promos := []model.PromoCode{}
	for rows.Next() {
		p := model.PromoCode{}
		err := rows.Scan(
			&p.ID, &p.Code, &p.DiscountPercent, &p.DiscountAmount, &p.MinOrderAmount, &p.MaxDiscountAmount, &p.MaxUses, &p.UsedCount, &p.ExpiresAt, &p.CreatedAt, &p.IsPublic, &p.StartsAt, &p.MinQty, &p.AffiliatorOnly, &p.ProductIDs,
		)
		if err != nil {
			return nil, err
		}
		promos = append(promos, p)
	}
	return promos, rows.Err()
}

// ListActivePublicPromos returns promo codes flagged for public listing whose
// validity still holds under the same rules ValidatePromo applies at
// store.go:389-394. is_public controls listing only, not a bypass of those
// rules — an expired or exhausted public code must not appear here.
func (r *Repository) ListActivePublicPromos(ctx context.Context) ([]model.PromoCode, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT id, code, discount_percent, discount_amount, min_order_amount, max_discount_amount, max_uses, used_count, expires_at, created_at, is_public, starts_at, min_qty, affiliator_only, product_ids::text[]
		FROM promo_code
		WHERE is_public = true
		AND affiliator_only = false
		AND (starts_at IS NULL OR starts_at <= now())
		AND (expires_at IS NULL OR expires_at > now())
		AND (max_uses IS NULL OR used_count < max_uses)
		ORDER BY created_at DESC`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	promos := []model.PromoCode{}
	for rows.Next() {
		p := model.PromoCode{}
		err := rows.Scan(
			&p.ID, &p.Code, &p.DiscountPercent, &p.DiscountAmount, &p.MinOrderAmount, &p.MaxDiscountAmount, &p.MaxUses, &p.UsedCount, &p.ExpiresAt, &p.CreatedAt, &p.IsPublic, &p.StartsAt, &p.MinQty, &p.AffiliatorOnly, &p.ProductIDs,
		)
		if err != nil {
			return nil, err
		}
		promos = append(promos, p)
	}
	return promos, rows.Err()
}

func (r *Repository) IncrementPromoUses(ctx context.Context, id uuid.UUID) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE promo_code SET used_count = used_count + 1 WHERE id = $1`,
		id,
	)
	return err
}

// IncrementPromoUsesTx is IncrementPromoUses inside a caller's transaction, so
// the usage count commits atomically with the order it belongs to. Free
// checkout needs this: it has no gateway round-trip to wait on, so there is no
// reason to settle the promo count outside the transaction and risk losing it.
func (r *Repository) IncrementPromoUsesTx(ctx context.Context, tx pgx.Tx, id uuid.UUID) error {
	_, err := tx.Exec(ctx,
		`UPDATE promo_code SET used_count = used_count + 1 WHERE id = $1`,
		id,
	)
	return err
}

// ListActiveAffiliatorPromos returns affiliator-only promo codes that are inside
// their date range and not exhausted.
func (r *Repository) ListActiveAffiliatorPromos(ctx context.Context) ([]model.PromoCode, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT id, code, discount_percent, discount_amount, min_order_amount, max_discount_amount, max_uses, used_count, expires_at, created_at, is_public, starts_at, min_qty, affiliator_only, product_ids::text[]
		FROM promo_code
		WHERE affiliator_only = true
		AND (starts_at IS NULL OR starts_at <= now())
		AND (expires_at IS NULL OR expires_at > now())
		AND (max_uses IS NULL OR used_count < max_uses)
		ORDER BY created_at DESC`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	promos := []model.PromoCode{}
	for rows.Next() {
		p := model.PromoCode{}
		if err := rows.Scan(
			&p.ID, &p.Code, &p.DiscountPercent, &p.DiscountAmount, &p.MinOrderAmount, &p.MaxDiscountAmount, &p.MaxUses, &p.UsedCount, &p.ExpiresAt, &p.CreatedAt, &p.IsPublic, &p.StartsAt, &p.MinQty, &p.AffiliatorOnly, &p.ProductIDs,
		); err != nil {
			return nil, err
		}
		promos = append(promos, p)
	}
	return promos, rows.Err()
}
