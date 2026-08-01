package handler

import (
	"errors"
	"net/http"

	"akademi-bimbel/internal/service"

	"github.com/labstack/echo/v4"
)

// printTokenUnauthorized is the fixed 401 body every print-data auth failure
// returns: no token, a garbage token, an expired token, a second redemption,
// and a wrong-kind/wrong-subject token are all indistinguishable from the
// caller's side (RedeemPrintToken collapses them into one sentinel,
// NFR-S3). A normal user Bearer token is never even inspected — these routes
// sit outside every JWT-authenticated group in routes.go, so a print token in
// the "token" query parameter is the ONLY credential accepted here (FR-21).
func printTokenUnauthorized(c echo.Context) error {
	return c.JSON(http.StatusUnauthorized, APIError{Code: "unauthorized", Message: "invalid or expired print token"})
}

// PrintGetCertificateData serves the certificate print route's data (FR-18,
// FR-19, NFR-S2): every value the rendered certificate displays, authored
// server-side from the session and exam records. The print token is redeemed
// (single-use) before any data is resolved.
func (h *Handler) PrintGetCertificateData(c echo.Context) error {
	sessionID := c.Param("id")
	token := c.QueryParam("token")
	if token == "" {
		return printTokenUnauthorized(c)
	}
	if err := h.svc.RedeemPrintToken(c.Request().Context(), token, service.PrintTokenKindCertificate, sessionID); err != nil {
		return printTokenUnauthorized(c)
	}
	data, err := h.svc.GetCertificatePrintData(c.Request().Context(), sessionID)
	if err != nil {
		if errors.Is(err, service.ErrCertificateGateDenied) {
			return c.JSON(http.StatusForbidden, APIError{Code: "certificate_gate_denied", Message: err.Error()})
		}
		return mapServiceError(c, err)
	}
	return c.JSON(http.StatusOK, data)
}

// PrintGetCardData serves the exam card print route's data (FR-20, NFR-S2).
// The print token is redeemed (single-use) before any data is resolved.
func (h *Handler) PrintGetCardData(c echo.Context) error {
	regID := c.Param("id")
	token := c.QueryParam("token")
	if token == "" {
		return printTokenUnauthorized(c)
	}
	if err := h.svc.RedeemPrintToken(c.Request().Context(), token, service.PrintTokenKindCard, regID); err != nil {
		return printTokenUnauthorized(c)
	}
	data, err := h.svc.GetCardPrintData(c.Request().Context(), regID)
	if err != nil {
		return mapServiceError(c, err)
	}
	return c.JSON(http.StatusOK, data)
}
