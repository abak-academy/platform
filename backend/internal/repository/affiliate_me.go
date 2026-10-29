package repository

import (
	"context"
	"encoding/json"
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
	ExpiresAt    *time.Time
	CreatedAt    time.Time
	TotalSpend   float64
}

type AffiliateCommissionFilter struct {
	Q           string
	Status      string
	ProductType string
	PaidFrom    *time.Time
	PaidTo      *time.Time
	Limit       int
	Offset      int
}

type AffiliateMeCounts struct {
	Registrations    int
	TotalStudents    int
	ActiveBindings   int
	NewStudents      int
	CommissionEarned float64
}

// affiliateBindingSpendExpr sums the student's paid orders (subtotal minus
// discount) landed inside the binding window: start inclusive, end exclusive.
const affiliateBindingSpendExpr = `COALESCE((
	SELECT SUM(o.subtotal - o.discount)
	FROM orders o
	WHERE o.student_id = b.student_id
	  AND o.status IN ('paid', 'processing', 'shipped', 'completed')
	  AND o.paid_at >= b.started_at
	  AND (b.expires_at IS NULL OR o.paid_at < b.expires_at)
), 0)`

func (r *Repository) GetAffiliatorByUserID(ctx context.Context, userID string) (*model.Affiliator, error) {
	a, err := scanAffiliator(r.pool.QueryRow(ctx,
		`SELECT `+affiliatorColumns+` `+affiliatorFrom+` WHERE a.user_id = $1`, userID))
	if isNotFound(err) {
		return nil, nil
	}
	return a, err
}

func (r *Repository) UpdateAffiliatorBank(ctx context.Context, id uuid.UUID, bankName, bankAccountNumber, bankAccountHolder string) error {
	bank, err := json.Marshal(affiliatorBankAccount{BankName: bankName, AccountNumber: bankAccountNumber, AccountHolder: bankAccountHolder})
	if err != nil {
		return err
	}
	_, err = r.pool.Exec(ctx, `UPDATE affiliators SET bank_account = $2 WHERE user_id = $1`, id, bank)
	return err
}

func affiliateBindingPredicate(q AffiliateListQuery) *affPredicate {
	p := newAffPredicate(2)
	p.eq("b.status", q.Status)
	p.eq("b.source", q.Type)
	p.dates("b.started_at", q.From, q.To)
	p.search(q.Q, "u.name")
	return p
}

func (r *Repository) ListAffiliateBindingsForAffiliator(ctx context.Context, affiliatorID uuid.UUID, q AffiliateListQuery, limit, offset int) ([]AffiliateBindingListRow, error) {
	p := affiliateBindingPredicate(q)
	args := append([]interface{}{affiliatorID}, p.args...)
	query := `SELECT b.id, b.status, b.source, b.started_at, b.expires_at, b.created_at,
	        u.name, u.created_at,
` + affiliateBindingSpendExpr + ` AS total_spend
		 FROM affiliate_bindings b
		 JOIN users u ON u.id = b.student_id
		 WHERE b.affiliator_id = $1` + p.sql + `
		 ORDER BY b.started_at DESC, b.id
		 LIMIT $` + strconv.Itoa(p.next) + ` OFFSET $` + strconv.Itoa(p.next+1)
	args = append(args, limit, offset)

	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := []AffiliateBindingListRow{}
	for rows.Next() {
		var row AffiliateBindingListRow
		if err := rows.Scan(&row.BindingID, &row.Status, &row.Source, &row.StartedAt, &row.ExpiresAt, &row.CreatedAt,
			&row.StudentName, &row.RegisteredAt, &row.TotalSpend); err != nil {
			return nil, err
		}
		out = append(out, row)
	}
	return out, rows.Err()
}

func (r *Repository) CountAffiliateBindingsForAffiliator(ctx context.Context, affiliatorID uuid.UUID, q AffiliateListQuery) (int, error) {
	p := affiliateBindingPredicate(q)
	args := append([]interface{}{affiliatorID}, p.args...)
	query := `SELECT count(*) FROM affiliate_bindings b JOIN users u ON u.id = b.student_id WHERE b.affiliator_id = $1` + p.sql

	var n int
	err := r.pool.QueryRow(ctx, query, args...).Scan(&n)
	return n, err
}

// AffiliateCommissionRow is one affiliator-facing commission with the student
// (first name + initials is applied by the service) and the product it came from.
type AffiliateCommissionRow struct {
	model.AffiliateCommission
	StudentName string `json:"student_name"`
	ProductName string `json:"product_name"`
}

func affiliateCommissionPredicate(f AffiliateCommissionFilter) *affPredicate {
	p := newAffPredicate(2)
	p.eq("c.status", f.Status)
	p.eq("c.product_type", f.ProductType)
	p.dates("c.paid_at", f.PaidFrom, f.PaidTo)
	p.search(f.Q, "u.name", "oi.name")
	return p
}

