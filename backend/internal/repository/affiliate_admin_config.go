package repository

import (
	"context"
	"strconv"
	"time"

	"akademi-bimbel/internal/model"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// AffiliateOverrideRow is one admin product-override list row with the
// product's display name.
type AffiliateOverrideRow struct {
	ProductID   uuid.UUID  `json:"product_id"`
	ProductName string     `json:"product_name"`
	ProductType string     `json:"product_type"`
	Percentage  float64    `json:"percentage"`
	UpdatedBy   *uuid.UUID `json:"updated_by"`
	UpdatedAt   time.Time  `json:"updated_at"`
}

// AffiliateFlagRow is one admin flag list row with its affiliator context.
type AffiliateFlagRow struct {
	model.AffiliateFlag
	AffiliatorCode string `json:"affiliator_code"`
	AffiliatorName string `json:"affiliator_name"`
}

// AffiliateAppealRow is one admin appeal list row (a violation carrying an
// appeal) with its affiliator context.
type AffiliateAppealRow struct {
	model.AffiliateViolation
	AffiliatorCode string `json:"affiliator_code"`
	AffiliatorName string `json:"affiliator_name"`
}

// AffiliateReportRow is one per-affiliator performance row (or the totals row).
type AffiliateReportRow struct {
	AffiliatorID   uuid.UUID          `json:"affiliator_id"`
	AffiliatorCode string             `json:"affiliator_code"`
	AffiliatorName string             `json:"affiliator_name"`
	Level          string             `json:"level"`
	StudentsNew    int                `json:"students_new"`
	NetRevenue     float64            `json:"net_revenue"`
	Commissions    map[string]float64 `json:"commissions"`
	Clicks         int                `json:"clicks"`
	Conversion     float64            `json:"conversion"`
}

// AffiliateLiabilityReport is the commission-liability report body.
type AffiliateLiabilityReport struct {
	ByStatus        map[string]float64 `json:"by_status"`
	PayoutLiability float64            `json:"payout_liability"`
}

// UpsertAffiliateSettingsTx replaces the affiliate_settings singleton row
// inside the caller's transaction.
func (r *Repository) UpsertAffiliateSettingsTx(ctx context.Context, tx pgx.Tx, s *model.AffiliateSettings) error {
	_, err := tx.Exec(ctx,
		`INSERT INTO affiliate_settings
		     (id, cookie_days, binding_months, min_extend_amount, digital_wait_days,
		      physical_received_days, physical_wait_days, min_payout, payout_day_of_month,
		      adjustment_threshold, adjustment_threshold_enabled, freeze_max_days, soon_end_days,
		      launch_date, eval_day, silver_threshold, gold_threshold, hold_thresholds,
		      daily_registration_cap, updated_by, updated_at)
		 VALUES (1, $1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19, now())
		 ON CONFLICT (id) DO UPDATE SET
		     cookie_days = $1, binding_months = $2, min_extend_amount = $3,
		     digital_wait_days = $4, physical_received_days = $5, physical_wait_days = $6,
		     min_payout = $7, payout_day_of_month = $8, adjustment_threshold = $9,
		     adjustment_threshold_enabled = $10, freeze_max_days = $11, soon_end_days = $12,
		     launch_date = $13, eval_day = $14, silver_threshold = $15, gold_threshold = $16,
		     hold_thresholds = $17, daily_registration_cap = $18,
		     updated_by = $19, updated_at = now()`,
		s.CookieDays, s.BindingMonths, s.MinExtendAmount, s.DigitalWaitDays,
		s.PhysicalReceivedDays, s.PhysicalWaitDays, s.MinPayout, s.PayoutDayOfMonth,
		s.AdjustmentThreshold, s.AdjustmentThresholdEnabled, s.FreezeMaxDays, s.SoonEndDays,
		s.LaunchDate, s.EvalDay, s.SilverThreshold, s.GoldThreshold, s.HoldThresholds,
		s.DailyRegistrationCap, s.UpdatedBy,
	)
	return err
}

// UpsertAffiliateLevelSettingsTx writes one level row inside the caller's
// transaction; nil optional fields clear to NULL — the PUT is a full replace
// of each submitted row.
func (r *Repository) UpsertAffiliateLevelSettingsTx(ctx context.Context, tx pgx.Tx, l *model.AffiliateLevelSettings) error {
	_, err := tx.Exec(ctx,
		`INSERT INTO affiliate_level_settings
		     (level, commission_pct, digital_wait_days, physical_received_days,
		      physical_wait_days, min_payout, payout_day_of_month, rise_threshold, hold_threshold)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		 ON CONFLICT (level) DO UPDATE SET
		     commission_pct = $2, digital_wait_days = $3, physical_received_days = $4,
		     physical_wait_days = $5, min_payout = $6, payout_day_of_month = $7,
		     rise_threshold = $8, hold_threshold = $9`,
		l.Level, l.CommissionPct, l.DigitalWaitDays, l.PhysicalReceivedDays,
		l.PhysicalWaitDays, l.MinPayout, l.PayoutDayOfMonth, l.RiseThreshold, l.HoldThreshold,
	)
	return err
}

// AffiliateProductExists reports whether a product row exists.
func (r *Repository) AffiliateProductExists(ctx context.Context, id uuid.UUID) (bool, error) {
	var exists bool
	err := r.pool.QueryRow(ctx,
		`SELECT EXISTS (SELECT 1 FROM product WHERE id = $1)`, id,
	).Scan(&exists)
	return exists, err
}

// UpsertAffiliateOverrideTx writes one product override inside the caller's
// transaction.
func (r *Repository) UpsertAffiliateOverrideTx(ctx context.Context, tx pgx.Tx, o *model.AffiliateProductOverride) error {
	_, err := tx.Exec(ctx,
		`INSERT INTO affiliate_product_overrides (product_id, percentage, updated_by, updated_at)
		 VALUES ($1, $2, $3, now())
		 ON CONFLICT (product_id) DO UPDATE SET
		     percentage = $2, updated_by = $3, updated_at = now()`,
		o.ProductID, o.Percentage, o.UpdatedBy,
	)
	return err
}

// DeleteAffiliateOverrideTx removes one product override inside the caller's
// transaction and reports whether a row was deleted.
func (r *Repository) DeleteAffiliateOverrideTx(ctx context.Context, tx pgx.Tx, productID uuid.UUID) (bool, error) {
	tag, err := tx.Exec(ctx,
		`DELETE FROM affiliate_product_overrides WHERE product_id = $1`, productID)
	if err != nil {
		return false, err
	}
	return tag.RowsAffected() > 0, nil
}

// ListAffiliateOverrideRows lists the overrides joined with the product, newest
// first, plus the total matching rows; q matches the product name.
func (r *Repository) ListAffiliateOverrideRows(ctx context.Context, q string, limit, offset int) ([]AffiliateOverrideRow, int, error) {
	p := newAffPredicate(1)
	p.search(q, "p.name")
	from := `FROM affiliate_product_overrides o
		 JOIN product p ON p.id = o.product_id
		 WHERE TRUE` + p.sql

	var total int
	if err := r.pool.QueryRow(ctx, `SELECT count(*) `+from, p.args...).Scan(&total); err != nil {
		return nil, 0, err
	}
	rows, err := r.pool.Query(ctx,
		`SELECT o.product_id, p.name, p.type, o.percentage, o.updated_by, o.updated_at `+from+`
		 ORDER BY o.updated_at DESC, o.product_id
		 LIMIT $`+strconv.Itoa(p.next)+` OFFSET $`+strconv.Itoa(p.next+1),
		append(p.args, limit, offset)...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	out := []AffiliateOverrideRow{}
	for rows.Next() {
		var row AffiliateOverrideRow
		if err := rows.Scan(&row.ProductID, &row.ProductName, &row.ProductType, &row.Percentage, &row.UpdatedBy, &row.UpdatedAt); err != nil {
			return nil, 0, err
		}
		out = append(out, row)
	}
	return out, total, rows.Err()
}

// AffiliateOverrideProduct is one product offered by the override picker, with
// the override already set on it (nil when none).
type AffiliateOverrideProduct struct {
	ID         uuid.UUID `json:"id"`
	Name       string    `json:"name"`
	Type       string    `json:"type"`
	Percentage *float64  `json:"percentage"`
}

// SearchAffiliateOverrideProducts finds non-archived products by name for the
// override picker, closest names first.
func (r *Repository) SearchAffiliateOverrideProducts(ctx context.Context, q string, limit int) ([]AffiliateOverrideProduct, error) {
	p := newAffPredicate(1)
	p.search(q, "p.name")
	rows, err := r.pool.Query(ctx,
		`SELECT p.id, p.name, p.type, o.percentage
		 FROM product p
		 LEFT JOIN affiliate_product_overrides o ON o.product_id = p.id
		 WHERE p.status <> 'archived'`+p.sql+`
		 ORDER BY p.name, p.id
		 LIMIT $`+strconv.Itoa(p.next),
		append(p.args, limit)...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := []AffiliateOverrideProduct{}
	for rows.Next() {
		var row AffiliateOverrideProduct
		if err := rows.Scan(&row.ID, &row.Name, &row.Type, &row.Percentage); err != nil {
			return nil, err
		}
		out = append(out, row)
	}
	return out, rows.Err()
}

// FindAffiliateFlagByID returns one flag row or (nil, nil) when missing.
func (r *Repository) FindAffiliateFlagByID(ctx context.Context, id uuid.UUID) (*model.AffiliateFlag, error) {
	var f model.AffiliateFlag
	err := r.pool.QueryRow(ctx,
		`SELECT id, affiliator_id, rule, payload, status, note, reviewed_by, reviewed_at, created_at
		 FROM affiliate_flags
		 WHERE id = $1`, id,
	).Scan(&f.ID, &f.AffiliatorID, &f.Rule, &f.Payload, &f.Status, &f.Note, &f.ReviewedBy, &f.ReviewedAt, &f.CreatedAt)
	if isNotFound(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &f, nil
}

// ListAffiliateFlagRows lists flags with the affiliator's code and name, newest
// first, plus the total matching rows. q.Status filters the flag status, q.Q
// matches the affiliator's name, code or the rule, and q.From/To bound the
// detection date.
func (r *Repository) ListAffiliateFlagRows(ctx context.Context, q AffiliateListQuery, limit, offset int) ([]AffiliateFlagRow, int, error) {
	p := newAffPredicate(1)
	p.eq("f.status", q.Status)
	p.dates("f.created_at", q.From, q.To)
	p.search(q.Q, "u.name", "a.code", "f.rule")
	from := `FROM affiliate_flags f
	         JOIN affiliators a ON a.id = f.affiliator_id
	         JOIN users u ON u.id = a.user_id
	         WHERE TRUE` + p.sql

	var total int
	if err := r.pool.QueryRow(ctx, `SELECT count(*) `+from, p.args...).Scan(&total); err != nil {
		return nil, 0, err
	}
	rows, err := r.pool.Query(ctx,
		`SELECT f.id, f.affiliator_id, f.rule, f.payload, f.status, f.note, f.reviewed_by, f.reviewed_at, f.created_at,
		        COALESCE(a.code, ''), u.name `+from+`
		 ORDER BY f.created_at DESC, f.id
		 LIMIT $`+strconv.Itoa(p.next)+` OFFSET $`+strconv.Itoa(p.next+1),
		append(p.args, limit, offset)...,
	)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	out := []AffiliateFlagRow{}
	for rows.Next() {
		var row AffiliateFlagRow
		if err := rows.Scan(&row.ID, &row.AffiliatorID, &row.Rule, &row.Payload, &row.Status, &row.Note,
			&row.ReviewedBy, &row.ReviewedAt, &row.CreatedAt, &row.AffiliatorCode, &row.AffiliatorName); err != nil {
			return nil, 0, err
		}
		out = append(out, row)
	}
	return out, total, rows.Err()
}

// UpdateAffiliateFlagReviewTx stores the review outcome inside the caller's
// transaction.
func (r *Repository) UpdateAffiliateFlagReviewTx(ctx context.Context, tx pgx.Tx, id uuid.UUID, status string, note *string, reviewedBy uuid.UUID) error {
	_, err := tx.Exec(ctx,
		`UPDATE affiliate_flags
		 SET status = $2, note = $3, reviewed_by = $4, reviewed_at = now()
		 WHERE id = $1`,
		id, status, note, reviewedBy,
	)
	return err
}

// FindAffiliateViolationByID returns one violation row or (nil, nil).
func (r *Repository) FindAffiliateViolationByID(ctx context.Context, id uuid.UUID) (*model.AffiliateViolation, error) {
	var v model.AffiliateViolation
	err := r.pool.QueryRow(ctx,
		`SELECT id, affiliator_id, category, description, sanction, suspension_until,
		        status, decided_by, decided_at, appeal_text, appeal_status,
		        appeal_decided_by, appeal_decided_at, created_at
		 FROM affiliate_violations
		 WHERE id = $1`, id,
	).Scan(&v.ID, &v.AffiliatorID, &v.Category, &v.Description, &v.Sanction, &v.SuspensionUntil,
		&v.Status, &v.DecidedBy, &v.DecidedAt, &v.AppealText, &v.AppealStatus,
		&v.AppealDecidedBy, &v.AppealDecidedAt, &v.CreatedAt)
	if isNotFound(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &v, nil
}

// ListAffiliateAppealRows lists violations that carry an appeal (appeal_status
// other than "none"), newest first, plus the total matching rows. q.Status
// filters the appeal status, q.Q matches the affiliator's name, code or the
// violation description, and q.From/To bound the violation date.
func (r *Repository) ListAffiliateAppealRows(ctx context.Context, q AffiliateListQuery, limit, offset int) ([]AffiliateAppealRow, int, error) {
	p := newAffPredicate(1)
	p.eq("v.appeal_status", q.Status)
	p.dates("v.created_at", q.From, q.To)
	p.search(q.Q, "u.name", "a.code", "v.description")
	from := `FROM affiliate_violations v
	         JOIN affiliators a ON a.id = v.affiliator_id
	         JOIN users u ON u.id = a.user_id
	         WHERE v.appeal_status <> 'none'` + p.sql

	var total int
	if err := r.pool.QueryRow(ctx, `SELECT count(*) `+from, p.args...).Scan(&total); err != nil {
		return nil, 0, err
	}
	rows, err := r.pool.Query(ctx,
		`SELECT v.id, v.affiliator_id, v.category, v.description, v.sanction, v.suspension_until,
		        v.status, v.decided_by, v.decided_at, v.appeal_text, v.appeal_status,
		        v.appeal_decided_by, v.appeal_decided_at, v.created_at,
		        COALESCE(a.code, ''), u.name `+from+`
		 ORDER BY v.created_at DESC, v.id
		 LIMIT $`+strconv.Itoa(p.next)+` OFFSET $`+strconv.Itoa(p.next+1),
		append(p.args, limit, offset)...,
	)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	out := []AffiliateAppealRow{}
	for rows.Next() {
		var row AffiliateAppealRow
		if err := rows.Scan(&row.ID, &row.AffiliatorID, &row.Category, &row.Description, &row.Sanction,
			&row.SuspensionUntil, &row.Status, &row.DecidedBy, &row.DecidedAt, &row.AppealText,
			&row.AppealStatus, &row.AppealDecidedBy, &row.AppealDecidedAt, &row.CreatedAt,
			&row.AffiliatorCode, &row.AffiliatorName); err != nil {
			return nil, 0, err
		}
		out = append(out, row)
	}
	return out, total, rows.Err()
}

// DecideAffiliateAppealTx stores the appeal outcome inside the caller's
// transaction: appeal_status + decided_by/at, and the violation itself moves to
// resolved — the uphold sanction stays a manual admin action.
func (r *Repository) DecideAffiliateAppealTx(ctx context.Context, tx pgx.Tx, id uuid.UUID, outcome string, decidedBy uuid.UUID) error {
	_, err := tx.Exec(ctx,
		`UPDATE affiliate_violations
		 SET appeal_status = $2, appeal_decided_by = $3, appeal_decided_at = now(), status = 'resolved'
		 WHERE id = $1`,
		id, outcome, decidedBy,
	)
	return err
}

// ListAffiliateReportRows returns every affiliator (capped), narrowed by q (name
// or code) and level, with the window's
// new bindings, net revenue and click counts; commission status buckets are
// stitched by the service.
func (r *Repository) ListAffiliateReportRows(ctx context.Context, from, to time.Time, q, level string) ([]AffiliateReportRow, error) {
	p := newAffPredicate(3)
	p.eq("a.level", level)
	p.search(q, "u.name", "a.code")
	rows, err := r.pool.Query(ctx,
		`SELECT a.id, COALESCE(a.code, ''), u.name, a.level,
		        COALESCE(bind.students_new, 0), COALESCE(rev.net_revenue, 0), COALESCE(clicks.n, 0)
		 FROM affiliators a
		 JOIN users u ON u.id = a.user_id
		 LEFT JOIN (
		     SELECT affiliator_id, count(*) AS students_new
		     FROM affiliate_bindings
		     WHERE started_at >= $1 AND started_at < $2
		     GROUP BY affiliator_id
		 ) bind ON bind.affiliator_id = a.id
		 LEFT JOIN (
		     SELECT affiliator_id, SUM(base_amount) AS net_revenue
		     FROM affiliate_commissions
		     WHERE status IN ('available', 'paid')
		       AND COALESCE(wait_until, paid_at) >= $1 AND COALESCE(wait_until, paid_at) < $2
		     GROUP BY affiliator_id
		 ) rev ON rev.affiliator_id = a.id
		 LEFT JOIN (
		     SELECT affiliator_id, count(*) AS n
		     FROM affiliate_clicks
		     WHERE created_at >= $1 AND created_at < $2
		     GROUP BY affiliator_id
		 ) clicks ON clicks.affiliator_id = a.id
		 WHERE TRUE`+p.sql+`
		 ORDER BY a.created_at
		 LIMIT 500`,
		append([]any{from, to}, p.args...)...,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := []AffiliateReportRow{}
	for rows.Next() {
		var row AffiliateReportRow
		row.Commissions = map[string]float64{}
		if err := rows.Scan(&row.AffiliatorID, &row.AffiliatorCode, &row.AffiliatorName, &row.Level,
			&row.StudentsNew, &row.NetRevenue, &row.Clicks); err != nil {
			return nil, err
		}
		out = append(out, row)
	}
	return out, rows.Err()
}

// ListAffiliateCommissionStatusTotals sums commission amounts per affiliator
// and status over the window the commissions were paid in.
func (r *Repository) ListAffiliateCommissionStatusTotals(ctx context.Context, from, to time.Time) (map[uuid.UUID]map[string]float64, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT affiliator_id, status, SUM(amount)
		 FROM affiliate_commissions
		 WHERE paid_at >= $1 AND paid_at < $2
		 GROUP BY affiliator_id, status`,
		from, to,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make(map[uuid.UUID]map[string]float64)
	for rows.Next() {
		var id uuid.UUID
		var status string
		var total float64
		if err := rows.Scan(&id, &status, &total); err != nil {
			return nil, err
		}
		if out[id] == nil {
			out[id] = map[string]float64{}
		}
		out[id][status] = total
	}
	return out, rows.Err()
}

// SumAffiliateCommissionLedgerByStatus nets the append-only ledger entries
// tied to a commission (commission, refund_deduction) per commission status.
func (r *Repository) SumAffiliateCommissionLedgerByStatus(ctx context.Context) (map[string]float64, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT c.status, COALESCE(SUM(e.amount), 0)
		 FROM affiliate_ledger_entries e
		 JOIN affiliate_commissions c ON c.id = e.commission_id
		 GROUP BY c.status`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make(map[string]float64)
	for rows.Next() {
		var status string
		var total float64
		if err := rows.Scan(&status, &total); err != nil {
			return nil, err
		}
		out[status] = total
	}
	return out, rows.Err()
}

// SumAffiliatePayoutLiability sums payout amounts still owed (requested +
// processing).
func (r *Repository) SumAffiliatePayoutLiability(ctx context.Context) (float64, error) {
	var total float64
	err := r.pool.QueryRow(ctx,
		`SELECT COALESCE(SUM(amount), 0) FROM affiliate_payouts WHERE status IN ('requested', 'processing')`,
	).Scan(&total)
	return total, err
}
