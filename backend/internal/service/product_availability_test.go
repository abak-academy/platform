package service

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"

	"akademi-bimbel/internal/model"
)

// The availability window is tri-state on update: a field the request omits must
// be preserved, an explicitly null one must clear the window, and a supplied
// value must replace it. The distinction lives in the AvailableFromSet/
// AvailableUntilSet flags, and it is easy to regress into either "every update
// wipes the window" or "the window can never be cleared" — neither of which a
// test that only sets values would notice.
//
// The three product kinds go through three separate update methods, each with
// its own copy of the overlay, so each is covered here.

func availabilityWindow(t *testing.T, svc *Service, id string) (*time.Time, *time.Time) {
	t.Helper()
	got, err := svc.GetProduct(context.Background(), id, RoleSuperAdmin)
	if err != nil {
		t.Fatalf("GetProduct: %v", err)
	}
	return got.AvailableFrom, got.AvailableUntil
}

func TestUpdateProduct_AvailabilityWindowIsTriState(t *testing.T) {
	svc, _ := newRealDBService(t)
	ctx := context.Background()

	from := time.Now().Add(-time.Hour).UTC().Truncate(time.Second)
	until := time.Now().Add(48 * time.Hour).UTC().Truncate(time.Second)

	created, err := svc.CreateProduct(ctx, model.Product{
		Type: "book", Name: "Tri-state Book " + uuid.NewString()[:8],
		Price: 1000, Stock: 5, Status: "published",
		AvailableFrom: &from, AvailableUntil: &until,
	}, RoleSuperAdmin)
	if err != nil {
		t.Fatalf("CreateProduct: %v", err)
	}

	// (1) Absent: an unrelated edit must not disturb the window.
	if _, err := svc.UpdateProduct(ctx, created.ID, model.Product{
		Name: "Tri-state Book renamed", Price: 2000, Stock: 5, Status: "published",
	}, RoleSuperAdmin); err != nil {
		t.Fatalf("UpdateProduct (absent): %v", err)
	}
	gotFrom, gotUntil := availabilityWindow(t, svc, created.ID)
	if gotFrom == nil || !gotFrom.Equal(from) {
		t.Errorf("available_from must be preserved when the field is absent, got %v want %v", gotFrom, from)
	}
	if gotUntil == nil || !gotUntil.Equal(until) {
		t.Errorf("available_until must be preserved when the field is absent, got %v want %v", gotUntil, until)
	}

	// (2) Explicit value: replaces the window.
	newUntil := until.Add(72 * time.Hour)
	if _, err := svc.UpdateProduct(ctx, created.ID, model.Product{
		Name: "Tri-state Book renamed", Price: 2000, Stock: 5, Status: "published",
		AvailableFrom: &from, AvailableFromSet: true,
		AvailableUntil: &newUntil, AvailableUntilSet: true,
	}, RoleSuperAdmin); err != nil {
		t.Fatalf("UpdateProduct (value): %v", err)
	}
	_, gotUntil = availabilityWindow(t, svc, created.ID)
	if gotUntil == nil || !gotUntil.Equal(newUntil) {
		t.Errorf("available_until must take the supplied value, got %v want %v", gotUntil, newUntil)
	}

	// (3) Explicit null: clears the window, so the product becomes always-available.
	if _, err := svc.UpdateProduct(ctx, created.ID, model.Product{
		Name: "Tri-state Book renamed", Price: 2000, Stock: 5, Status: "published",
		AvailableFrom: nil, AvailableFromSet: true,
		AvailableUntil: nil, AvailableUntilSet: true,
	}, RoleSuperAdmin); err != nil {
		t.Fatalf("UpdateProduct (null): %v", err)
	}
	gotFrom, gotUntil = availabilityWindow(t, svc, created.ID)
	if gotFrom != nil || gotUntil != nil {
		t.Errorf("an explicitly null window must be cleared, got from=%v until=%v", gotFrom, gotUntil)
	}
}

