package handler

import (
	"net/http"
	"time"

	"github.com/labstack/echo/v4"
)

// AdminDashboard serves the super-admin dashboard in one request.
//
// One endpoint rather than four: the page shows nine aggregates on one screen,
// and separate calls mean separate spinners resolving at different times.
func (h *Handler) AdminDashboard(c echo.Context) error {
	// parseDayRange is shared with the order list and revenue report: dates are
	// read in Asia/Jakarta and `to` is advanced a day, because the queries are
	// half-open on created_at.
	fromParam, toParam, err := parseDayRange(c.QueryParam("from"), c.QueryParam("to"))
	if err != nil {
		return badRequest(c, err.Error())
	}

	bucket := c.QueryParam("bucket")
	if bucket != "" && bucket != "day" && bucket != "week" {
		return badRequest(c, "bucket must be day or week")
	}

	now := time.Now().In(jakarta)
	from := now.AddDate(0, 0, -30)
	if fromParam != nil {
		from = *fromParam
	}
	to := now
	if toParam != nil {
		to = *toParam
	}

	resp, err := h.svc.AdminDashboard(c.Request().Context(), from, to, bucket)
	if err != nil {
		return mapServiceError(c, err)
	}
	return c.JSON(http.StatusOK, resp)
}
