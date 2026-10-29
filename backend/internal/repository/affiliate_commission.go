package repository

import (
	"context"
	"errors"

	"akademi-bimbel/internal/model"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// ErrAffiliateCommissionExists reports that the order item already carries a
// commission (uq on order_item_id): a replayed OrderPaid event, not a failure.
var ErrAffiliateCommissionExists = errors.New("affiliate commission already exists for order item")

// GetAffiliatorByID returns one affiliator row, read inside the caller's
// transaction when tx is non-nil so the commission step sees the same state
// as the locked order row. A missing row returns (nil, nil).
func (r *Repository) GetAffiliatorByID(ctx context.Context, tx pgx.Tx, id uuid.UUID) (*model.Affiliator, error) {
	a, err := scanAffiliator(r.affiliateQueryer(tx).QueryRow(ctx,
		`SELECT `+affiliatorColumns+` `+affiliatorFrom+` WHERE a.user_id = $1`, id))
	if isNotFound(err) {
		return nil, nil
	}
	return a, err
}

// InsertAffiliateCommissionTx inserts one pending commission inside tx and
// fills ID/CreatedAt/UpdatedAt. ON CONFLICT (order_item_id) DO NOTHING makes a
// replayed OrderPaid event a no-op, reported as ErrAffiliateCommissionExists.
func (r *Repository) InsertAffiliateCommissionTx(ctx context.Context, tx pgx.Tx, c *model.AffiliateCommission) error {
	err := tx.QueryRow(ctx,
		`INSERT INTO affiliate_commissions
		     (affiliator_id, student_id, order_id, order_item_id, product_type,
		      quantity, base_amount, commission_type, commission_value, level_snapshot, amount, status, wait_until, paid_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14)
		 ON CONFLICT (order_item_id) DO NOTHING
		 RETURNING id, created_at, updated_at`,
		c.AffiliatorID, c.StudentID, c.OrderID, c.OrderItemID, c.ProductType,
		c.Quantity, c.BaseAmount, c.CommissionType, c.CommissionValue, c.LevelSnapshot, c.Amount, c.Status, c.WaitUntil, c.PaidAt,
	).Scan(&c.ID, &c.CreatedAt, &c.UpdatedAt)
	if isNotFound(err) {
		return ErrAffiliateCommissionExists
	}
	return err
}
