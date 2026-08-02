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

func TestPreviewBulkExamOrder_Integration(t *testing.T) {
	svc, repo := newRealDBService(t)
	ctx := context.Background()

	schoolID := createTestSchool(t, svc)
	otherSchoolID := createTestSchool(t, svc)

	examID := createTestExamForBulk(t, repo)
	_ = createTestExamProductForBulk(t, repo, examID, 50000)

	// Create 4 students in the school
	studentIDs := make([]string, 4)
	for i := range studentIDs {
		var id string
		err := repo.Pool().QueryRow(ctx,
			`INSERT INTO users (name, school_id, role, status, username, password_hash, jenjang)
			 VALUES ($1, $2, 'student', 'active', $3, '', 'sma')
			 RETURNING id`,
			"Bulk Student "+uniqueSuffix(), schoolID, "pbulk_"+uniqueSuffix(),
		).Scan(&id)
		if err != nil {
			t.Fatalf("insert student %d: %v", i, err)
		}
		studentIDs[i] = id
	}

	var crossID string
	err := repo.Pool().QueryRow(ctx,
		`INSERT INTO users (name, school_id, role, status, username, password_hash, jenjang)
		 VALUES ($1, $2, 'student', 'active', $3, '', 'sma')
		 RETURNING id`,
		"Cross School", otherSchoolID, "pcross_"+uniqueSuffix(),
	).Scan(&crossID)
	if err != nil {
		t.Fatalf("insert cross-school student: %v", err)
	}

	// Register first 2 students as already registered for this exam
	for i := 0; i < 2; i++ {
		_, err := repo.Pool().Exec(ctx,
			`INSERT INTO exam_registration (student_id, exam_id, token, status)
			 VALUES ($1, $2, $3, 'registered')`,
			studentIDs[i], examID, "TOKEN"+uniqueSuffix()[:5],
		)
		if err != nil {
			t.Fatalf("insert exam_registration %d: %v", i, err)
		}
	}

	t.Run("preview with 2 already registered and 2 new returns net_new_count=2", func(t *testing.T) {
		preview, err := svc.PreviewBulkExamOrder(ctx, schoolID, examID, ParticipantSelector{
			StudentIDs: studentIDs,
		})
		if err != nil {
			t.Fatalf("PreviewBulkExamOrder: %v", err)
		}
		if preview.NetNewCount != 2 {
			t.Errorf("NetNewCount: want 2, got %d", preview.NetNewCount)
		}
		if len(preview.Excluded) != 2 {
			t.Errorf("Excluded: want 2 entries, got %d", len(preview.Excluded))
		}
		for _, ex := range preview.Excluded {
			if ex.Reason != "already_registered" {
				t.Errorf("excluded reason: want already_registered, got %s", ex.Reason)
			}
		}
		if preview.UnitPrice != 50000 {
			t.Errorf("UnitPrice: want 50000, got %f", preview.UnitPrice)
		}
		if preview.Total != 100000 {
			t.Errorf("Total: want 100000, got %f", preview.Total)
		}
	})

	t.Run("preview with all-new students returns net_new_count=2", func(t *testing.T) {
		preview, err := svc.PreviewBulkExamOrder(ctx, schoolID, examID, ParticipantSelector{
			StudentIDs: studentIDs[2:],
		})
		if err != nil {
			t.Fatalf("PreviewBulkExamOrder: %v", err)
		}
		if preview.NetNewCount != 2 {
			t.Errorf("NetNewCount: want 2, got %d", preview.NetNewCount)
		}
		if len(preview.Excluded) != 0 {
			t.Errorf("Excluded: want 0 entries, got %d", len(preview.Excluded))
		}
		if preview.UnitPrice != 50000 {
			t.Errorf("UnitPrice: want 50000, got %f", preview.UnitPrice)
		}
		if preview.Total != 100000 {
			t.Errorf("Total: want 100000, got %f", preview.Total)
		}
	})

	t.Run("preview with cross-school student id fails (FR-BULK-03)", func(t *testing.T) {
		_, err := svc.PreviewBulkExamOrder(ctx, schoolID, examID, ParticipantSelector{
			StudentIDs: []string{studentIDs[0], crossID},
		})
		if err == nil {
			t.Fatal("expected error for cross-school student, got nil")
		}
		if !errors.Is(err, ErrCrossSchoolStudent) {
			t.Errorf("want ErrCrossSchoolStudent, got %v", err)
		}
	})

	t.Run("preview with non-existent exam returns ErrExamNotOrderable (FR-BULK-08)", func(t *testing.T) {
		_, err := svc.PreviewBulkExamOrder(ctx, schoolID, uuid.New().String(), ParticipantSelector{
			StudentIDs: studentIDs[:1],
		})
		if err == nil {
			t.Fatal("expected error for non-orderable exam, got nil")
		}
		if !errors.Is(err, ErrExamNotOrderable) {
			t.Errorf("want ErrExamNotOrderable, got %v", err)
		}
	})

	t.Run("preview with duplicate student ids fails (FR-BULK-03)", func(t *testing.T) {
		_, err := svc.PreviewBulkExamOrder(ctx, schoolID, examID, ParticipantSelector{
			StudentIDs: []string{studentIDs[0], studentIDs[1], studentIDs[0]},
		})
		if err == nil {
			t.Fatal("expected error for duplicate student id, got nil")
		}
		if !errors.Is(err, ErrDuplicateParticipant) {
			t.Errorf("want ErrDuplicateParticipant, got %v", err)
		}
	})

	t.Run("preview with draft product returns ErrExamNotOrderable", func(t *testing.T) {
		draftExamID := createTestExamForBulk(t, repo)
		draftProductID := createTestExamProductForBulk(t, repo, draftExamID, 50000)

		_, err := repo.Pool().Exec(ctx,
			`UPDATE product SET status = 'draft' WHERE id = $1`,
			draftProductID,
		)
		if err != nil {
			t.Fatalf("update product to draft: %v", err)
		}

		_, err = svc.PreviewBulkExamOrder(ctx, schoolID, draftExamID, ParticipantSelector{
			StudentIDs: studentIDs[:1],
		})
		if err == nil {
			t.Fatal("expected error for draft product, got nil")
		}
		if !errors.Is(err, ErrExamNotOrderable) {
			t.Errorf("want ErrExamNotOrderable for draft product, got %v", err)
		}
	})
}

