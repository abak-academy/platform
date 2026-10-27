package handler

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"akademi-bimbel/internal/infra"
	"akademi-bimbel/internal/service"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/require"
)

func TestAffiliatorWriteGuard(t *testing.T) {
	active := func(_ context.Context, _ string) (bool, error) { return true, nil }
	inactive := func(_ context.Context, _ string) (bool, error) { return false, nil }
	broken := func(_ context.Context, _ string) (bool, error) { return false, errors.New("db down") }

	tests := []struct {
		name     string
		role     string
		method   string
		isActive func(context.Context, string) (bool, error)
		want     int
	}{
		{"deactivated affiliator may read", service.RoleAffiliator, http.MethodGet, inactive, http.StatusOK},
		{"deactivated affiliator cannot write", service.RoleAffiliator, http.MethodPost, inactive, http.StatusForbidden},
		{"deactivated affiliator cannot delete", service.RoleAffiliator, http.MethodDelete, inactive, http.StatusForbidden},
		{"active affiliator may write", service.RoleAffiliator, http.MethodPost, active, http.StatusOK},
		{"other roles are not checked", service.RoleAdminSchool, http.MethodPost, inactive, http.StatusOK},
		{"lookup failure does not let a write through", service.RoleAffiliator, http.MethodPost, broken, http.StatusInternalServerError},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := echo.New()
			e.Use(func(next echo.HandlerFunc) echo.HandlerFunc {
				return func(c echo.Context) error {
					c.Set(claimsKey, &infra.Claims{Sub: "u1", Role: tt.role})
					return next(c)
				}
			})
			e.Use(AffiliatorWriteGuard(tt.isActive))
			e.Any("/x", func(c echo.Context) error { return c.NoContent(http.StatusOK) })

			rec := httptest.NewRecorder()
			e.ServeHTTP(rec, httptest.NewRequest(tt.method, "/x", nil))

			require.Equal(t, tt.want, rec.Code)
			if tt.want == http.StatusForbidden {
				require.Contains(t, rec.Body.String(), "account_deactivated")
			}
		})
	}
}
