package worker

import (
	"context"
	"encoding/json"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"

	"akademi-bimbel/internal/model"
	"akademi-bimbel/internal/repository"
)

// OrderPaidPayload unmarshals from the outbox event payload written by service.OrderPaidPayload
type OrderPaidPayload struct {
	OrderID uuid.UUID       `json:"order_id"`
	Items   []OrderItemMini `json:"items"`
}

// OrderItemMini contains minimal item info for access provisioning
type OrderItemMini struct {
	ProductID   uuid.UUID `json:"product_id"`
	ProductType string    `json:"product_type"`
}

// outboxRepository interface defines the methods needed from the repository
type outboxRepository interface {
	ClaimOutboxEvents(context.Context, int) ([]model.OutboxEvent, error)
	MarkOutboxProcessed(context.Context, pgx.Tx, int64) error
	GetOrderByIDForUpdateTx(context.Context, pgx.Tx, uuid.UUID) (model.Order, error)
	SetOrderStatus(context.Context, pgx.Tx, uuid.UUID, string, string) error
	CreateCourseSession(context.Context, pgx.Tx, model.CourseSession) error
	GetCoursesByProductID(context.Context, uuid.UUID) ([]model.Course, error)
	BeginTx(context.Context) (pgx.Tx, error)
	GetExpiredPaymentOrders(context.Context, int) ([]uuid.UUID, error)
	ExpirePaymentOrderTx(context.Context, pgx.Tx, uuid.UUID) (bool, error)
	GetExamsByProductID(context.Context, uuid.UUID) ([]model.Exam, error)
	CreateExamRegistration(context.Context, pgx.Tx, model.ExamRegistration) error
	StampOrderItemFulfilledAt(context.Context, pgx.Tx, uuid.UUID, uuid.UUID) error
	GetOrderParticipants(context.Context, uuid.UUID) ([]uuid.UUID, error)
	GetOrderDigitalSnapshot(context.Context, uuid.UUID) (model.OrderDigitalSnapshot, error)
}

type Worker struct {
	pool                     *pgxpool.Pool
	rdb                      *redis.Client
	repo                     outboxRepository
	interval                 time.Duration
	sweeperInterval          time.Duration
	announcementPollInterval time.Duration
	dispatcher               announcementDispatcher
	jobRepo                  jobRepository
	objectStore              objectStore
	svc                      bulkProcessor
	expirySvc                expiryProcessor
	jobPollInterval          time.Duration
	privateBucket            string
	certGen                  certificateGenerator
}

func New(pool *pgxpool.Pool, rdb *redis.Client, repo outboxRepository, interval, sweeperInterval, announcementPollInterval time.Duration, dispatcher announcementDispatcher, jobRepo jobRepository, objectStore objectStore, svc bulkProcessor, jobPollInterval time.Duration, privateBucket string, certGen certificateGenerator) *Worker {
	return &Worker{
		pool:                     pool,
		rdb:                      rdb,
		repo:                     repo,
		interval:                 interval,
		sweeperInterval:          sweeperInterval,
		announcementPollInterval: announcementPollInterval,
		dispatcher:               dispatcher,
		jobRepo:                  jobRepo,
		objectStore:              objectStore,
		svc:                      svc,
		expirySvc:                resolveExpiryProcessor(svc),
		jobPollInterval:          jobPollInterval,
		privateBucket:            privateBucket,
		certGen:                  certGen,
	}
}

// Run polls the transactional outbox, runs the stale-payment sweeper, dispatches
// due announcements, and polls the job table until ctx is cancelled.
func (w *Worker) Run(ctx context.Context) {
	outboxTicker := time.NewTicker(w.interval)
	defer outboxTicker.Stop()

	sweeperTicker := time.NewTicker(w.sweeperInterval)
	defer sweeperTicker.Stop()

	announcementTicker := time.NewTicker(w.announcementPollInterval)
	defer announcementTicker.Stop()

	jobTicker := time.NewTicker(w.jobPollInterval)
	defer jobTicker.Stop()

	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case <-sweeperTicker.C:
				w.sweepStalePayments(ctx)
			}
		}
	}()

	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case <-announcementTicker.C:
				w.pollAnnouncements(ctx)
			}
		}
	}()

	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case <-jobTicker.C:
				w.pollJobs(ctx)
			}
		}
	}()

	go w.pollActiveExamSessions(ctx)
	go w.pollExamExpiryLoop(ctx)

	for {
		select {
		case <-ctx.Done():
			return
		case <-outboxTicker.C:
			w.pollOutbox(ctx)
		}
	}
}

