package repository

import (
	"context"

	"akademi-bimbel/internal/model"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
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
		`INSERT INTO affiliators (user_id, program_status, terms_version) VALUES ($1, 'pending_approval', $2)`,
		u.ID, termsVersion,
	); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// CreateActiveAffiliatorAccount inserts an admin-created user with a temporary
// password and its already-approved profile, with the referral code, in one tx.
func (r *Repository) CreateActiveAffiliatorAccount(ctx context.Context, u *model.User, termsVersion, code string) error {
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
		`INSERT INTO users (email, username, phone, password_hash, role, name, status, auth_provider, password_temporary)
		 VALUES ($1, $2, $3, $4, $5, $6, 'active', $7, true)
		 RETURNING id, created_at, updated_at`,
		u.Email, u.Username, u.Phone, u.PasswordHash, u.Role, u.Name, u.AuthProvider,
	).Scan(&u.ID, &u.CreatedAt, &u.UpdatedAt); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx,
		`INSERT INTO affiliators (user_id, referral_code, program_status, terms_version) VALUES ($1, $2, 'active', $3)`,
		u.ID, code, termsVersion,
	); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// ReopenRejectedAffiliatorTx puts a rejected profile back in the approval
// queue and clears the rejection reason; false means it was not rejected.
func (r *Repository) ReopenRejectedAffiliatorTx(ctx context.Context, tx pgx.Tx, id uuid.UUID) (bool, error) {
	tag, err := tx.Exec(ctx,
		`UPDATE affiliators SET program_status = 'pending_approval', rejection_reason = NULL
		 WHERE user_id = $1 AND program_status = 'rejected'`, id)
	if err != nil {
		return false, err
	}
	return tag.RowsAffected() > 0, nil
}
