package handler_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"akademi-bimbel/internal/model"
	"akademi-bimbel/internal/service"

	"github.com/redis/go-redis/v9"
)

func TestAdminListProductGroupings_NoToken_Returns401(t *testing.T) {
	env := newCourseTestEnv(t)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/product-groupings", nil)
	rec := httptest.NewRecorder()
	env.e.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("want 401, got %d", rec.Code)
	}
}

func TestAdminCreateProductGrouping_AdminExamToken_Returns403(t *testing.T) {
	env := newCourseTestEnv(t)
	env.repo.seed(&model.User{
		ID:     "admin_exam_grouping_user",
		Email:  strptr("admin_exam_grouping@example.com"),
		Role:   service.RoleAdminExam,
		Status: "active",
	})
	rdb := redis.NewClient(&redis.Options{Addr: env.mr.Addr()})
	token, jti, _ := env.signer.SignAccess("admin_exam_grouping_user", service.RoleAdminExam, nil, []string{})
	rdb.Set(context.Background(), "session:access:"+jti, "admin_exam_grouping_user", 15*time.Minute)

	body, _ := json.Marshal(map[string]string{"name": "Paket Hemat"})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/product-groupings", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	env.e.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Errorf("want 403, got %d body=%s", rec.Code, rec.Body.String())
	}
}
