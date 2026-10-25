package handler

import (
	"net/http"
	"strconv"

	"akademi-bimbel/internal/service"

	"github.com/labstack/echo/v4"
)

type adminAdjustmentRequest struct {
	Category string  `json:"category"`
	Amount   float64 `json:"amount"`
	Reason   string  `json:"reason"`
	Note     *string `json:"note"`
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
		Note:     req.Note,
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

// AdminListAffiliatePayouts backs GET /admin/affiliate-payouts — the payout
// queue, defaulting to the undecided statuses.
func (h *Handler) AdminListAffiliatePayouts(c echo.Context) error {
	limit, offset := adminMoneyPage(c)
	rows, total, err := h.svc.AdminListAffiliatePayouts(c.Request().Context(), c.QueryParam("status"), limit, offset)
	if err != nil {
		return mapServiceError(c, err)
	}
	return c.JSON(http.StatusOK, map[string]any{"data": rows, "total": total})
}

// AdminAffiliatePayoutsExport backs GET /admin/affiliate-payouts/export — the
// CSV of the undecided queue for the manual bank transfer.
func (h *Handler) AdminAffiliatePayoutsExport(c echo.Context) error {
	data, err := h.svc.AdminAffiliatePayoutsCSV(c.Request().Context())
	if err != nil {
		return mapServiceError(c, err)
	}
	c.Response().Header().Set(echo.HeaderContentDisposition, `attachment; filename="affiliate-payouts.csv"`)
	return c.Blob(http.StatusOK, "text/csv; charset=utf-8", data)
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
