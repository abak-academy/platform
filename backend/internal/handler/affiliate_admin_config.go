package handler

import (
	"net/http"

	"akademi-bimbel/internal/service"

	"github.com/labstack/echo/v4"
)

type adminAffiliateOverrideRequest struct {
	Percentage float64 `json:"percentage"`
}

type adminAffiliateFlagReviewRequest struct {
	Status string  `json:"status"`
	Note   *string `json:"note"`
}

type adminAffiliateAppealDecisionRequest struct {
	Outcome string  `json:"outcome"`
	Note    *string `json:"note"`
}

// AdminGetAffiliateSettings backs GET /admin/affiliate-settings.
func (h *Handler) AdminGetAffiliateSettings(c echo.Context) error {
	view, err := h.svc.GetAffiliateAdminSettings(c.Request().Context())
	if err != nil {
		return mapServiceError(c, err)
	}
	return c.JSON(http.StatusOK, view)
}

// AdminUpdateAffiliateSettings backs PUT /admin/affiliate-settings.
func (h *Handler) AdminUpdateAffiliateSettings(c echo.Context) error {
	claims := claimsFromContext(c)
	var in service.AffiliateAdminSettingsInput
	if err := c.Bind(&in); err != nil {
		return badRequest(c, "invalid request body")
	}
	view, err := h.svc.UpdateAffiliateAdminSettings(c.Request().Context(), claims.Sub, in)
	if err != nil {
		return mapServiceError(c, err)
	}
	return c.JSON(http.StatusOK, view)
}

// AdminListAffiliateOverrides backs GET /admin/affiliate-product-overrides?q=&page=&per_page=.
func (h *Handler) AdminListAffiliateOverrides(c echo.Context) error {
	page, perPage, err := adminQueuePaging(c)
	if err != nil {
		return badRequest(c, err.Error())
	}
	rows, total, err := h.svc.AdminListAffiliateOverrides(c.Request().Context(), affiliateSearchText(c.QueryParam("q")), perPage, (page-1)*perPage)
	if err != nil {
		return mapServiceError(c, err)
	}
	return c.JSON(http.StatusOK, queuePage(rows, total, page, perPage))
}

// AdminSearchAffiliateOverrideProducts backs GET /admin/affiliate-product-overrides/products?q=.
func (h *Handler) AdminSearchAffiliateOverrideProducts(c echo.Context) error {
	rows, err := h.svc.AdminSearchAffiliateOverrideProducts(c.Request().Context(), affiliateSearchText(c.QueryParam("q")), parseLimit(c.QueryParam("limit"), 10, 20))
	if err != nil {
		return mapServiceError(c, err)
	}
	return c.JSON(http.StatusOK, map[string]any{"data": rows})
}

// AdminUpsertAffiliateOverride backs PUT /admin/affiliate-product-overrides/:product_id.
func (h *Handler) AdminUpsertAffiliateOverride(c echo.Context) error {
	actorID, ok := actorFromClaims(c)
	if !ok {
		return c.JSON(http.StatusUnauthorized, APIError{Code: "unauthorized", Message: "missing auth"})
	}
	var req adminAffiliateOverrideRequest
	if err := c.Bind(&req); err != nil {
		return badRequest(c, "invalid request body")
	}
	override, err := h.svc.AdminUpsertAffiliateOverride(c.Request().Context(), actorID, c.Param("product_id"), req.Percentage)
	if err != nil {
		return mapServiceError(c, err)
	}
	return c.JSON(http.StatusOK, override)
}

// AdminDeleteAffiliateOverride backs DELETE /admin/affiliate-product-overrides/:product_id.
func (h *Handler) AdminDeleteAffiliateOverride(c echo.Context) error {
	actorID, ok := actorFromClaims(c)
	if !ok {
		return c.JSON(http.StatusUnauthorized, APIError{Code: "unauthorized", Message: "missing auth"})
	}
	if err := h.svc.AdminDeleteAffiliateOverride(c.Request().Context(), actorID, c.Param("product_id")); err != nil {
		return mapServiceError(c, err)
	}
	return c.JSON(http.StatusOK, map[string]any{"deleted": true})
}

// AdminListAffiliateFlags backs GET /admin/affiliate-flags; no status (or all)
// lists every flag.
func (h *Handler) AdminListAffiliateFlags(c echo.Context) error {
	filter, page, perPage, err := adminAffiliateQueueQuery(c)
	if err != nil {
		return badRequest(c, err.Error())
	}
	rows, total, err := h.svc.AdminListAffiliateFlags(c.Request().Context(), filter)
	if err != nil {
		return mapServiceError(c, err)
	}
	return c.JSON(http.StatusOK, queuePage(rows, total, page, perPage))
}

// AdminAffiliateFlagsExport backs GET /admin/affiliate-flags/export.
func (h *Handler) AdminAffiliateFlagsExport(c echo.Context) error {
	filter, _, _, err := adminAffiliateQueueQuery(c)
	if err != nil {
		return badRequest(c, err.Error())
	}
	data, err := h.svc.AdminAffiliateFlagsCSV(c.Request().Context(), filter)
	if err != nil {
		return mapServiceError(c, err)
	}
	return csvDownload(c, "affiliate-flags.csv", data)
}

// AdminReviewAffiliateFlag backs POST /admin/affiliate-flags/:id/review.
func (h *Handler) AdminReviewAffiliateFlag(c echo.Context) error {
	actorID, ok := actorFromClaims(c)
	if !ok {
		return c.JSON(http.StatusUnauthorized, APIError{Code: "unauthorized", Message: "missing auth"})
	}
	var req adminAffiliateFlagReviewRequest
	if err := c.Bind(&req); err != nil {
		return badRequest(c, "invalid request body")
	}
	flag, err := h.svc.AdminReviewAffiliateFlag(c.Request().Context(), actorID, c.Param("id"), service.AffiliateAdminFlagReviewInput{
		Status: req.Status,
		Note:   req.Note,
	})
	if err != nil {
		return mapServiceError(c, err)
	}
	return c.JSON(http.StatusOK, flag)
}

