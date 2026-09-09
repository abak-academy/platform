package repository

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	"akademi-bimbel/internal/model"
)

// TestListProducts_AvailabilityWindow verifies the public catalog (VisibleOnly)
// hides products outside their availability window (P-A), while an admin listing
// (no VisibleOnly) still sees them.
func TestListProducts_AvailabilityWindow(t *testing.T) {
	ctx := context.Background()
	pool := newGradingTestPool(t)
	r := New(pool)

	now := time.Now()
	tomorrow := now.Add(24 * time.Hour)
	yesterday := now.Add(-24 * time.Hour)

	mk := func(name string, from, until *time.Time) {
		t.Helper()
		p := model.Product{
			Type: "book", Name: name, Price: 1000, Stock: 1, Status: "published",
			AvailableFrom: from, AvailableUntil: until,
		}
		if err := r.CreateProduct(ctx, &p); err != nil {
			t.Fatalf("create %s: %v", name, err)
		}
	}
	mk("always", nil, nil)
	mk("future", &tomorrow, nil)   // not yet available
	mk("expired", nil, &yesterday) // window has passed

	names := func(f ProductFilter) map[string]bool {
		t.Helper()
		got, _, err := r.ListProducts(ctx, f)
		if err != nil {
			t.Fatalf("list products: %v", err)
		}
		m := map[string]bool{}
		for _, p := range got {
			m[p.Name] = true
		}
		return m
	}

	public := names(ProductFilter{VisibleOnly: true, Limit: 100})
	if !public["always"] {
		t.Error("always-available product should be in the public catalog")
	}
	if public["future"] {
		t.Error("not-yet-available product must be hidden from the public catalog")
	}
	if public["expired"] {
		t.Error("expired product must be hidden from the public catalog")
	}

	admin := names(ProductFilter{Limit: 100})
	if !admin["always"] || !admin["future"] || !admin["expired"] {
		t.Errorf("admin listing should include all products regardless of window, got %v", admin)
	}
}

// linkedExamProduct creates an exam plus an exam-type product linked to it via
// product_exam, with the given availability window.
func linkedExamProduct(t *testing.T, r *Repository, title string, from, until *time.Time) (uuid.UUID, uuid.UUID) {
	return linkedScheduledExamProduct(t, r, title, nil, nil, from, until)
}

func linkedScheduledExamProduct(t *testing.T, r *Repository, title string, scheduledAt, scheduledEndAt, from, until *time.Time) (uuid.UUID, uuid.UUID) {
	return linkedScheduledExamProductWithTiming(t, r, title, scheduledAt, scheduledEndAt, nil, nil, from, until)
}

func linkedScheduledExamProductWithTiming(t *testing.T, r *Repository, title string, scheduledAt, scheduledEndAt *time.Time, durationMinutes, graceWindowMinutes *int, from, until *time.Time) (uuid.UUID, uuid.UUID) {
	t.Helper()
	ctx := context.Background()

	e := model.Exam{
		Title:              title + " " + uuid.NewString()[:8],
		ResultConfig:       "hidden",
		ScheduledAt:        scheduledAt,
		ScheduledEndAt:     scheduledEndAt,
		DurationMinutes:    durationMinutes,
		GraceWindowMinutes: graceWindowMinutes,
	}
	if err := r.CreateExam(ctx, &e); err != nil {
		t.Fatalf("create exam %s: %v", title, err)
	}

	p := model.Product{
		Type: "exam", Name: title + " Product " + uuid.NewString()[:8],
		Price: 1000, Stock: 0, Status: "published",
		AvailableFrom: from, AvailableUntil: until,
	}
	tx, err := r.BeginTx(ctx)
	if err != nil {
		t.Fatalf("begin tx: %v", err)
	}
	defer tx.Rollback(ctx)
	if err := r.CreateProductWithExams(ctx, tx, &p, []uuid.UUID{e.ID}); err != nil {
		t.Fatalf("create product for %s: %v", title, err)
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatalf("commit: %v", err)
	}

	pID, err := uuid.Parse(p.ID)
	if err != nil {
		t.Fatalf("parse product id: %v", err)
	}
	return e.ID, pID
}

