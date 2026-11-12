package repository

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"akademi-bimbel/internal/model"
)

// insertOrderParticipantProduct creates a minimal exam-type product and
// returns its id, for order_item rows that need a valid product_id FK.
func insertOrderParticipantProduct(t *testing.T, repo *Repository, name string) uuid.UUID {
	t.Helper()
	p := model.Product{Type: "exam", Name: name, Price: 1000, Status: "published"}
	if err := repo.CreateProduct(context.Background(), &p); err != nil {
		t.Fatalf("insert product %s: %v", name, err)
	}
	id, err := uuid.Parse(p.ID)
	if err != nil {
		t.Fatalf("parse product id: %v", err)
	}
	return id
}

// insertOrderParticipantUser inserts a student user and returns the ID.
func insertOrderParticipantUser(t *testing.T, pool interface {
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}, name string) uuid.UUID {
	t.Helper()
	ctx := context.Background()
	var id uuid.UUID
	err := pool.QueryRow(ctx,
		`INSERT INTO users (email, role, name) VALUES ($1, $2, $3) RETURNING id`,
		name+"-op-test@local", "student", name,
	).Scan(&id)
	if err != nil {
		t.Fatalf("insert user %s: %v", name, err)
	}
	return id
}

// insertOrderParticipantOrder inserts an order and returns the ID.
func insertOrderParticipantOrder(t *testing.T, pool interface {
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}, buyerID uuid.UUID) uuid.UUID {
	t.Helper()
	ctx := context.Background()
	var orderID uuid.UUID
	err := pool.QueryRow(ctx,
		`INSERT INTO orders (student_id) VALUES ($1) RETURNING id`,
		buyerID,
	).Scan(&orderID)
	if err != nil {
		t.Fatalf("insert order: %v", err)
	}
	return orderID
}

// insertOrderParticipantExam inserts an exam and returns the ID.
func insertOrderParticipantExam(t *testing.T, pool interface {
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}) uuid.UUID {
	t.Helper()
	ctx := context.Background()
	var examID uuid.UUID
	err := pool.QueryRow(ctx,
		`INSERT INTO exam (title) VALUES ($1) RETURNING id`,
		"OrderParticipant Exam",
	).Scan(&examID)
	if err != nil {
		t.Fatalf("insert exam: %v", err)
	}
	return examID
}

// insertOrderParticipantRegistration inserts an exam_registration row.
func insertOrderParticipantRegistration(t *testing.T, pool interface {
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}, studentID, examID uuid.UUID) {
	t.Helper()
	ctx := context.Background()
	var id uuid.UUID
	err := pool.QueryRow(ctx,
		`INSERT INTO exam_registration (student_id, exam_id, token) VALUES ($1, $2, $3) RETURNING id`,
		studentID, examID, uuid.NewString(),
	).Scan(&id)
	if err != nil {
		t.Fatalf("insert exam_registration for %s: %v", studentID, err)
	}
}

