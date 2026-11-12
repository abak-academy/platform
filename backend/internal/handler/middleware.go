package handler

import (
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

// AffiliatorOnlyMiddleware role-gates affiliate routes: super_admin's "*" must never satisfy them.
func AffiliatorOnlyMiddleware() echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			claims, _ := c.Get(claimsKey).(*infra.Claims)
			if claims == nil {
				return c.JSON(http.StatusUnauthorized, map[string]string{"code": "unauthorized", "message": "missing or invalid token"})
			}
			if claims.Role != service.RoleAffiliator {
				return c.JSON(http.StatusForbidden, map[string]string{"code": "forbidden", "message": "insufficient permissions"})
			}
			return next(c)
		}
	}
}

const partnerStatesKey = "partnerStates"

// partnerStatesFromContext returns the program states PartnerGate resolved for
// the request.
func partnerStatesFromContext(c echo.Context) *service.AffiliatorProgramStates {
	states, _ := c.Get(partnerStatesKey).(*service.AffiliatorProgramStates)
	return states
}

func isReadMethod(method string) bool {
	return method == http.MethodGet || method == http.MethodHead || method == http.MethodOptions
}

// programRefusal is the 403 body for an account that cannot use a program.
func programRefusal(c echo.Context, state service.ProgramState, states *service.AffiliatorProgramStates) (bool, error) {
	switch state {
	case service.ProgramNotEnrolled:
		return true, c.JSON(http.StatusForbidden, map[string]string{"code": "program_not_enrolled", "message": "this account is not registered in the program"})
	case service.ProgramPending:
		return true, c.JSON(http.StatusForbidden, map[string]string{"code": "program_pending", "message": "the registration is waiting for approval"})
	case service.ProgramRejected:
		body := map[string]string{"code": "program_rejected", "message": "the registration was rejected"}
		if states.RejectionReason != nil {
			body["reason"] = *states.RejectionReason
		}
		return true, c.JSON(http.StatusForbidden, body)
	}
	return false, nil
}

// PartnerGate lets only an approved Mitra account through. A deactivated
// account, or a switched-off program, keeps the reads and loses the writes.
func PartnerGate(svc *service.Service) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			claims, _ := c.Get(claimsKey).(*infra.Claims)
			if claims == nil {
				return c.JSON(http.StatusUnauthorized, map[string]string{"code": "unauthorized", "message": "missing or invalid token"})
			}
			if claims.Role != service.RoleAffiliator {
				return c.JSON(http.StatusForbidden, map[string]string{"code": "forbidden", "message": "insufficient permissions"})
			}
			ctx := c.Request().Context()
			states, err := svc.AffiliatorProgramStates(ctx, claims.Sub)
			if err != nil {
				return err
			}
			if refused, err := programRefusal(c, states.Partner, states); refused {
				return err
			}
			if !isReadMethod(c.Request().Method) {
				if !states.AccountActive {
					return c.JSON(http.StatusForbidden, map[string]string{"code": "account_deactivated", "message": "account is deactivated and read-only"})
				}
				if !svc.PartnerProgramEnabled(ctx) {
					return c.JSON(http.StatusForbidden, map[string]string{"code": "partner_program_off", "message": "the partner program is switched off"})
				}
			}
			c.Set(partnerStatesKey, states)
			return next(c)
		}
	}
}

// AffiliateGate keeps an account that is not in the Affiliator program out of
// the affiliate routes. Pending and rejected profiles still pass: those routes
// show them their own registration status. A deactivated account is read-only.
func AffiliateGate(svc *service.Service) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			claims, _ := c.Get(claimsKey).(*infra.Claims)
			if claims == nil {
				return c.JSON(http.StatusUnauthorized, map[string]string{"code": "unauthorized", "message": "missing or invalid token"})
			}
			states, err := svc.AffiliatorProgramStates(c.Request().Context(), claims.Sub)
			if err != nil {
				return err
			}
			if states.Affiliate == service.ProgramNotEnrolled {
				_, err := programRefusal(c, states.Affiliate, states)
				return err
			}
			if !isReadMethod(c.Request().Method) && !states.AccountActive {
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
