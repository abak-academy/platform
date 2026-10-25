package repository

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"akademi-bimbel/internal/model"

	"github.com/google/uuid"
)

// BR-PRV-01: every query in this file is scoped to one affiliator_id the
// caller resolved from the JWT user_id — none of them take a raw affiliator
// id from the request.

// AffiliateBindingListRow is one binding of an affiliator's own list: the
// student identity carries only name, registration date and the binding's
// own fields plus the aggregate spend (BR-PRV-02).
type AffiliateBindingListRow struct {
	BindingID    uuid.UUID
	StudentName  string
	RegisteredAt time.Time
	Status       string
	Source       string
	StartedAt    time.Time
	EndedAt      *time.Time
	CreatedAt    time.Time
	TotalSpend   float64
}

type AffiliateCommissionFilter struct {
	Status      string
	ProductType string
	PaidFrom    *time.Time
	PaidTo      *time.Time
	Limit       int
	Offset      int
}

type AffiliateMeCounts struct {
	Clicks         int
	Registrations  int
	TotalStudents  int
	ActiveBindings int
}

// affiliateBindingSpendExpr sums the student's paid orders (subtotal minus
// discount) landed inside the binding window: start inclusive, end exclusive.
const affiliateBindingSpendExpr = `COALESCE((
	SELECT SUM(o.subtotal - o.discount)
	FROM orders o
	WHERE o.student_id = b.student_id
	  AND o.status IN ('paid', 'processing', 'shipped', 'completed')
	  AND o.paid_at >= b.started_at
	  AND (b.ended_at IS NULL OR o.paid_at < b.ended_at)
), 0)`

func affiliateBindingPredicateFrom(status string, startArg int) (string, []interface{}, int) {
	query := ""
	args := []interface{}{}
	argNum := startArg
	if status != "" {
		query += fmt.Sprintf(` AND b.status = $%d`, argNum)
		args = append(args, status)
		argNum++
	}
	return query, args, argNum
}

