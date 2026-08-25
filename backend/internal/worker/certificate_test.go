package worker

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"akademi-bimbel/internal/model"
	"akademi-bimbel/internal/service"
)

type fakeCertGenerator struct {
	calls       []uuid.UUID
	bundleCalls int
	generateFn  func(ctx context.Context, sessionID uuid.UUID) error
}

func (f *fakeCertGenerator) GenerateCertificatePDF(ctx context.Context, sessionID uuid.UUID) error {
	f.calls = append(f.calls, sessionID)
	if f.generateFn != nil {
		return f.generateFn(ctx, sessionID)
	}
	return nil
}

func (f *fakeCertGenerator) GenerateQuestionBundlePDF(ctx context.Context, payload service.QuestionBundleNeededPayload) error {
	f.bundleCalls++
	return nil
}

// TestHandleCertificateNeeded_CallsGenerateAndMarksProcessed proves the
// worker dispatches a "CertificateNeeded" event to GenerateCertificatePDF —
// the only place a certificate is generated — and marks the event processed
// once that call succeeds (even when GenerateCertificatePDF itself no-ops,
// which is its ordinary "not ready yet" outcome).
func TestHandleCertificateNeeded_CallsGenerateAndMarksProcessed(t *testing.T) {
	ctx := context.Background()
	sessionID := uuid.New()
	outboxID := int64(7)

	certGen := &fakeCertGenerator{}
	marked := false

	repo := &mockRepository{
		claimOutboxEventsFn: func(ctx context.Context, limit int) ([]model.OutboxEvent, error) {
			payload, _ := json.Marshal(service.CertificateNeededPayload{SessionID: sessionID})
			return []model.OutboxEvent{
				{ID: outboxID, AggregateID: sessionID, EventType: "CertificateNeeded", Payload: payload, CreatedAt: time.Now().String()},
			}, nil
		},
		markOutboxProcessedFn: func(ctx context.Context, tx pgx.Tx, id int64) error {
			if id != outboxID {
				t.Errorf("MarkOutboxProcessed id = %d, want %d", id, outboxID)
			}
			marked = true
			return nil
		},
		beginTxFn: func(ctx context.Context) (pgx.Tx, error) {
			return &mockTx{commitFn: func(ctx context.Context) error { return nil }, rollbackFn: func(ctx context.Context) error { return nil }}, nil
		},
	}

	w := &Worker{repo: repo, certGen: certGen}
	w.pollOutbox(ctx)

	if len(certGen.calls) != 1 || certGen.calls[0] != sessionID {
		t.Fatalf("GenerateCertificatePDF calls = %v, want exactly one call for session %s", certGen.calls, sessionID)
	}
	if !marked {
		t.Error("expected the outbox event to be marked processed")
	}
}

// TestHandleCertificateNeeded_GenerateFailure_LeavesEventUnprocessed proves a
// generation failure (e.g. Gotenberg down) does not mark the event
// processed — it must be retried on a later poll rather than silently lost.
func TestHandleCertificateNeeded_GenerateFailure_LeavesEventUnprocessed(t *testing.T) {
	ctx := context.Background()
	sessionID := uuid.New()

	certGen := &fakeCertGenerator{generateFn: func(ctx context.Context, sessionID uuid.UUID) error {
		return errors.New("gotenberg unreachable")
	}}
	marked := false

	repo := &mockRepository{
		claimOutboxEventsFn: func(ctx context.Context, limit int) ([]model.OutboxEvent, error) {
			payload, _ := json.Marshal(service.CertificateNeededPayload{SessionID: sessionID})
			return []model.OutboxEvent{
				{ID: 1, AggregateID: sessionID, EventType: "CertificateNeeded", Payload: payload, CreatedAt: time.Now().String()},
			}, nil
		},
		markOutboxProcessedFn: func(ctx context.Context, tx pgx.Tx, id int64) error {
			marked = true
			return nil
		},
		beginTxFn: func(ctx context.Context) (pgx.Tx, error) {
			return &mockTx{commitFn: func(ctx context.Context) error { return nil }, rollbackFn: func(ctx context.Context) error { return nil }}, nil
		},
	}

	w := &Worker{repo: repo, certGen: certGen}
	w.pollOutbox(ctx)

	if marked {
		t.Error("expected the outbox event NOT to be marked processed after a generation failure")
	}
}

func TestHandleQuestionBundleNeeded_InvalidLegacyPayloadIsDiscarded(t *testing.T) {
	ctx := context.Background()
	marked := false
	certGen := &fakeCertGenerator{}
	repo := &mockRepository{
		claimOutboxEventsFn: func(ctx context.Context, limit int) ([]model.OutboxEvent, error) {
			return []model.OutboxEvent{{
				ID: 42, AggregateID: uuid.New(), EventType: "QuestionBundleNeeded",
				Payload: []byte(`{"bundle_id":"11111111-1111-1111-1111-111111111111"}`), CreatedAt: time.Now().String(),
			}}, nil
		},
		markOutboxProcessedFn: func(ctx context.Context, tx pgx.Tx, id int64) error {
			marked = id == 42
			return nil
		},
		beginTxFn: func(ctx context.Context) (pgx.Tx, error) {
			return &mockTx{commitFn: func(ctx context.Context) error { return nil }, rollbackFn: func(ctx context.Context) error { return nil }}, nil
		},
	}

	(&Worker{repo: repo, certGen: certGen}).pollOutbox(ctx)
	if !marked {
		t.Fatal("invalid legacy QuestionBundleNeeded event must be marked processed")
	}
	if certGen.bundleCalls != 0 {
		t.Fatalf("GenerateQuestionBundlePDF calls = %d, want 0 for invalid payload", certGen.bundleCalls)
	}
}
