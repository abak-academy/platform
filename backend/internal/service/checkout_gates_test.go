package service

import (
	"context"
	"errors"
	"testing"

	"akademi-bimbel/internal/repository"

	"github.com/alicebob/miniredis/v2"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

// newCheckoutTestService builds a real-DB Service with redis + a noop payment
// client, for exercising the Checkout gates end to end.
func newCheckoutTestService(t *testing.T) (*Service, *repository.Repository) {
	t.Helper()
	_, repo := newRealDBService(t)
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("miniredis: %v", err)
	}
	t.Cleanup(mr.Close)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	svc := NewWithStore(repo, repo, rdb, nil, &NoopOTPProvider{}, &NoopEmailProvider{}, &NoopPaymentClient{}, &NoopLogisticsClient{}, nil, nil)
	return svc, repo
}

func insertCheckoutStudent(t *testing.T, repo *repository.Repository, name, prefix string) string {
	t.Helper()
	var id string
	if err := repo.Pool().QueryRow(context.Background(),
		`INSERT INTO users (name, role, status, username, password_hash)
		 VALUES ($1, 'student', 'active', $2, '') RETURNING id`,
		name, prefix+uniqueSuffix(),
	).Scan(&id); err != nil {
		t.Fatalf("insert student: %v", err)
	}
	return id
}

// TestCheckout_FreeOrder_SkipsGateway verifies a zero-total order settles
// directly (Free=true, no gateway ref, status paid) without the payment client.
func TestCheckout_FreeOrder_SkipsGateway(t *testing.T) {
	svc, repo := newCheckoutTestService(t)
	ctx := context.Background()

	// Free digital product (course → no shipping, not gated on biodata).
	var productID string
	if err := repo.Pool().QueryRow(ctx,
		`INSERT INTO product (type, name, price, stock, status)
		 VALUES ('course', $1, 0, 0, 'published') RETURNING id`,
		"Free Course "+uuid.New().String(),
	).Scan(&productID); err != nil {
		t.Fatalf("create product: %v", err)
	}

	studentID := insertCheckoutStudent(t, repo, "Free Student", "freestu_")

	order, _, err := svc.MintCart(ctx, studentID)
	if err != nil {
		t.Fatalf("MintCart: %v", err)
	}
	if err := svc.AddItem(ctx, studentID, order.ID.String(), productID, 1); err != nil {
		t.Fatalf("AddItem: %v", err)
	}

	result, err := svc.Checkout(ctx, studentID, order.ID.String(), "free-key-"+uniqueSuffix())
	if err != nil {
		t.Fatalf("Checkout: %v", err)
	}
	if !result.Free {
		t.Error("want result.Free = true for a zero-total order")
	}
	if result.GatewayRef != "" {
		t.Errorf("want empty GatewayRef (gateway skipped), got %q", result.GatewayRef)
	}

	got, err := svc.GetStudentOrder(ctx, studentID, order.ID.String())
	if err != nil {
		t.Fatalf("GetStudentOrder: %v", err)
	}
	if got.Status != "paid" {
		t.Errorf("want order status paid, got %q", got.Status)
	}
}

// TestCheckout_FreeOrder_PostCommitCacheFailureStillSucceeds pins that work done
// after the free-checkout commit cannot fail the call. The order is already paid
// and fulfilment already queued at that point, so surfacing the error would tell
// the student the checkout failed while their retry hits a non-cart order.
func TestCheckout_FreeOrder_PostCommitCacheFailureStillSucceeds(t *testing.T) {
	_, repo := newRealDBService(t)
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("miniredis: %v", err)
	}
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	svc := NewWithStore(repo, repo, rdb, nil, &NoopOTPProvider{}, &NoopEmailProvider{}, &NoopPaymentClient{}, &NoopLogisticsClient{}, nil, nil)
	ctx := context.Background()

	var productID string
	if err := repo.Pool().QueryRow(ctx,
		`INSERT INTO product (type, name, price, stock, status)
		 VALUES ('course', $1, 0, 0, 'published') RETURNING id`,
		"Free Course "+uuid.New().String(),
	).Scan(&productID); err != nil {
		t.Fatalf("create product: %v", err)
	}

	studentID := insertCheckoutStudent(t, repo, "Cache Down Student", "cachestu_")

	order, _, err := svc.MintCart(ctx, studentID)
	if err != nil {
		t.Fatalf("MintCart: %v", err)
	}
	if err := svc.AddItem(ctx, studentID, order.ID.String(), productID, 1); err != nil {
		t.Fatalf("AddItem: %v", err)
	}

	// Redis is unreachable for the whole checkout, so caching the idempotency
	// sentinel after the commit fails.
	mr.Close()

	result, err := svc.Checkout(ctx, studentID, order.ID.String(), "cache-down-key-"+uniqueSuffix())
	if err != nil {
		t.Fatalf("Checkout must succeed despite a post-commit cache failure, got: %v", err)
	}
	if !result.Free {
		t.Error("want result.Free = true for a zero-total order")
	}

	got, err := svc.GetStudentOrder(ctx, studentID, order.ID.String())
	if err != nil {
		t.Fatalf("GetStudentOrder: %v", err)
	}
	if got.Status != "paid" {
		t.Errorf("want order status paid, got %q", got.Status)
	}
}

