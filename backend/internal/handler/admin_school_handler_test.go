package handler_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestAdminCreateSchool_MissingFields(t *testing.T) {
	env := newAdminSystemEnv(t)

	body := map[string]string{"code": "test"}
	b, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/admin/schools", bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	c := env.e.NewContext(req, rec)
	setAdminClaims(c, "u1")

	err := env.h.AdminCreateSchool(c)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rec.Code != http.StatusBadRequest {
		t.Errorf("want 400, got %d", rec.Code)
	}
}

func TestAdminCreateSchool_MissingCode(t *testing.T) {
	env := newAdminSystemEnv(t)

	body := map[string]string{"name": "Test School"}
	b, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/admin/schools", bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	c := env.e.NewContext(req, rec)
	setAdminClaims(c, "u1")

	err := env.h.AdminCreateSchool(c)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rec.Code != http.StatusBadRequest {
		t.Errorf("want 400 for missing code, got %d", rec.Code)
	}
}

func TestAdminCreateSchool_InvalidName(t *testing.T) {
	env := newAdminSystemEnv(t)

	body := map[string]string{"name": "...", "code": "test"}
	b, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/admin/schools", bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	c := env.e.NewContext(req, rec)
	setAdminClaims(c, "u1")

	err := env.h.AdminCreateSchool(c)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	assertInvalidSchoolNameResponse(t, rec)
}

func TestAdminUpdateSchool_InvalidName(t *testing.T) {
	env := newAdminSystemEnv(t)

	body := map[string]string{"name": " --- "}
	b, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPut, "/admin/schools/s1", bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	c := env.e.NewContext(req, rec)
	c.SetParamNames("id")
	c.SetParamValues("s1")
	setAdminClaims(c, "u1")

	err := env.h.AdminUpdateSchool(c)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	assertInvalidSchoolNameResponse(t, rec)
}

func assertInvalidSchoolNameResponse(t *testing.T, rec *httptest.ResponseRecorder) {
	t.Helper()
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("want 400, got %d body=%s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp["code"] != "invalid_request" {
		t.Errorf("want code=invalid_request, got %v", resp["code"])
	}
	if resp["message"] != "school name must contain at least one letter or digit" {
		t.Errorf("want invalid school name message, got %v", resp["message"])
	}
}

func TestAdminListSchools_InvalidStatusFilter(t *testing.T) {
	env := newAdminSystemEnv(t)

	req := httptest.NewRequest(http.MethodGet, "/admin/schools?status=pending", nil)
	rec := httptest.NewRecorder()
	c := env.e.NewContext(req, rec)
	setAdminClaims(c, "u1")

	err := env.h.AdminListSchools(c)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rec.Code != http.StatusBadRequest {
		t.Errorf("want 400 for invalid status filter, got %d", rec.Code)
	}
}

func TestAdminChangeSchoolStatus_InvalidStatus(t *testing.T) {
	env := newAdminSystemEnv(t)

	body := map[string]string{"status": "pending"}
	b, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPatch, "/admin/schools/s1", bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	c := env.e.NewContext(req, rec)
	c.SetParamNames("id")
	c.SetParamValues("s1")
	setAdminClaims(c, "u1")

	err := env.h.AdminChangeSchoolStatus(c)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rec.Code != http.StatusBadRequest {
		t.Errorf("want 400 for invalid status, got %d", rec.Code)
	}
}

func TestAdminChangeSchoolStatus_EmptyBody(t *testing.T) {
	env := newAdminSystemEnv(t)

	req := httptest.NewRequest(http.MethodPatch, "/admin/schools/s1", bytes.NewReader([]byte("{}")))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	c := env.e.NewContext(req, rec)
	c.SetParamNames("id")
	c.SetParamValues("s1")
	setAdminClaims(c, "u1")

	err := env.h.AdminChangeSchoolStatus(c)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rec.Code != http.StatusBadRequest {
		t.Errorf("want 400 for missing status field, got %d", rec.Code)
	}
}
