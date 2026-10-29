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
	Days   *int   `json:"days"`
}

type affiliateFreezeRequest struct {
	Days   *int   `json:"days"`
	Reason string `json:"reason"`
}

type affiliateLevelRequest struct {
	Level     string `json:"level"`
	LockUntil string `json:"lock_until"`
	Reason    string `json:"reason"`
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
	if req.Decision != "approve" && req.Decision != "reject" {
		return badRequest(c, "decision must be approve or reject")
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
	if req.Status == "suspended" && req.Days == nil {
		return badRequest(c, "days is required to suspend")
	}
	claims := claimsFromContext(c)
	aff, err := h.svc.ChangeAffiliateStatus(c.Request().Context(), claims.Sub, c.Param("id"), req.Status, req.Reason, req.Days)
	if err != nil {
		return mapServiceError(c, err)
	}
	return c.JSON(http.StatusOK, aff)
}

// AdminFreezeAffiliateBalance backs POST /admin/affiliates/:id/freeze.
func (h *Handler) AdminFreezeAffiliateBalance(c echo.Context) error {
	var req affiliateFreezeRequest
	if err := c.Bind(&req); err != nil {
		return badRequest(c, "invalid request body")
	}
	if req.Days == nil {
		return badRequest(c, "days is required")
	}
	claims := claimsFromContext(c)
	aff, err := h.svc.FreezeAffiliateBalance(c.Request().Context(), claims.Sub, c.Param("id"), *req.Days, req.Reason)
	if err != nil {
		return mapServiceError(c, err)
	}
	return c.JSON(http.StatusOK, aff)
}

// AdminUnfreezeAffiliateBalance backs POST /admin/affiliates/:id/unfreeze.
func (h *Handler) AdminUnfreezeAffiliateBalance(c echo.Context) error {
	var req affiliateFreezeRequest
	if err := c.Bind(&req); err != nil {
		return badRequest(c, "invalid request body")
	}
	claims := claimsFromContext(c)
	aff, err := h.svc.UnfreezeAffiliateBalance(c.Request().Context(), claims.Sub, c.Param("id"), req.Reason)
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
	aff, err := h.svc.ChangeAffiliateLevel(c.Request().Context(), claims.Sub, c.Param("id"), req.Level, req.LockUntil, req.Reason)
	if err != nil {
		return mapServiceError(c, err)
	}
	return c.JSON(http.StatusOK, aff)
}
