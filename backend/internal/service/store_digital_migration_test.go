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

// A student's digital order left payment_pending with no deadline is claimed by
// the backfill but the sweeper only expires orders that have one, so it would
// hold its targets (and any physical stock) forever. 0068 gives exactly those
// orders a standard window and leaves every other flow alone. Rolled back like
// the backfill test, since the statement touches every matching row.
func TestMigration0068_BoundsOnlyStudentDigitalPendingOrdersWithoutADeadline(t *testing.T) {
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
	tail := string(raw)[start:]

	student := newDigitalStudent(t, repo)
	school := newDigitalStudent(t, repo)
	if _, err := repo.Pool().Exec(ctx, `UPDATE users SET role = 'admin_school' WHERE id = $1`, school); err != nil {
		t.Fatalf("make school buyer: %v", err)
	}
	digitalLine := func() [2]any {
		return [2]any{seedDigitalProduct(t, repo, "course", 50000, seedCourseTarget(t, repo)), "course"}
	}
	var book uuid.UUID
	if err := repo.Pool().QueryRow(ctx,
		`INSERT INTO product (type, name, price, stock, status) VALUES ('book', $1, 1000, 5, 'published') RETURNING id`, "Book "+uuid.NewString(),
	).Scan(&book); err != nil {
		t.Fatalf("seed book: %v", err)
	}

	seed := func(buyer uuid.UUID, status string, deadline *time.Time, lines ...[2]any) uuid.UUID {
		t.Helper()
		var orderID uuid.UUID
		if err := repo.Pool().QueryRow(ctx,
			`INSERT INTO orders (student_id, status, subtotal, discount, shipping_cost, total, checked_out_at, payment_expires_at)
			 VALUES ($1, $2, 1, 0, 0, 1, now() - interval '3 days', $3) RETURNING id`, buyer, status, deadline,
		).Scan(&orderID); err != nil {
			t.Fatalf("seed order: %v", err)
		}
		for _, l := range lines {
			if _, err := repo.Pool().Exec(ctx,
				`INSERT INTO order_item (order_id, product_id, product_type, name, unit_price, qty, jumlah)
				 VALUES ($1, $2, $3, 'x', 1, 1, 1)`, orderID, l[0], l[1],
			); err != nil {
				t.Fatalf("seed item: %v", err)
			}
		}
		return orderID
	}
	bookLine := [2]any{book, "book"}

	studentDigital := seed(student, "payment_pending", nil, digitalLine())
	studentMixed := seed(student, "payment_pending", nil, digitalLine(), bookLine)
	physicalOnly := seed(student, "payment_pending", nil, bookLine)
	bulk := seed(student, "payment_pending", nil, digitalLine())
	if _, err := repo.Pool().Exec(ctx, `INSERT INTO order_participant (order_id, student_id) VALUES ($1, $2)`, bulk, student); err != nil {
		t.Fatalf("seed participant: %v", err)
	}
	schoolDigital := seed(school, "payment_pending", nil, digitalLine())
	paidNoDeadline := seed(student, "paid", nil, digitalLine())
	existing := time.Now().Add(90 * time.Minute).Truncate(time.Microsecond)
	withDeadline := seed(student, "payment_pending", &existing, digitalLine())

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

	if expired, err := repo.ExpirePaymentOrderTx(ctx, tx, studentDigital); err != nil || expired {
		t.Fatalf("before the migration a deadline-less order expired = %v, %v; want it stuck", expired, err)
	}
	if _, err := tx.Exec(ctx, tail); err != nil {
		t.Fatalf("migration tail: %v", err)
	}

	deadlineOf := func(id uuid.UUID) *time.Time {
		var d *time.Time
		if err := tx.QueryRow(ctx, `SELECT payment_expires_at FROM orders WHERE id = $1`, id).Scan(&d); err != nil {
			t.Fatalf("read deadline: %v", err)
		}
		return d
	}
	for name, id := range map[string]uuid.UUID{"student digital": studentDigital, "student mixed digital+physical": studentMixed} {
		d := deadlineOf(id)
		if d == nil {
			t.Errorf("%s order still has no deadline", name)
			continue
		}
		if left := time.Until(*d); left < 23*time.Hour || left > 24*time.Hour+time.Minute {
			t.Errorf("%s order deadline is %v away, want about 24h", name, left)
		}
	}
	for name, id := range map[string]uuid.UUID{
		"physical-only": physicalOnly, "bulk participant": bulk, "non-student buyer": schoolDigital, "paid": paidNoDeadline,
	} {
		if d := deadlineOf(id); d != nil {
			t.Errorf("%s order got deadline %v; the migration must not touch it", name, *d)
		}
	}
	if d := deadlineOf(withDeadline); d == nil || !d.Equal(existing) {
		t.Errorf("order with a deadline changed to %v, want %v", d, existing)
	}

	if _, err := tx.Exec(ctx, `UPDATE orders SET payment_expires_at = now() - interval '1 minute' WHERE id = $1`, studentDigital); err != nil {
		t.Fatalf("elapse deadline: %v", err)
	}
	if expired, err := repo.ExpirePaymentOrderTx(ctx, tx, studentDigital); err != nil || !expired {
		t.Fatalf("bounded order expired = %v, %v; want true", expired, err)
	}
	var held, live int
	if err := tx.QueryRow(ctx,
		`SELECT count(*), count(*) FILTER (WHERE released_at IS NULL) FROM order_digital_target WHERE order_id = $1`, studentDigital,
	).Scan(&held, &live); err != nil {
		t.Fatalf("count claims: %v", err)
	}
	if held != 1 || live != 0 {
		t.Errorf("expired order has %d claims, %d still live; want its 1 backfilled claim released", held, live)
	}
}
