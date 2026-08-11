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

	// Default window mirrors parseDayRange's own convention: a midnight-aligned
	// `to` advanced one day, so an unparameterized request (the frontend's
	// default 30-day preset) includes all of today, not everything up to the
	// instant the request happened to land.
	//
	// `to` is today+1 (exclusive), so a window of exactly 30 days must start at
	// today-29, not today-30 — the latter yields 31 days.
	now := time.Now().In(jakarta)
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, jakarta)
	from := today.AddDate(0, 0, -29)
	if fromParam != nil {
		from = *fromParam
	}
	to := today.AddDate(0, 0, 1)
	if toParam != nil {
		to = *toParam
	}

	// parseDayRange never compares from/to itself, and an inverted or empty
	// window (from >= to) makes generate_series produce zero rows — every
	// aggregate silently comes back empty instead of failing loudly.
	if !from.Before(to) {
		return badRequest(c, "from must be before to")
	}

	resp, err := h.svc.AdminDashboard(c.Request().Context(), from, to, bucket)
	if err != nil {
		return mapServiceError(c, err)
	}
	return c.JSON(http.StatusOK, resp)
}