func (w *Worker) pollOutbox(ctx context.Context) {
	events, err := w.repo.ClaimOutboxEvents(ctx, 10)
	if err != nil {
		slog.Error("claim outbox events", "err", err)
		return
	}

	for _, event := range events {
		slog.Info("outbox event claimed", "event_id", event.ID, "type", event.EventType)
		switch event.EventType {
		case "OrderPaid":
			w.handleOrderPaid(ctx, event)
		case "CertificateNeeded":
			w.handleCertificateNeeded(ctx, event)
		default:
			slog.Warn("unknown event type", "type", event.EventType)
		}
	}
}

func (w *Worker) handleOrderPaid(ctx context.Context, event model.OutboxEvent) {
	var payload OrderPaidPayload
	if err := json.Unmarshal(event.Payload, &payload); err != nil {
		slog.Error("unmarshal OrderPaid payload", "event_id", event.ID, "err", err)
		return
	}

	tx, err := w.repo.BeginTx(ctx)
	if err != nil {
		slog.Error("begin tx", "err", err)
		return
	}
	defer tx.Rollback(ctx)

	// The order is read under its row lock inside the fulfilment transaction,
	// so a refund or expiry that commits first is seen here and one that comes
	// later waits for this commit and then revokes what it granted.
	order, err := w.repo.GetOrderByIDForUpdateTx(ctx, tx, payload.OrderID)
	if err != nil {
		slog.Error("get order by id", "order_id", payload.OrderID, "err", err)
		return
	}
	if order.ID == uuid.Nil {
		slog.Error("order for OrderPaid event not found", "order_id", payload.OrderID, "event_id", event.ID)
		return
	}
	if order.Status != "paid" {
		// A refunded, expired or cancelled order must not be granted access, and
		// a fulfilled one already was; either way the event is spent.
		slog.Warn("OrderPaid event skipped: order is not awaiting fulfilment",
			"order_id", payload.OrderID, "event_id", event.ID, "status", order.Status)
		if err := w.repo.MarkOutboxProcessed(ctx, tx, event.ID); err != nil {
			slog.Error("mark outbox processed", "event_id", event.ID, "err", err)
			return
		}
		if err := tx.Commit(ctx); err != nil {
			slog.Error("commit tx", "err", err)
		}
		return
	}

	snapshot, err := w.repo.GetOrderDigitalSnapshot(ctx, payload.OrderID)
	if err != nil {
		slog.Error("get order digital snapshot", "order_id", payload.OrderID, "err", err)
		return
	}

	// Provision access for each item; track whether any physical item requires shipping.
	hasPhysicalItem := false
	for _, item := range payload.Items {
		switch item.ProductType {
		case "course":
			courseIDs, err := w.digitalTargetIDs(ctx, snapshot, item, model.DigitalTargetCourse)
			if err != nil {
				slog.Error("resolve course targets", "order_id", payload.OrderID, "product_id", item.ProductID, "err", err)
				return
			}
			if len(courseIDs) == 0 {
				slog.Warn("no courses linked to product, skipping", "order_id", payload.OrderID, "product_id", item.ProductID)
				continue
			}
			for _, courseID := range courseIDs {
				session := model.CourseSession{
					StudentID:  order.StudentID,
					CourseID:   courseID,
					OrderID:    &payload.OrderID,
					Status:     "active",
					Source:     "order",
					EnrolledAt: time.Now(),
				}
				if err := w.repo.CreateCourseSession(ctx, tx, session); err != nil {
					slog.Error("create course session", "order_id", payload.OrderID, "course_id", courseID, "err", err)
					return
				}
			}
		case "exam":
			examIDs, err := w.digitalTargetIDs(ctx, snapshot, item, model.DigitalTargetExam)
			if err != nil {
				slog.Error("resolve exam targets", "order_id", payload.OrderID, "product_id", item.ProductID, "err", err)
				return
			}
			if len(examIDs) == 0 {
				slog.Warn("no exams linked to product, skipping", "order_id", payload.OrderID, "product_id", item.ProductID)
				continue
			}

			// Fan-out: if order_participant rows exist, register each
			// participant instead of the buyer (order.StudentID).
			registerIDs := []uuid.UUID{order.StudentID}
			participants, err := w.repo.GetOrderParticipants(ctx, payload.OrderID)
			if err != nil {
				slog.Error("get order participants", "order_id", payload.OrderID, "err", err)
				return
			}
			if len(participants) > 0 {
				registerIDs = participants
			}
			for _, examID := range examIDs {
				for _, sid := range registerIDs {
					if err := w.repo.CreateExamRegistration(ctx, tx, model.ExamRegistration{
						StudentID: sid,
						ExamID:    examID,
						Token:     repository.GenerateExamToken(),
						Status:    "registered",
					}); err != nil {
						slog.Error("create exam registration", "order_id", payload.OrderID, "exam_id", examID, "err", err)
						return
					}
				}
			}
			if err := w.repo.StampOrderItemFulfilledAt(ctx, tx, payload.OrderID, item.ProductID); err != nil {
				slog.Error("stamp order_item fulfilled_at", "order_id", payload.OrderID, "product_id", item.ProductID, "err", err)
				return
			}
		case "book", "merchandise", "medal":
			// Physical-type classification widened in place (pre-existing chokepoint);
			// Merchandise and medals ship like books, so they must gate auto-completion.
			hasPhysicalItem = true
		default:
			slog.Warn("unknown product type in order", "product_type", item.ProductType, "product_id", item.ProductID)
		}
	}

	// Digital-only orders complete immediately; physical orders need admin to ship.
	nextStatus := "completed"
	if hasPhysicalItem {
		nextStatus = "processing"
	}
	if err := w.repo.SetOrderStatus(ctx, tx, payload.OrderID, nextStatus, ""); err != nil {
		slog.Error("set order status", "order_id", payload.OrderID, "err", err)
		return
	}

	// Mark outbox event processed
	if err := w.repo.MarkOutboxProcessed(ctx, tx, event.ID); err != nil {
		slog.Error("mark outbox processed", "event_id", event.ID, "err", err)
		return
	}

	if err := tx.Commit(ctx); err != nil {
		slog.Error("commit tx", "err", err)
		return
	}

	slog.Info("order paid processed", "order_id", payload.OrderID, "event_id", event.ID)
}

