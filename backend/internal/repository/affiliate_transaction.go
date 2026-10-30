package repository

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"akademi-bimbel/internal/model"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// ErrAffiliateCommissionExists reports that the order item already carries a
// commission (unique on order_item_id): a replayed OrderPaid event, not a failure.
var ErrAffiliateCommissionExists = errors.New("affiliate commission already exists for order item")

// ErrAffiliatePayoutInFlight reports a second undecided payout for one
// affiliator, refused by the partial unique index.
var ErrAffiliatePayoutInFlight = errors.New("affiliate payout already in flight")

// affiliate_transactions holds commissions, refund deductions, payouts, payout
// reversals and adjustments. The projections below give each kind back its
// model shape; the prefix is the table alias (or table name in RETURNING).

func affiliateCommissionSelect(t string) string {
	return t + `.id, ` + t + `.affiliator_id, ` + t + `.student_id, ` + t + `.order_id, ` + t + `.order_item_id, ` + t + `.product_type, ` +
		t + `.base_amount, ` + t + `.detail->'rule'->>'type', (` + t + `.detail->'rule'->>'value')::numeric, ` + t + `.detail->>'level', ` +
		t + `.amount, ` + t + `.status, ` + t + `.release_at, ` + t + `.order_paid_at, ` + t + `.effective_at, ` + t + `.created_at, ` + t + `.updated_at`
}

func affiliateCommissionDest(c *model.AffiliateCommission) []any {
	return []any{&c.ID, &c.AffiliatorID, &c.StudentID, &c.OrderID, &c.OrderItemID, &c.ProductType,
		&c.BaseAmount, &c.CommissionType, &c.CommissionValue, &c.LevelSnapshot,
		&c.Amount, &c.Status, &c.WaitUntil, &c.PaidAt, &c.ReceivedAt, &c.CreatedAt, &c.UpdatedAt}
}

func affiliatePayoutSelect(t string) string {
	return t + `.id, ` + t + `.affiliator_id, -` + t + `.amount, ` + t + `.status, ` +
		`COALESCE(` + t + `.detail->'bank_account'->>'bank_name', ''), COALESCE(` + t + `.detail->'bank_account'->>'account_number', ''), ` +
		`COALESCE(` + t + `.detail->'bank_account'->>'account_holder', ''), ` +
		t + `.detail->>'proof_url', ` + t + `.detail->>'reject_reason', (` + t + `.detail->>'processed_by')::uuid, ` +
		`(` + t + `.detail->>'processed_at')::timestamptz, ` + t + `.created_at, ` + t + `.created_at, ` + t + `.updated_at`
}

func affiliatePayoutDest(p *model.AffiliatePayout) []any {
	return []any{&p.ID, &p.AffiliatorID, &p.Amount, &p.Status, &p.BankName, &p.BankAccountNumber, &p.BankAccountHolder,
		&p.ProofURL, &p.RejectReason, &p.ProcessedBy, &p.ProcessedAt, &p.RequestedAt, &p.CreatedAt, &p.UpdatedAt}
}

func affiliateAdjustmentSelect(t string) string {
	return t + `.id, ` + t + `.affiliator_id, ` + t + `.detail->>'category', ` + t + `.amount, COALESCE(` + t + `.note, ''), ` +
		t + `.actor_id, ` + t + `.created_at`
}

func affiliateAdjustmentDest(a *model.AffiliateAdjustment) []any {
	return []any{&a.ID, &a.AffiliatorID, &a.Category, &a.Amount, &a.Reason, &a.CreatedBy, &a.CreatedAt}
}

// affiliateLedgerSelect projects a taken-effect transaction as a ledger entry;
// the commission, payout and adjustment ids are derived from the row's parent.
func affiliateLedgerSelect(t string) string {
	return t + `.id, ` + t + `.affiliator_id, ` + t + `.type, ` + t + `.amount, ` +
		`CASE ` + t + `.type WHEN 'commission' THEN ` + t + `.id WHEN 'refund_deduction' THEN ` + t + `.parent_id END, ` +
		`CASE ` + t + `.type WHEN 'payout' THEN ` + t + `.id WHEN 'payout_reversal' THEN ` + t + `.parent_id END, ` +
		`CASE ` + t + `.type WHEN 'adjustment' THEN ` + t + `.id END, ` +
		t + `.note, ` + t + `.actor_id, ` + t + `.effective_at`
}

func affiliateLedgerDest(e *model.AffiliateLedgerEntry) []any {
	return []any{&e.ID, &e.AffiliatorID, &e.EntryType, &e.Amount, &e.CommissionID, &e.PayoutID, &e.AdjustmentID,
		&e.Note, &e.ActorID, &e.CreatedAt}
}

