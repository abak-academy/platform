package handler_test

import (
	"net/http"
	"testing"

	"akademi-bimbel/internal/handler"
	"akademi-bimbel/internal/model"
	"akademi-bimbel/internal/service"

	"github.com/labstack/echo/v4"
)

// TestAffiliatorOnlyMiddleware_RoleGate covers the affiliate gate contract:
// it checks the role itself, so an affiliator token passes while a
// super_admin token (capabilities "*") is still rejected with 403.
func TestAffiliatorOnlyMiddleware_RoleGate(t *testing.T) {
	env := newTestEnv(t)
	e := echo.New()
	e.HideBanner = true
	v1 := e.Group("/api/v1")
	aff := v1.Group("/affiliates")
	aff.Use(handler.JWTMiddleware(env.svc, env.signer))
	aff.Use(handler.AffiliatorOnlyMiddleware())
	aff.GET("/me", func(c echo.Context) error {
		return c.JSON(http.StatusOK, map[string]string{"ok": "true"})
	})

	affEmail := "affiliator@example.com"
	env.repo.seed(&model.User{
		ID:           "u-aff",
		Email:        &affEmail,
		PasswordHash: mustHash("password123"),
		Role:         service.RoleAffiliator,
		Status:       "active",
	})
	superEmail := "super-admin@example.com"
	env.repo.seed(&model.User{
		ID:           "u-super",
		Email:        &superEmail,
		PasswordHash: mustHash("password123"),
		Role:         service.RoleSuperAdmin,
		Status:       "active",
	})

	affToken := loginToken(t, env, affEmail, "password123")
	superToken := loginToken(t, env, superEmail, "password123")

	rec := getWithToken(t, e, "/api/v1/affiliates/me", affToken)
	if rec.Code != http.StatusOK {
		t.Fatalf("want 200 for affiliator, got %d body=%s", rec.Code, rec.Body.String())
	}

	rec = getWithToken(t, e, "/api/v1/affiliates/me", superToken)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("want 403 for super_admin, got %d body=%s", rec.Code, rec.Body.String())
	}
}