// digitalTargetIDs returns the courses or exams an order line grants: the
// targets checkout reserved when the order has a snapshot, otherwise the
// product's live links (orders placed before snapshots existed).
func (w *Worker) digitalTargetIDs(ctx context.Context, snapshot model.OrderDigitalSnapshot, item OrderItemMini, kind string) ([]uuid.UUID, error) {
	var ids []uuid.UUID
	if snapshot.Taken {
		for _, t := range snapshot.Targets {
			if t.ProductID == item.ProductID && t.Kind == kind {
				ids = append(ids, t.TargetID)
			}
		}
		return ids, nil
	}
	if kind == model.DigitalTargetCourse {
		courses, err := w.repo.GetCoursesByProductID(ctx, item.ProductID)
		if err != nil {
			return nil, err
		}
		for _, c := range courses {
			ids = append(ids, c.ID)
		}
		return ids, nil
	}
	exams, err := w.repo.GetExamsByProductID(ctx, item.ProductID)
	if err != nil {
		return nil, err
	}
	for _, e := range exams {
		ids = append(ids, e.ID)
	}
	return ids, nil
}

func (w *Worker) sweepStalePayments(ctx context.Context) {
	orderIDs, err := w.repo.GetExpiredPaymentOrders(ctx, 50)
	if err != nil {
		slog.Error("get expired payment orders", "err", err)
		return
	}

	for _, orderID := range orderIDs {
		expired, err := w.expirePaymentOrder(ctx, orderID)
		if err != nil {
			slog.Error("expire payment order", "order_id", orderID, "err", err)
			continue
		}
		if expired {
			slog.Info("stale payment expired", "order_id", orderID)
		}
	}
}

// expirePaymentOrder leaves status, deadline, stock and claims to one guarded
// repository transition, so a webhook or refund that wins the row lock first
// turns this into a no-op instead of a second restore.
func (w *Worker) expirePaymentOrder(ctx context.Context, orderID uuid.UUID) (bool, error) {
	tx, err := w.repo.BeginTx(ctx)
	if err != nil {
		return false, err
	}
	defer tx.Rollback(ctx)

	expired, err := w.repo.ExpirePaymentOrderTx(ctx, tx, orderID)
	if err != nil || !expired {
		return false, err
	}
	if err := tx.Commit(ctx); err != nil {
		return false, err
	}
	return true, nil
}