func TestCreateBulkExamOrder_Integration(t *testing.T) {
	svc, repo := newRealDBService(t)
	ctx := context.Background()

	schoolID := createTestSchool(t, svc)
	otherSchoolID := createTestSchool(t, svc)

	examID := createTestExamForBulk(t, repo)
	_ = createTestExamProductForBulk(t, repo, examID, 75000)

	var adminID string
	err := repo.Pool().QueryRow(ctx,
		`INSERT INTO users (name, role, status, username, password_hash, jenjang)
		 VALUES ($1, 'admin_school', 'active', $2, '', 'sma')
		 RETURNING id`,
		"Bulk Admin", "pbulkadmin_"+uniqueSuffix(),
	).Scan(&adminID)
	if err != nil {
		t.Fatalf("insert admin: %v", err)
	}

	studentIDs := make([]string, 3)
	for i := range studentIDs {
		var id string
		err := repo.Pool().QueryRow(ctx,
			`INSERT INTO users (name, school_id, role, status, username, password_hash, jenjang)
			 VALUES ($1, $2, 'student', 'active', $3, '', 'sma')
			 RETURNING id`,
			"Create Test Student "+uniqueSuffix(), schoolID, "pcreate_"+uniqueSuffix(),
		).Scan(&id)
		if err != nil {
			t.Fatalf("insert student %d: %v", i, err)
		}
		studentIDs[i] = id
	}

	// Register student 0 as already registered
	_, err = repo.Pool().Exec(ctx,
		`INSERT INTO exam_registration (student_id, exam_id, token, status)
		 VALUES ($1, $2, $3, 'registered')`,
		studentIDs[0], examID, "TOKEN"+uniqueSuffix()[:5],
	)
	if err != nil {
		t.Fatalf("insert exam_registration: %v", err)
	}

	var crossID string
	err = repo.Pool().QueryRow(ctx,
		`INSERT INTO users (name, school_id, role, status, username, password_hash, jenjang)
		 VALUES ($1, $2, 'student', 'active', $3, '', 'sma')
		 RETURNING id`,
		"Cross School", otherSchoolID, "pcreatecross_"+uniqueSuffix(),
	).Scan(&crossID)
	if err != nil {
		t.Fatalf("insert cross-school student: %v", err)
	}

	t.Run("create order with 2 net-new succeeds", func(t *testing.T) {
		order, err := svc.CreateBulkExamOrder(ctx, adminID, schoolID, examID, ParticipantSelector{
			StudentIDs: studentIDs,
		})
		if err != nil {
			t.Fatalf("CreateBulkExamOrder: %v", err)
		}

		if order.ID.String() == "" {
			t.Fatal("expected non-empty order ID")
		}
		if order.Status != "cart" {
			t.Errorf("Status: want cart, got %s", order.Status)
		}
		if order.StudentID.String() != adminID {
			t.Errorf("StudentID: want %s, got %s", adminID, order.StudentID.String())
		}
		if order.Subtotal != 150000 {
			t.Errorf("Subtotal: want 150000, got %f", order.Subtotal)
		}
		if order.Total != 150000 {
			t.Errorf("Total: want 150000, got %f", order.Total)
		}
		if len(order.Items) != 1 {
			t.Fatalf("Items: want 1 item, got %d", len(order.Items))
		}
		if order.Items[0].ProductType != "exam" {
			t.Errorf("ProductType: want exam, got %s", order.Items[0].ProductType)
		}
		if order.Items[0].Qty != 2 {
			t.Errorf("Qty: want 2, got %d", order.Items[0].Qty)
		}
		if order.Items[0].UnitPrice != 75000 {
			t.Errorf("UnitPrice: want 75000, got %f", order.Items[0].UnitPrice)
		}

		participants, err := repo.GetOrderParticipants(ctx, order.ID)
		if err != nil {
			t.Fatalf("GetOrderParticipants: %v", err)
		}
		if len(participants) != 2 {
			t.Errorf("order_participant count: want 2, got %d", len(participants))
		}
		wantIDs := map[string]bool{studentIDs[1]: true, studentIDs[2]: true}
		for _, p := range participants {
			if !wantIDs[p.String()] {
				t.Errorf("unexpected participant: %s", p.String())
			}
		}
	})

	t.Run("create order with cross-school student id fails (FR-BULK-03)", func(t *testing.T) {
		_, err := svc.CreateBulkExamOrder(ctx, adminID, schoolID, examID, ParticipantSelector{
			StudentIDs: []string{studentIDs[1], crossID},
		})
		if err == nil {
			t.Fatal("expected error for cross-school student, got nil")
		}
		if !errors.Is(err, ErrCrossSchoolStudent) {
			t.Errorf("want ErrCrossSchoolStudent, got %v", err)
		}
	})

	t.Run("create order with zero net-new participants rejected (FR-BULK-05)", func(t *testing.T) {
		_, err := svc.CreateBulkExamOrder(ctx, adminID, schoolID, examID, ParticipantSelector{
			StudentIDs: studentIDs[:1],
		})
		if err == nil {
			t.Fatal("expected error for zero net-new, got nil")
		}
		if !errors.Is(err, ErrZeroNetNewParticipants) {
			t.Errorf("want ErrZeroNetNewParticipants, got %v", err)
		}
	})

	t.Run("create order with duplicate student ids fails (FR-BULK-03)", func(t *testing.T) {
		_, err := svc.CreateBulkExamOrder(ctx, adminID, schoolID, examID, ParticipantSelector{
			StudentIDs: []string{studentIDs[1], studentIDs[2], studentIDs[1]},
		})
		if err == nil {
			t.Fatal("expected error for duplicate student id, got nil")
		}
		if !errors.Is(err, ErrDuplicateParticipant) {
			t.Errorf("want ErrDuplicateParticipant, got %v", err)
		}
	})

	t.Run("create order with draft product returns ErrExamNotOrderable", func(t *testing.T) {
		draftExamID := createTestExamForBulk(t, repo)
		draftProductID := createTestExamProductForBulk(t, repo, draftExamID, 75000)

		_, err := repo.Pool().Exec(ctx,
			`UPDATE product SET status = 'draft' WHERE id = $1`,
			draftProductID,
		)
		if err != nil {
			t.Fatalf("update product to draft: %v", err)
		}

		_, err = svc.CreateBulkExamOrder(ctx, adminID, schoolID, draftExamID, ParticipantSelector{
			StudentIDs: studentIDs[1:],
		})
		if err == nil {
			t.Fatal("expected error for draft product, got nil")
		}
		if !errors.Is(err, ErrExamNotOrderable) {
			t.Errorf("want ErrExamNotOrderable for draft product, got %v", err)
		}
	})
}

