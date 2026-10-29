package repository

import (
	"context"
	"errors"
	"time"

	"akademi-bimbel/internal/model"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// ErrAffiliateBindingExists reports that the student already holds an active
// binding (uq_affiliate_bindings_student_active).
var ErrAffiliateBindingExists = errors.New("affiliate binding already active for student")

// AffiliatorByCode is the code owner joined with the owner user's contact
// fields, which the self-purchase rule compares against the student's.
type AffiliatorByCode struct {
	ID     uuid.UUID
	Status string
	Email  *string
	Phone  *string
}

// GetAffiliatorByCode returns the affiliator owning a referral code together
// with the owner user's contact fields (self-purchase detection). A missing
// code returns (nil, nil).
func (r *Repository) GetAffiliatorByCode(ctx context.Context, code string) (*AffiliatorByCode, error) {
	owner := &AffiliatorByCode{}
	err := r.pool.QueryRow(ctx,
		`SELECT a.user_id, a.program_status, u.email, u.phone
		 FROM affiliators a JOIN users u ON u.id = a.user_id
		 WHERE a.referral_code = $1`,
		code,
	).Scan(&owner.ID, &owner.Status, &owner.Email, &owner.Phone)
	if isNotFound(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return owner, nil
}

func (r *Repository) affiliateQueryer(tx pgx.Tx) queryer {
	if tx != nil {
		return tx
	}
	return r.pool
}

// GetActiveBindingByStudent returns the student's active binding, or (nil, nil)
// when none exists. A nil tx reads outside any transaction.
func (r *Repository) GetActiveBindingByStudent(ctx context.Context, tx pgx.Tx, studentID string) (*model.AffiliateBinding, error) {
	var b model.AffiliateBinding
	err := r.affiliateQueryer(tx).QueryRow(ctx,
		`SELECT id, affiliator_id, student_id, status, source, started_at, expires_at, created_at, updated_at
		 FROM affiliate_bindings
		 WHERE student_id = $1 AND status = 'active'`,
		studentID,
	).Scan(&b.ID, &b.AffiliatorID, &b.StudentID, &b.Status, &b.Source, &b.StartedAt, &b.ExpiresAt, &b.CreatedAt, &b.UpdatedAt)
	if isNotFound(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &b, nil
}

// InsertBinding inserts one binding inside tx, honouring
// uq_affiliate_bindings_student_active via ON CONFLICT DO NOTHING; the
// already-exists sentinel is returned when no row was inserted.
func (r *Repository) InsertBinding(ctx context.Context, tx pgx.Tx, b *model.AffiliateBinding) error {
	err := tx.QueryRow(ctx,
		`INSERT INTO affiliate_bindings (affiliator_id, student_id, status, source, started_at, expires_at)
		 VALUES ($1, $2, $3, $4, $5, $6)
		 ON CONFLICT (student_id) WHERE status = 'active' DO NOTHING
		 RETURNING id, created_at, updated_at`,
		b.AffiliatorID, b.StudentID, b.Status, b.Source, b.StartedAt, b.ExpiresAt,
	).Scan(&b.ID, &b.CreatedAt, &b.UpdatedAt)
	if isNotFound(err) {
		return ErrAffiliateBindingExists
	}
	return err
}

// ExtendBindingTx moves an active binding's expires_at inside tx.
func (r *Repository) ExtendBindingTx(ctx context.Context, tx pgx.Tx, bindingID uuid.UUID, newExpiresAt time.Time) error {
	_, err := tx.Exec(ctx,
		`UPDATE affiliate_bindings SET expires_at = $2, updated_at = now()
		 WHERE id = $1 AND status = 'active'`,
		bindingID, newExpiresAt,
	)
	return err
}

// ExpireDueBindings flips active bindings with expires_at < now to 'ended' and
// returns how many rows changed (0 on repeat runs).
func (r *Repository) ExpireDueBindings(ctx context.Context, now time.Time) (int64, error) {
	tag, err := r.pool.Exec(ctx,
		`UPDATE affiliate_bindings SET status = 'ended', updated_at = now()
		 WHERE status = 'active' AND expires_at < $1`,
		now,
	)
	if err != nil {
		return 0, err
	}
	return tag.RowsAffected(), nil
}