// InsertAffiliateCommissionTx inserts one pending commission inside tx and
// fills ID/CreatedAt/UpdatedAt. ON CONFLICT DO NOTHING makes a replayed
// OrderPaid event a no-op, reported as ErrAffiliateCommissionExists.
func (r *Repository) InsertAffiliateCommissionTx(ctx context.Context, tx pgx.Tx, c *model.AffiliateCommission) error {
	detail, err := json.Marshal(map[string]any{
		"rule":  map[string]any{"type": c.CommissionType, "value": c.CommissionValue},
		"level": c.LevelSnapshot,
	})
	if err != nil {
		return err
	}
	err = tx.QueryRow(ctx,
		`INSERT INTO affiliate_transactions
		     (affiliator_id, type, status, amount, student_id, order_id, order_item_id, product_type,
		      quantity, base_amount, release_at, order_paid_at, detail)
		 VALUES ($1, 'commission', $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
		 ON CONFLICT (order_item_id) WHERE type = 'commission' DO NOTHING
		 RETURNING id, created_at, updated_at`,
		c.AffiliatorID, c.Status, c.Amount, c.StudentID, c.OrderID, c.OrderItemID, c.ProductType,
		c.Quantity, c.BaseAmount, c.WaitUntil, c.PaidAt, detail,
	).Scan(&c.ID, &c.CreatedAt, &c.UpdatedAt)
	if isNotFound(err) {
		return ErrAffiliateCommissionExists
	}
	return err
}