// TestCheckout_FreeOrder_PromoUsageCommitsWithTheOrder pins that a promo-backed
// free checkout counts its redemption. The count used to be incremented after
// the transaction committed, where a failure was discarded — an under-counted
// promo lets max_uses admit redemptions past its limit. The earlier
// post-commit-failure test never reached this branch because it created no promo.
func TestCheckout_FreeOrder_PromoUsageCommitsWithTheOrder(t *testing.T) {
	svc, repo := newCheckoutTestService(t)
	ctx := context.Background()

	var productID string
	if err := repo.Pool().QueryRow(ctx,
		`INSERT INTO product (type, name, price, stock, status)
		 VALUES ('course', $1, 0, 0, 'published') RETURNING id`,
		"Promo Free Course "+uuid.New().String(),
	).Scan(&productID); err != nil {
		t.Fatalf("create product: %v", err)
	}

	var promoID string
	if err := repo.Pool().QueryRow(ctx,
		`INSERT INTO promo_code (code, discount_percent, max_uses, used_count)
		 VALUES ($1, 100, 1, 0) RETURNING id`,
		"FREE"+uniqueSuffix(),
	).Scan(&promoID); err != nil {
		t.Fatalf("create promo: %v", err)
	}

	studentID := insertCheckoutStudent(t, repo, "Promo Student", "promostu_")

	order, _, err := svc.MintCart(ctx, studentID)
	if err != nil {
		t.Fatalf("MintCart: %v", err)
	}
	if err := svc.AddItem(ctx, studentID, order.ID.String(), productID, 1); err != nil {
		t.Fatalf("AddItem: %v", err)
	}
	if _, err := repo.Pool().Exec(ctx,
		`UPDATE orders SET promo_code_id = $1 WHERE id = $2`, promoID, order.ID,
	); err != nil {
		t.Fatalf("attach promo to order: %v", err)
	}

	result, err := svc.Checkout(ctx, studentID, order.ID.String(), "promo-key-"+uniqueSuffix())
	if err != nil {
		t.Fatalf("Checkout: %v", err)
	}
	if !result.Free {
		t.Error("want result.Free = true for a zero-total order")
	}

	var usedCount int
	if err := repo.Pool().QueryRow(ctx,
		`SELECT used_count FROM promo_code WHERE id = $1`, promoID,
	).Scan(&usedCount); err != nil {
		t.Fatalf("read used_count: %v", err)
	}
	if usedCount != 1 {
		t.Errorf("used_count = %d, want 1 — the redemption must be counted", usedCount)
	}
}

// TestCheckout_ExamRequiresBiodata verifies a student self-purchasing an exam is
// blocked until school/class/dob are complete.
func TestCheckout_ExamRequiresBiodata(t *testing.T) {
	svc, repo := newCheckoutTestService(t)
	ctx := context.Background()

	examID := createTestExamForBulk(t, repo)
	productID := createTestExamProductForBulk(t, repo, examID, 50000)

	studentID := insertCheckoutStudent(t, repo, "Biodata Student", "biostu_")

	order, _, err := svc.MintCart(ctx, studentID)
	if err != nil {
		t.Fatalf("MintCart: %v", err)
	}
	if err := svc.AddItem(ctx, studentID, order.ID.String(), productID, 1); err != nil {
		t.Fatalf("AddItem: %v", err)
	}

	// Incomplete biodata → blocked.
	if _, err := svc.Checkout(ctx, studentID, order.ID.String(), "bio-key-1-"+uniqueSuffix()); !errors.Is(err, ErrBiodataIncomplete) {
		t.Fatalf("want ErrBiodataIncomplete, got %v", err)
	}

	// Complete biodata (unlisted school avoids needing a school row) → allowed.
	if _, err := repo.Pool().Exec(ctx,
		`UPDATE users SET unlisted_school_name = 'SMA Test', grade = 12, dob = '2008-01-01' WHERE id = $1`,
		studentID,
	); err != nil {
		t.Fatalf("set biodata: %v", err)
	}

	result, err := svc.Checkout(ctx, studentID, order.ID.String(), "bio-key-2-"+uniqueSuffix())
	if err != nil {
		t.Fatalf("Checkout after biodata complete: %v", err)
	}
	if result.GatewayRef == "" {
		t.Error("want a gateway ref once biodata is complete (payment proceeds)")
	}
}
