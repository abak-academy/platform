package repository

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"akademi-bimbel/internal/model"
)

// ErrPromoQuotaExceeded reports that a promo cannot take the uses asked for.
var ErrPromoQuotaExceeded = errors.New("promo quota exceeded")

const promoColumns = `id, code, discount_percent, discount_amount, min_order_amount, max_discount_amount, max_uses, used_count, expires_at, created_at, is_public, affiliator_only, partner`

type promoRow interface {
	Scan(dest ...any) error
}

func scanPromo(row promoRow, p *model.PromoCode) error {
	var partner []byte
	if err := row.Scan(
		&p.ID, &p.Code, &p.DiscountPercent, &p.DiscountAmount, &p.MinOrderAmount, &p.MaxDiscountAmount, &p.MaxUses, &p.UsedCount, &p.ExpiresAt, &p.CreatedAt, &p.IsPublic, &p.AffiliatorOnly, &partner,
	); err != nil {
		return err
	}
	p.Partner = nil
	if len(partner) > 0 {
		var v model.PartnerVoucher
		if err := json.Unmarshal(partner, &v); err != nil {
			return err
		}
		if v.IDs == nil {
			v.IDs = []string{}
		}
		p.Partner = &v
	}
	return nil
}

func partnerVoucherParam(v *model.PartnerVoucher) (any, error) {
	if v == nil {
		return nil, nil
	}
	if v.IDs == nil {
		v.IDs = []string{}
	}
	return json.Marshal(v)
}

func (r *Repository) scanPromos(rows pgx.Rows) ([]model.PromoCode, error) {
	defer rows.Close()
	promos := []model.PromoCode{}
	for rows.Next() {
		p := model.PromoCode{}
		if err := scanPromo(rows, &p); err != nil {
			return nil, err
		}
		promos = append(promos, p)
	}
	return promos, rows.Err()
}

func (r *Repository) GetPromoByCode(ctx context.Context, code string) (model.PromoCode, error) {
	p := model.PromoCode{}
	err := scanPromo(r.pool.QueryRow(ctx, `SELECT `+promoColumns+` FROM promo_code WHERE code = $1`, code), &p)
	if err != nil {
		if isNotFound(err) {
			return model.PromoCode{}, nil
		}
		return model.PromoCode{}, err
	}
	return p, nil
}

// GetPromoByID returns the promo with this id, or a zero value when it does not exist.
func (r *Repository) GetPromoByID(ctx context.Context, id uuid.UUID) (model.PromoCode, error) {
	p := model.PromoCode{}
	err := scanPromo(r.pool.QueryRow(ctx, `SELECT `+promoColumns+` FROM promo_code WHERE id = $1`, id), &p)
	if err != nil {
		if isNotFound(err) {
			return model.PromoCode{}, nil
		}
		return model.PromoCode{}, err
	}
	return p, nil
}

func (r *Repository) CreatePromoCode(ctx context.Context, p model.PromoCode) (model.PromoCode, error) {
	partner, err := partnerVoucherParam(p.Partner)
	if err != nil {
		return model.PromoCode{}, err
	}
	err = scanPromo(r.pool.QueryRow(ctx,
		`INSERT INTO promo_code (code, discount_percent, discount_amount, min_order_amount, max_discount_amount, max_uses, used_count, expires_at, is_public, affiliator_only, partner)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
		RETURNING `+promoColumns,
		p.Code, p.DiscountPercent, p.DiscountAmount, p.MinOrderAmount, p.MaxDiscountAmount, p.MaxUses, p.UsedCount, p.ExpiresAt, p.IsPublic, p.AffiliatorOnly, partner,
	), &p)
	return p, err
}

// PartnerVoucherChange says whether an update touches the Mitra voucher
// setting. Set false leaves it as it is; Set true with a nil Value turns the
// promo back into an ordinary one.
type PartnerVoucherChange struct {
	Set   bool
	Value *model.PartnerVoucher
}

