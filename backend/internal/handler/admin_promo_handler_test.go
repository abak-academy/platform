package handler_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"akademi-bimbel/internal/service"
)

// TestAdminPromoCode_IsPublic_RoundTripsThroughCreateUpdateList is FR-13:
// is_public persists through admin create, admin update, and shows up
// correctly in the admin list — using the same DB-backed route table as the
// active-listing tests (promo_active_handler_test.go), since promo admin
// routes hang off the same server.RegisterRoutesForTest wiring.
func TestAdminPromoCode_IsPublic_RoundTripsThroughCreateUpdateList(t *testing.T) {
	env := newPromoActiveDBEnv(t)

	var adminID string
	email := "admin-promo-" + time.Now().Format("150405.000000") + "@test.local"
	if err := env.pool.QueryRow(context.Background(),
		`INSERT INTO users (email, role, name) VALUES ($1, $2, $3) RETURNING id`,
		email, service.RoleAdminStore, "Admin Promo Test",
	).Scan(&adminID); err != nil {
		t.Fatalf("insert admin: %v", err)
	}
	token := promoActiveToken(t, env, adminID, service.RoleAdminStore)

	code := "ADMIN-ROUNDTRIP-" + time.Now().Format("150405.000000")
	createBody := `{"code":"` + code + `","discount_percent":10,"is_public":true}`
	createReq := httptest.NewRequest(http.MethodPost, "/api/v1/admin/promo-codes", strings.NewReader(createBody))
	createReq.Header.Set("Content-Type", "application/json")
	createReq.Header.Set("Authorization", "Bearer "+token)
	createRec := httptest.NewRecorder()
	env.e.ServeHTTP(createRec, createReq)
	if createRec.Code != http.StatusCreated {
		t.Fatalf("admin create promo: want 201, got %d body=%s", createRec.Code, createRec.Body.String())
	}
	var created struct {
		ID       string `json:"id"`
		IsPublic bool   `json:"is_public"`
	}
	if err := json.Unmarshal(createRec.Body.Bytes(), &created); err != nil {
		t.Fatalf("decode create response: %v", err)
	}
	if !created.IsPublic {
		t.Fatalf("create response: want is_public=true, got false (body=%s)", createRec.Body.String())
	}

	// FR-13: flip it off via admin update, and it must persist.
	updateBody := `{"is_public":false}`
	updateReq := httptest.NewRequest(http.MethodPut, "/api/v1/admin/promo-codes/"+created.ID, strings.NewReader(updateBody))
	updateReq.Header.Set("Content-Type", "application/json")
	updateReq.Header.Set("Authorization", "Bearer "+token)
	updateRec := httptest.NewRecorder()
	env.e.ServeHTTP(updateRec, updateReq)
	if updateRec.Code != http.StatusOK {
		t.Fatalf("admin update promo: want 200, got %d body=%s", updateRec.Code, updateRec.Body.String())
	}

	listReq := httptest.NewRequest(http.MethodGet, "/api/v1/admin/promo-codes", nil)
	listReq.Header.Set("Authorization", "Bearer "+token)
	listRec := httptest.NewRecorder()
	env.e.ServeHTTP(listRec, listReq)
	if listRec.Code != http.StatusOK {
		t.Fatalf("admin list promos: want 200, got %d body=%s", listRec.Code, listRec.Body.String())
	}
	var listResp struct {
		Data []struct {
			ID       string `json:"id"`
			IsPublic bool   `json:"is_public"`
		} `json:"data"`
	}
	if err := json.Unmarshal(listRec.Body.Bytes(), &listResp); err != nil {
		t.Fatalf("decode list response: %v", err)
	}
	var foundAfterUpdate bool
	for _, p := range listResp.Data {
		if p.ID == created.ID {
			foundAfterUpdate = true
			if p.IsPublic {
				t.Errorf("admin list after update: want is_public=false for %s, got true", created.ID)
			}
		}
	}
	if !foundAfterUpdate {
		t.Fatalf("created promo %s not found in admin list", created.ID)
	}
}