func TestListProducts_VisibleOnlyRespectsExamWindow(t *testing.T) {
	ctx := context.Background()
	pool := newGradingTestPool(t)
	r := New(pool)

	now := time.Now()
	future := now.Add(24 * time.Hour)
	yesterday := now.Add(-24 * time.Hour)
	justStarted := now.Add(-5 * time.Minute)
	duration := 60
	grace := 10

	_, futureProduct := linkedScheduledExamProduct(t, r, "Visible Future Exam", &future, nil, nil, nil)
	_, startedProduct := linkedScheduledExamProductWithTiming(t, r, "Visible Started Exam", &justStarted, nil, &duration, &grace, nil, nil)
	_, expiredProduct := linkedScheduledExamProduct(t, r, "Visible Expired Exam", &yesterday, nil, nil, nil)

	products, _, err := r.ListProducts(ctx, ProductFilter{VisibleOnly: true, Type: "exam", Limit: 100})
	if err != nil {
		t.Fatalf("ListProducts: %v", err)
	}
	seen := map[string]bool{}
	for _, p := range products {
		seen[p.ID] = true
	}

	if !seen[futureProduct.String()] {
		t.Error("future exam product must stay visible in the public catalog")
	}
	if !seen[startedProduct.String()] {
		t.Error("started exam product must stay visible while duration+grace is still open")
	}
	if seen[expiredProduct.String()] {
		t.Error("expired exam product must be hidden from the public catalog")
	}
}

// TestGetProductByExamID_AvailabilityWindow covers the student-facing lookup that
// decides whether an exam can be bought. It shares the window predicate with
// ListProducts but is a separate query, so it needs its own coverage: a leak here
// exposes an exam product outside its selling window.
func TestGetProductByExamID_AvailabilityWindow(t *testing.T) {
	ctx := context.Background()
	pool := newGradingTestPool(t)
	r := New(pool)

	now := time.Now()
	tomorrow := now.Add(24 * time.Hour)
	yesterday := now.Add(-24 * time.Hour)

	openExam, openProduct := linkedExamProduct(t, r, "Open Exam", nil, nil)
	futureExam, _ := linkedExamProduct(t, r, "Future Exam", &tomorrow, nil)
	expiredExam, _ := linkedExamProduct(t, r, "Expired Exam", nil, &yesterday)

	got, err := r.GetProductByExamID(ctx, openExam)
	if err != nil {
		t.Fatalf("an in-window exam product must be found: %v", err)
	}
	if got.ID != openProduct.String() {
		t.Errorf("got product %s, want %s", got.ID, openProduct)
	}

	if _, err := r.GetProductByExamID(ctx, futureExam); !errors.Is(err, ErrNotFound) {
		t.Errorf("a not-yet-available exam product must not be returned, got err=%v", err)
	}
	if _, err := r.GetProductByExamID(ctx, expiredExam); !errors.Is(err, ErrNotFound) {
		t.Errorf("an expired exam product must not be returned, got err=%v", err)
	}
}