func TestUpdateProductWithExams_AvailabilityWindowIsTriState(t *testing.T) {
	svc, _ := newRealDBService(t)
	ctx := context.Background()

	exam, err := svc.CreateExam(ctx, model.Exam{Title: "Tri-state Exam " + uniqueSuffix()})
	if err != nil {
		t.Fatalf("CreateExam: %v", err)
	}

	from := time.Now().Add(-time.Hour).UTC().Truncate(time.Second)
	until := time.Now().Add(48 * time.Hour).UTC().Truncate(time.Second)

	created, err := svc.CreateProductWithExams(ctx, model.Product{
		Type: "exam", Name: "Tri-state Exam Product " + uuid.NewString()[:8],
		Price: 5000, Status: "published",
		AvailableFrom: &from, AvailableUntil: &until,
	}, []string{exam.ID.String()}, RoleSuperAdmin)
	if err != nil {
		t.Fatalf("CreateProductWithExams: %v", err)
	}

	// Absent preserves.
	if _, err := svc.UpdateProductWithExams(ctx, created.ID, model.Product{
		Name: "Tri-state Exam Product renamed", Price: 6000, Status: "published",
	}, []string{exam.ID.String()}, RoleSuperAdmin); err != nil {
		t.Fatalf("UpdateProductWithExams (absent): %v", err)
	}
	gotFrom, gotUntil := availabilityWindow(t, svc, created.ID)
	if gotFrom == nil || !gotFrom.Equal(from) || gotUntil == nil || !gotUntil.Equal(until) {
		t.Errorf("window must survive an update that omits it, got from=%v until=%v", gotFrom, gotUntil)
	}

	// Explicit null clears.
	if _, err := svc.UpdateProductWithExams(ctx, created.ID, model.Product{
		Name: "Tri-state Exam Product renamed", Price: 6000, Status: "published",
		AvailableFrom: nil, AvailableFromSet: true,
		AvailableUntil: nil, AvailableUntilSet: true,
	}, []string{exam.ID.String()}, RoleSuperAdmin); err != nil {
		t.Fatalf("UpdateProductWithExams (null): %v", err)
	}
	gotFrom, gotUntil = availabilityWindow(t, svc, created.ID)
	if gotFrom != nil || gotUntil != nil {
		t.Errorf("an explicitly null window must be cleared, got from=%v until=%v", gotFrom, gotUntil)
	}
}

func TestUpdateProductWithCourses_AvailabilityWindowIsTriState(t *testing.T) {
	svc, repo := newRealDBService(t)
	ctx := context.Background()

	var courseID uuid.UUID
	if err := repo.Pool().QueryRow(ctx,
		`INSERT INTO course (title) VALUES ($1) RETURNING id`,
		"Tri-state Course "+uuid.NewString()[:8],
	).Scan(&courseID); err != nil {
		t.Fatalf("create course: %v", err)
	}

	from := time.Now().Add(-time.Hour).UTC().Truncate(time.Second)
	until := time.Now().Add(48 * time.Hour).UTC().Truncate(time.Second)

	created, err := svc.CreateProductWithCourses(ctx, model.Product{
		Type: "course", Name: "Tri-state Course Product " + uuid.NewString()[:8],
		Price: 7000, Status: "published",
		AvailableFrom: &from, AvailableUntil: &until,
	}, []string{courseID.String()}, RoleSuperAdmin)
	if err != nil {
		t.Fatalf("CreateProductWithCourses: %v", err)
	}

	// Absent preserves.
	if _, err := svc.UpdateProductWithCourses(ctx, created.ID, model.Product{
		Name: "Tri-state Course Product renamed", Price: 8000, Status: "published",
	}, []string{courseID.String()}, RoleSuperAdmin); err != nil {
		t.Fatalf("UpdateProductWithCourses (absent): %v", err)
	}
	gotFrom, gotUntil := availabilityWindow(t, svc, created.ID)
	if gotFrom == nil || !gotFrom.Equal(from) || gotUntil == nil || !gotUntil.Equal(until) {
		t.Errorf("window must survive an update that omits it, got from=%v until=%v", gotFrom, gotUntil)
	}

	// Explicit null clears.
	if _, err := svc.UpdateProductWithCourses(ctx, created.ID, model.Product{
		Name: "Tri-state Course Product renamed", Price: 8000, Status: "published",
		AvailableFrom: nil, AvailableFromSet: true,
		AvailableUntil: nil, AvailableUntilSet: true,
	}, []string{courseID.String()}, RoleSuperAdmin); err != nil {
		t.Fatalf("UpdateProductWithCourses (null): %v", err)
	}
	gotFrom, gotUntil = availabilityWindow(t, svc, created.ID)
	if gotFrom != nil || gotUntil != nil {
		t.Errorf("an explicitly null window must be cleared, got from=%v until=%v", gotFrom, gotUntil)
	}
}

