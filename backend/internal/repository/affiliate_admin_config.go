package repository

import (
	"context"
	"encoding/json"
	"strconv"
	"time"

	"akademi-bimbel/internal/model"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// AffiliateOverrideRow is one admin product-override list row with the
// product's display name.
type AffiliateOverrideRow struct {
	ProductID   uuid.UUID `json:"product_id"`
	ProductName string    `json:"product_name"`
	ProductType string    `json:"product_type"`
	Type        string    `json:"type"`
	Value       float64   `json:"value"`
	UpdatedAt   time.Time `json:"updated_at"`
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
}

// AffiliateLiabilityReport is the commission-liability report body.
type AffiliateLiabilityReport struct {
	ByStatus        map[string]float64 `json:"by_status"`
	PayoutLiability float64            `json:"payout_liability"`
}

// UpsertAffiliateSettingsTx replaces the affiliate_settings singleton row
// inside the caller's transaction. The levels column is left alone: it is
// written level by level through UpsertAffiliateLevelSettingsTx.
func (r *Repository) UpsertAffiliateSettingsTx(ctx context.Context, tx pgx.Tx, s *model.AffiliateSettings) error {
	_, err := tx.Exec(ctx,
		`INSERT INTO affiliate_settings
		     (id, cookie_days, binding_months, min_extend_amount, digital_wait_days,
		      physical_wait_days, min_payout, payout_day_of_month,
		      launch_date, eval_day, daily_registration_cap, updated_at)
		 VALUES (1, $1, $2, $3, $4, $5, $6, $7, $8, $9, $10, now())
		 ON CONFLICT (id) DO UPDATE SET
		     cookie_days = $1, binding_months = $2, min_extend_amount = $3,
		     digital_wait_days = $4, physical_wait_days = $5,
		     min_payout = $6, payout_day_of_month = $7,
		     launch_date = $8, eval_day = $9, daily_registration_cap = $10,
		     updated_at = now()`,
		s.CookieDays, s.BindingMonths, s.MinExtendAmount, s.DigitalWaitDays,
		s.PhysicalWaitDays, s.MinPayout, s.PayoutDayOfMonth,
		s.LaunchDate, s.EvalDay, s.DailyRegistrationCap,
	)
	return err
}

// UpsertAffiliateLevelSettingsTx writes one level into the settings row's
// levels object inside the caller's transaction; nil optional fields are
// dropped — the PUT is a full replace of each submitted level. The settings
// row must already exist (the service upserts it first).
func (r *Repository) UpsertAffiliateLevelSettingsTx(ctx context.Context, tx pgx.Tx, l *model.AffiliateLevelSettings) error {
	rule, err := json.Marshal(affiliateLevelRule{
		CommissionRule: l.CommissionRule, BindingMonths: l.BindingMonths, DigitalWaitDays: l.DigitalWaitDays,
		PhysicalWaitDays: l.PhysicalWaitDays, MinPayout: l.MinPayout, PayoutDayOfMonth: l.PayoutDayOfMonth,
		RiseThreshold: l.RiseThreshold, HoldThreshold: l.HoldThreshold,
	})
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx,
		`UPDATE affiliate_settings SET levels = jsonb_set(levels, ARRAY[$1::text], $2::jsonb, true), updated_at = now()
		 WHERE id = 1`,
		l.Level, rule,
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

// UpsertAffiliateOverrideTx writes one product's commission override inside
// the caller's transaction, stamping the time it was set.
func (r *Repository) UpsertAffiliateOverrideTx(ctx context.Context, tx pgx.Tx, o *model.AffiliateProductOverride) error {
	raw, err := json.Marshal(affiliateProductOverride{Type: o.Type, Value: o.Value, UpdatedAt: time.Now().UTC()})
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `UPDATE product SET affiliate_commission = $2 WHERE id = $1`, o.ProductID, raw)
	return err
}

// DeleteAffiliateOverrideTx removes one product's override inside the caller's
// transaction and reports whether there was one to remove.
func (r *Repository) DeleteAffiliateOverrideTx(ctx context.Context, tx pgx.Tx, productID uuid.UUID) (bool, error) {
	tag, err := tx.Exec(ctx,
		`UPDATE product SET affiliate_commission = NULL WHERE id = $1 AND affiliate_commission IS NOT NULL`, productID)
	if err != nil {
		return false, err
	}
	return tag.RowsAffected() > 0, nil
}

// ListAffiliateOverrideRows lists the products carrying an override, newest
// first, plus the total matching rows; q matches the product name.
func (r *Repository) ListAffiliateOverrideRows(ctx context.Context, q string, limit, offset int) ([]AffiliateOverrideRow, int, error) {
	p := newAffPredicate(1)
	p.search(q, "p.name")
	from := `FROM product p
		 WHERE p.affiliate_commission IS NOT NULL` + p.sql

	var total int
	if err := r.pool.QueryRow(ctx, `SELECT count(*) `+from, p.args...).Scan(&total); err != nil {
		return nil, 0, err
	}
	rows, err := r.pool.Query(ctx,
		`SELECT p.id, p.name, p.type, p.affiliate_commission->>'type',
		        (p.affiliate_commission->>'value')::numeric, (p.affiliate_commission->>'updated_at')::timestamptz `+from+`
		 ORDER BY (p.affiliate_commission->>'updated_at')::timestamptz DESC, p.id
		 LIMIT $`+strconv.Itoa(p.next)+` OFFSET $`+strconv.Itoa(p.next+1),
		append(p.args, limit, offset)...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	out := []AffiliateOverrideRow{}
	for rows.Next() {
		var row AffiliateOverrideRow
		if err := rows.Scan(&row.ProductID, &row.ProductName, &row.ProductType, &row.Type, &row.Value, &row.UpdatedAt); err != nil {
			return nil, 0, err
		}
		out = append(out, row)
	}
	return out, total, rows.Err()
}

// AffiliateOverrideProduct is one product offered by the override picker, with
// the override already set on it (nil when none).
type AffiliateOverrideProduct struct {
	ID            uuid.UUID `json:"id"`
	Name          string    `json:"name"`
	Type          string    `json:"type"`
	OverrideType  *string   `json:"override_type"`
	OverrideValue *float64  `json:"override_value"`
}

// SearchAffiliateOverrideProducts finds non-archived products by name for the
// override picker, closest names first.
func (r *Repository) SearchAffiliateOverrideProducts(ctx context.Context, q string, limit int) ([]AffiliateOverrideProduct, error) {
	p := newAffPredicate(1)
	p.search(q, "p.name")
	rows, err := r.pool.Query(ctx,
		`SELECT p.id, p.name, p.type, p.affiliate_commission->>'type', (p.affiliate_commission->>'value')::numeric
		 FROM product p
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
		if err := rows.Scan(&row.ID, &row.Name, &row.Type, &row.OverrideType, &row.OverrideValue); err != nil {
			return nil, err
		}
		out = append(out, row)
	}
	return out, rows.Err()
}

// ListAffiliateReportRows returns every affiliator (capped), narrowed by q (name
// or code) and level, with the window's
// new bindings and net revenue; commission status buckets are
// stitched by the service.
func (r *Repository) ListAffiliateReportRows(ctx context.Context, from, to time.Time, q, level string) ([]AffiliateReportRow, error) {
	p := newAffPredicate(3)
	p.eq("a.level", level)
	p.search(q, "u.name", "a.referral_code")
	rows, err := r.pool.Query(ctx,
		`SELECT a.user_id, COALESCE(a.referral_code, ''), u.name, a.level,
		        COALESCE(bind.students_new, 0), COALESCE(rev.net_revenue, 0)
		 FROM affiliators a
		 JOIN users u ON u.id = a.user_id
		 LEFT JOIN (
		     SELECT affiliator_id, count(*) AS students_new
		     FROM affiliate_bindings
		     WHERE started_at >= $1 AND started_at < $2
		     GROUP BY affiliator_id
		 ) bind ON bind.affiliator_id = a.user_id
		 LEFT JOIN (
		     SELECT affiliator_id, SUM(base_amount) AS net_revenue
		     FROM affiliate_commissions
		     WHERE status = 'available'
		       AND COALESCE(wait_until, paid_at) >= $1 AND COALESCE(wait_until, paid_at) < $2
		     GROUP BY affiliator_id
		 ) rev ON rev.affiliator_id = a.user_id
		 WHERE TRUE`+p.sql+`
		 ORDER BY u.created_at
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
			&row.StudentsNew, &row.NetRevenue); err != nil {
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