func TestCheckoutBulkOrder_ReusesExistingCheckout_Integration(t *testing.T) {
	svcBase, repo := newRealDBService(t)

	// Create a service with redis so checkout idempotency works.
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("miniredis: %v", err)
	}
	t.Cleanup(mr.Close)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	svc := NewWithStore(repo, repo, rdb, nil, &NoopOTPProvider{}, &NoopEmailProvider{}, &NoopPaymentClient{}, &NoopLogisticsClient{}, nil, nil, nil)

	ctx := context.Background()

	schoolID := createTestSchool(t, svcBase)
	examID := createTestExamForBulk(t, repo)
	_ = createTestExamProductForBulk(t, repo, examID, 50000)

	var adminID string
	err = repo.Pool().QueryRow(ctx,
		`INSERT INTO users (name, role, status, username, password_hash, jenjang)
		 VALUES ($1, 'admin_school', 'active', $2, '', 'sma')
		 RETURNING id`,
		"Checkout Admin", "pchkadm_"+uniqueSuffix(),
	).Scan(&adminID)
	if err != nil {
		t.Fatalf("insert admin: %v", err)
	}

	var studentID string
	err = repo.Pool().QueryRow(ctx,
		`INSERT INTO users (name, school_id, role, status, username, password_hash, jenjang)
		 VALUES ($1, $2, 'student', 'active', $3, '', 'sma')
		 RETURNING id`,
		"Checkout Student", schoolID, "pchkstu_"+uniqueSuffix(),
	).Scan(&studentID)
	if err != nil {
		t.Fatalf("insert student: %v", err)
	}

	order, err := svc.CreateBulkExamOrder(ctx, adminID, schoolID, examID, ParticipantSelector{
		StudentIDs: []string{studentID},
	})
	if err != nil {
		t.Fatalf("CreateBulkExamOrder: %v", err)
	}

	t.Run("checkout returns snap token via existing Checkout", func(t *testing.T) {
		result, err := svc.Checkout(ctx, adminID, order.ID.String(), "test-key-"+uniqueSuffix())
		if err != nil {
			t.Fatalf("Checkout: %v", err)
		}
		if result.GatewayRef == "" {
			t.Error("GatewayRef: want non-empty")
		}
		if result.GatewayRef != "noop-"+order.ID.String() {
			t.Errorf("GatewayRef: want noop-%s, got %s", order.ID.String(), result.GatewayRef)
		}
	})

}

