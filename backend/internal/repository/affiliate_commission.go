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
	var a model.Affiliator
	err := r.affiliateQueryer(tx).QueryRow(ctx,
		`SELECT id, user_id, COALESCE(code, ''), status, rejection_reason, level, level_locked_until,
		        warn_quarters, suspended_until, frozen_until, terms_version, terms_accepted_at,
		        bank_name, bank_account_number, bank_account_holder, created_at, updated_at
		 FROM affiliators
		 WHERE id = $1`,
		id,
	).Scan(&a.ID, &a.UserID, &a.Code, &a.Status, &a.RejectionReason, &a.Level, &a.LevelLockedUntil,
		&a.WarnQuarters, &a.SuspendedUntil, &a.FrozenUntil, &a.TermsVersion, &a.TermsAcceptedAt,
		&a.BankName, &a.BankAccountNumber, &a.BankAccountHolder, &a.CreatedAt, &a.UpdatedAt)
	if isNotFound(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &a, nil
}

// InsertAffiliateCommissionTx inserts one pending commission inside tx and
// fills ID/CreatedAt/UpdatedAt. ON CONFLICT (order_item_id) DO NOTHING makes a
// replayed OrderPaid event a no-op, reported as ErrAffiliateCommissionExists.
func (r *Repository) InsertAffiliateCommissionTx(ctx context.Context, tx pgx.Tx, c *model.AffiliateCommission) error {
	err := tx.QueryRow(ctx,
		`INSERT INTO affiliate_commissions
		     (affiliator_id, student_id, order_id, order_item_id, product_type,
		      base_amount, percentage, level_snapshot, amount, status, wait_until, paid_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
		 ON CONFLICT (order_item_id) DO NOTHING
		 RETURNING id, created_at, updated_at`,
		c.AffiliatorID, c.StudentID, c.OrderID, c.OrderItemID, c.ProductType,
		c.BaseAmount, c.Percentage, c.LevelSnapshot, c.Amount, c.Status, c.WaitUntil, c.PaidAt,
	).Scan(&c.ID, &c.CreatedAt, &c.UpdatedAt)
	if isNotFound(err) {
		return ErrAffiliateCommissionExists
	}
	return err
}
