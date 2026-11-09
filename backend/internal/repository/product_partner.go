package repository

import (
	"context"
	"encoding/json"
	"errors"

	"akademi-bimbel/internal/model"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// GetProductPartnerAccess returns nil for a product nobody opened to a Mitra.
func (r *Repository) GetProductPartnerAccess(ctx context.Context, productID uuid.UUID) (*model.PartnerAccess, error) {
	var raw []byte
	err := r.pool.QueryRow(ctx, `SELECT partner_access FROM product WHERE id = $1`, productID).Scan(&raw)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if len(raw) == 0 {
		return nil, nil
	}
	var access model.PartnerAccess
	if err := json.Unmarshal(raw, &access); err != nil {
		return nil, err
	}
	if access.IDs == nil {
		access.IDs = []string{}
	}
	return &access, nil
}

// SetProductPartnerAccess stores a product's Mitra access; nil removes it, which
// reads as scope "none". It is a column of its own that UpdateProduct never
// writes, so saving the product form cannot overwrite it.
func (r *Repository) SetProductPartnerAccess(ctx context.Context, productID uuid.UUID, access *model.PartnerAccess) error {
	var value any
	if access != nil {
		raw, err := json.Marshal(access)
		if err != nil {
			return err
		}
		value = raw
	}
	_, err := r.pool.Exec(ctx, `UPDATE product SET partner_access = $2 WHERE id = $1`, productID, value)
	return err
}

// ProductOpenToPartner reports whether the admin opened a product to this Mitra.
func (r *Repository) ProductOpenToPartner(ctx context.Context, productID uuid.UUID, partnerID string) (bool, error) {
	var ok bool
	err := r.pool.QueryRow(ctx,
		`SELECT EXISTS (
			SELECT 1 FROM product p
			WHERE p.id = $1
			  AND (p.partner_access->>'scope' = 'all'
			       OR (p.partner_access->>'scope' = 'selected' AND p.partner_access->'ids' @> to_jsonb($2::text)))
		)`, productID, partnerID,
	).Scan(&ok)
	return ok, err
}
