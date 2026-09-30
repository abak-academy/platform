package repository

import (
	"context"
	"testing"

	"github.com/google/uuid"
)

// A checkout holds a connection for its whole transaction, so its biodata read
// must land on that transaction instead of asking the pool for a second one.
// Uncommitted visibility is what tells the two apart.
func TestGetUserByIDTxReadsOnTheCallersTransaction(t *testing.T) {
	pool := newGradingTestPool(t)
	repo := New(pool)
	ctx := context.Background()

	email := "user-tx-test-" + uuid.NewString() + "@local"
	tx, err := repo.BeginTx(ctx)
	if err != nil {
		t.Fatalf("BeginTx: %v", err)
	}
	defer tx.Rollback(ctx)

	var id uuid.UUID
	if err := tx.QueryRow(ctx,
		`INSERT INTO users (email, role, name) VALUES ($1, 'student', 'Tx User') RETURNING id`,
		email,
	).Scan(&id); err != nil {
		t.Fatalf("insert user in tx: %v", err)
	}

	got, err := repo.GetUserByIDTx(ctx, tx, id)
	if err != nil {
		t.Fatalf("GetUserByIDTx: %v", err)
	}
	if got == nil || got.Email == nil || *got.Email != email {
		t.Fatalf("GetUserByIDTx = %+v, want the uncommitted user %s", got, email)
	}

	// The pool read cannot see that row yet, which is what makes the check above
	// evidence about the transaction rather than about the row existing.
	fromPool, err := repo.GetUserByID(ctx, id.String())
	if err != nil {
		t.Fatalf("GetUserByID: %v", err)
	}
	if fromPool != nil {
		t.Fatalf("GetUserByID = %+v before commit, want none", fromPool)
	}
}
