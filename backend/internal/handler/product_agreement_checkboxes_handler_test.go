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

func TestAdminProduct_AgreementCheckboxesRoundTrip(t *testing.T) {
	env := newAdminProductDBEnv(t)
	token := mintProductToken(t, env, "00000000-0000-0000-0000-0000000000b1", "super_admin")

	body := `{
		"type": "book",
		"name": "Buku dengan Syarat",
		"price": 20000,
		"stock": 9,
		"agreement_checkboxes": [
			{"title": "Syarat Pembelian", "body": "Saya menyetujui syarat pembelian"}
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
	if len(created.AgreementCheckboxes) != 1 {
		t.Fatalf("want 1 agreement checkbox persisted, got %d (%+v)", len(created.AgreementCheckboxes), created.AgreementCheckboxes)
	}
	if created.AgreementCheckboxes[0].ID == "" {
		t.Fatalf("want a server-generated ID for a new checkbox, got empty")
	}
	if created.AgreementCheckboxes[0].Title != "Syarat Pembelian" || created.AgreementCheckboxes[0].Body != "Saya menyetujui syarat pembelian" {
		t.Fatalf("checkbox content mismatch: %+v", created.AgreementCheckboxes[0])
	}

	persisted, err := repository.New(env.pool).GetProductByID(context.Background(), created.ID)
	if err != nil {
		t.Fatalf("get persisted product: %v", err)
	}
	if len(persisted.AgreementCheckboxes) != 1 || persisted.AgreementCheckboxes[0].ID != created.AgreementCheckboxes[0].ID {
		t.Fatalf("persisted agreement checkbox mismatch: %+v", persisted.AgreementCheckboxes)
	}
}

// TestAdminProduct_AgreementCheckboxIDsStableAcrossEdit proves a checkbox's ID
// is preserved when the update request echoes it back, instead of minting a
// fresh one on every PATCH — which would orphan any order_item.agreement_snapshot
// already referencing the original ID.
func TestAdminProduct_AgreementCheckboxIDsStableAcrossEdit(t *testing.T) {
	env := newAdminProductDBEnv(t)
	token := mintProductToken(t, env, "00000000-0000-0000-0000-0000000000b2", "super_admin")

	createBody := `{
		"type": "book", "name": "Buku Checkbox", "price": 1000, "stock": 1,
		"agreement_checkboxes": [{"title": "Judul", "body": "Isi"}]
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
	originalID := created.AgreementCheckboxes[0].ID

	patch := `{"name": "Buku Checkbox v2", "price": 2000, "stock": 1, "agreement_checkboxes": [{"id": "` + originalID + `", "title": "Judul Baru", "body": "Isi Baru"}]}`
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
	if len(updated.AgreementCheckboxes) != 1 || updated.AgreementCheckboxes[0].ID != originalID {
		t.Fatalf("want the checkbox ID preserved as %s, got %+v", originalID, updated.AgreementCheckboxes)
	}
	if updated.AgreementCheckboxes[0].Title != "Judul Baru" {
		t.Fatalf("want updated title, got %+v", updated.AgreementCheckboxes[0])
	}
}

func TestAdminProduct_OmittedAgreementCheckboxesPreserveExisting(t *testing.T) {
	env := newAdminProductDBEnv(t)
	token := mintProductToken(t, env, "00000000-0000-0000-0000-0000000000b3", "super_admin")

	createBody := `{
		"type": "book", "name": "Buku Checkbox Preserve", "price": 1000, "stock": 1,
		"agreement_checkboxes": [{"title": "Judul", "body": "Isi"}]
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

	patch := `{"name": "Buku Checkbox Preserve v2", "price": 2000, "stock": 1}`
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
	if len(updated.AgreementCheckboxes) != 1 || updated.AgreementCheckboxes[0].Title != "Judul" {
		t.Fatalf("omitted agreement_checkboxes should be preserved, got %+v", updated.AgreementCheckboxes)
	}
}

func TestAdminProduct_RejectsMalformedAgreementCheckboxes(t *testing.T) {
	env := newAdminProductDBEnv(t)
	token := mintProductToken(t, env, "00000000-0000-0000-0000-0000000000b4", "super_admin")

	body := `{
		"type": "book", "name": "Buku Rusak Checkbox", "price": 1000, "stock": 1,
		"agreement_checkboxes": [{"title": "", "body": "Tanpa judul"}]
	}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/products", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	env.e.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("want 400 for malformed agreement checkboxes, got %d body %s", rec.Code, rec.Body.String())
	}
}
