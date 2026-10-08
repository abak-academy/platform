package handler

import (
	"errors"
	"net/http"

	"akademi-bimbel/internal/infra"
	"akademi-bimbel/internal/model"
	"akademi-bimbel/internal/service"
	"github.com/labstack/echo/v4"
)

func (h *Handler) Register(c echo.Context) error {
	var req struct {
		Email    string `json:"email"`
		Password string `json:"password"`
		Name     string `json:"name"`
	}
	if err := c.Bind(&req); err != nil {
		return badRequest(c, "invalid request body")
	}
	if req.Email == "" || req.Password == "" || req.Name == "" {
		return badRequest(c, "email, password and name are required")
	}
	chall, err := h.svc.Register(c.Request().Context(), req.Email, req.Password, req.Name, c.RealIP())
	if err != nil {
		return mapServiceError(c, err)
	}
	return c.JSON(http.StatusCreated, map[string]any{
		"otp_required":  true,
		"pending_token": chall.PendingToken,
		"email_failed":  chall.EmailFailed,
		"otp_active":    chall.OTPActive,
		"resend_after":  chall.ResendAfter,
	})
}

func (h *Handler) Login(c echo.Context) error {
	var req struct {
		Identifier string `json:"identifier"`
		Password   string `json:"password"`
	}
	if err := c.Bind(&req); err != nil {
		return badRequest(c, "invalid request body")
	}
	if req.Identifier == "" || req.Password == "" {
		return badRequest(c, "identifier and password are required")
	}
	access, refresh, pendingToken, err := h.svc.Login(c.Request().Context(), req.Identifier, req.Password, c.RealIP())
	if errors.Is(err, service.ErrVerificationPending) {
		emailFailed, otpActive, resendAfter := false, false, 0
		var vp *service.VerificationPendingError
		if errors.As(err, &vp) {
			emailFailed = vp.EmailFailed
			otpActive = vp.OTPActive
			resendAfter = vp.ResendAfter
		}
		return c.JSON(http.StatusForbidden, map[string]any{
			"code":          "verification_pending",
			"otp_required":  true,
			"pending_token": pendingToken,
			"id":            req.Identifier,
			"email_failed":  emailFailed,
			"otp_active":    otpActive,
			"resend_after":  resendAfter,
		})
	}
	if err != nil {
		return mapServiceError(c, err)
	}
	claims, _ := h.svc.ParseAccess(access)
	user, err := h.svc.Me(c.Request().Context(), claims.Sub)
	if err != nil {
		return mapServiceError(c, err)
	}
	return c.JSON(http.StatusOK, map[string]any{
		"access_token":  access,
		"refresh_token": refresh,
		"user":          userPayload(user),
	})
}

func (h *Handler) GoogleLogin(c echo.Context) error {
	var req struct {
		IDToken string `json:"id_token"`
	}
	if err := c.Bind(&req); err != nil {
		return badRequest(c, "invalid request body")
	}
	if req.IDToken == "" {
		return badRequest(c, "id_token is required")
	}
	access, refresh, err := h.svc.GoogleLogin(c.Request().Context(), req.IDToken)
	if err != nil {
		return mapServiceError(c, err)
	}
	claims, _ := h.svc.ParseAccess(access)
	user, err := h.svc.Me(c.Request().Context(), claims.Sub)
	if err != nil {
		return mapServiceError(c, err)
	}
	return c.JSON(http.StatusOK, map[string]any{
		"access_token":  access,
		"refresh_token": refresh,
		"user":          userPayload(user),
	})
}

func (h *Handler) SendOTP(c echo.Context) error {
	var req struct {
		PendingToken string `json:"pending_token"`
		Email        string `json:"email"`
	}
	if err := c.Bind(&req); err != nil {
		return badRequest(c, "invalid request body")
	}
	if req.PendingToken == "" && req.Email == "" {
		return badRequest(c, "pending_token or email is required")
	}

	res, err := h.svc.ResendOTP(c.Request().Context(), req.PendingToken, req.Email, c.RealIP())
	if err != nil {
		return mapServiceError(c, err)
	}
	return c.JSON(http.StatusOK, map[string]any{
		"message":       "OTP sent",
		"pending_token": res.PendingToken,
		"resend_after":  res.ResendAfter,
	})
}

func (h *Handler) VerifyOTP(c echo.Context) error {
	var req struct {
		PendingToken string `json:"pending_token"`
		Code         string `json:"code"`
	}
	if err := c.Bind(&req); err != nil {
		return badRequest(c, "invalid request body")
	}
	if req.PendingToken == "" || req.Code == "" {
		return badRequest(c, "pending_token and code are required")
	}
	access, refresh, err := h.svc.VerifyOTP(c.Request().Context(), req.PendingToken, req.Code, c.RealIP())
	if err != nil {
		return mapServiceError(c, err)
	}
	claims, _ := h.svc.ParseAccess(access)
	user, err := h.svc.Me(c.Request().Context(), claims.Sub)
	if err != nil {
		return mapServiceError(c, err)
	}
	return c.JSON(http.StatusOK, map[string]any{
		"access_token":  access,
		"refresh_token": refresh,
		"user":          userPayload(user),
	})
}

