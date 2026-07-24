package handler_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"akademi-bimbel/internal/model"
	"akademi-bimbel/internal/repository"
)

func TestAdminProduct_SpecsRoundTrip(t *testing.T) {
	env := newAdminProductDBEnv(t)
	token := mintProductToken(t, env, "00000000-0000-0000-0000-0000000000a1", "super_admin")

	body := `{
		"type": "book",
		"name": "Kumpulan Soal KoSSMI Fisika",
		"price": 20000,
		"stock": 9,
		"specs": [
			{"key": "penerbit", "label": "Perusahaan Penerbit", "value": "Yayasan Abak Cendekia"},
			{"key": "jenis_cover", "label": "Jenis Cover", "value": "Hard Cover"}
		]
	}`

	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/products", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	env.e.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated && rec.Code != http.StatusOK {
		t.Fatalf("create product: status %d body %s", rec.Code, rec.Body.String())
	}

	var created model.Product
	if err := json.Unmarshal(rec.Body.Bytes(), &created); err != nil {
		t.Fatalf("decode created product: %v", err)
	}
	if len(created.Specs) != 2 {
		t.Fatalf("want 2 specs persisted, got %d (%+v)", len(created.Specs), created.Specs)
	}
	if created.Specs[0].Key != "penerbit" || created.Specs[1].Label != "Jenis Cover" {
		t.Fatalf("spec order or content not preserved: %+v", created.Specs)
	}

	persisted, err := repository.New(env.pool).GetProductByID(context.Background(), created.ID)
	if err != nil {
		t.Fatalf("get persisted product: %v", err)
	}
	if len(persisted.Specs) != 2 {
		t.Fatalf("want 2 specs persisted in DB, got %d (%+v)", len(persisted.Specs), persisted.Specs)
	}
	if persisted.Specs[0].Key != "penerbit" || persisted.Specs[0].Label != "Perusahaan Penerbit" || persisted.Specs[0].Value != "Yayasan Abak Cendekia" {
		t.Fatalf("persisted spec[0] mismatch: %+v", persisted.Specs[0])
	}
	if persisted.Specs[1].Key != "jenis_cover" || persisted.Specs[1].Label != "Jenis Cover" || persisted.Specs[1].Value != "Hard Cover" {
		t.Fatalf("persisted spec[1] mismatch: %+v", persisted.Specs[1])
	}
}

func TestAdminProduct_OmittedSpecsPreserveExisting(t *testing.T) {
	env := newAdminProductDBEnv(t)
	token := mintProductToken(t, env, "00000000-0000-0000-0000-0000000000a2", "super_admin")

	createBody := `{
		"type": "book", "name": "Buku Spesifikasi", "price": 1000, "stock": 1,
		"specs": [{"key": "isbn", "label": "ISBN", "value": "978-1"}]
	}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/products", strings.NewReader(createBody))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	env.e.ServeHTTP(rec, req)

	var created model.Product
	if err := json.Unmarshal(rec.Body.Bytes(), &created); err != nil {
		t.Fatalf("decode: %v", err)
	}

	// PATCH without a specs field must not wipe the stored specs.
	patch := `{"name": "Buku Spesifikasi v2", "price": 2000, "stock": 1}`
	req = httptest.NewRequest(http.MethodPatch, "/api/v1/admin/products/"+created.ID, strings.NewReader(patch))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	rec = httptest.NewRecorder()
	env.e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("patch product: status %d body %s", rec.Code, rec.Body.String())
	}

	var updated model.Product
	if err := json.Unmarshal(rec.Body.Bytes(), &updated); err != nil {
		t.Fatalf("decode updated: %v", err)
	}
	if len(updated.Specs) != 1 || updated.Specs[0].Key != "isbn" {
		t.Fatalf("omitted specs should be preserved, got %+v", updated.Specs)
	}
}

func TestAdminProduct_RejectsMalformedSpecs(t *testing.T) {
	env := newAdminProductDBEnv(t)
	token := mintProductToken(t, env, "00000000-0000-0000-0000-0000000000a3", "super_admin")

	body := `{
		"type": "book", "name": "Buku Rusak", "price": 1000, "stock": 1,
		"specs": [{"key": "", "label": "Tanpa Key", "value": "x"}]
	}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/products", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	env.e.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("want 400 for malformed specs, got %d body %s", rec.Code, rec.Body.String())
	}
}

// TestAdminProduct_SpecsPersistForExamProduct proves specs survive the
// CreateProductWithExams path — a different repository function from the
// book/merch CreateProduct path exercised by SpecsRoundTrip above.
func TestAdminProduct_SpecsPersistForExamProduct(t *testing.T) {
	env := newAdminProductDBEnv(t)
	token := mintProductToken(t, env, "00000000-0000-0000-0000-0000000000a4", "super_admin")

	exam := &model.Exam{
		Title:               "Try Out Fisika Specs",
		ResultConfig:        "hidden",
		CertificateTemplate: "classic",
		Status:              "draft",
	}
	if err := repository.New(env.pool).CreateExam(context.Background(), exam); err != nil {
		t.Fatalf("create exam fixture: %v", err)
	}

	body := `{
		"type": "exam",
		"name": "Try Out Fisika Specs",
		"price": 15000,
		"exam_ids": ["` + exam.ID.String() + `"],
		"specs": [{"key": "format", "label": "Format", "value": "Online"}]
	}`

	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/products", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	env.e.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated && rec.Code != http.StatusOK {
		t.Fatalf("create exam product: status %d body %s", rec.Code, rec.Body.String())
	}

	var created model.Product
	if err := json.Unmarshal(rec.Body.Bytes(), &created); err != nil {
		t.Fatalf("decode created product: %v", err)
	}
	if len(created.Specs) != 1 || created.Specs[0].Key != "format" || created.Specs[0].Value != "Online" {
		t.Fatalf("want 1 spec {format:Online} persisted for exam product, got %+v", created.Specs)
	}

	persisted, err := repository.New(env.pool).GetProductByID(context.Background(), created.ID)
	if err != nil {
		t.Fatalf("get persisted product: %v", err)
	}
	if len(persisted.Specs) != 1 || persisted.Specs[0].Key != "format" || persisted.Specs[0].Label != "Format" || persisted.Specs[0].Value != "Online" {
		t.Fatalf("want 1 spec {format:Format:Online} persisted in DB for exam product, got %+v", persisted.Specs)
	}
}
