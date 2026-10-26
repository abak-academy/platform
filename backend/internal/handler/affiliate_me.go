package handler

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	"akademi-bimbel/internal/service"
	"github.com/labstack/echo/v4"
)

// affiliateBankUpdateRequest is the body only the /affiliates/me/bank patch reads.
type affiliateBankUpdateRequest struct {
	BankName          string `json:"bank_name"`
	BankAccountNumber string `json:"bank_account_number"`
	BankAccountHolder string `json:"bank_account_holder"`
}

func (h *Handler) AffiliateMe(c echo.Context) error {
	claims := claimsFromContext(c)
	if claims == nil || claims.Sub == "" {
		return c.JSON(http.StatusUnauthorized, APIError{Code: "unauthorized", Message: "missing auth"})
	}
	profile, err := h.svc.GetMyAffiliateProfile(c.Request().Context(), claims.Sub)
	if err != nil {
		return mapServiceError(c, err)
	}
	return c.JSON(http.StatusOK, profile)
}

func (h *Handler) AffiliateMeUpdateBank(c echo.Context) error {
	claims := claimsFromContext(c)
	if claims == nil || claims.Sub == "" {
		return c.JSON(http.StatusUnauthorized, APIError{Code: "unauthorized", Message: "missing auth"})
	}
	var req affiliateBankUpdateRequest
	if err := c.Bind(&req); err != nil {
		return badRequest(c, "invalid request body")
	}
	profile, err := h.svc.UpdateMyAffiliateBank(c.Request().Context(), claims.Sub, req.BankName, req.BankAccountNumber, req.BankAccountHolder)
	if err != nil {
		return mapServiceError(c, err)
	}
	return c.JSON(http.StatusOK, profile)
}

func (h *Handler) AffiliateMeBindings(c echo.Context) error {
	filter, err := affiliateMeListQuery(c)
	if err != nil {
		return badRequest(c, err.Error())
	}
	claims := claimsFromContext(c)
	if claims == nil || claims.Sub == "" {
		return c.JSON(http.StatusUnauthorized, APIError{Code: "unauthorized", Message: "missing auth"})
	}
	page, err := h.svc.ListMyAffiliateBindings(c.Request().Context(), claims.Sub, filter)
	if err != nil {
		return mapServiceError(c, err)
	}
	return c.JSON(http.StatusOK, page)
}

func (h *Handler) AffiliateMeCommissions(c echo.Context) error {
	filter, err := affiliateMeListQuery(c)
	if err != nil {
		return badRequest(c, err.Error())
	}
	if err := validateOrderProductType(filter.ProductType); err != nil {
		return badRequest(c, err.Error())
	}
	claims := claimsFromContext(c)
	if claims == nil || claims.Sub == "" {
		return c.JSON(http.StatusUnauthorized, APIError{Code: "unauthorized", Message: "missing auth"})
	}
	page, err := h.svc.ListMyAffiliateCommissions(c.Request().Context(), claims.Sub, filter)
	if err != nil {
		return mapServiceError(c, err)
	}
	return c.JSON(http.StatusOK, page)
}

func (h *Handler) AffiliateMeLedger(c echo.Context) error {
	filter, err := affiliateMeListQuery(c)
	if err != nil {
		return badRequest(c, err.Error())
	}
	claims := claimsFromContext(c)
	if claims == nil || claims.Sub == "" {
		return c.JSON(http.StatusUnauthorized, APIError{Code: "unauthorized", Message: "missing auth"})
	}
	page, err := h.svc.GetMyAffiliateLedger(c.Request().Context(), claims.Sub, filter)
	if err != nil {
		return mapServiceError(c, err)
	}
	return c.JSON(http.StatusOK, page)
}

func (h *Handler) AffiliateMeStats(c echo.Context) error {
	claims := claimsFromContext(c)
	if claims == nil || claims.Sub == "" {
		return c.JSON(http.StatusUnauthorized, APIError{Code: "unauthorized", Message: "missing auth"})
	}
	stats, err := h.svc.GetMyAffiliateStats(c.Request().Context(), claims.Sub)
	if err != nil {
		return mapServiceError(c, err)
	}
	return c.JSON(http.StatusOK, stats)
}

// affiliateMePayoutRequest is the body only POST /affiliates/me/payouts reads;
// an empty body defaults the amount to the full available saldo.
type affiliateMePayoutRequest struct {
	Amount *float64 `json:"amount"`
}

// affiliateMeAppealRequest is the body only the violation appeal route reads.
type affiliateMeAppealRequest struct {
	Text string `json:"text"`
}

// AffiliateMeRequestPayout backs POST /affiliates/me/payouts (BR-PCR-01..05).
func (h *Handler) AffiliateMeRequestPayout(c echo.Context) error {
	claims := claimsFromContext(c)
	if claims == nil || claims.Sub == "" {
		return c.JSON(http.StatusUnauthorized, APIError{Code: "unauthorized", Message: "missing auth"})
	}
	var req affiliateMePayoutRequest
	if err := c.Bind(&req); err != nil {
		return badRequest(c, "invalid request body")
	}
	if req.Amount != nil && *req.Amount <= 0 {
		return badRequest(c, "amount must be positive")
	}
	payout, err := h.svc.RequestMyAffiliatePayout(c.Request().Context(), claims.Sub, req.Amount)
	if err != nil {
		return mapServiceError(c, err)
	}
	return c.JSON(http.StatusCreated, payout)
}