// GetCommissionsByOrderTx returns the order's affiliate commissions as read
// inside the caller's transaction, so the refund settles exactly the rows it
// mutates. The rows stay locked until the tx ends, so a concurrent release
// job waits and then sees the refunded status. An order without commissions
// returns an empty slice.
func (r *Repository) GetCommissionsByOrderTx(ctx context.Context, tx pgx.Tx, orderID uuid.UUID) ([]model.AffiliateCommission, error) {
	rows, err := r.affiliateQueryer(tx).Query(ctx,
		`SELECT `+affiliateCommissionSelect("c")+`
		 FROM affiliate_transactions c
		 WHERE c.type = 'commission' AND c.order_id = $1
		 ORDER BY c.created_at
		 FOR UPDATE`, orderID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []model.AffiliateCommission
	for rows.Next() {
		var c model.AffiliateCommission
		if err := rows.Scan(affiliateCommissionDest(&c)...); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// GetAffiliateCommissionsByIDsTx locks and returns the named commissions that
// belong to the affiliator; ids that match nothing are simply absent.
func (r *Repository) GetAffiliateCommissionsByIDsTx(ctx context.Context, tx pgx.Tx, affiliatorID uuid.UUID, ids []uuid.UUID) ([]model.AffiliateCommission, error) {
	rows, err := tx.Query(ctx,
		`SELECT `+affiliateCommissionSelect("c")+`
		 FROM affiliate_transactions c
		 WHERE c.type = 'commission' AND c.affiliator_id = $1 AND c.id = ANY($2)
		 ORDER BY c.created_at
		 FOR UPDATE`, affiliatorID, ids)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := []model.AffiliateCommission{}
	for rows.Next() {
		var c model.AffiliateCommission
		if err := rows.Scan(affiliateCommissionDest(&c)...); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// SetCommissionStatusFromTx moves one commission from one status to another
// inside tx and reports whether it was still in the expected status.
func (r *Repository) SetCommissionStatusFromTx(ctx context.Context, tx pgx.Tx, commissionID uuid.UUID, from, to string) (bool, error) {
	tag, err := tx.Exec(ctx,
		`UPDATE affiliate_transactions SET status = $3, updated_at = now()
		 WHERE id = $1 AND type = 'commission' AND status = $2`,
		commissionID, from, to)
	if err != nil {
		return false, err
	}
	return tag.RowsAffected() > 0, nil
}

// InsertRefundDeductionTx posts the negative counterpart of one released
// commission, copying its student and order references so ledger rows still
// name the student and product. A commission that already has its deduction is
// left alone: the refund guard refuses a repeat before this runs.
func (r *Repository) InsertRefundDeductionTx(ctx context.Context, tx pgx.Tx, commissionID, actorID uuid.UUID, note string) error {
	_, err := tx.Exec(ctx,
		`INSERT INTO affiliate_transactions
		     (affiliator_id, type, status, amount, effective_at, parent_id, actor_id, note,
		      student_id, order_id, order_item_id, product_type)
		 SELECT affiliator_id, 'refund_deduction', 'posted', -amount, now(), id, $2, $3,
		        student_id, order_id, order_item_id, product_type
		 FROM affiliate_transactions
		 WHERE id = $1 AND type = 'commission'
		 ON CONFLICT (parent_id) WHERE type = 'refund_deduction' DO NOTHING`,
		commissionID, actorID, note)
	return err
}

// InsertAffiliatePayoutTx persists one requested payout inside tx, debiting the
// balance from the moment of the request, and fills the model from the row.
func (r *Repository) InsertAffiliatePayoutTx(ctx context.Context, tx pgx.Tx, p *model.AffiliatePayout) error {
	detail, err := json.Marshal(map[string]any{"bank_account": affiliatorBankAccount{
		BankName: p.BankName, AccountNumber: p.BankAccountNumber, AccountHolder: p.BankAccountHolder,
	}})
	if err != nil {
		return err
	}
	err = tx.QueryRow(ctx,
		`INSERT INTO affiliate_transactions (affiliator_id, type, status, amount, effective_at, actor_id, note, detail)
		 VALUES ($1, 'payout', $2, -$3::numeric, now(), $1, 'payout requested', $4)
		 RETURNING `+affiliatePayoutSelect("affiliate_transactions"),
		p.AffiliatorID, p.Status, p.Amount, detail,
	).Scan(affiliatePayoutDest(p)...)
	if isUniqueViolation(err) {
		return ErrAffiliatePayoutInFlight
	}
	return err
}

// GetAffiliatePayoutForUpdateTx returns one payout locked FOR UPDATE inside
// tx, or (nil, nil) when the id is unknown.
func (r *Repository) GetAffiliatePayoutForUpdateTx(ctx context.Context, tx pgx.Tx, id uuid.UUID) (*model.AffiliatePayout, error) {
	var p model.AffiliatePayout
	err := tx.QueryRow(ctx,
		`SELECT `+affiliatePayoutSelect("p")+`
		 FROM affiliate_transactions p
		 WHERE p.id = $1 AND p.type = 'payout'
		 FOR UPDATE`,
		id,
	).Scan(affiliatePayoutDest(&p)...)
	if isNotFound(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &p, nil
}

// UpdateAffiliatePayoutDecisionTx moves one payout to a new status with the
// proof or rejection reason and the acting admin, and returns the updated row.
func (r *Repository) UpdateAffiliatePayoutDecisionTx(ctx context.Context, tx pgx.Tx, id uuid.UUID, status string, proofURL, rejectReason *string, processedBy uuid.UUID) (*model.AffiliatePayout, error) {
	var p model.AffiliatePayout
	err := tx.QueryRow(ctx,
		`UPDATE affiliate_transactions
		 SET status = $2,
		     detail = detail || jsonb_strip_nulls(jsonb_build_object(
		         'proof_url', $3::text, 'reject_reason', $4::text,
		         'processed_by', $5::text, 'processed_at', now())),
		     updated_at = now()
		 WHERE id = $1 AND type = 'payout'
		 RETURNING `+affiliatePayoutSelect("affiliate_transactions"),
		id, status, proofURL, rejectReason, processedBy,
	).Scan(affiliatePayoutDest(&p)...)
	if err != nil {
		return nil, err
	}
	return &p, nil
}

// InsertAffiliatePayoutReversalTx returns a rejected payout's amount to the
// balance; a payout that already has its reversal is left alone.
func (r *Repository) InsertAffiliatePayoutReversalTx(ctx context.Context, tx pgx.Tx, payoutID, actorID uuid.UUID, note string) error {
	_, err := tx.Exec(ctx,
		`INSERT INTO affiliate_transactions (affiliator_id, type, status, amount, effective_at, parent_id, actor_id, note)
		 SELECT affiliator_id, 'payout_reversal', 'posted', -amount, now(), id, $2, $3
		 FROM affiliate_transactions
		 WHERE id = $1 AND type = 'payout'
		 ON CONFLICT (parent_id) WHERE type = 'payout_reversal' DO NOTHING`,
		payoutID, actorID, note)
	return err
}

// InsertAffiliateAdjustmentTx posts one admin adjustment inside tx, taking
// effect immediately, and fills ID/CreatedAt.
func (r *Repository) InsertAffiliateAdjustmentTx(ctx context.Context, tx pgx.Tx, a *model.AffiliateAdjustment) error {
	raw, err := json.Marshal(map[string]any{"category": a.Category})
	if err != nil {
		return err
	}
	return tx.QueryRow(ctx,
		`INSERT INTO affiliate_transactions (affiliator_id, type, status, amount, effective_at, actor_id, note, detail)
		 VALUES ($1, 'adjustment', 'posted', $2, now(), $3, $4, $5)
		 RETURNING id, created_at`,
		a.AffiliatorID, a.Amount, a.CreatedBy, a.Reason, raw,
	).Scan(&a.ID, &a.CreatedAt)
}

// SumAffiliateBalanceAt is the affiliator's balance at a moment: everything
// that had taken effect by then.
func (r *Repository) SumAffiliateBalanceAt(ctx context.Context, affiliatorID uuid.UUID, at time.Time) (float64, error) {
	var saldo float64
	err := r.pool.QueryRow(ctx,
		`SELECT COALESCE(SUM(amount), 0) FROM affiliate_transactions WHERE affiliator_id = $1 AND effective_at <= $2`,
		affiliatorID, at,
	).Scan(&saldo)
	return saldo, err
}
