package service

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	dbmigrations "akademi-bimbel/db"
)

// The backfill statement of 0068 runs against historical in-flight orders: a paid
// order keeps a contested target over an unpaid one (earliest placed breaks ties),
// and the losers are recorded in the conflict table instead of being dropped. The shared test DB already holds
// claims from other tests, so the statement runs in a transaction that starts
// empty and is rolled back.
func TestMigration0068_BackfillPrefersPaidOrderAndRecordsConflicts(t *testing.T) {
	_, repo := newRealDBService(t)
	ctx := context.Background()

	raw, err := dbmigrations.FS.ReadFile("migrations/0068_order_digital_target.up.sql")
	if err != nil {
		t.Fatalf("read migration: %v", err)
	}
	start := strings.Index(string(raw), "WITH candidate AS")
	if start < 0 {
		t.Fatal("backfill statement not found in migration")
	}
	backfill := string(raw)[start:]

	student := newDigitalStudent(t, repo)
	other := newDigitalStudent(t, repo)
	bulkBuyer := newDigitalStudent(t, repo)
	course := seedCourseTarget(t, repo)
	skuA := seedDigitalProduct(t, repo, "course", 50000, course)
	skuB := seedDigitalProduct(t, repo, "course", 40000, course)

	seed := func(buyer uuid.UUID, status string, placed time.Time, product uuid.UUID) (orderID, itemID uuid.UUID) {
		t.Helper()
		if status == "cart" {
			if err := repo.Pool().QueryRow(ctx,
				`INSERT INTO orders (student_id, status, subtotal, discount, shipping_cost, total)
				 VALUES ($1, 'cart', 0, 0, 0, 0) RETURNING id`, buyer,
			).Scan(&orderID); err != nil {
				t.Fatalf("seed cart: %v", err)
			}
		} else if err := repo.Pool().QueryRow(ctx,
			`INSERT INTO orders (student_id, status, subtotal, discount, shipping_cost, total, checked_out_at)
			 VALUES ($1, $2, 1, 0, 0, 1, $3) RETURNING id`, buyer, status, placed,
		).Scan(&orderID); err != nil {
			t.Fatalf("seed order: %v", err)
		}
		if err := repo.Pool().QueryRow(ctx,
			`INSERT INTO order_item (order_id, product_id, product_type, name, unit_price, qty, jumlah)
			 VALUES ($1, $2, 'course', 'x', 1, 1, 1) RETURNING id`, orderID, product,
		).Scan(&itemID); err != nil {
			t.Fatalf("seed item: %v", err)
		}
		return orderID, itemID
	}

	now := time.Now()
	pending, pendingItem := seed(student, "payment_pending", now.Add(-3*time.Hour), skuA)
	paid, _ := seed(student, "paid", now.Add(-2*time.Hour), skuB)
	expired, _ := seed(other, "payment_expired", now.Add(-4*time.Hour), skuA)
	pendingOther, _ := seed(other, "payment_pending", now.Add(-1*time.Hour), skuA)
	bulk, _ := seed(bulkBuyer, "payment_pending", now.Add(-1*time.Hour), skuA)
	if _, err := repo.Pool().Exec(ctx, `INSERT INTO order_participant (order_id, student_id) VALUES ($1, $2)`, bulk, other); err != nil {
		t.Fatalf("seed participant: %v", err)
	}

	tx, err := repo.BeginTx(ctx)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	defer tx.Rollback(ctx)
	if _, err := tx.Exec(ctx, `DELETE FROM order_digital_target_backfill_conflict`); err != nil {
		t.Fatalf("clear conflicts: %v", err)
	}
	if _, err := tx.Exec(ctx, `DELETE FROM order_digital_target`); err != nil {
		t.Fatalf("clear claims: %v", err)
	}
	if _, err := tx.Exec(ctx, backfill); err != nil {
		t.Fatalf("backfill: %v", err)
	}

	claimed := map[uuid.UUID]bool{}
	rows, err := tx.Query(ctx, `SELECT order_id FROM order_digital_target WHERE target_id = $1 AND released_at IS NULL`, course)
	if err != nil {
		t.Fatalf("read claims: %v", err)
	}
	for rows.Next() {
		var id uuid.UUID
		if err := rows.Scan(&id); err != nil {
			t.Fatalf("scan: %v", err)
		}
		claimed[id] = true
	}
	rows.Close()

	if !claimed[paid] || !claimed[pendingOther] || len(claimed) != 2 {
		t.Errorf("claimed orders = %v, want paid %v and other student's pending %v only", claimed, paid, pendingOther)
	}
	for name, id := range map[string]uuid.UUID{"earlier unpaid duplicate": pending, "expired": expired, "bulk": bulk} {
		if claimed[id] {
			t.Errorf("%s order %v must not hold a claim", name, id)
		}
	}

	var winner *uuid.UUID
	var conflicts int
	if err := tx.QueryRow(ctx,
		`SELECT count(*), max(winning_order_id::text)::uuid FROM order_digital_target_backfill_conflict WHERE order_item_id = $1`, pendingItem,
	).Scan(&conflicts, &winner); err != nil {
		t.Fatalf("read conflicts: %v", err)
	}
	if conflicts != 1 || winner == nil || *winner != paid {
		t.Errorf("conflict rows = %d winner %v, want 1 recorded against %v", conflicts, winner, paid)
	}

	var stamped int
	if err := tx.QueryRow(ctx,
		`SELECT count(*) FROM orders WHERE id = ANY($1::uuid[]) AND digital_targets_snapshot_at IS NOT NULL`,
		[]string{pending.String(), paid.String()},
	).Scan(&stamped); err != nil {
		t.Fatalf("read snapshot marker: %v", err)
	}
	if stamped != 0 {
		t.Errorf("%d backfilled orders carry a snapshot marker; historical orders must keep live-link fulfilment", stamped)
	}
}
