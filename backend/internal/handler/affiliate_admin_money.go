package handler

import (
	"errors"
	"net/http"
	"strconv"

	"akademi-bimbel/internal/service"

	"github.com/labstack/echo/v4"
)

type adminAdjustmentRequest struct {
	Category string  `json:"category"`
	Amount   float64 `json:"amount"`
	Reason   string  `json:"reason"`
}

// AdminCreateAffiliateAdjustment backs POST /admin/affiliates/:id/adjustments.
func (h *Handler) AdminCreateAffiliateAdjustment(c echo.Context) error {
	actorID, ok := actorFromClaims(c)
	if !ok {
		return c.JSON(http.StatusUnauthorized, APIError{Code: "unauthorized", Message: "missing auth"})
	}
	var req adminAdjustmentRequest
	if err := c.Bind(&req); err != nil {
		return badRequest(c, "invalid request body")
	}
	adj, err := h.svc.AdminCreateAffiliateAdjustment(c.Request().Context(), actorID, c.Param("id"), service.AdminAdjustmentInput{
		Category: req.Category,
		Amount:   req.Amount,
		Reason:   req.Reason,
	})
	if err != nil {
		return mapServiceError(c, err)
	}
	return c.JSON(http.StatusCreated, adj)
}

// AdminListAffiliateAdjustments backs GET /admin/affiliates/:id/adjustments.
func (h *Handler) AdminListAffiliateAdjustments(c echo.Context) error {
	limit, offset := adminMoneyPage(c)
	rows, total, err := h.svc.AdminListAffiliateAdjustments(c.Request().Context(), c.Param("id"), limit, offset)
	if err != nil {
		return mapServiceError(c, err)
	}
	return c.JSON(http.StatusOK, map[string]any{"data": rows, "total": total})
}

// adminAffiliateQueueQuery parses the list query shared by the payout, flag and
// appeal queues: q, status, from/to, page and per_page (limit is the legacy name).
func adminAffiliateQueueQuery(c echo.Context) (service.AffiliateAdminQueueFilter, int, int, error) {
	from, to, err := parseDayRange(c.QueryParam("from"), c.QueryParam("to"))
	if err != nil {
		return service.AffiliateAdminQueueFilter{}, 0, 0, err
	}
	page, perPage, err := adminQueuePaging(c)
	if err != nil {
		return service.AffiliateAdminQueueFilter{}, 0, 0, err
	}
	return service.AffiliateAdminQueueFilter{
		Q:      affiliateSearchText(c.QueryParam("q")),
		Status: c.QueryParam("status"),
		Type:   c.QueryParam("type"),
		From:   from,
		To:     to,
		Limit:  perPage,
		Offset: (page - 1) * perPage,
	}, page, perPage, nil
}

// adminQueuePaging reads page and per_page (limit is the legacy name).
func adminQueuePaging(c echo.Context) (page, perPage int, err error) {
	raw := c.QueryParam("per_page")
	if raw == "" {
		raw = c.QueryParam("limit")
	}
	perPage = parseLimit(raw, 20, 100)
	page = 1
	if p := c.QueryParam("page"); p != "" {
		n, convErr := strconv.Atoi(p)
		if convErr != nil || n < 1 {
			return 0, 0, errors.New("page must be a positive integer")
		}
		page = n
	}
	return page, perPage, nil
}

func queuePage(rows any, total, page, perPage int) map[string]any {
	return map[string]any{"data": rows, "total": total, "page": page, "per_page": perPage}
}

func csvDownload(c echo.Context, filename string, data []byte) error {
	c.Response().Header().Set(echo.HeaderContentDisposition, `attachment; filename="`+filename+`"`)
	return c.Blob(http.StatusOK, "text/csv; charset=utf-8", data)
}

// AdminListAffiliatePayouts backs GET /admin/affiliate-payouts — without a
// status the undecided queue, status=all for every payout.
func (h *Handler) AdminListAffiliatePayouts(c echo.Context) error {
	filter, page, perPage, err := adminAffiliateQueueQuery(c)
	if err != nil {
		return badRequest(c, err.Error())
	}
	rows, total, err := h.svc.AdminListAffiliatePayouts(c.Request().Context(), filter)
	if err != nil {
		return mapServiceError(c, err)
	}
	return c.JSON(http.StatusOK, queuePage(rows, total, page, perPage))
}

// AdminAffiliatePayoutsExport backs GET /admin/affiliate-payouts/export — the
// CSV of the payouts matching the same filters as the list (the undecided queue
// by default) for the manual bank transfer.
func (h *Handler) AdminAffiliatePayoutsExport(c echo.Context) error {
	filter, _, _, err := adminAffiliateQueueQuery(c)
	if err != nil {
		return badRequest(c, err.Error())
	}
	data, err := h.svc.AdminAffiliatePayoutsCSV(c.Request().Context(), filter)
	if err != nil {
		return mapServiceError(c, err)
	}
	return csvDownload(c, "affiliate-payouts.csv", data)
}

type adminPayoutDecisionRequest struct {
	Decision string `json:"decision"`
	ProofURL string `json:"proof_url"`
	Reason   string `json:"reason"`
}

// AdminDecideAffiliatePayout backs POST /admin/affiliate-payouts/:id/decide.
func (h *Handler) AdminDecideAffiliatePayout(c echo.Context) error {
	actorID, ok := actorFromClaims(c)
	if !ok {
		return c.JSON(http.StatusUnauthorized, APIError{Code: "unauthorized", Message: "missing auth"})
	}
	var req adminPayoutDecisionRequest
	if err := c.Bind(&req); err != nil {
		return badRequest(c, "invalid request body")
	}
	if req.Decision == "paid" && !validPaymentProofKey(req.ProofURL) {
		return badRequest(c, "proof_url is required and must be a payment_proof/ upload key")
	}
	payout, err := h.svc.AdminDecideAffiliatePayout(c.Request().Context(), actorID, c.Param("id"), service.AdminPayoutDecisionInput{
		Decision: req.Decision,
		ProofURL: req.ProofURL,
		Reason:   req.Reason,
	})
	if err != nil {
		return mapServiceError(c, err)
	}
	return c.JSON(http.StatusOK, payout)
}

// AdminProcessAffiliatePayout backs POST /admin/affiliate-payouts/:id/process.
func (h *Handler) AdminProcessAffiliatePayout(c echo.Context) error {
	actorID, ok := actorFromClaims(c)
	if !ok {
		return c.JSON(http.StatusUnauthorized, APIError{Code: "unauthorized", Message: "missing auth"})
	}
	payout, err := h.svc.AdminProcessAffiliatePayout(c.Request().Context(), actorID, c.Param("id"))
	if err != nil {
		return mapServiceError(c, err)
	}
	return c.JSON(http.StatusOK, payout)
}

// adminMoneyPage parses the page/limit query pair (1-based page, default 20,
// hard cap 100) into limit + offset.
func adminMoneyPage(c echo.Context) (limit, offset int) {
	limit = 20
	if l := c.QueryParam("limit"); l != "" {
		if n, err := strconv.Atoi(l); err == nil && n > 0 {
			limit = n
		}
	}
	if limit > 100 {
		limit = 100
	}
	page := 1
	if p := c.QueryParam("page"); p != "" {
		if n, err := strconv.Atoi(p); err == nil && n > 0 {
			page = n
		}
	}
	return limit, (page - 1) * limit
}