// AffiliateMePayouts backs GET /affiliates/me/payouts — own payout history
// newest first with the saldo and the payout schedule.
func (h *Handler) AffiliateMePayouts(c echo.Context) error {
	filter, err := affiliateMeListQuery(c)
	if err != nil {
		return badRequest(c, err.Error())
	}
	claims := claimsFromContext(c)
	if claims == nil || claims.Sub == "" {
		return c.JSON(http.StatusUnauthorized, APIError{Code: "unauthorized", Message: "missing auth"})
	}
	page, err := h.svc.ListMyAffiliatePayouts(c.Request().Context(), claims.Sub, filter)
	if err != nil {
		return mapServiceError(c, err)
	}
	return c.JSON(http.StatusOK, page)
}

// AffiliateMeAdjustments backs GET /affiliates/me/adjustments — own adjustment
// history with the applied flag.
func (h *Handler) AffiliateMeAdjustments(c echo.Context) error {
	filter, err := affiliateMeListQuery(c)
	if err != nil {
		return badRequest(c, err.Error())
	}
	claims := claimsFromContext(c)
	if claims == nil || claims.Sub == "" {
		return c.JSON(http.StatusUnauthorized, APIError{Code: "unauthorized", Message: "missing auth"})
	}
	page, err := h.svc.ListMyAffiliateAdjustments(c.Request().Context(), claims.Sub, filter)
	if err != nil {
		return mapServiceError(c, err)
	}
	return c.JSON(http.StatusOK, page)
}

// AffiliateMeViolations backs GET /affiliates/me/violations — own violations
// with sanction and appeal fields.
func (h *Handler) AffiliateMeViolations(c echo.Context) error {
	filter, err := affiliateMeListQuery(c)
	if err != nil {
		return badRequest(c, err.Error())
	}
	claims := claimsFromContext(c)
	if claims == nil || claims.Sub == "" {
		return c.JSON(http.StatusUnauthorized, APIError{Code: "unauthorized", Message: "missing auth"})
	}
	page, err := h.svc.ListMyAffiliateViolations(c.Request().Context(), claims.Sub, filter)
	if err != nil {
		return mapServiceError(c, err)
	}
	return c.JSON(http.StatusOK, page)
}

// AffiliateMeAppealViolation backs POST /affiliates/me/violations/:id/appeal
// (BR-PLG-06).
func (h *Handler) AffiliateMeAppealViolation(c echo.Context) error {
	claims := claimsFromContext(c)
	if claims == nil || claims.Sub == "" {
		return c.JSON(http.StatusUnauthorized, APIError{Code: "unauthorized", Message: "missing auth"})
	}
	var req affiliateMeAppealRequest
	if err := c.Bind(&req); err != nil {
		return badRequest(c, "invalid request body")
	}
	violation, err := h.svc.AppealMyAffiliateViolation(c.Request().Context(), claims.Sub, c.Param("id"), req.Text)
	if err != nil {
		return mapServiceError(c, err)
	}
	return c.JSON(http.StatusOK, violation)
}

// affiliateSearchText trims the search box text and caps it so a pasted
// paragraph cannot become a huge ILIKE pattern.
func affiliateSearchText(raw string) string {
	text := strings.TrimSpace(raw)
	if runes := []rune(text); len(runes) > 100 {
		text = string(runes[:100])
	}
	return text
}

// affiliateMeListQuery parses the shared list query params: page/per_page
// (per_page clamped like parseLimit), optional status filter and a half-open
// from/to date range (parseDayRange).
func affiliateMeListQuery(c echo.Context) (service.AffiliateMeListFilter, error) {
	filter := service.AffiliateMeListFilter{
		Page:        1,
		PerPage:     parseLimit(c.QueryParam("per_page"), 20, 50),
		Query:       affiliateSearchText(c.QueryParam("q")),
		Type:        c.QueryParam("type"),
		Status:      c.QueryParam("status"),
		ProductType: c.QueryParam("product_type"),
		Order:       strings.ToLower(c.QueryParam("order")),
	}
	if order := filter.Order; order != "" && order != "asc" && order != "desc" {
		return filter, errors.New("order must be asc or desc")
	}
	if p := c.QueryParam("page"); p != "" {
		n, err := strconv.Atoi(p)
		if err != nil || n < 1 {
			return filter, errors.New("page must be a positive integer")
		}
		filter.Page = n
	}
	from, to, err := parseDayRange(c.QueryParam("from"), c.QueryParam("to"))
	if err != nil {
		return filter, err
	}
	filter.From, filter.To = from, to
	return filter, nil
}