func (h *Handler) Refresh(c echo.Context) error {
	var req struct {
		RefreshToken string `json:"refresh_token"`
	}
	if err := c.Bind(&req); err != nil {
		return badRequest(c, "invalid request body")
	}
	if req.RefreshToken == "" {
		return badRequest(c, "refresh_token is required")
	}
	access, refresh, err := h.svc.Refresh(c.Request().Context(), req.RefreshToken)
	if err != nil {
		return mapServiceError(c, err)
	}
	return c.JSON(http.StatusOK, map[string]string{
		"access_token":  access,
		"refresh_token": refresh,
	})
}

func claimsFromContext(c echo.Context) *infra.Claims {
	claims, _ := c.Get("claims").(*infra.Claims)
	return claims
}

// actorFromClaims returns the authenticated user id for audit attribution.
// ok is false when there is no actor to attribute — audit_log.actor_id is
// NOT NULL, so an empty sub must be rejected before it reaches the service.
func actorFromClaims(c echo.Context) (actorID string, ok bool) {
	claims := claimsFromContext(c)
	if claims == nil || claims.Sub == "" {
		return "", false
	}
	return claims.Sub, true
}

func (h *Handler) Logout(c echo.Context) error {
	claims := claimsFromContext(c)
	var req struct {
		RefreshToken string `json:"refresh_token"`
	}
	_ = c.Bind(&req)
	if err := h.svc.Logout(c.Request().Context(), claims.RegisteredClaims.ID, req.RefreshToken); err != nil {
		return mapServiceError(c, err)
	}
	return c.NoContent(http.StatusNoContent)
}

func (h *Handler) ForgotPassword(c echo.Context) error {
	var req struct {
		Email string `json:"email"`
	}
	if err := c.Bind(&req); err != nil {
		return badRequest(c, "invalid request body")
	}
	if req.Email == "" {
		return badRequest(c, "email is required")
	}
	if err := h.svc.ForgotPassword(c.Request().Context(), req.Email, c.RealIP()); err != nil {
		return mapServiceError(c, err)
	}
	return c.JSON(http.StatusOK, map[string]string{
		"message": "if the email exists, you will receive a reset OTP",
	})
}

func (h *Handler) ResetPassword(c echo.Context) error {
	var req struct {
		Token       string `json:"token"`
		OTP         string `json:"otp"`
		NewPassword string `json:"new_password"`
	}
	if err := c.Bind(&req); err != nil {
		return badRequest(c, "invalid request body")
	}
	if req.Token == "" || req.OTP == "" || req.NewPassword == "" {
		return badRequest(c, "token, otp and new_password are required")
	}
	if err := h.svc.ResetPassword(c.Request().Context(), req.Token, req.OTP, req.NewPassword, c.RealIP()); err != nil {
		return mapServiceError(c, err)
	}
	return c.JSON(http.StatusOK, map[string]string{"message": "password reset successful"})
}

func (h *Handler) ChangePassword(c echo.Context) error {
	claims := claimsFromContext(c)
	var req struct {
		CurrentPassword string `json:"current_password"`
		NewPassword     string `json:"new_password"`
	}
	if err := c.Bind(&req); err != nil {
		return badRequest(c, "invalid request body")
	}
	if req.CurrentPassword == "" || req.NewPassword == "" {
		return badRequest(c, "current_password and new_password are required")
	}
	if err := h.svc.ChangePassword(c.Request().Context(), claims.Sub, req.CurrentPassword, req.NewPassword, claims.ID); err != nil {
		return mapServiceError(c, err)
	}
	return c.JSON(http.StatusOK, map[string]string{"message": "password changed"})
}

func (h *Handler) Me(c echo.Context) error {
	claims := claimsFromContext(c)
	user, err := h.svc.Me(c.Request().Context(), claims.Sub)
	if err != nil {
		return mapServiceError(c, err)
	}
	return c.JSON(http.StatusOK, map[string]any{
		"id":                 user.ID,
		"email":              derefStr(user.Email),
		"username":           derefStr(user.Username),
		"name":               user.Name,
		"role":               user.Role,
		"school_id":          user.SchoolID,
		"auth_provider":      user.AuthProvider,
		"status":             user.Status,
		"photo_url":          derefStr(user.PhotoURL),
		"password_temporary": user.PasswordTemporary,
	})
}

func userPayload(user *model.User) map[string]any {
	return map[string]any{
		"id":                 user.ID,
		"role":               user.Role,
		"name":               user.Name,
		"email":              derefStr(user.Email),
		"username":           derefStr(user.Username),
		"auth_provider":      user.AuthProvider,
		"school_id":          user.SchoolID,
		"grade":              user.Grade,
		"photo_url":          derefStr(user.PhotoURL),
		"password_temporary": user.PasswordTemporary,
	}
}

func derefStr(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
