package repository

import (
	"context"
	"encoding/json"
	"errors"

	"akademi-bimbel/internal/model"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// affiliateMembershipSQL selects the accounts that take part in the Affiliator
// program: a NULL programs column is a plain Affiliator account.
const affiliateMembershipSQL = `(a.programs IS NULL OR (a.programs->>'affiliate')::boolean IS NOT FALSE)`

// partnerMembershipSQL selects the accounts that take part in the Mitra program.
const partnerMembershipSQL = `jsonb_typeof(a.programs->'partner') = 'object'`

// AffiliatorAccess is what a program gate needs about one account: the
// registration status of its profile, the login status of its user, and the
// programs it belongs to.
type AffiliatorAccess struct {
	UserStatus      string
	ProgramStatus   string
	RejectionReason *string
	Programs        model.AffiliatorPrograms
}

// GetAffiliatorAccess returns nil when the user has no affiliator profile.
func (r *Repository) GetAffiliatorAccess(ctx context.Context, userID string) (*AffiliatorAccess, error) {
	var access AffiliatorAccess
	var programs []byte
	err := r.pool.QueryRow(ctx,
		`SELECT u.status, a.program_status, a.rejection_reason, a.programs
		 FROM affiliators a JOIN users u ON u.id = a.user_id
		 WHERE a.user_id = $1`, userID,
	).Scan(&access.UserStatus, &access.ProgramStatus, &access.RejectionReason, &programs)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if len(programs) > 0 {
		if err := json.Unmarshal(programs, &access.Programs); err != nil {
			return nil, err
		}
	}
	return &access, nil
}

// programsParam is the affiliators.programs value for p: NULL for a plain
// Affiliator account, so legacy rows and new ones mean the same thing.
func programsParam(p model.AffiliatorPrograms) (any, error) {
	if p.Affiliate == nil && p.Partner == nil {
		return nil, nil
	}
	raw, err := json.Marshal(p)
	if err != nil {
		return nil, err
	}
	return raw, nil
}

// SetAffiliatorProgramsTx replaces an account's program membership inside the
// caller's transaction.
func (r *Repository) SetAffiliatorProgramsTx(ctx context.Context, tx pgx.Tx, userID uuid.UUID, p model.AffiliatorPrograms) error {
	param, err := programsParam(p)
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `UPDATE affiliators SET programs = $2 WHERE user_id = $1`, userID, param)
	return err
}

// SetAffiliatorReferralCodeIfNoneTx stores a referral code on an account that
// has none yet and reports whether it was stored.
func (r *Repository) SetAffiliatorReferralCodeIfNoneTx(ctx context.Context, tx pgx.Tx, userID uuid.UUID, code string) (bool, error) {
	tag, err := tx.Exec(ctx,
		`UPDATE affiliators SET referral_code = $2 WHERE user_id = $1 AND referral_code IS NULL`, userID, code)
	if err != nil {
		return false, err
	}
	return tag.RowsAffected() > 0, nil
}
