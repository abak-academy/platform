package handler

import (
	"net/http"
	"strconv"

	"akademi-bimbel/internal/service"

	"github.com/labstack/echo/v4"
)

type affiliateApprovalRequest struct {
	Decision string `json:"decision"`
	Reason   string `json:"reason"`
}

type affiliateStatusRequest struct {
	Status string `json:"status"`
	Reason string `json:"reason"`
}

type affiliateLevelRequest struct {
	Level  string `json:"level"`
	Reason string `json:"reason"`
}

// AdminListAffiliates backs GET /admin/affiliates?status&level&q — the
// super_admin management list plus the approval queue.
func (h *Handler) AdminListAffiliates(c echo.Context) error {
	filter := service.AffiliateAdminListFilter{
		Status:  c.QueryParam("status"),
		Level:   c.QueryParam("level"),
		Query:   c.QueryParam("q"),
		Page:    1,
		PerPage: parseLimit(c.QueryParam("per_page"), 20, 50),
	}
	if p := c.QueryParam("page"); p != "" {
		n, err := strconv.Atoi(p)
		if err != nil || n < 1 {
			return badRequest(c, "page must be a positive integer")
		}
		filter.Page = n
	}
	list, err := h.svc.ListAffiliateAdmin(c.Request().Context(), filter)
	if err != nil {
		return mapServiceError(c, err)
	}
	return c.JSON(http.StatusOK, list)
}

// AdminDecideAffiliateApproval backs POST /admin/affiliates/:id/approval.
func (h *Handler) AdminDecideAffiliateApproval(c echo.Context) error {
	var req affiliateApprovalRequest
	if err := c.Bind(&req); err != nil {
		return badRequest(c, "invalid request body")
	}
	if req.Decision != "approve" && req.Decision != "reject" && req.Decision != "reopen" {
		return badRequest(c, "decision must be approve, reject or reopen")
	}
	claims := claimsFromContext(c)
	aff, err := h.svc.DecideAffiliateApproval(c.Request().Context(), claims.Sub, c.Param("id"), req.Decision, req.Reason)
	if err != nil {
		return mapServiceError(c, err)
	}
	return c.JSON(http.StatusOK, aff)
}

// AdminChangeAffiliateStatus backs PATCH /admin/affiliates/:id/status.
func (h *Handler) AdminChangeAffiliateStatus(c echo.Context) error {
	var req affiliateStatusRequest
	if err := c.Bind(&req); err != nil {
		return badRequest(c, "invalid request body")
	}
	if req.Status != "active" && req.Status != "suspended" && req.Status != "deactivated" {
		return badRequest(c, "status must be active, suspended or deactivated")
	}
	claims := claimsFromContext(c)
	aff, err := h.svc.ChangeAffiliateStatus(c.Request().Context(), claims.Sub, c.Param("id"), req.Status, req.Reason)
	if err != nil {
		return mapServiceError(c, err)
	}
	return c.JSON(http.StatusOK, aff)
}

// AdminChangeAffiliateLevel backs POST /admin/affiliates/:id/level.
func (h *Handler) AdminChangeAffiliateLevel(c echo.Context) error {
	var req affiliateLevelRequest
	if err := c.Bind(&req); err != nil {
		return badRequest(c, "invalid request body")
	}
	claims := claimsFromContext(c)
	aff, err := h.svc.ChangeAffiliateLevel(c.Request().Context(), claims.Sub, c.Param("id"), req.Level, req.Reason)
	if err != nil {
		return mapServiceError(c, err)
	}
	return c.JSON(http.StatusOK, aff)
}

// AdminCreateAffiliator backs POST /admin/affiliates: one affiliator added by an admin.
func (h *Handler) AdminCreateAffiliator(c echo.Context) error {
	var req service.AdminCreateAffiliatorInput
	if err := c.Bind(&req); err != nil {
		return badRequest(c, "invalid request body")
	}
	claims := claimsFromContext(c)
	created, err := h.svc.AdminCreateAffiliator(c.Request().Context(), claims.Sub, req)
	if err != nil {
		return mapServiceError(c, err)
	}
	return c.JSON(http.StatusCreated, created)
}

// AdminBulkCreateAffiliators backs POST /admin/affiliates/bulk: the parsed rows
// of an upload, each created independently.
func (h *Handler) AdminBulkCreateAffiliators(c echo.Context) error {
	var req struct {
		Rows []service.AdminCreateAffiliatorInput `json:"rows"`
	}
	if err := c.Bind(&req); err != nil || len(req.Rows) == 0 {
		return badRequest(c, "rows is required")
	}
	claims := claimsFromContext(c)
	results, err := h.svc.AdminBulkCreateAffiliators(c.Request().Context(), claims.Sub, req.Rows)
	if err != nil {
		return mapServiceError(c, err)
	}
	return c.JSON(http.StatusOK, map[string]any{"data": results})
}
