package worker

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"strings"
	"testing"
	"time"

	"akademi-bimbel/internal/service"
)

type fakeAffiliateProcessor struct {
	runFn func(ctx context.Context, now time.Time) error
	calls int
}

func (f *fakeAffiliateProcessor) RunAffiliateDaily(ctx context.Context, now time.Time) error {
	f.calls++
	if f.runFn != nil {
		return f.runFn(ctx, now)
	}
	return nil
}

func TestAffiliatePollInterval(t *testing.T) {
	if affiliatePollInterval != 15*time.Minute {
		t.Fatalf("affiliate poll interval = %s, want 15m", affiliatePollInterval)
	}
}

// TestAffiliateProcessorWiring pins that *service.Service stays assignable to
// the worker's affiliate loop interface — the wiring is an interface assertion,
// so breaking the method signature must fail this test.
func TestAffiliateProcessorWiring(t *testing.T) {
	var svc *service.Service
	if _, ok := interface{}(svc).(affiliateProcessor); !ok {
		t.Fatal("*service.Service does not satisfy the worker affiliateProcessor interface")
	}
}

func TestPollAffiliateCallsProcessorAndLogsFailures(t *testing.T) {
	ctx := context.Background()
	boom := errors.New("affiliate boom")
	var gotNow time.Time
	affiliate := &fakeAffiliateProcessor{
		runFn: func(ctx context.Context, now time.Time) error {
			gotNow = now
			return boom
		},
	}
	var logs bytes.Buffer
	prev := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&logs, nil)))
	t.Cleanup(func() { slog.SetDefault(prev) })

	w := &Worker{affiliateSvc: affiliate}
	w.pollAffiliate(ctx)

	if affiliate.calls != 1 {
		t.Fatalf("RunAffiliateDaily calls = %d, want 1", affiliate.calls)
	}
	if gotNow.IsZero() {
		t.Fatal("RunAffiliateDaily received a zero now")
	}
	if got := logs.String(); !strings.Contains(got, "affiliate daily job") || !strings.Contains(got, "affiliate boom") {
		t.Fatalf("failure log = %q, want the affiliate job error", got)
	}
}

func TestPollAffiliateBoundsSweepDuration(t *testing.T) {
	affiliate := &fakeAffiliateProcessor{
		runFn: func(ctx context.Context, now time.Time) error {
			deadline, ok := ctx.Deadline()
			if !ok {
				t.Fatal("RunAffiliateDaily context has no deadline")
			}
			remaining := time.Until(deadline)
			if remaining <= 0 || remaining > affiliateSweepTimeout {
				t.Fatalf("affiliate sweep deadline remaining = %s, want within (0, %s]", remaining, affiliateSweepTimeout)
			}
			return nil
		},
	}

	w := &Worker{affiliateSvc: affiliate}
	w.pollAffiliate(context.Background())
}

func TestPollAffiliateReturnsWhenContextCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	affiliate := &fakeAffiliateProcessor{
		runFn: func(ctx context.Context, now time.Time) error {
			t.Fatal("RunAffiliateDaily should not be called after context cancellation")
			return nil
		},
	}

	w := &Worker{affiliateSvc: affiliate}
	w.pollAffiliate(ctx)
	if affiliate.calls != 0 {
		t.Fatalf("RunAffiliateDaily calls = %d, want 0", affiliate.calls)
	}
}

func TestPollAffiliateNilProcessorIsNoop(t *testing.T) {
	w := &Worker{}
	w.pollAffiliateLoop(context.Background())
	w.pollAffiliate(context.Background())
}
