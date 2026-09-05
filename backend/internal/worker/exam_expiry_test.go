package worker

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"akademi-bimbel/internal/model"
	"akademi-bimbel/internal/service"
)

func TestPollExamExpiryUsesFixedPageAndLogsCandidateFailures(t *testing.T) {
	ctx := context.Background()
	failedSession := uuid.New()
	okSession := uuid.New()
	var gotPageSize int
	expiry := &fakeExpiryProcessor{
		expireFn: func(ctx context.Context, now time.Time, pageSize int) ([]service.ExamExpiryResult, error) {
			gotPageSize = pageSize
			return []service.ExamExpiryResult{
				{
					Candidate: model.ExamExpiryCandidate{SessionID: failedSession},
					Err:       errors.New("promotion failed"),
				},
				{
					Candidate: model.ExamExpiryCandidate{SessionID: okSession},
					Processed: true,
				},
			}, nil
		},
	}
	var logs bytes.Buffer
	prev := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&logs, nil)))
	t.Cleanup(func() { slog.SetDefault(prev) })

	w := &Worker{expirySvc: expiry}
	w.pollExamExpiry(ctx)

	if gotPageSize != examExpiryPageSize {
		t.Fatalf("page size = %d, want %d", gotPageSize, examExpiryPageSize)
	}
	gotLog := logs.String()
	if !strings.Contains(gotLog, "expire exam session candidate") || !strings.Contains(gotLog, failedSession.String()) {
		t.Fatalf("failure log = %q, want failed session id", gotLog)
	}
}

func TestExamExpiryPollInterval(t *testing.T) {
	if examExpiryPollInterval != 30*time.Minute {
		t.Fatalf("expiry poll interval = %s, want 30m", examExpiryPollInterval)
	}
}

func TestPollExamExpiryBoundsSweepDuration(t *testing.T) {
	expiry := &fakeExpiryProcessor{
		expireFn: func(ctx context.Context, now time.Time, pageSize int) ([]service.ExamExpiryResult, error) {
			deadline, ok := ctx.Deadline()
			if !ok {
				t.Fatal("ExpireExamSessions context has no deadline")
			}
			remaining := time.Until(deadline)
			if remaining <= 0 || remaining > 5*time.Minute {
				t.Fatalf("expiry sweep deadline remaining = %s, want within (0, 5m]", remaining)
			}
			return nil, nil
		},
	}

	w := &Worker{expirySvc: expiry}
	w.pollExamExpiry(context.Background())
}

func TestPollExamExpiryReturnsWhenContextCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	expiry := &fakeExpiryProcessor{
		expireFn: func(ctx context.Context, now time.Time, pageSize int) ([]service.ExamExpiryResult, error) {
			t.Fatal("ExpireExamSessions should not be called after context cancellation")
			return nil, nil
		},
	}

	w := &Worker{expirySvc: expiry}
	w.pollExamExpiry(ctx)
}

type fakeExpiryProcessor struct {
	expireFn func(context.Context, time.Time, int) ([]service.ExamExpiryResult, error)
}

func (f *fakeExpiryProcessor) ExpireExamSessions(ctx context.Context, now time.Time, pageSize int) ([]service.ExamExpiryResult, error) {
	return f.expireFn(ctx, now, pageSize)
}
