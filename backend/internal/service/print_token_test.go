package service

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
)

func newPrintTokenTestService(t *testing.T) (*Service, *miniredis.Miniredis) {
	t.Helper()
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("miniredis setup failed: %v", err)
	}
	t.Cleanup(mr.Close)

	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { rdb.Close() })

	return &Service{rdb: rdb}, mr
}

func TestPrintToken_MintAndRedeem_Success(t *testing.T) {
	svc, _ := newPrintTokenTestService(t)
	ctx := context.Background()

	token, err := svc.MintPrintToken(ctx, PrintTokenKindCertificate, "session-1")
	if err != nil {
		t.Fatalf("MintPrintToken: %v", err)
	}
	if token == "" {
		t.Fatal("expected non-empty token")
	}

	if err := svc.RedeemPrintToken(ctx, token, PrintTokenKindCertificate, "session-1"); err != nil {
		t.Fatalf("RedeemPrintToken: %v", err)
	}
}

func TestPrintToken_SecondRedemptionFails(t *testing.T) {
	svc, _ := newPrintTokenTestService(t)
	ctx := context.Background()

	token, err := svc.MintPrintToken(ctx, PrintTokenKindCard, "reg-1")
	if err != nil {
		t.Fatalf("MintPrintToken: %v", err)
	}

	if err := svc.RedeemPrintToken(ctx, token, PrintTokenKindCard, "reg-1"); err != nil {
		t.Fatalf("first RedeemPrintToken: %v", err)
	}

	err = svc.RedeemPrintToken(ctx, token, PrintTokenKindCard, "reg-1")
	if err == nil {
		t.Fatal("expected error on second redemption, got nil")
	}
	if err != ErrPrintTokenInvalid {
		t.Fatalf("expected ErrPrintTokenInvalid, got %v", err)
	}
}

func TestPrintToken_WrongKindFails(t *testing.T) {
	svc, _ := newPrintTokenTestService(t)
	ctx := context.Background()

	token, err := svc.MintPrintToken(ctx, PrintTokenKindCard, "reg-1")
	if err != nil {
		t.Fatalf("MintPrintToken: %v", err)
	}

	err = svc.RedeemPrintToken(ctx, token, PrintTokenKindCertificate, "reg-1")
	if err != ErrPrintTokenInvalid {
		t.Fatalf("expected ErrPrintTokenInvalid for wrong kind, got %v", err)
	}
}

func TestPrintToken_WrongSubjectFails(t *testing.T) {
	svc, _ := newPrintTokenTestService(t)
	ctx := context.Background()

	token, err := svc.MintPrintToken(ctx, PrintTokenKindCertificate, "session-A")
	if err != nil {
		t.Fatalf("MintPrintToken: %v", err)
	}

	err = svc.RedeemPrintToken(ctx, token, PrintTokenKindCertificate, "session-B")
	if err != ErrPrintTokenInvalid {
		t.Fatalf("expected ErrPrintTokenInvalid for wrong subject, got %v", err)
	}
}

func TestPrintToken_ExpiredFails(t *testing.T) {
	svc, mr := newPrintTokenTestService(t)
	ctx := context.Background()

	token, err := svc.MintPrintToken(ctx, PrintTokenKindCertificate, "session-1")
	if err != nil {
		t.Fatalf("MintPrintToken: %v", err)
	}

	mr.FastForward(61 * time.Second)

	err = svc.RedeemPrintToken(ctx, token, PrintTokenKindCertificate, "session-1")
	if err != ErrPrintTokenInvalid {
		t.Fatalf("expected ErrPrintTokenInvalid for expired token, got %v", err)
	}
}

func TestPrintToken_ConcurrentRedemption_ExactlyOneSucceeds(t *testing.T) {
	svc, _ := newPrintTokenTestService(t)
	ctx := context.Background()

	token, err := svc.MintPrintToken(ctx, PrintTokenKindCertificate, "session-1")
	if err != nil {
		t.Fatalf("MintPrintToken: %v", err)
	}

	const n = 200
	var successes int64
	var wg sync.WaitGroup
	start := make(chan struct{})

	for i := 0; i < n; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			if err := svc.RedeemPrintToken(ctx, token, PrintTokenKindCertificate, "session-1"); err == nil {
				atomic.AddInt64(&successes, 1)
			}
		}()
	}
	close(start)
	wg.Wait()

	if successes != 1 {
		t.Fatalf("expected exactly 1 successful redemption out of %d concurrent attempts, got %d", n, successes)
	}
}
