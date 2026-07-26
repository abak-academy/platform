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
