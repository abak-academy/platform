package handler

import (
	"net/http"
	"strconv"

	"strings"

	"akademi-bimbel/internal/service"

	"github.com/labstack/echo/v4"
)

// AdminListAccounts lists admin accounts, filterable by role and status, cursor-paginated.
func (h *Handler) AdminListAccounts(c echo.Context) error {
	roleFilter := c.QueryParam("role")
	statusFilter := c.QueryParam("status")
	cursor := c.QueryParam("cursor")

	limit := 20
	if l := c.QueryParam("limit"); l != "" {
		if n, err := strconv.Atoi(l); err == nil && n > 0 {
			limit = n
			if limit > 100 {
				limit = 100
			}
		}
	}

	accounts, nextCursor, err := h.svc.ListAdminAccounts(c.Request().Context(), roleFilter, statusFilter, limit, cursor)
	if err != nil {
		return mapServiceError(c, err)
	}

	return c.JSON(http.StatusOK, map[string]any{
		"data":        accounts,
		"next_cursor": nextCursor,
	})
}

// AdminCreateAccount creates a new admin account.
func (h *Handler) AdminCreateAccount(c echo.Context) error {
	var req struct {
		Email    string  `json:"email"`
		Name     string  `json:"name"`
		Role     string  `json:"role"`
		Password string  `json:"password"`
		SchoolID *string `json:"school_id"`
	}
	if err := c.Bind(&req); err != nil {
		return badRequest(c, "invalid request body")
	}
	if req.Email == "" || req.Name == "" || req.Role == "" || req.Password == "" {
		return badRequest(c, "email, name, role and password are required")
	}

	actorID := ClaimsFromContext(c).Sub
	account, err := h.svc.CreateAdminAccount(c.Request().Context(), actorID, req.Email, req.Name, req.Role, req.Password, req.SchoolID)
	if err != nil {
		return mapServiceError(c, err)
	}
	return c.JSON(http.StatusCreated, account)
}

// AdminChangeAccountRole changes an account's role.
func (h *Handler) AdminChangeAccountRole(c echo.Context) error {
	id := c.Param("id")

	var req struct {
		Role     string  `json:"role"`
		SchoolID *string `json:"school_id"`
	}
	if err := c.Bind(&req); err != nil {
		return badRequest(c, "invalid request body")
	}
	if req.Role == "" {
		return badRequest(c, "role is required")
	}

	actorID := ClaimsFromContext(c).Sub
	if err := h.svc.ChangeAccountRole(c.Request().Context(), actorID, id, req.Role, req.SchoolID); err != nil {
		return mapServiceError(c, err)
	}
	return c.JSON(http.StatusOK, map[string]string{"message": "role updated"})
}

// AdminChangeAccountStatus deactivates or reactivates an account.
func (h *Handler) AdminChangeAccountStatus(c echo.Context) error {
	id := c.Param("id")

	var req struct {
		Status string `json:"status"`
	}
	if err := c.Bind(&req); err != nil {
		return badRequest(c, "invalid request body")
	}
	if req.Status == "" {
		return badRequest(c, "status is required")
	}
	if req.Status != "active" && req.Status != "deactivated" {
		return badRequest(c, "status must be active or deactivated")
	}

	actorID := ClaimsFromContext(c).Sub
	if err := h.svc.ChangeAccountStatus(c.Request().Context(), actorID, id, req.Status); err != nil {
		return mapServiceError(c, err)
	}
	return c.JSON(http.StatusOK, map[string]string{"message": "status updated"})
}

// AdminResetAccountPassword triggers a password reset for an admin account.
func (h *Handler) AdminResetAccountPassword(c echo.Context) error {
	id := c.Param("id")

	actorID := ClaimsFromContext(c).Sub
	if err := h.svc.TriggerAccountPasswordReset(c.Request().Context(), actorID, id, c.RealIP()); err != nil {
		return mapServiceError(c, err)
	}
	return c.JSON(http.StatusOK, map[string]string{"message": "password reset triggered"})
}

// AdminListAuditLog returns audit log entries with optional filters, page-paginated.
func (h *Handler) AdminListAuditLog(c echo.Context) error {
	actorID := c.QueryParam("actor_id")
	from := c.QueryParam("from")
	to := c.QueryParam("to")
	targetType := c.QueryParam("target_type")
	q := c.QueryParam("q")
	page := 1
	if p := c.QueryParam("page"); p != "" {
		n, err := strconv.Atoi(p)
		if err != nil || n < 1 {
			return badRequest(c, "page must be a positive integer")
		}
		page = n
	}
	limit := parseLimit(c.QueryParam("limit"), 20, 100)

	entries, total, err := h.svc.ListAuditLog(c.Request().Context(), service.AuditLogFilter{
		ActorID:    actorID,
		From:       from,
		To:         to,
		TargetType: targetType,
		Q:          q,
		Page:       page,
		Limit:      limit,
	})
	if err != nil {
		return mapServiceError(c, err)
	}

	return c.JSON(http.StatusOK, map[string]any{
		"data":     entries,
		"total":    total,
		"page":     page,
		"per_page": limit,
	})
}

// AdminGetSystemConfig returns the full system config with secrets masked.
func (h *Handler) AdminGetSystemConfig(c echo.Context) error {
	cfg, err := h.svc.GetSystemConfig(c.Request().Context())
	if err != nil {
		return mapServiceError(c, err)
	}
	return c.JSON(http.StatusOK, cfg)
}

// AdminUpdateSystemConfig upserts config keys and returns the masked config map.
func (h *Handler) AdminUpdateSystemConfig(c echo.Context) error {
	var req map[string]string
	if err := c.Bind(&req); err != nil {
		return badRequest(c, "invalid request body")
	}

	actorID := ClaimsFromContext(c).Sub
	cfg, err := h.svc.UpdateSystemConfig(c.Request().Context(), actorID, req)
	if err != nil {
		return mapServiceError(c, err)
	}

	for key := range req {
		if strings.HasPrefix(key, "midtrans_") {
			h.svc.ReloadPaymentClient(c.Request().Context())
			break
		}
	}
	for key := range req {
		if strings.HasPrefix(key, "biteship_") {
			h.svc.ReloadLogisticsClient(c.Request().Context())
			break
		}
	}

	return c.JSON(http.StatusOK, cfg)
}
