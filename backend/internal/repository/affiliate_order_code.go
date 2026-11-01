package repository

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// SetOrderAffiliateCodeTx stashes the checkout-time referral code on the order
// row inside the caller's transaction; the settlement paths read it back when
// the order is paid. There is no dedicated binding state at checkout — the
// code on the row is only attribution input, revalidated at settlement.
func (r *Repository) SetOrderAffiliateCodeTx(ctx context.Context, tx pgx.Tx, orderID uuid.UUID, code string) error {
	_, err := tx.Exec(ctx,
		`UPDATE orders SET affiliate_code = $1, updated_at = now() WHERE id = $2`,
		code, orderID,
	)
	return err
}

// GetOrderAffiliateCodeTx returns the order's stored referral code, or nil
// when the order carries none.
func (r *Repository) GetOrderAffiliateCodeTx(ctx context.Context, tx pgx.Tx, orderID uuid.UUID) (*string, error) {
	var code *string
	err := tx.QueryRow(ctx,
		`SELECT affiliate_code FROM orders WHERE id = $1`,
		orderID,
	).Scan(&code)
	if err != nil {
		return nil, err
	}
	return code, nil
}
