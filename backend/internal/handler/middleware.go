package handler

import (
	"context"
	"net/http"
	"slices"
	"strings"
	"time"

	"akademi-bimbel/internal/infra"
	"akademi-bimbel/internal/service"

	"github.com/labstack/echo/v4"
	echomw "github.com/labstack/echo/v4/middleware"
)

const claimsKey = "claims"

func JWTMiddleware(svc *service.Service, signer *infra.JWTSigner) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			header := c.Request().Header.Get("Authorization")
			token, ok := strings.CutPrefix(header, "Bearer ")
			if !ok || token == "" {
				return c.JSON(http.StatusUnauthorized, map[string]string{"code": "unauthorized", "message": "missing or invalid token"})
			}
			claims, err := signer.ParseAccess(token)
			if err != nil {
				return c.JSON(http.StatusUnauthorized, map[string]string{"code": "unauthorized", "message": "missing or invalid token"})
			}
			if !svc.SessionActive(c.Request().Context(), claims.ID) {
				return c.JSON(http.StatusUnauthorized, map[string]string{"code": "unauthorized", "message": "missing or invalid token"})
			}
			c.Set(claimsKey, claims)
			return next(c)
		}
	}
}

func RBACMiddleware(required string) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			claims, _ := c.Get(claimsKey).(*infra.Claims)
			if claims == nil {
				return c.JSON(http.StatusUnauthorized, map[string]string{"code": "unauthorized", "message": "missing or invalid token"})
			}
			if !service.HasCapability(claims.Role, required) {
				return c.JSON(http.StatusForbidden, map[string]string{"code": "forbidden", "message": "insufficient permissions"})
			}
			return next(c)
		}
	}
}

// AffiliatorWriteGuard keeps a deactivated affiliator read-only: login still works,
// every non-read request is rejected. Other roles pass through untouched.
func AffiliatorWriteGuard(isActive func(ctx context.Context, userID string) (bool, error)) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			claims, _ := c.Get(claimsKey).(*infra.Claims)
			if claims == nil || claims.Role != service.RoleAffiliator {
				return next(c)
			}
			switch c.Request().Method {
			case http.MethodGet, http.MethodHead, http.MethodOptions:
				return next(c)
			}
			active, err := isActive(c.Request().Context(), claims.Sub)
			if err != nil {
				return err
			}
			if !active {
				return c.JSON(http.StatusForbidden, map[string]string{"code": "account_deactivated", "message": "account is deactivated and read-only"})
			}
			return next(c)
		}
	}
}

// ForbidRoles rejects the listed roles on routes that sit in a group they can
// otherwise enter.
func ForbidRoles(roles ...string) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			claims, _ := c.Get(claimsKey).(*infra.Claims)
			if claims != nil && slices.Contains(roles, claims.Role) {
				return c.JSON(http.StatusForbidden, map[string]string{"code": "forbidden", "message": "insufficient permissions"})
			}
			return next(c)
		}
	}
}

// StudentOnlyMiddleware gates the student self-service routes on the role itself
// rather than a capability — students hold none, and super_admin's "*" would
// satisfy any capability we invented for them.
func StudentOnlyMiddleware() echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			claims, _ := c.Get(claimsKey).(*infra.Claims)
			if claims == nil {
				return c.JSON(http.StatusUnauthorized, map[string]string{"code": "unauthorized", "message": "missing or invalid token"})
			}
			if claims.Role != service.RoleStudent {
				return c.JSON(http.StatusForbidden, map[string]string{"code": "forbidden", "message": "insufficient permissions"})
			}
			return next(c)
		}
	}
}

func LoginRateLimiter() echo.MiddlewareFunc {
	return echomw.RateLimiterWithConfig(echomw.RateLimiterConfig{
		Skipper: echomw.DefaultSkipper,
		Store: echomw.NewRateLimiterMemoryStoreWithConfig(
			echomw.RateLimiterMemoryStoreConfig{
				Rate:      10,
				Burst:     10,
				ExpiresIn: 1 * time.Minute,
			},
		),
		IdentifierExtractor: func(c echo.Context) (string, error) {
			return c.RealIP(), nil
		},
		ErrorHandler: func(c echo.Context, err error) error {
			return c.JSON(http.StatusTooManyRequests, map[string]string{
				"code":    "rate_limited",
				"message": "too many login attempts",
			})
		},
		DenyHandler: func(c echo.Context, identifier string, err error) error {
			c.Response().Header().Set("Retry-After", "1")
			return c.JSON(http.StatusTooManyRequests, map[string]string{
				"code":    "rate_limited",
				"message": "too many login attempts",
			})
		},
	})
}

func ClaimsFromContext(c echo.Context) *infra.Claims {
	claims, _ := c.Get(claimsKey).(*infra.Claims)
	return claims
}
