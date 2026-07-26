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
	mk("future", &tomorrow, nil)  // not yet available
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
	t.Helper()
	ctx := context.Background()

	e := model.Exam{Title: title + " " + uuid.NewString()[:8], ResultConfig: "hidden"}
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
