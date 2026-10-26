package repository

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"akademi-bimbel/internal/model"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// AffiliateAdminFilter carries the optional admin list filters; Query matches
// the affiliator's name, email or referral code as a substring.
type AffiliateAdminFilter struct {
	Status string
	Level  string
	Query  string
	Offset int
	Limit  int
}

// AffiliateAdminListRow is one admin list row: the affiliator profile joined
// with the owning user's contact fields.
type AffiliateAdminListRow struct {
	model.Affiliator
	Name  string  `json:"name"`
	Email *string `json:"email"`
	Phone *string `json:"phone"`
}

// affiliateAdminFilterSQL renders the shared WHERE tail for the admin list and
// its count so the two cannot drift.
func affiliateAdminFilterSQL(filter AffiliateAdminFilter, argIdx int) (string, []interface{}, int) {
	query := ""
	args := []interface{}{}
	if filter.Status != "" {
		query += fmt.Sprintf(` AND a.status = $%d`, argIdx)
		args = append(args, filter.Status)
		argIdx++
	}
	if filter.Level != "" {
		query += fmt.Sprintf(` AND a.level = $%d`, argIdx)
		args = append(args, filter.Level)
		argIdx++
	}
	if filter.Query != "" {
		query += fmt.Sprintf(` AND (LOWER(u.name) LIKE LOWER($%d) OR LOWER(u.email) LIKE LOWER($%d) OR LOWER(a.code) LIKE LOWER($%d))`, argIdx, argIdx, argIdx)
		args = append(args, "%"+filter.Query+"%")
		argIdx++
	}
	return query, args, argIdx
}

