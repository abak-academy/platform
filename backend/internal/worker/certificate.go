package worker

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"

	"github.com/google/uuid"

	"akademi-bimbel/internal/model"
	"akademi-bimbel/internal/service"
)

// certificateGenerator is satisfied by *service.Service; narrowed so a fake
// can stand in for the worker-dispatch tests without a real DB or Gotenberg
// (mirrors studentBulkProcessor's split in student_bulk.go).
type certificateGenerator interface {
	GenerateCertificatePDF(ctx context.Context, sessionID uuid.UUID) error
	GenerateQuestionBundlePDF(ctx context.Context, payload service.QuestionBundleNeededPayload) error
}

// handleCertificateNeeded is the "CertificateNeeded" outbox event handler
// (async redesign 2026-08-02): GenerateCertificatePDF is the only place a
// certificate is ever generated, and it is idempotent — a no-op when the
// session isn't ready yet — so the event is marked processed only after a
// successful (possibly no-op) call. Unlike handleOrderPaid, the business
// effect (GenerateCertificatePDF's own render/upload/persist) is not inside
// the same transaction as MarkOutboxProcessed: a Gotenberg round trip has no
// business holding a Postgres transaction open. If the process crashes
// between the two, the event is reclaimed and reprocessed on the next poll —
// safe because a session whose certificate is already current makes
// GenerateCertificatePDF a no-op.
func (w *Worker) handleCertificateNeeded(ctx context.Context, event model.OutboxEvent) {
	var payload service.CertificateNeededPayload
	if err := json.Unmarshal(event.Payload, &payload); err != nil {
		slog.Error("unmarshal CertificateNeeded payload", "event_id", event.ID, "err", err)
		return
	}

	if err := w.certGen.GenerateCertificatePDF(ctx, payload.SessionID); err != nil {
		slog.Error("generate certificate pdf", "event_id", event.ID, "session_id", payload.SessionID, "err", err)
		return
	}

	tx, err := w.repo.BeginTx(ctx)
	if err != nil {
		slog.Error("begin tx", "event_id", event.ID, "err", err)
		return
	}
	defer tx.Rollback(ctx)

	if err := w.repo.MarkOutboxProcessed(ctx, tx, event.ID); err != nil {
		slog.Error("mark outbox processed", "event_id", event.ID, "err", err)
		return
	}
	if err := tx.Commit(ctx); err != nil {
		slog.Error("commit tx", "event_id", event.ID, "err", err)
		return
	}
}

// handleQuestionBundleNeeded renders the async admin question-bundle PDF and
// marks the event processed after the bundle reaches a terminal persisted state.
func (w *Worker) handleQuestionBundleNeeded(ctx context.Context, event model.OutboxEvent) {
	var payload service.QuestionBundleNeededPayload
	if err := json.Unmarshal(event.Payload, &payload); err != nil {
		slog.Error("unmarshal QuestionBundleNeeded payload", "event_id", event.ID, "err", err)
		w.markQuestionBundleEventProcessed(ctx, event.ID)
		return
	}
	if err := service.ValidateQuestionBundlePayload(payload); err != nil {
		slog.Error("discard invalid QuestionBundleNeeded payload", "event_id", event.ID, "err", err)
		w.markQuestionBundleEventProcessed(ctx, event.ID)
		return
	}

	if err := w.certGen.GenerateQuestionBundlePDF(ctx, payload); err != nil {
		slog.Error("generate question bundle pdf", "event_id", event.ID, "test_id", payload.TestID, "variant", payload.Variant, "err", err)
		if errors.Is(err, service.ErrStorageNotConfigured) || errors.Is(err, service.ErrPDFRendererNotConfigured) {
			w.markQuestionBundleEventProcessed(ctx, event.ID)
		}
		return
	}

	w.markQuestionBundleEventProcessed(ctx, event.ID)
}

func (w *Worker) markQuestionBundleEventProcessed(ctx context.Context, eventID int64) {
	tx, err := w.repo.BeginTx(ctx)
	if err != nil {
		slog.Error("begin tx", "event_id", eventID, "err", err)
		return
	}
	defer tx.Rollback(ctx)

	if err := w.repo.MarkOutboxProcessed(ctx, tx, eventID); err != nil {
		slog.Error("mark outbox processed", "event_id", eventID, "err", err)
		return
	}
	if err := tx.Commit(ctx); err != nil {
		slog.Error("commit tx", "event_id", eventID, "err", err)
	}
}