func TestGetProductByExamID_RejectsPastExamWindow(t *testing.T) {
	ctx := context.Background()
	pool := newGradingTestPool(t)
	r := New(pool)

	now := time.Now()
	future := now.Add(24 * time.Hour)
	oneHourAgo := now.Add(-1 * time.Hour)
	yesterday := now.Add(-24 * time.Hour)
	tomorrow := now.Add(24 * time.Hour)

	futureExam, futureProduct := linkedScheduledExamProduct(t, r, "Future Scheduled Exam", &future, nil, nil, nil)
	pastFixedExam, _ := linkedScheduledExamProduct(t, r, "Past Fixed Exam", &yesterday, nil, nil, nil)
	openWindowExam, openWindowProduct := linkedScheduledExamProduct(t, r, "Open Flexible Exam", &yesterday, &tomorrow, nil, nil)
	closedWindowExam, _ := linkedScheduledExamProduct(t, r, "Closed Flexible Exam", &yesterday, &oneHourAgo, nil, nil)

	got, err := r.GetProductByExamID(ctx, futureExam)
	if err != nil {
		t.Fatalf("a future scheduled exam product must be found: %v", err)
	}
	if got.ID != futureProduct.String() {
		t.Errorf("got product %s, want %s", got.ID, futureProduct)
	}

	got, err = r.GetProductByExamID(ctx, openWindowExam)
	if err != nil {
		t.Fatalf("an exam whose scheduled window is still open must be found: %v", err)
	}
	if got.ID != openWindowProduct.String() {
		t.Errorf("got product %s, want %s", got.ID, openWindowProduct)
	}

	if _, err := r.GetProductByExamID(ctx, pastFixedExam); !errors.Is(err, ErrNotFound) {
		t.Errorf("a fixed-schedule exam after scheduled_at must not be orderable, got err=%v", err)
	}
	if _, err := r.GetProductByExamID(ctx, closedWindowExam); !errors.Is(err, ErrNotFound) {
		t.Errorf("a flexible-window exam after scheduled_end_at must not be orderable, got err=%v", err)
	}
}

func TestGetProductByExamID_AllowsStartedExamUntilDurationAndGraceEnd(t *testing.T) {
	ctx := context.Background()
	pool := newGradingTestPool(t)
	r := New(pool)

	now := time.Now()
	started := now.Add(-5 * time.Minute)
	ended := now.Add(-20 * time.Minute)
	duration := 10
	grace := 15

	openExam, openProduct := linkedScheduledExamProductWithTiming(t, r, "Started Open Exam", &started, nil, &duration, &grace, nil, nil)
	closedExam, _ := linkedScheduledExamProductWithTiming(t, r, "Started Closed Exam", &ended, nil, &duration, nil, nil, nil)

	got, err := r.GetProductByExamID(ctx, openExam)
	if err != nil {
		t.Fatalf("an exam product inside duration+grace must be orderable: %v", err)
	}
	if got.ID != openProduct.String() {
		t.Errorf("got product %s, want %s", got.ID, openProduct)
	}
	if _, err := r.GetProductByExamID(ctx, closedExam); !errors.Is(err, ErrNotFound) {
		t.Errorf("an exam product past duration+grace must not be orderable, got err=%v", err)
	}
}

// TestListExams_HasPublishedProduct_RespectsAvailabilityWindow covers the
// has_published_product flag the exam list renders its buy affordance from —
// a third copy of the window predicate, in a correlated EXISTS.
func TestListExams_HasPublishedProduct_RespectsAvailabilityWindow(t *testing.T) {
	ctx := context.Background()
	pool := newGradingTestPool(t)
	r := New(pool)

	now := time.Now()
	tomorrow := now.Add(24 * time.Hour)
	yesterday := now.Add(-24 * time.Hour)

	openExam, _ := linkedExamProduct(t, r, "Listed Open Exam", nil, nil)
	futureExam, _ := linkedExamProduct(t, r, "Listed Future Exam", &tomorrow, nil)
	expiredExam, _ := linkedExamProduct(t, r, "Listed Expired Exam", nil, &yesterday)

	flags := map[uuid.UUID]bool{}
	cursor := ""
	for {
		exams, next, err := r.ListExams(ctx, ExamFilter{Limit: 100, Cursor: cursor})
		if err != nil {
			t.Fatalf("list exams: %v", err)
		}
		for _, e := range exams {
			flags[e.ID] = e.HasPublishedProduct
		}
		if next == "" || len(exams) == 0 {
			break
		}
		cursor = next
	}

	if !flags[openExam] {
		t.Error("an exam whose product is inside its window must report has_published_product")
	}
	if flags[futureExam] {
		t.Error("an exam whose product is not yet available must not report has_published_product")
	}
	if flags[expiredExam] {
		t.Error("an exam whose product has expired must not report has_published_product")
	}
}

