package worker

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"akademi-bimbel/internal/model"
)

func snapshotWorkerRepo(orderID, studentID uuid.UUID, items []OrderItemMini, snapshot model.OrderDigitalSnapshot) *mockRepository {
	return &mockRepository{
		claimOutboxEventsFn: func(ctx context.Context, limit int) ([]model.OutboxEvent, error) {
			payload, _ := json.Marshal(OrderPaidPayload{OrderID: orderID, Items: items})
			return []model.OutboxEvent{
				{ID: 1, AggregateID: orderID, EventType: "OrderPaid", Payload: payload, CreatedAt: time.Now().String()},
			}, nil
		},
		getOrderByIDFn: func(ctx context.Context, id uuid.UUID) (model.Order, error) {
			return model.Order{ID: orderID, StudentID: studentID, Status: "paid"}, nil
		},
		getOrderDigitalSnapshotFn: func(ctx context.Context, tx pgx.Tx, id uuid.UUID) (model.OrderDigitalSnapshot, error) {
			return snapshot, nil
		},
		setOrderStatusFn:      func(ctx context.Context, tx pgx.Tx, id uuid.UUID, status, reason string) error { return nil },
		markOutboxProcessedFn: func(ctx context.Context, tx pgx.Tx, id int64) error { return nil },
		beginTxFn: func(ctx context.Context) (pgx.Tx, error) {
			return &mockTx{commitFn: func(ctx context.Context) error { return nil }, rollbackFn: func(ctx context.Context) error { return nil }}, nil
		},
	}
}

func TestOrderPaidHandler_SnapshotCourseTargetsSurviveProductLinkEdit(t *testing.T) {
	orderID, studentID, productID := uuid.New(), uuid.New(), uuid.New()
	itemID := uuid.New()
	snapshotCourse, editedCourse := uuid.New(), uuid.New()

	var sessions []model.CourseSession
	repo := snapshotWorkerRepo(orderID, studentID,
		[]OrderItemMini{{ProductID: productID, ProductType: "course"}},
		model.OrderDigitalSnapshot{Taken: true, Targets: []model.DigitalTarget{
			{OrderItemID: itemID, ProductID: productID, Kind: model.DigitalTargetCourse, TargetID: snapshotCourse},
		}},
	)
	repo.getCoursesByProductIDFn = func(ctx context.Context, pid uuid.UUID) ([]model.Course, error) {
		return []model.Course{{ID: editedCourse}}, nil
	}
	repo.createCourseSessionFn = func(ctx context.Context, tx pgx.Tx, s model.CourseSession) error {
		sessions = append(sessions, s)
		return nil
	}

	(&Worker{repo: repo}).pollOutbox(context.Background())

	if len(sessions) != 1 || sessions[0].CourseID != snapshotCourse {
		t.Fatalf("course sessions = %+v, want exactly one for snapshot course %v (link edited to %v)", sessions, snapshotCourse, editedCourse)
	}
}

func TestOrderPaidHandler_SnapshotExamTargetsSurviveProductLinkEdit(t *testing.T) {
	orderID, studentID, productID := uuid.New(), uuid.New(), uuid.New()
	itemID := uuid.New()
	snapshotExam, editedExam := uuid.New(), uuid.New()

	var registered []uuid.UUID
	repo := snapshotWorkerRepo(orderID, studentID,
		[]OrderItemMini{{ProductID: productID, ProductType: "exam"}},
		model.OrderDigitalSnapshot{Taken: true, Targets: []model.DigitalTarget{
			{OrderItemID: itemID, ProductID: productID, Kind: model.DigitalTargetExam, TargetID: snapshotExam},
		}},
	)
	repo.getExamsByProductIDFn = func(ctx context.Context, pid uuid.UUID) ([]model.Exam, error) {
		return []model.Exam{{ID: editedExam}}, nil
	}
	repo.createExamRegistrationFn = func(ctx context.Context, tx pgx.Tx, reg model.ExamRegistration) error {
		if reg.StudentID != studentID {
			t.Errorf("registered student = %v, want buyer %v", reg.StudentID, studentID)
		}
		registered = append(registered, reg.ExamID)
		return nil
	}

	(&Worker{repo: repo}).pollOutbox(context.Background())

	if len(registered) != 1 || registered[0] != snapshotExam {
		t.Fatalf("exam registrations = %v, want exactly snapshot exam %v (link edited to %v)", registered, snapshotExam, editedExam)
	}
}

