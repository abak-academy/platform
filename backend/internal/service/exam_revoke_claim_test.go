package service

import (
	"context"
	"testing"

	"github.com/google/uuid"

	"akademi-bimbel/internal/model"
	"akademi-bimbel/internal/repository"
)

// fulfilledExamPurchase leaves the student with what the worker produces for a
// paid exam order: a completed order holding its claim and a live registration.
func fulfilledExamPurchase(t *testing.T, svc *Service, repo *repository.Repository, student, exam uuid.UUID) (orderID, product uuid.UUID) {
	t.Helper()
	product = seedDigitalProduct(t, repo, "exam", 0, exam)
	orderID = cartOf(t, svc, student, product)
	if _, err := svc.Checkout(context.Background(), student.String(), orderID.String(), checkoutKey()); err != nil {
		t.Fatalf("Checkout: %v", err)
	}
	registerForExam(t, repo, student, exam, "registered")
	if _, err := repo.Pool().Exec(context.Background(), `UPDATE orders SET status = 'completed' WHERE id = $1`, orderID); err != nil {
		t.Fatalf("complete order: %v", err)
	}
	return orderID, product
}

func claimReleased(t *testing.T, repo *repository.Repository, orderID uuid.UUID) (released bool, reason string) {
	t.Helper()
	var at *string
	var why *string
	if err := repo.Pool().QueryRow(context.Background(),
		`SELECT released_at::text, release_reason FROM order_digital_target WHERE order_id = $1`, orderID,
	).Scan(&at, &why); err != nil {
		t.Fatalf("read claim: %v", err)
	}
	if why != nil {
		reason = *why
	}
	return at != nil, reason
}

func TestRevokeExamAccess_ReleasesPurchaseClaimSoTheExamCanBeBoughtAgain(t *testing.T) {
	svc, repo, _ := newDigitalPurchaseService(t)
	ctx := context.Background()
	student := newDigitalStudent(t, repo)
	exam := seedExamTarget(t, repo)
	actor := seedRevokeActor(t, repo)
	orderID, product := fulfilledExamPurchase(t, svc, repo, student, exam)

	cart, _, err := svc.MintCart(ctx, student.String())
	if err != nil {
		t.Fatalf("MintCart: %v", err)
	}
	blocked := svc.AddItem(ctx, student.String(), cart.ID.String(), product.String(), 1)
	wantConflict(t, blocked, model.DigitalConflictOwned)

	result, err := svc.RevokeExamAccess(ctx, actor.String(), exam.String(), []uuid.UUID{student})
	if err != nil || result.RevokedCount != 1 {
		t.Fatalf("RevokeExamAccess = %+v, %v; want 1 revoked", result, err)
	}

	if released, reason := claimReleased(t, repo, orderID); !released || reason != "revoked" {
		t.Errorf("claim released = %v (reason %q), want released with reason revoked", released, reason)
	}
	if err := svc.AddItem(ctx, student.String(), cart.ID.String(), product.String(), 1); err != nil {
		t.Fatalf("AddItem after revoke: %v", err)
	}
	if _, err := svc.Checkout(ctx, student.String(), cart.ID.String(), checkoutKey()); err != nil {
		t.Fatalf("repurchase after revoke: %v", err)
	}
}

func TestRevokeExamAccess_KeepsClaimOfAnInFlightPurchase(t *testing.T) {
	svc, repo, _ := newDigitalPurchaseService(t)
	ctx := context.Background()
	student := newDigitalStudent(t, repo)
	exam := seedExamTarget(t, repo)
	actor := seedRevokeActor(t, repo)
	product := seedDigitalProduct(t, repo, "exam", 50000, exam)
	pendingID := cartOf(t, svc, student, product)
	if _, err := svc.Checkout(ctx, student.String(), pendingID.String(), checkoutKey()); err != nil {
		t.Fatalf("Checkout: %v", err)
	}
	registerForExam(t, repo, student, exam, "registered")

	if _, err := svc.RevokeExamAccess(ctx, actor.String(), exam.String(), []uuid.UUID{student}); err != nil {
		t.Fatalf("RevokeExamAccess: %v", err)
	}

	if released, _ := claimReleased(t, repo, pendingID); released {
		t.Error("claim of a payment_pending order was released by a manual revoke")
	}
}

func TestRevokeExamAccess_KeepsClaimOfPaidOrderAwaitingFulfilment(t *testing.T) {
	svc, repo, _ := newDigitalPurchaseService(t)
	ctx := context.Background()
	student := newDigitalStudent(t, repo)
	exam := seedExamTarget(t, repo)
	actor := seedRevokeActor(t, repo)
	orderID, _ := fulfilledExamPurchase(t, svc, repo, student, exam)
	if _, err := repo.Pool().Exec(ctx, `UPDATE orders SET status = 'paid' WHERE id = $1`, orderID); err != nil {
		t.Fatalf("reset to paid: %v", err)
	}

	if _, err := svc.RevokeExamAccess(ctx, actor.String(), exam.String(), []uuid.UUID{student}); err != nil {
		t.Fatalf("RevokeExamAccess: %v", err)
	}

	if released, _ := claimReleased(t, repo, orderID); released {
		t.Error("claim of a paid, not yet fulfilled order was released by a manual revoke")
	}
}

func TestRevokeExamAccess_UnrevokedStudentKeepsClaim(t *testing.T) {
	svc, repo, _ := newDigitalPurchaseService(t)
	ctx := context.Background()
	student, bystander := newDigitalStudent(t, repo), newDigitalStudent(t, repo)
	exam := seedExamTarget(t, repo)
	actor := seedRevokeActor(t, repo)
	orderID, _ := fulfilledExamPurchase(t, svc, repo, student, exam)

	result, err := svc.RevokeExamAccess(ctx, actor.String(), exam.String(), []uuid.UUID{bystander})
	if err != nil || result.RevokedCount != 0 {
		t.Fatalf("RevokeExamAccess = %+v, %v; want nothing revoked", result, err)
	}

	if released, _ := claimReleased(t, repo, orderID); released {
		t.Error("claim released although this student's access was not revoked")
	}
	if status, _ := registrationStatus(t, repo, exam, student); status != "registered" {
		t.Errorf("registration = %q, want registered", status)
	}
}

func TestRevokeExamAccessBulk_ReleasesPurchaseClaim(t *testing.T) {
	svc, repo, _ := newDigitalPurchaseService(t)
	ctx := context.Background()
	student := newDigitalStudent(t, repo)
	exam := seedExamTarget(t, repo)
	actor := seedRevokeActor(t, repo)
	orderID, _ := fulfilledExamPurchase(t, svc, repo, student, exam)
	var username string
	if err := repo.Pool().QueryRow(ctx, `SELECT username FROM users WHERE id = $1`, student).Scan(&username); err != nil {
		t.Fatalf("username: %v", err)
	}

	rows, err := svc.RevokeExamAccessBulk(ctx, actor.String(), exam.String(), []string{username})
	if err != nil || len(rows) != 1 || rows[0].Status != "revoked" {
		t.Fatalf("RevokeExamAccessBulk = %+v, %v; want one revoked row", rows, err)
	}

	if released, _ := claimReleased(t, repo, orderID); !released {
		t.Error("claim not released after bulk revoke of the purchased exam")
	}
}