func (r *Repository) GetAffiliatorByUserID(ctx context.Context, userID string) (*model.Affiliator, error) {
	var a model.Affiliator
	err := r.pool.QueryRow(ctx,
		`SELECT id, user_id, COALESCE(code, ''), status, rejection_reason, level, level_locked_until,
		        warn_quarters, suspended_until, frozen_until, terms_version, terms_accepted_at,
		        bank_name, bank_account_number, bank_account_holder, created_at, updated_at
		 FROM affiliators
		 WHERE user_id = $1`,
		userID,
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

func (r *Repository) UpdateAffiliatorBank(ctx context.Context, id uuid.UUID, bankName, bankAccountNumber, bankAccountHolder string) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE affiliators
		 SET bank_name = $2, bank_account_number = $3, bank_account_holder = $4, updated_at = now()
		 WHERE id = $1`,
		id, bankName, bankAccountNumber, bankAccountHolder,
	)
	return err
}

func (r *Repository) ListAffiliateBindingsForAffiliator(ctx context.Context, affiliatorID uuid.UUID, status string, limit, offset int) ([]AffiliateBindingListRow, error) {
	predicate, args, next := affiliateBindingPredicateFrom(status, 2)
	args = append([]interface{}{affiliatorID}, args...)
	query := `SELECT b.id, b.status, b.source, b.started_at, b.ended_at, b.created_at,
	        u.name, u.created_at,
` + affiliateBindingSpendExpr + ` AS total_spend
		 FROM affiliate_bindings b
		 JOIN users u ON u.id = b.student_id
		 WHERE b.affiliator_id = $1` + predicate + `
		 ORDER BY b.started_at DESC, b.id
		 LIMIT $` + strconv.Itoa(next) + ` OFFSET $` + strconv.Itoa(next+1)
	args = append(args, limit, offset)

	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []AffiliateBindingListRow
	for rows.Next() {
		var row AffiliateBindingListRow
		if err := rows.Scan(&row.BindingID, &row.Status, &row.Source, &row.StartedAt, &row.EndedAt, &row.CreatedAt,
			&row.StudentName, &row.RegisteredAt, &row.TotalSpend); err != nil {
			return nil, err
		}
		out = append(out, row)
	}
	return out, rows.Err()
}

func (r *Repository) CountAffiliateBindingsForAffiliator(ctx context.Context, affiliatorID uuid.UUID, status string) (int, error) {
	predicate, args, _ := affiliateBindingPredicateFrom(status, 2)
	args = append([]interface{}{affiliatorID}, args...)
	query := `SELECT count(*) FROM affiliate_bindings b WHERE b.affiliator_id = $1` + predicate

	var n int
	err := r.pool.QueryRow(ctx, query, args...).Scan(&n)
	return n, err
}

func affiliateCommissionPredicateFrom(f AffiliateCommissionFilter, startArg int) (string, []interface{}, int) {
	query := ""
	args := []interface{}{}
	argNum := startArg
	if f.Status != "" {
		query += fmt.Sprintf(` AND c.status = $%d`, argNum)
		args = append(args, f.Status)
		argNum++
	}
	if f.ProductType != "" {
		query += fmt.Sprintf(` AND c.product_type = $%d`, argNum)
		args = append(args, f.ProductType)
		argNum++
	}
	if f.PaidFrom != nil {
		query += fmt.Sprintf(` AND c.paid_at >= $%d`, argNum)
		args = append(args, *f.PaidFrom)
		argNum++
	}
	if f.PaidTo != nil {
		query += fmt.Sprintf(` AND c.paid_at < $%d`, argNum)
		args = append(args, *f.PaidTo)
		argNum++
	}
	return query, args, argNum
}

func (r *Repository) ListAffiliateCommissionsForAffiliator(ctx context.Context, affiliatorID uuid.UUID, f AffiliateCommissionFilter) ([]model.AffiliateCommission, error) {
	predicate, args, next := affiliateCommissionPredicateFrom(f, 2)
	args = append([]interface{}{affiliatorID}, args...)
	query := `SELECT id, affiliator_id, student_id, order_id, order_item_id, product_type, base_amount, percentage,
	        level_snapshot, amount, status, wait_until, paid_at, received_at, created_at, updated_at
		 FROM affiliate_commissions c
		 WHERE c.affiliator_id = $1` + predicate + `
		 ORDER BY c.paid_at DESC NULLS LAST, c.id DESC
		 LIMIT $` + strconv.Itoa(next) + ` OFFSET $` + strconv.Itoa(next+1)
	args = append(args, f.Limit, f.Offset)

	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []model.AffiliateCommission
	for rows.Next() {
		var c model.AffiliateCommission
		if err := rows.Scan(&c.ID, &c.AffiliatorID, &c.StudentID, &c.OrderID, &c.OrderItemID, &c.ProductType,
			&c.BaseAmount, &c.Percentage, &c.LevelSnapshot, &c.Amount, &c.Status, &c.WaitUntil, &c.PaidAt,
			&c.ReceivedAt, &c.CreatedAt, &c.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

func (r *Repository) CountAffiliateCommissionsForAffiliator(ctx context.Context, affiliatorID uuid.UUID, f AffiliateCommissionFilter) (int, error) {
	predicate, args, _ := affiliateCommissionPredicateFrom(f, 2)
	args = append([]interface{}{affiliatorID}, args...)
	query := `SELECT count(*) FROM affiliate_commissions c WHERE c.affiliator_id = $1` + predicate

	var n int
	err := r.pool.QueryRow(ctx, query, args...).Scan(&n)
	return n, err
}

func (r *Repository) ListAffiliateLedgerForAffiliator(ctx context.Context, affiliatorID uuid.UUID, ascending bool, limit, offset int) ([]model.AffiliateLedgerEntry, error) {
	direction := "DESC"
	if ascending {
		direction = "ASC"
	}
	rows, err := r.pool.Query(ctx,
		`SELECT id, affiliator_id, entry_type, amount, commission_id, payout_id, adjustment_id, note, actor_id, created_at
		 FROM affiliate_ledger_entries
		 WHERE affiliator_id = $1
		 ORDER BY created_at `+direction+`, id `+direction+`
		 LIMIT $2 OFFSET $3`,
		affiliatorID, limit, offset,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []model.AffiliateLedgerEntry
	for rows.Next() {
		var e model.AffiliateLedgerEntry
		if err := rows.Scan(&e.ID, &e.AffiliatorID, &e.EntryType, &e.Amount, &e.CommissionID, &e.PayoutID,
			&e.AdjustmentID, &e.Note, &e.ActorID, &e.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

func (r *Repository) CountAffiliateLedgerEntriesForAffiliator(ctx context.Context, affiliatorID uuid.UUID) (int, error) {
	var n int
	err := r.pool.QueryRow(ctx,
		`SELECT count(*) FROM affiliate_ledger_entries WHERE affiliator_id = $1`,
		affiliatorID,
	).Scan(&n)
	return n, err
}

func (r *Repository) SumAffiliateLedgerForAffiliator(ctx context.Context, affiliatorID uuid.UUID) (float64, error) {
	var saldo float64
	err := r.pool.QueryRow(ctx,
		`SELECT COALESCE(SUM(amount), 0) FROM affiliate_ledger_entries WHERE affiliator_id = $1`,
		affiliatorID,
	).Scan(&saldo)
	return saldo, err
}

func (r *Repository) GetAffiliateMeCounts(ctx context.Context, affiliatorID uuid.UUID) (AffiliateMeCounts, error) {
	var counts AffiliateMeCounts
	err := r.pool.QueryRow(ctx,
		`SELECT
		    (SELECT count(*) FROM affiliate_clicks WHERE affiliator_id = $1),
		    (SELECT count(*) FROM affiliate_bindings WHERE affiliator_id = $1),
		    (SELECT count(DISTINCT student_id) FROM affiliate_bindings WHERE affiliator_id = $1),
		    (SELECT count(*) FROM affiliate_bindings WHERE affiliator_id = $1 AND status = 'active')`,
		affiliatorID,
	).Scan(&counts.Clicks, &counts.Registrations, &counts.TotalStudents, &counts.ActiveBindings)
	return counts, err
}

func (r *Repository) CountAffiliateBindingsBySource(ctx context.Context, affiliatorID uuid.UUID) (map[string]int, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT source, count(*) FROM affiliate_bindings WHERE affiliator_id = $1 GROUP BY source`,
		affiliatorID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	bySource := make(map[string]int)
	for rows.Next() {
		var source string
		var n int
		if err := rows.Scan(&source, &n); err != nil {
			return nil, err
		}
		bySource[source] = n
	}
	return bySource, rows.Err()
}