func (r *Repository) UpdatePromoCode(ctx context.Context, id uuid.UUID, maxUses *int, expiresAt *time.Time, isPublic, affiliatorOnly bool, partner PartnerVoucherChange) error {
	value, err := partnerVoucherParam(partner.Value)
	if err != nil {
		return err
	}
	_, err = r.pool.Exec(ctx,
		`UPDATE promo_code SET max_uses = $1, expires_at = $2, is_public = $3, affiliator_only = $4,
		     partner = CASE WHEN $5 THEN $6::jsonb ELSE partner END
		 WHERE id = $7`,
		maxUses, expiresAt, isPublic, affiliatorOnly, partner.Set, value, id,
	)
	return err
}

func (r *Repository) DeletePromoCode(ctx context.Context, id uuid.UUID) error {
	_, err := r.pool.Exec(ctx,
		`DELETE FROM promo_code WHERE id = $1`,
		id,
	)
	return err
}

func (r *Repository) ListPromoCodes(ctx context.Context) ([]model.PromoCode, error) {
	rows, err := r.pool.Query(ctx, `SELECT `+promoColumns+` FROM promo_code ORDER BY created_at DESC`)
	if err != nil {
		return nil, err
	}
	return r.scanPromos(rows)
}

// ListActivePublicPromos returns promo codes flagged for public listing whose
// validity still holds under the same rules ValidatePromo applies at
// store.go:389-394. is_public controls listing only, not a bypass of those
// rules — an expired or exhausted public code must not appear here.
func (r *Repository) ListActivePublicPromos(ctx context.Context) ([]model.PromoCode, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT `+promoColumns+`
		FROM promo_code
		WHERE is_public = true AND NOT affiliator_only AND partner IS NULL
		AND (expires_at IS NULL OR expires_at > now())
		AND (max_uses IS NULL OR used_count < max_uses)
		ORDER BY created_at DESC`,
	)
	if err != nil {
		return nil, err
	}
	return r.scanPromos(rows)
}

// ListAffiliatorOnlyPromos returns the still-valid promo codes meant for
// affiliators to share, under the same validity rules as the public listing.
func (r *Repository) ListAffiliatorOnlyPromos(ctx context.Context) ([]model.PromoCode, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT `+promoColumns+`
		FROM promo_code
		WHERE affiliator_only = true
		AND (expires_at IS NULL OR expires_at > now())
		AND (max_uses IS NULL OR used_count < max_uses)
		ORDER BY created_at DESC`,
	)
	if err != nil {
		return nil, err
	}
	return r.scanPromos(rows)
}

// ListPartnerPromos returns the still-valid Mitra vouchers a Mitra may use: the
// ones for every Mitra and the ones that name this Mitra.
func (r *Repository) ListPartnerPromos(ctx context.Context, partnerID string) ([]model.PromoCode, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT `+promoColumns+`
		FROM promo_code
		WHERE partner IS NOT NULL
		AND (jsonb_array_length(partner->'ids') = 0 OR partner->'ids' @> to_jsonb($1::text))
		AND (expires_at IS NULL OR expires_at > now())
		AND (max_uses IS NULL OR used_count < max_uses)
		ORDER BY created_at DESC`,
		partnerID,
	)
	if err != nil {
		return nil, err
	}
	return r.scanPromos(rows)
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
// It takes n uses at once and refuses, with ErrPromoQuotaExceeded, when that
// would pass max_uses: the limit is checked by the UPDATE itself, so two
// checkouts racing for the last use cannot both get it.
func (r *Repository) IncrementPromoUsesTx(ctx context.Context, tx pgx.Tx, id uuid.UUID, n int) error {
	tag, err := tx.Exec(ctx,
		`UPDATE promo_code SET used_count = used_count + $2
		 WHERE id = $1 AND (max_uses IS NULL OR used_count + $2 <= max_uses)`,
		id, n,
	)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrPromoQuotaExceeded
	}
	return nil
}