func TestOrderParticipantRepositoryMethods(t *testing.T) {
	pool := newGradingTestPool(t)
	repo := New(pool)
	ctx := context.Background()

	// Seed initial data: two student users and a buyer for orders.
	studentA := insertOrderParticipantUser(t, pool, "Student A")
	studentB := insertOrderParticipantUser(t, pool, "Student B")
	studentC := insertOrderParticipantUser(t, pool, "Student C")
	buyer := insertOrderParticipantUser(t, pool, "Buyer")
	noPartBuyer := insertOrderParticipantUser(t, pool, "NoPartBuyer")

	// All three tests share the same exam.
	examID := insertOrderParticipantExam(t, pool)

	t.Run("InsertOrderItemParticipantsTx then GetOrderParticipants/GetOrderItemParticipants round-trip the same set", func(t *testing.T) {
		orderID := insertOrderParticipantOrder(t, pool, buyer)
		productID := insertOrderParticipantProduct(t, repo, "RoundTrip Product")
		studentIDs := []uuid.UUID{studentA, studentB}

		tx, err := repo.BeginTx(ctx)
		if err != nil {
			t.Fatalf("BeginTx: %v", err)
		}
		defer tx.Rollback(ctx)

		itemID, err := repo.InsertOrderItemTx(ctx, tx, orderID, model.OrderItem{
			ProductID: productID, ProductType: "exam", Name: "RoundTrip Product", UnitPrice: 1000, Qty: len(studentIDs),
		})
		if err != nil {
			t.Fatalf("InsertOrderItemTx: %v", err)
		}
		if err := repo.InsertOrderItemParticipantsTx(ctx, tx, orderID, itemID, studentIDs); err != nil {
			t.Fatalf("InsertOrderItemParticipantsTx: %v", err)
		}

		if err := tx.Commit(ctx); err != nil {
			t.Fatalf("Commit: %v", err)
		}

		got, err := repo.GetOrderParticipants(ctx, orderID)
		if err != nil {
			t.Fatalf("GetOrderParticipants: %v", err)
		}
		if len(got) != len(studentIDs) {
			t.Fatalf("GetOrderParticipants returned %d participants, want %d", len(got), len(studentIDs))
		}

		gotSet := make(map[uuid.UUID]bool, len(got))
		for _, id := range got {
			gotSet[id] = true
		}
		for _, wantID := range studentIDs {
			if !gotSet[wantID] {
				t.Errorf("participant %v was inserted but not returned by GetOrderParticipants", wantID)
			}
		}

		gotItem, err := repo.GetOrderItemParticipants(ctx, itemID)
		if err != nil {
			t.Fatalf("GetOrderItemParticipants: %v", err)
		}
		if len(gotItem) != len(studentIDs) {
			t.Fatalf("GetOrderItemParticipants returned %d participants, want %d", len(gotItem), len(studentIDs))
		}
	})

	// A multi-item order can serve a different student set per item — this is
	// exactly what lets a Mitra buy several products in one order while
	// excluding a student from only the one product they already own.
	t.Run("GetOrderItemParticipants isolates participants per item on a multi-item order", func(t *testing.T) {
		isolationBuyer := insertOrderParticipantUser(t, pool, "Isolation Buyer")
		orderID := insertOrderParticipantOrder(t, pool, isolationBuyer)
		productID := insertOrderParticipantProduct(t, repo, "Isolation Product")

		tx, err := repo.BeginTx(ctx)
		if err != nil {
			t.Fatalf("BeginTx: %v", err)
		}
		defer tx.Rollback(ctx)

		item1, err := repo.InsertOrderItemTx(ctx, tx, orderID, model.OrderItem{
			ProductID: productID, ProductType: "exam", Name: "Isolation Product 1", UnitPrice: 1000, Qty: 1,
		})
		if err != nil {
			t.Fatalf("InsertOrderItemTx (1): %v", err)
		}
		item2, err := repo.InsertOrderItemTx(ctx, tx, orderID, model.OrderItem{
			ProductID: productID, ProductType: "exam", Name: "Isolation Product 2", UnitPrice: 1000, Qty: 1,
		})
		if err != nil {
			t.Fatalf("InsertOrderItemTx (2): %v", err)
		}
		if err := repo.InsertOrderItemParticipantsTx(ctx, tx, orderID, item1, []uuid.UUID{studentA}); err != nil {
			t.Fatalf("InsertOrderItemParticipantsTx (1): %v", err)
		}
		if err := repo.InsertOrderItemParticipantsTx(ctx, tx, orderID, item2, []uuid.UUID{studentB}); err != nil {
			t.Fatalf("InsertOrderItemParticipantsTx (2): %v", err)
		}
		if err := tx.Commit(ctx); err != nil {
			t.Fatalf("Commit: %v", err)
		}

		item1Participants, err := repo.GetOrderItemParticipants(ctx, item1)
		if err != nil {
			t.Fatalf("GetOrderItemParticipants (1): %v", err)
		}
		if len(item1Participants) != 1 || item1Participants[0] != studentA {
			t.Fatalf("item1 participants = %v, want [%v]", item1Participants, studentA)
		}

		item2Participants, err := repo.GetOrderItemParticipants(ctx, item2)
		if err != nil {
			t.Fatalf("GetOrderItemParticipants (2): %v", err)
		}
		if len(item2Participants) != 1 || item2Participants[0] != studentB {
			t.Fatalf("item2 participants = %v, want [%v]", item2Participants, studentB)
		}

		all, err := repo.GetOrderParticipants(ctx, orderID)
		if err != nil {
			t.Fatalf("GetOrderParticipants: %v", err)
		}
		if len(all) != 2 {
			t.Fatalf("GetOrderParticipants (order-wide) = %v, want both students", all)
		}
	})

	t.Run("GetOrderParticipants on an order with no participant rows returns empty slice", func(t *testing.T) {
		// An order that has no participant rows — use a distinct buyer so
		// idx_orders_student_cart (one cart per student) is not violated.
		noPartOrderID := insertOrderParticipantOrder(t, pool, noPartBuyer)

		got, err := repo.GetOrderParticipants(ctx, noPartOrderID)
		if err != nil {
			t.Fatalf("GetOrderParticipants: %v", err)
		}
		if got == nil {
			t.Fatal("GetOrderParticipants returned nil, want empty slice (non-nil)")
		}
		if len(got) != 0 {
			t.Fatalf("GetOrderParticipants returned %d participants, want 0", len(got))
		}
	})

	t.Run("FilterAlreadyRegistered returns only the registered subset", func(t *testing.T) {
		// Register studentA for the exam, leave studentB and studentC unregistered.
		insertOrderParticipantRegistration(t, pool, studentA, examID)

		allIDs := []uuid.UUID{studentA, studentB, studentC}
		registered, err := repo.FilterAlreadyRegistered(ctx, examID, allIDs)
		if err != nil {
			t.Fatalf("FilterAlreadyRegistered: %v", err)
		}

		if len(registered) != 1 {
			t.Fatalf("FilterAlreadyRegistered returned %d, want 1 (only studentA)", len(registered))
		}
		if registered[0] != studentA {
			t.Errorf("FilterAlreadyRegistered returned %v, want %v (studentA)", registered[0], studentA)
		}
	})

	t.Run("FilterAlreadyRegistered with empty input returns empty slice", func(t *testing.T) {
		registered, err := repo.FilterAlreadyRegistered(ctx, examID, []uuid.UUID{})
		if err != nil {
			t.Fatalf("FilterAlreadyRegistered: %v", err)
		}
		if registered == nil {
			t.Fatal("FilterAlreadyRegistered returned nil, want empty slice (non-nil)")
		}
		if len(registered) != 0 {
			t.Fatalf("FilterAlreadyRegistered returned %d, want 0", len(registered))
		}
	})
}
