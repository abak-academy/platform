package repository

import (
	"context"

	"akademi-bimbel/internal/model"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

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
