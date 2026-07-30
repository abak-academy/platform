package handler

import (
	"encoding/json"
	"io"
	"net/http"

	"github.com/labstack/echo/v4"
)

type midtransWebhookBody struct {
	SignatureKey  string `json:"signature_key"`
	TransactionID string `json:"transaction_id"`
}

func (h *Handler) HandlePaymentWebhook(c echo.Context) error {
	payload, err := io.ReadAll(c.Request().Body)
	if err != nil {
		return badRequest(c, "invalid request body")
	}

	var body midtransWebhookBody
	if err := json.Unmarshal(payload, &body); err != nil {
		return badRequest(c, "invalid request body")
	}
	if body.TransactionID == "" {
		return badRequest(c, "transaction_id is required")
	}

	err = h.svc.HandlePaymentWebhook(c.Request().Context(), payload, body.SignatureKey, body.TransactionID)
	if err != nil {
		return mapServiceError(c, err)
	}

	return c.JSON(http.StatusOK, map[string]string{
		"message": "webhook processed",
	})
}

// HandleShippingWebhook reads the raw body so the signature is compared
// against exactly what Biteship sent (bind-and-re-marshal would drift from
// that byte sequence). Verification happens inside the service call — this
// handler never treats the request as authenticated on its own.
func (h *Handler) HandleShippingWebhook(c echo.Context) error {
	payload, err := io.ReadAll(c.Request().Body)
	if err != nil {
		return badRequest(c, "invalid request body")
	}

	signature := c.Request().Header.Get("X-Biteship-Signature")

	err = h.svc.HandleShippingWebhook(c.Request().Context(), payload, signature)
	if err != nil {
		return mapServiceError(c, err)
	}

	return c.JSON(http.StatusOK, map[string]string{
		"message": "webhook processed",
	})
}