// ListAffiliateAdmins returns affiliators for the admin list plus the total
// count for the same filters (offset pagination).
func (r *Repository) ListAffiliateAdmins(ctx context.Context, filter AffiliateAdminFilter) ([]AffiliateAdminListRow, int, error) {
	where, args, next := affiliateAdminFilterSQL(filter, 1)

	var total int
	err := r.pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM affiliators a JOIN users u ON u.id = a.user_id WHERE 1=1`+where,
		args...,
	).Scan(&total)
	if err != nil {
		return nil, 0, err
	}

	query := `SELECT a.id, a.user_id, COALESCE(a.code, ''), a.status, a.rejection_reason, a.level, a.level_locked_until,
	        a.warn_quarters, a.suspended_until, a.frozen_until, a.terms_version, a.terms_accepted_at,
	        a.bank_name, a.bank_account_number, a.bank_account_holder, a.created_at, a.updated_at,
	        u.name, u.email, u.phone
	 FROM affiliators a
	 JOIN users u ON u.id = a.user_id
	 WHERE 1=1` + where + `
	 ORDER BY a.created_at DESC, a.id
	 LIMIT $` + strconv.Itoa(next) + ` OFFSET $` + strconv.Itoa(next+1)
	args = append(args, filter.Limit, filter.Offset)

	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	out := []AffiliateAdminListRow{}
	for rows.Next() {
		var row AffiliateAdminListRow
		if err := rows.Scan(&row.ID, &row.UserID, &row.Code, &row.Status, &row.RejectionReason, &row.Level,
			&row.LevelLockedUntil, &row.WarnQuarters, &row.SuspendedUntil, &row.FrozenUntil,
			&row.TermsVersion, &row.TermsAcceptedAt, &row.BankName, &row.BankAccountNumber,
			&row.BankAccountHolder, &row.CreatedAt, &row.UpdatedAt,
			&row.Name, &row.Email, &row.Phone); err != nil {
			return nil, 0, err
		}
		out = append(out, row)
	}
	return out, total, rows.Err()
}

// AffiliateAdminUserContact is the owning user's header fields for the
// admin detail page.
type AffiliateAdminUserContact struct {
	Name  string
	Email *string
}

// GetAffiliateAdminUserContact returns the display name and email of the user
// behind one affiliator.
func (r *Repository) GetAffiliateAdminUserContact(ctx context.Context, userID uuid.UUID) (AffiliateAdminUserContact, error) {
	var c AffiliateAdminUserContact
	err := r.pool.QueryRow(ctx,
		`SELECT name, email FROM users WHERE id = $1`,
		userID,
	).Scan(&c.Name, &c.Email)
	return c, err
}

// AffiliateCommissionStatusTotal aggregates one commission status: the row
// count and the summed amounts behind it.
type AffiliateCommissionStatusTotal struct {
	Status string
	Count  int
	Amount float64
}

// SumAffiliateCommissionsByStatus returns one count/amount row per commission
// status the affiliator has commissions in.
func (r *Repository) SumAffiliateCommissionsByStatus(ctx context.Context, affiliatorID uuid.UUID) ([]AffiliateCommissionStatusTotal, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT status, count(*), COALESCE(SUM(amount), 0)
		 FROM affiliate_commissions
		 WHERE affiliator_id = $1
		 GROUP BY status`,
		affiliatorID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := []AffiliateCommissionStatusTotal{}
	for rows.Next() {
		var t AffiliateCommissionStatusTotal
		if err := rows.Scan(&t.Status, &t.Count, &t.Amount); err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

// ListAffiliateLevelHistory returns the affiliator's level history newest
// first, capped at limit rows.
func (r *Repository) ListAffiliateLevelHistory(ctx context.Context, affiliatorID uuid.UUID, limit int) ([]model.AffiliateLevelHistory, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT id, affiliator_id, quarter, from_level, to_level, net_revenue, source, reason, created_by, created_at
		 FROM affiliate_level_history
		 WHERE affiliator_id = $1
		 ORDER BY created_at DESC, id DESC
		 LIMIT $2`,
		affiliatorID, limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := []model.AffiliateLevelHistory{}
	for rows.Next() {
		var h model.AffiliateLevelHistory
		if err := rows.Scan(&h.ID, &h.AffiliatorID, &h.Quarter, &h.FromLevel, &h.ToLevel, &h.NetRevenue,
			&h.Source, &h.Reason, &h.CreatedBy, &h.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, h)
	}
	return out, rows.Err()
}

// InsertAffiliateLevelHistoryManualTx appends one manual level-history row,
// recording the acting admin; InsertAffiliateLevelHistoryTx covers the auto
// path where created_by stays NULL.
func (r *Repository) InsertAffiliateLevelHistoryManualTx(ctx context.Context, tx pgx.Tx, h *model.AffiliateLevelHistory) error {
	return tx.QueryRow(ctx,
		`INSERT INTO affiliate_level_history (affiliator_id, quarter, from_level, to_level, net_revenue, source, reason, created_by)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		 RETURNING id, created_at`,
		h.AffiliatorID, h.Quarter, h.FromLevel, h.ToLevel, h.NetRevenue, h.Source, h.Reason, h.CreatedBy,
	).Scan(&h.ID, &h.CreatedAt)
}

// AffiliateCodeExists reports whether a referral code is already taken.
func (r *Repository) AffiliateCodeExists(ctx context.Context, code string) (bool, error) {
	var exists bool
	err := r.pool.QueryRow(ctx,
		`SELECT EXISTS (SELECT 1 FROM affiliators WHERE code = $1)`,
		code,
	).Scan(&exists)
	return exists, err
}

// ApproveAffiliatorTx activates a pending affiliator and stores the generated
// referral code inside the caller's transaction. It reports false when the
// profile is no longer pending, so a double submit cannot overwrite a code that
// was already handed out.
func (r *Repository) ApproveAffiliatorTx(ctx context.Context, tx pgx.Tx, id uuid.UUID, code string) (bool, error) {
	tag, err := tx.Exec(ctx,
		`UPDATE affiliators SET status = 'active', code = $2, updated_at = now()
		 WHERE id = $1 AND status = 'pending_approval'`,
		id, code,
	)
	if err != nil {
		return false, err
	}
	return tag.RowsAffected() > 0, nil
}

// RejectAffiliatorTx marks a pending affiliator rejected with a mandatory
// reason; false means it was no longer pending.
func (r *Repository) RejectAffiliatorTx(ctx context.Context, tx pgx.Tx, id uuid.UUID, reason string) (bool, error) {
	tag, err := tx.Exec(ctx,
		`UPDATE affiliators SET status = 'rejected', rejection_reason = $2, updated_at = now()
		 WHERE id = $1 AND status = 'pending_approval'`,
		id, reason,
	)
	if err != nil {
		return false, err
	}
	return tag.RowsAffected() > 0, nil
}

// CancelPendingCommissionsByAffiliatorTx cancels every commission of the
// affiliator still waiting out its hold period (BR-PLG-07) and returns how many.
func (r *Repository) CancelPendingCommissionsByAffiliatorTx(ctx context.Context, tx pgx.Tx, affiliatorID uuid.UUID) (int64, error) {
	tag, err := tx.Exec(ctx,
		`UPDATE affiliate_commissions SET status = 'cancelled', updated_at = now()
		 WHERE affiliator_id = $1 AND status = 'pending'`,
		affiliatorID,
	)
	if err != nil {
		return 0, err
	}
	return tag.RowsAffected(), nil
}

// GetAffiliateCommissionsByIDsTx locks and returns the named commissions that
// belong to the affiliator; ids that match nothing are simply absent.
func (r *Repository) GetAffiliateCommissionsByIDsTx(ctx context.Context, tx pgx.Tx, affiliatorID uuid.UUID, ids []uuid.UUID) ([]model.AffiliateCommission, error) {
	rows, err := tx.Query(ctx,
		`SELECT id, affiliator_id, student_id, order_id, order_item_id, product_type, base_amount, percentage,
		        level_snapshot, amount, status, wait_until, paid_at
		 FROM affiliate_commissions
		 WHERE affiliator_id = $1 AND id = ANY($2)
		 ORDER BY created_at
		 FOR UPDATE`, affiliatorID, ids)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := []model.AffiliateCommission{}
	for rows.Next() {
		var c model.AffiliateCommission
		if err := rows.Scan(&c.ID, &c.AffiliatorID, &c.StudentID, &c.OrderID, &c.OrderItemID,
			&c.ProductType, &c.BaseAmount, &c.Percentage, &c.LevelSnapshot, &c.Amount, &c.Status, &c.WaitUntil, &c.PaidAt); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// InsertAffiliateViolationTx records one violation as decided by an admin
// (status open, no appeal) and fills the generated fields.
func (r *Repository) InsertAffiliateViolationTx(ctx context.Context, tx pgx.Tx, v *model.AffiliateViolation) error {
	return tx.QueryRow(ctx,
		`INSERT INTO affiliate_violations (affiliator_id, category, description, sanction, suspension_until, decided_by)
		 VALUES ($1, $2, $3, $4, $5, $6)
		 RETURNING id, status, decided_at, appeal_status, created_at`,
		v.AffiliatorID, v.Category, v.Description, v.Sanction, v.SuspensionUntil, v.DecidedBy,
	).Scan(&v.ID, &v.Status, &v.DecidedAt, &v.AppealStatus, &v.CreatedAt)
}

// SetAffiliateStatusTx writes the affiliator status; suspendedUntil is only
// set for suspensions and nil otherwise.
func (r *Repository) SetAffiliateStatusTx(ctx context.Context, tx pgx.Tx, id uuid.UUID, status string, suspendedUntil *time.Time) error {
	_, err := tx.Exec(ctx,
		`UPDATE affiliators SET status = $2, suspended_until = $3, updated_at = now() WHERE id = $1`,
		id, status, suspendedUntil,
	)
	return err
}

// SetAffiliateFrozenUntilTx writes the balance freeze deadline; nil clears it.
func (r *Repository) SetAffiliateFrozenUntilTx(ctx context.Context, tx pgx.Tx, id uuid.UUID, until *time.Time) error {
	_, err := tx.Exec(ctx,
		`UPDATE affiliators SET frozen_until = $2, updated_at = now() WHERE id = $1`,
		id, until,
	)
	return err
}

// SetAffiliateLevelLockedUntilTx writes the level-lock deadline; nil clears it.
func (r *Repository) SetAffiliateLevelLockedUntilTx(ctx context.Context, tx pgx.Tx, id uuid.UUID, until *time.Time) error {
	_, err := tx.Exec(ctx,
		`UPDATE affiliators SET level_locked_until = $2, updated_at = now() WHERE id = $1`,
		id, until,
	)
	return err
}