func TestListExams_HasPublishedProduct_RespectsExamWindow(t *testing.T) {
	ctx := context.Background()
	pool := newGradingTestPool(t)
	r := New(pool)

	now := time.Now()
	future := now.Add(24 * time.Hour)
	oneHourAgo := now.Add(-1 * time.Hour)
	yesterday := now.Add(-24 * time.Hour)
	tomorrow := now.Add(24 * time.Hour)

	futureExam, _ := linkedScheduledExamProduct(t, r, "Listed Future Scheduled Exam", &future, nil, nil, nil)
	pastFixedExam, _ := linkedScheduledExamProduct(t, r, "Listed Past Fixed Exam", &yesterday, nil, nil, nil)
	openWindowExam, _ := linkedScheduledExamProduct(t, r, "Listed Open Flexible Exam", &yesterday, &tomorrow, nil, nil)
	closedWindowExam, _ := linkedScheduledExamProduct(t, r, "Listed Closed Flexible Exam", &yesterday, &oneHourAgo, nil, nil)

	flags := map[uuid.UUID]bool{}
	cursor := ""
	for {
		exams, next, err := r.ListExams(ctx, ExamFilter{Limit: 100, Cursor: cursor})
		if err != nil {
			t.Fatalf("list exams: %v", err)
		}
		for _, e := range exams {
			flags[e.ID] = e.HasPublishedProduct
		}
		if next == "" || len(exams) == 0 {
			break
		}
		cursor = next
	}

	if !flags[futureExam] {
		t.Error("a future scheduled exam product must report has_published_product")
	}
	if !flags[openWindowExam] {
		t.Error("an exam whose scheduled window is still open must report has_published_product")
	}
	if flags[pastFixedExam] {
		t.Error("a fixed-schedule exam after scheduled_at must not report has_published_product")
	}
	if flags[closedWindowExam] {
		t.Error("a flexible-window exam after scheduled_end_at must not report has_published_product")
	}
}

func TestListExams_OrdersByCreatedAtNewestFirst_WithCursor(t *testing.T) {
	ctx := context.Background()
	pool := newGradingTestPool(t)
	r := New(pool)

	prefix := "Created Sort " + uuid.NewString()[:8]
	olderExam := uuid.New()
	newerLowID := olderExam
	newerHighID := olderExam
	olderExam[15] = 1
	newerLowID[15] = 2
	newerHighID[15] = 3

	base := time.Now().UTC().Add(-2 * time.Hour).Truncate(time.Microsecond)
	for _, row := range []struct {
		id        uuid.UUID
		title     string
		createdAt time.Time
	}{
		{olderExam, prefix + " Older", base},
		{newerLowID, prefix + " Newer A", base.Add(time.Hour)},
		{newerHighID, prefix + " Newer B", base.Add(time.Hour)},
	} {
		if _, err := pool.Exec(ctx, `INSERT INTO exam (id, title, result_config, created_at) VALUES ($1, $2, 'hidden', $3)`, row.id, row.title, row.createdAt); err != nil {
			t.Fatalf("insert exam %s: %v", row.title, err)
		}
	}

	page1, cursor, err := r.ListExams(ctx, ExamFilter{Q: prefix, Limit: 2})
	if err != nil {
		t.Fatalf("list page 1: %v", err)
	}
	if len(page1) != 2 || page1[0].ID != newerLowID || page1[1].ID != newerHighID {
		t.Fatalf("page 1 should return newest exams first, tie-broken by id ASC; got %+v", page1)
	}
	if cursor == "" {
		t.Fatal("page 1 should return a cursor")
	}

	page2, next, err := r.ListExams(ctx, ExamFilter{Q: prefix, Limit: 2, Cursor: cursor})
	if err != nil {
		t.Fatalf("list page 2: %v", err)
	}
	if len(page2) != 1 || page2[0].ID != olderExam {
		t.Fatalf("page 2 should return older exam after cursor, got %+v want %s", page2, olderExam)
	}
	if next != "" {
		t.Fatalf("page 2 should be the last page, got cursor %q", next)
	}
}