func createTestExamForBulk(t *testing.T, repo *repository.Repository) string {
	t.Helper()
	ctx := context.Background()
	var testID string
	err := repo.Pool().QueryRow(ctx,
		`INSERT INTO test (title, subject, topic, duration_minutes)
		 VALUES ($1, $2, $3, $4) RETURNING id`,
		"Bulk Test "+uuid.New().String(), "General", "General", 60,
	).Scan(&testID)
	if err != nil {
		t.Fatalf("create test: %v", err)
	}

	var examID string
	err = repo.Pool().QueryRow(ctx,
		`INSERT INTO exam (title, status)
		 VALUES ($1, 'draft') RETURNING id`,
		"Bulk Exam "+uuid.New().String(),
	).Scan(&examID)
	if err != nil {
		t.Fatalf("create exam: %v", err)
	}

	_, err = repo.Pool().Exec(ctx,
		`INSERT INTO exam_test (exam_id, test_id, sort_order) VALUES ($1, $2, 0)`,
		examID, testID,
	)
	if err != nil {
		t.Fatalf("link test to exam: %v", err)
	}
	return examID
}

func createTestExamProductForBulk(t *testing.T, repo *repository.Repository, examID string, price int64) string {
	t.Helper()
	ctx := context.Background()
	var productID string
	err := repo.Pool().QueryRow(ctx,
		`INSERT INTO product (type, name, price, stock, status)
		 VALUES ('exam', $1, $2, 0, 'published') RETURNING id`,
		"Bulk Exam Product "+uuid.New().String(), price,
	).Scan(&productID)
	if err != nil {
		t.Fatalf("create product: %v", err)
	}

	_, err = repo.Pool().Exec(ctx,
		`INSERT INTO product_exam (product_id, exam_id) VALUES ($1, $2)`,
		productID, examID,
	)
	if err != nil {
		t.Fatalf("link product to exam: %v", err)
	}
	return productID
}
