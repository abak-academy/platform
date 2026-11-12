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
	// Programs lets the admin change the requested program on approve.
	Programs []string `json:"programs"`
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
		Program: c.QueryParam("program"),
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
	aff, err := h.svc.DecideAffiliateApproval(c.Request().Context(), claims.Sub, c.Param("id"), req.Decision, req.Reason, req.Programs)
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

// AdminAffiliatorsExport backs GET /admin/affiliates/export: the CSV of the
// affiliators matching the list filters. The level arrives as "type", like the
// other admin exports.
func (h *Handler) AdminAffiliatorsExport(c echo.Context) error {
	level := c.QueryParam("type")
	if level == "" {
		level = c.QueryParam("level")
	}
	data, err := h.svc.AdminAffiliatorsCSV(c.Request().Context(), service.AffiliateAdminListFilter{
		Status: c.QueryParam("status"),
		Level:  level,
		Query:  c.QueryParam("q"),
	})
	if err != nil {
		return mapServiceError(c, err)
	}
	return csvDownload(c, "affiliators.csv", data)
}

// AdminAddAffiliatorProgram backs POST /admin/affiliates/:id/programs: it adds
// one program to an approved account and never removes one.
func (h *Handler) AdminAddAffiliatorProgram(c echo.Context) error {
	var req struct {
		Program string `json:"program"`
	}
	if err := c.Bind(&req); err != nil {
		return badRequest(c, "invalid request body")
	}
	claims := claimsFromContext(c)
	aff, err := h.svc.AdminAddAffiliatorProgram(c.Request().Context(), claims.Sub, c.Param("id"), req.Program)
	if err != nil {
		return mapServiceError(c, err)
	}
	return c.JSON(http.StatusOK, aff)
}

// AdminSetPartnerSchools backs PUT /admin/affiliates/:id/partner/schools.
func (h *Handler) AdminSetPartnerSchools(c echo.Context) error {
	var req struct {
		SchoolIDs []string `json:"school_ids"`
	}
	if err := c.Bind(&req); err != nil {
		return badRequest(c, "invalid request body")
	}
	claims := claimsFromContext(c)
	aff, err := h.svc.AdminSetPartnerSchools(c.Request().Context(), claims.Sub, c.Param("id"), req.SchoolIDs)
	if err != nil {
		return mapServiceError(c, err)
	}
	return c.JSON(http.StatusOK, aff)
}

// AdminSetPartnerProgram backs PUT /admin/partner-settings/program: the switch
// of the Mitra program, separate from the Affiliator switch.
func (h *Handler) AdminSetPartnerProgram(c echo.Context) error {
	var req struct {
		Enabled bool `json:"enabled"`
	}
	if err := c.Bind(&req); err != nil {
		return badRequest(c, "invalid request body")
	}
	claims := claimsFromContext(c)
	if err := h.svc.SetPartnerProgramEnabled(c.Request().Context(), claims.Sub, req.Enabled); err != nil {
		return mapServiceError(c, err)
	}
	return c.JSON(http.StatusOK, map[string]bool{"program_enabled": req.Enabled})
}

// PartnerMe backs GET /partners/me: the Mitra's own program standing and
// schools, for the portal menu.
func (h *Handler) PartnerMe(c echo.Context) error {
	states := partnerStatesFromContext(c)
	return c.JSON(http.StatusOK, map[string]any{
		"states":          states,
		"program_enabled": h.svc.PartnerProgramEnabled(c.Request().Context()),
	})
}

// AdminGetProductPartnerAccess backs GET /admin/products/:id/partner-access.
func (h *Handler) AdminGetProductPartnerAccess(c echo.Context) error {
	access, err := h.svc.AdminGetProductPartnerAccess(c.Request().Context(), c.Param("id"))
	if err != nil {
		return mapServiceError(c, err)
	}
	return c.JSON(http.StatusOK, access)
}

// AdminSetProductPartnerAccess backs PUT /admin/products/:id/partner-access. It
// is separate from the product update so saving the product form cannot
// overwrite which Mitra can see the product.
func (h *Handler) AdminSetProductPartnerAccess(c echo.Context) error {
	var req struct {
		Scope string   `json:"scope"`
		IDs   []string `json:"ids"`
	}
	if err := c.Bind(&req); err != nil {
		return badRequest(c, "invalid request body")
	}
	claims := claimsFromContext(c)
	access, err := h.svc.AdminSetProductPartnerAccess(c.Request().Context(), claims.Sub, c.Param("id"), req.Scope, req.IDs)
	if err != nil {
		return mapServiceError(c, err)
	}
	return c.JSON(http.StatusOK, access)
}