func TestOrderPaidHandler_SnapshotWithNoTargetsGrantsNothingEvenIfLinksAppearLater(t *testing.T) {
	orderID, productID := uuid.New(), uuid.New()

	sessionCreated := false
	repo := snapshotWorkerRepo(orderID, uuid.New(),
		[]OrderItemMini{{ProductID: productID, ProductType: "course"}},
		model.OrderDigitalSnapshot{Taken: true},
	)
	repo.getCoursesByProductIDFn = func(ctx context.Context, pid uuid.UUID) ([]model.Course, error) {
		return []model.Course{{ID: uuid.New()}}, nil
	}
	repo.createCourseSessionFn = func(ctx context.Context, tx pgx.Tx, s model.CourseSession) error {
		sessionCreated = true
		return nil
	}

	(&Worker{repo: repo}).pollOutbox(context.Background())

	if sessionCreated {
		t.Fatal("order snapshotted with no targets must not grant links added after checkout")
	}
}

func TestOrderPaidHandler_OrderWithoutSnapshotStillFulfilsFromLiveLinks(t *testing.T) {
	orderID, studentID, productID := uuid.New(), uuid.New(), uuid.New()
	liveCourse := uuid.New()

	var sessions []model.CourseSession
	repo := snapshotWorkerRepo(orderID, studentID,
		[]OrderItemMini{{ProductID: productID, ProductType: "course"}},
		model.OrderDigitalSnapshot{},
	)
	repo.getCoursesByProductIDFn = func(ctx context.Context, pid uuid.UUID) ([]model.Course, error) {
		return []model.Course{{ID: liveCourse}}, nil
	}
	repo.createCourseSessionFn = func(ctx context.Context, tx pgx.Tx, s model.CourseSession) error {
		sessions = append(sessions, s)
		return nil
	}

	(&Worker{repo: repo}).pollOutbox(context.Background())

	if len(sessions) != 1 || sessions[0].CourseID != liveCourse {
		t.Fatalf("legacy order sessions = %+v, want one for live course %v", sessions, liveCourse)
	}
}

func TestOrderPaidHandler_ReadsTheSnapshotOnTheFulfilmentTransaction(t *testing.T) {
	orderID, studentID := uuid.New(), uuid.New()
	repo := snapshotWorkerRepo(orderID, studentID, nil, model.OrderDigitalSnapshot{})
	fulfilmentTx := &mockTx{commitFn: func(ctx context.Context) error { return nil }, rollbackFn: func(ctx context.Context) error { return nil }}
	repo.beginTxFn = func(ctx context.Context) (pgx.Tx, error) { return fulfilmentTx, nil }
	var readOn pgx.Tx
	repo.getOrderDigitalSnapshotFn = func(ctx context.Context, tx pgx.Tx, id uuid.UUID) (model.OrderDigitalSnapshot, error) {
		readOn = tx
		return model.OrderDigitalSnapshot{}, nil
	}

	(&Worker{repo: repo}).pollOutbox(context.Background())

	if readOn != pgx.Tx(fulfilmentTx) {
		t.Fatalf("snapshot read on tx %v, want the fulfilment transaction %v: a pool read would need a second connection", readOn, fulfilmentTx)
	}
}

func TestOrderPaidHandler_ReadsTheParticipantsOnTheFulfilmentTransaction(t *testing.T) {
	orderID, studentID, productID := uuid.New(), uuid.New(), uuid.New()
	repo := snapshotWorkerRepo(orderID, studentID,
		[]OrderItemMini{{ProductID: productID, ProductType: "exam"}},
		model.OrderDigitalSnapshot{Taken: true, Targets: []model.DigitalTarget{
			{OrderItemID: uuid.New(), ProductID: productID, Kind: model.DigitalTargetExam, TargetID: uuid.New()},
		}},
	)
	fulfilmentTx := &mockTx{commitFn: func(ctx context.Context) error { return nil }, rollbackFn: func(ctx context.Context) error { return nil }}
	repo.beginTxFn = func(ctx context.Context) (pgx.Tx, error) { return fulfilmentTx, nil }

	(&Worker{repo: repo}).pollOutbox(context.Background())

	if repo.participantsReadTx != pgx.Tx(fulfilmentTx) {
		t.Fatalf("participants read on tx %v, want the fulfilment transaction %v: a pool read would need a second connection", repo.participantsReadTx, fulfilmentTx)
	}
}