// AdminListAffiliateAppeals backs GET /admin/affiliate-appeals; no status (or
// all) lists every appeal.
func (h *Handler) AdminListAffiliateAppeals(c echo.Context) error {
	filter, page, perPage, err := adminAffiliateQueueQuery(c)
	if err != nil {
		return badRequest(c, err.Error())
	}
	rows, total, err := h.svc.AdminListAffiliateAppeals(c.Request().Context(), filter)
	if err != nil {
		return mapServiceError(c, err)
	}
	return c.JSON(http.StatusOK, queuePage(rows, total, page, perPage))
}

// AdminAffiliateAppealsExport backs GET /admin/affiliate-appeals/export.
func (h *Handler) AdminAffiliateAppealsExport(c echo.Context) error {
	filter, _, _, err := adminAffiliateQueueQuery(c)
	if err != nil {
		return badRequest(c, err.Error())
	}
	data, err := h.svc.AdminAffiliateAppealsCSV(c.Request().Context(), filter)
	if err != nil {
		return mapServiceError(c, err)
	}
	return csvDownload(c, "affiliate-appeals.csv", data)
}

// AdminDecideAffiliateAppeal backs POST /admin/affiliate-appeals/:id/decide.
func (h *Handler) AdminDecideAffiliateAppeal(c echo.Context) error {
	actorID, ok := actorFromClaims(c)
	if !ok {
		return c.JSON(http.StatusUnauthorized, APIError{Code: "unauthorized", Message: "missing auth"})
	}
	var req adminAffiliateAppealDecisionRequest
	if err := c.Bind(&req); err != nil {
		return badRequest(c, "invalid request body")
	}
	violation, err := h.svc.AdminDecideAffiliateAppeal(c.Request().Context(), actorID, c.Param("id"), service.AffiliateAdminAppealDecisionInput{
		Outcome: req.Outcome,
		Note:    req.Note,
	})
	if err != nil {
		return mapServiceError(c, err)
	}
	return c.JSON(http.StatusOK, violation)
}

// AdminAffiliateReportSummary backs GET /admin/affiliate-reports/summary?from&to,
// optionally narrowed by q and level and paged with page and per_page.
func (h *Handler) AdminAffiliateReportSummary(c echo.Context) error {
	page, perPage, err := adminQueuePaging(c)
	if err != nil {
		return badRequest(c, err.Error())
	}
	report, err := h.svc.AdminAffiliateReportSummary(c.Request().Context(), c.QueryParam("from"), c.QueryParam("to"), service.AffiliateReportOptions{
		Q:     affiliateSearchText(c.QueryParam("q")),
		Level: c.QueryParam("level"),
		Page:  page,
		Limit: perPage,
	})
	if err != nil {
		return mapServiceError(c, err)
	}
	return c.JSON(http.StatusOK, report)
}

// AdminAffiliateReportLiability backs GET /admin/affiliate-reports/liability.
func (h *Handler) AdminAffiliateReportLiability(c echo.Context) error {
	report, err := h.svc.AdminAffiliateReportLiability(c.Request().Context())
	if err != nil {
		return mapServiceError(c, err)
	}
	return c.JSON(http.StatusOK, report)
}

// AdminListAffiliateCommissions backs GET /admin/affiliates/:id/commissions.
func (h *Handler) AdminListAffiliateCommissions(c echo.Context) error {
	filter, page, perPage, err := adminAffiliateQueueQuery(c)
	if err != nil {
		return badRequest(c, err.Error())
	}
	rows, total, err := h.svc.AdminListAffiliateCommissions(c.Request().Context(), c.Param("id"), filter)
	if err != nil {
		return mapServiceError(c, err)
	}
	return c.JSON(http.StatusOK, queuePage(rows, total, page, perPage))
}

// AdminAffiliateCommissionsExport backs GET /admin/affiliates/:id/commissions/export.
func (h *Handler) AdminAffiliateCommissionsExport(c echo.Context) error {
	filter, _, _, err := adminAffiliateQueueQuery(c)
	if err != nil {
		return badRequest(c, err.Error())
	}
	data, err := h.svc.AdminAffiliateCommissionsCSV(c.Request().Context(), c.Param("id"), filter)
	if err != nil {
		return mapServiceError(c, err)
	}
	return csvDownload(c, "affiliate-commissions.csv", data)
}

// AdminListAffiliateReportOrders backs GET /admin/affiliate-reports/orders.
func (h *Handler) AdminListAffiliateReportOrders(c echo.Context) error {
	filter, page, perPage, err := adminAffiliateQueueQuery(c)
	if err != nil {
		return badRequest(c, err.Error())
	}
	rows, total, err := h.svc.AdminListAffiliateReportOrders(c.Request().Context(), filter)
	if err != nil {
		return mapServiceError(c, err)
	}
	return c.JSON(http.StatusOK, queuePage(rows, total, page, perPage))
}

// AdminAffiliateReportOrdersExport backs GET /admin/affiliate-reports/orders/export.
func (h *Handler) AdminAffiliateReportOrdersExport(c echo.Context) error {
	filter, _, _, err := adminAffiliateQueueQuery(c)
	if err != nil {
		return badRequest(c, err.Error())
	}
	data, err := h.svc.AdminAffiliateReportOrdersCSV(c.Request().Context(), filter)
	if err != nil {
		return mapServiceError(c, err)
	}
	return csvDownload(c, "affiliate-report-orders.csv", data)
}