func TestListOrderableExams_RespectsExamWindow(t *testing.T) {
	svc, _ := newRealDBService(t)
	ctx := context.Background()

	now := time.Now()
	future := now.Add(24 * time.Hour)
	yesterday := now.Add(-24 * time.Hour)
	tomorrow := now.Add(24 * time.Hour)
	justStarted := now.Add(-5 * time.Minute)
	duration := 60
	grace := 10

	futureExam, err := svc.CreateExam(ctx, model.Exam{Title: "Orderable Future Exam " + uuid.NewString()[:8], ScheduledAt: &future})
	if err != nil {
		t.Fatalf("CreateExam future: %v", err)
	}
	pastExam, err := svc.CreateExam(ctx, model.Exam{Title: "Orderable Past Exam " + uuid.NewString()[:8], ScheduledAt: &yesterday})
	if err != nil {
		t.Fatalf("CreateExam past: %v", err)
	}
	openWindowExam, err := svc.CreateExam(ctx, model.Exam{Title: "Orderable Open Window Exam " + uuid.NewString()[:8], ScheduledAt: &yesterday, ScheduledEndAt: &tomorrow})
	if err != nil {
		t.Fatalf("CreateExam open window: %v", err)
	}
	startedExam, err := svc.CreateExam(ctx, model.Exam{Title: "Orderable Started Exam " + uuid.NewString()[:8], ScheduledAt: &justStarted, DurationMinutes: &duration, GraceWindowMinutes: &grace})
	if err != nil {
		t.Fatalf("CreateExam started: %v", err)
	}

	futureProduct, err := svc.CreateProductWithExams(ctx, model.Product{Type: "exam", Name: "Future Exam Product " + uuid.NewString()[:8], Price: 1000, Status: "published"}, []string{futureExam.ID.String()}, RoleSuperAdmin)
	if err != nil {
		t.Fatalf("CreateProductWithExams future: %v", err)
	}
	pastProduct, err := svc.CreateProductWithExams(ctx, model.Product{Type: "exam", Name: "Past Exam Product " + uuid.NewString()[:8], Price: 1000, Status: "published"}, []string{pastExam.ID.String()}, RoleSuperAdmin)
	if err != nil {
		t.Fatalf("CreateProductWithExams past: %v", err)
	}
	openWindowProduct, err := svc.CreateProductWithExams(ctx, model.Product{Type: "exam", Name: "Open Window Exam Product " + uuid.NewString()[:8], Price: 1000, Status: "published"}, []string{openWindowExam.ID.String()}, RoleSuperAdmin)
	if err != nil {
		t.Fatalf("CreateProductWithExams open window: %v", err)
	}
	startedProduct, err := svc.CreateProductWithExams(ctx, model.Product{Type: "exam", Name: "Started Exam Product " + uuid.NewString()[:8], Price: 1000, Status: "published"}, []string{startedExam.ID.String()}, RoleSuperAdmin)
	if err != nil {
		t.Fatalf("CreateProductWithExams started: %v", err)
	}

	products, err := svc.ListOrderableExams(ctx, RoleAdminSchool)
	if err != nil {
		t.Fatalf("ListOrderableExams: %v", err)
	}
	seen := map[string]bool{}
	for _, product := range products {
		seen[product.ID] = true
	}

	if !seen[futureProduct.ID] {
		t.Error("future scheduled exam product must be orderable")
	}
	if !seen[openWindowProduct.ID] {
		t.Error("exam product inside scheduled_end_at window must be orderable")
	}
	if !seen[startedProduct.ID] {
		t.Error("started exam product must remain orderable while duration+grace is still open")
	}
	if seen[pastProduct.ID] {
		t.Error("past scheduled exam product must not be orderable")
	}
}

func TestGetProduct_StudentRejectsExpiredExamProduct(t *testing.T) {
	svc, _ := newRealDBService(t)
	ctx := context.Background()

	yesterday := time.Now().Add(-24 * time.Hour)
	exam, err := svc.CreateExam(ctx, model.Exam{Title: "Student Expired Exam " + uuid.NewString()[:8], ScheduledAt: &yesterday})
	if err != nil {
		t.Fatalf("CreateExam: %v", err)
	}
	product, err := svc.CreateProductWithExams(ctx, model.Product{Type: "exam", Name: "Student Expired Exam Product " + uuid.NewString()[:8], Price: 1000, Status: "published"}, []string{exam.ID.String()}, RoleSuperAdmin)
	if err != nil {
		t.Fatalf("CreateProductWithExams: %v", err)
	}

	if _, err := svc.GetProduct(ctx, product.ID, RoleStudent); err != ErrProductNotFound {
		t.Fatalf("student GetProduct for expired exam must return ErrProductNotFound, got %v", err)
	}
}
