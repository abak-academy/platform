package repository

import (
	"context"

	"akademi-bimbel/internal/model"
)

// CreateAffiliatorAccount inserts the pending-verification user and its
// pending_approval profile in one tx; the referral code is assigned at approval.
func (r *Repository) CreateAffiliatorAccount(ctx context.Context, u *model.User, termsVersion string) error {
	u.Email = normalizeOptionalEmail(u.Email)
	if u.AuthProvider == "" {
		u.AuthProvider = "password"
	}
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if err := tx.QueryRow(ctx,
		`INSERT INTO users (email, username, phone, password_hash, role, name, status, otp_enabled, auth_provider)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		 RETURNING id, created_at, updated_at`,
		u.Email, u.Username, u.Phone, u.PasswordHash, u.Role, u.Name, u.Status, u.OTPEnabled, u.AuthProvider,
	).Scan(&u.ID, &u.CreatedAt, &u.UpdatedAt); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx,
		`INSERT INTO affiliators (user_id, status, terms_version) VALUES ($1, 'pending_approval', $2)`,
		u.ID, termsVersion,
	); err != nil {
		return err
	}
	return tx.Commit(ctx)
}