const affiliateCommissionFrom = `FROM affiliate_commissions c
		 JOIN users u ON u.id = c.student_id
		 JOIN order_item oi ON oi.id = c.order_item_id`

func (r *Repository) ListAffiliateCommissionsForAffiliator(ctx context.Context, affiliatorID uuid.UUID, f AffiliateCommissionFilter) ([]AffiliateCommissionRow, error) {
	p := affiliateCommissionPredicate(f)
	args := append([]interface{}{affiliatorID}, p.args...)
	query := `SELECT c.id, c.affiliator_id, c.student_id, c.order_id, c.order_item_id, c.product_type, c.base_amount, c.commission_type, c.commission_value,
	        c.level_snapshot, c.amount, c.status, c.wait_until, c.paid_at, c.received_at, c.created_at, c.updated_at,
	        u.name, oi.name
		 ` + affiliateCommissionFrom + `
		 WHERE c.affiliator_id = $1` + p.sql + `
		 ORDER BY c.paid_at DESC NULLS LAST, c.id DESC
		 LIMIT $` + strconv.Itoa(p.next) + ` OFFSET $` + strconv.Itoa(p.next+1)
	args = append(args, f.Limit, f.Offset)

	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := []AffiliateCommissionRow{}
	for rows.Next() {
		var c AffiliateCommissionRow
		if err := rows.Scan(&c.ID, &c.AffiliatorID, &c.StudentID, &c.OrderID, &c.OrderItemID, &c.ProductType,
			&c.BaseAmount, &c.CommissionType, &c.CommissionValue, &c.LevelSnapshot, &c.Amount, &c.Status, &c.WaitUntil, &c.PaidAt,
			&c.ReceivedAt, &c.CreatedAt, &c.UpdatedAt, &c.StudentName, &c.ProductName); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

func (r *Repository) CountAffiliateCommissionsForAffiliator(ctx context.Context, affiliatorID uuid.UUID, f AffiliateCommissionFilter) (int, error) {
	p := affiliateCommissionPredicate(f)
	args := append([]interface{}{affiliatorID}, p.args...)
	query := `SELECT count(*) ` + affiliateCommissionFrom + ` WHERE c.affiliator_id = $1` + p.sql

	var n int
	err := r.pool.QueryRow(ctx, query, args...).Scan(&n)
	return n, err
}

// AffiliateLedgerRow is one ledger entry with the student and product behind a
// commission entry, so the web can say "Komisi dari Sari W." instead of a raw note.
type AffiliateLedgerRow struct {
	model.AffiliateLedgerEntry
	StudentName *string `json:"student_name"`
	ProductName *string `json:"product_name"`
}

func affiliateLedgerPredicate(q AffiliateListQuery) *affPredicate {
	p := newAffPredicate(2)
	p.eq("l.entry_type", q.Type)
	p.dates("l.created_at", q.From, q.To)
	p.search(q.Q, "u.name", "oi.name", "adj.reason")
	return p
}

const affiliateLedgerFrom = `FROM affiliate_ledger_entries l
		 LEFT JOIN affiliate_commissions c ON c.id = l.commission_id
		 LEFT JOIN users u ON u.id = c.student_id
		 LEFT JOIN order_item oi ON oi.id = c.order_item_id
		 LEFT JOIN affiliate_adjustments adj ON adj.id = l.adjustment_id`

func (r *Repository) ListAffiliateLedgerForAffiliator(ctx context.Context, affiliatorID uuid.UUID, ascending bool, q AffiliateListQuery, limit, offset int) ([]AffiliateLedgerRow, error) {
	direction := "DESC"
	if ascending {
		direction = "ASC"
	}
	p := affiliateLedgerPredicate(q)
	args := append([]interface{}{affiliatorID}, p.args...)
	query := `SELECT l.id, l.affiliator_id, l.entry_type, l.amount, l.commission_id, l.payout_id, l.adjustment_id,
	        l.note, l.actor_id, l.created_at, u.name, oi.name
		 ` + affiliateLedgerFrom + `
		 WHERE l.affiliator_id = $1` + p.sql + `
		 ORDER BY l.created_at ` + direction + `, l.id ` + direction + `
		 LIMIT $` + strconv.Itoa(p.next) + ` OFFSET $` + strconv.Itoa(p.next+1)
	args = append(args, limit, offset)

	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := []AffiliateLedgerRow{}
	for rows.Next() {
		var e AffiliateLedgerRow
		if err := rows.Scan(&e.ID, &e.AffiliatorID, &e.EntryType, &e.Amount, &e.CommissionID, &e.PayoutID,
			&e.AdjustmentID, &e.Note, &e.ActorID, &e.CreatedAt, &e.StudentName, &e.ProductName); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

func (r *Repository) CountAffiliateLedgerEntriesForAffiliator(ctx context.Context, affiliatorID uuid.UUID, q AffiliateListQuery) (int, error) {
	p := affiliateLedgerPredicate(q)
	args := append([]interface{}{affiliatorID}, p.args...)
	query := `SELECT count(*) ` + affiliateLedgerFrom + ` WHERE l.affiliator_id = $1` + p.sql

	var n int
	err := r.pool.QueryRow(ctx, query, args...).Scan(&n)
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

// GetAffiliateMeCounts counts registrations, new students and earned
// commission inside [from, to); a nil bound leaves that side open. Total
// students and active bindings are never period-scoped.
func (r *Repository) GetAffiliateMeCounts(ctx context.Context, affiliatorID uuid.UUID, from, to *time.Time) (AffiliateMeCounts, error) {
	var counts AffiliateMeCounts
	err := r.pool.QueryRow(ctx,
		`SELECT
		    (SELECT count(*) FROM affiliate_bindings WHERE affiliator_id = $1
		        AND ($2::timestamptz IS NULL OR created_at >= $2) AND ($3::timestamptz IS NULL OR created_at < $3)),
		    (SELECT count(DISTINCT student_id) FROM affiliate_bindings WHERE affiliator_id = $1),
		    (SELECT count(*) FROM affiliate_bindings WHERE affiliator_id = $1 AND status = 'active'),
		    (SELECT count(DISTINCT student_id) FROM affiliate_bindings WHERE affiliator_id = $1
		        AND ($2::timestamptz IS NULL OR created_at >= $2) AND ($3::timestamptz IS NULL OR created_at < $3)),
		    (SELECT COALESCE(SUM(amount), 0) FROM affiliate_ledger_entries WHERE affiliator_id = $1 AND entry_type = 'commission'
		        AND ($2::timestamptz IS NULL OR created_at >= $2) AND ($3::timestamptz IS NULL OR created_at < $3))`,
		affiliatorID, from, to,
	).Scan(&counts.Registrations, &counts.TotalStudents, &counts.ActiveBindings, &counts.NewStudents, &counts.CommissionEarned)
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

// AffiliateReportOrderRow is one commission in the admin cross-affiliator order
// report: the commission with its student, product and the affiliator it belongs to.
type AffiliateReportOrderRow struct {
	AffiliateCommissionRow
	AffiliatorName string `json:"affiliator_name"`
	AffiliatorCode string `json:"affiliator_code"`
}

// AffiliateReportOrderFilter narrows the order report. Q matches the student,
// the product, the affiliator's name or code; PaidFrom/PaidTo bound the payment date.
type AffiliateReportOrderFilter struct {
	Q           string
	Status      string
	ProductType string
	PaidFrom    *time.Time
	PaidTo      *time.Time
	Limit       int
	Offset      int
}

// ListAffiliateReportOrders lists commissions across every affiliator, newest
// payment first, plus the total matching rows.
func (r *Repository) ListAffiliateReportOrders(ctx context.Context, f AffiliateReportOrderFilter) ([]AffiliateReportOrderRow, int, error) {
	p := newAffPredicate(1)
	p.eq("c.status", f.Status)
	p.eq("c.product_type", f.ProductType)
	p.dates("c.paid_at", f.PaidFrom, f.PaidTo)
	p.search(f.Q, "u.name", "oi.name", "au.name", "a.referral_code")
	from := `FROM affiliate_commissions c
		 JOIN users u ON u.id = c.student_id
		 JOIN order_item oi ON oi.id = c.order_item_id
		 JOIN affiliators a ON a.user_id = c.affiliator_id
		 JOIN users au ON au.id = a.user_id
		 WHERE TRUE` + p.sql

	var total int
	if err := r.pool.QueryRow(ctx, `SELECT count(*) `+from, p.args...).Scan(&total); err != nil {
		return nil, 0, err
	}
	rows, err := r.pool.Query(ctx,
		`SELECT c.id, c.affiliator_id, c.student_id, c.order_id, c.order_item_id, c.product_type, c.base_amount, c.commission_type, c.commission_value,
		        c.level_snapshot, c.amount, c.status, c.wait_until, c.paid_at, c.received_at, c.created_at, c.updated_at,
		        u.name, oi.name, au.name, COALESCE(a.referral_code, '') `+from+`
		 ORDER BY c.paid_at DESC NULLS LAST, c.id DESC
		 LIMIT $`+strconv.Itoa(p.next)+` OFFSET $`+strconv.Itoa(p.next+1),
		append(p.args, f.Limit, f.Offset)...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	out := []AffiliateReportOrderRow{}
	for rows.Next() {
		var c AffiliateReportOrderRow
		if err := rows.Scan(&c.ID, &c.AffiliatorID, &c.StudentID, &c.OrderID, &c.OrderItemID, &c.ProductType,
			&c.BaseAmount, &c.CommissionType, &c.CommissionValue, &c.LevelSnapshot, &c.Amount, &c.Status, &c.WaitUntil, &c.PaidAt,
			&c.ReceivedAt, &c.CreatedAt, &c.UpdatedAt, &c.StudentName, &c.ProductName, &c.AffiliatorName, &c.AffiliatorCode); err != nil {
			return nil, 0, err
		}
		out = append(out, c)
	}
	return out, total, rows.Err()
}
