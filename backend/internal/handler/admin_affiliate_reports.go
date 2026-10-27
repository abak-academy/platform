package handler

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"akademi-bimbel/internal/repository"
	"akademi-bimbel/internal/service"

	"github.com/labstack/echo/v4"
)

// affiliateReportQuery reads the period, search and paging shared by every report.
func affiliateReportQuery(c echo.Context) (repository.AffiliateReportQuery, error) {
	from, to, err := parseDayRange(c.QueryParam("from"), c.QueryParam("to"))
	if err != nil {
		return repository.AffiliateReportQuery{}, err
	}
	if from != nil && to != nil && !from.Before(*to) {
		return repository.AffiliateReportQuery{}, fmt.Errorf("from must not be after to")
	}
	q := repository.AffiliateReportQuery{
		From:         from,
		To:           to,
		Q:            strings.TrimSpace(c.QueryParam("q")),
		InactiveOnly: c.QueryParam("inactive") == "true",
		Limit:        20,
	}
	if l := c.QueryParam("limit"); l != "" {
		if n, err := strconv.Atoi(l); err == nil && n > 0 {
			q.Limit = min(n, 100)
		}
	}
	if o := c.QueryParam("offset"); o != "" {
		if n, err := strconv.Atoi(o); err == nil && n > 0 {
			q.Offset = n
		}
	}
	return q, nil
}

func reportPage[T any](c echo.Context, rows []T, total int, q repository.AffiliateReportQuery, err error) error {
	if err != nil {
		return mapServiceError(c, err)
	}
	return c.JSON(http.StatusOK, map[string]any{"data": rows, "total": total, "limit": q.Limit, "offset": q.Offset})
}

// AdminAffiliatorReport serves GET /admin/system/affiliate-reports/affiliators.
func (h *Handler) AdminAffiliatorReport(c echo.Context) error {
	q, err := affiliateReportQuery(c)
	if err != nil {
		return badRequest(c, err.Error())
	}
	rows, total, err := h.svc.AffiliatorReport(c.Request().Context(), q)
	return reportPage(c, rows, total, q, err)
}

// AdminAffiliateSchoolReport serves .../schools.
func (h *Handler) AdminAffiliateSchoolReport(c echo.Context) error {
	q, err := affiliateReportQuery(c)
	if err != nil {
		return badRequest(c, err.Error())
	}
	rows, total, err := h.svc.AffiliateSchoolReport(c.Request().Context(), q)
	return reportPage(c, rows, total, q, err)
}

// AdminAffiliateProductReport serves .../products.
func (h *Handler) AdminAffiliateProductReport(c echo.Context) error {
	q, err := affiliateReportQuery(c)
	if err != nil {
		return badRequest(c, err.Error())
	}
	rows, total, err := h.svc.AffiliateProductReport(c.Request().Context(), q)
	return reportPage(c, rows, total, q, err)
}

// AdminAffiliatePromoReport serves .../promos.
func (h *Handler) AdminAffiliatePromoReport(c echo.Context) error {
	q, err := affiliateReportQuery(c)
	if err != nil {
		return badRequest(c, err.Error())
	}
	rows, total, err := h.svc.AffiliatePromoReport(c.Request().Context(), q)
	return reportPage(c, rows, total, q, err)
}

// AdminAffiliateTrendReport serves .../trend: the latest 24 months, no paging.
func (h *Handler) AdminAffiliateTrendReport(c echo.Context) error {
	q, err := affiliateReportQuery(c)
	if err != nil {
		return badRequest(c, err.Error())
	}
	rows, err := h.svc.AffiliateTrendReport(c.Request().Context(), q)
	if err != nil {
		return mapServiceError(c, err)
	}
	return c.JSON(http.StatusOK, map[string]any{"data": rows})
}

// AdminExportAffiliateReport serves .../:report/export as a CSV attachment.
func (h *Handler) AdminExportAffiliateReport(c echo.Context) error {
	name := c.Param("report")
	known := false
	for _, n := range service.AffiliateReportNames {
		known = known || n == name
	}
	if !known {
		return badRequest(c, "unknown report")
	}
	q, err := affiliateReportQuery(c)
	if err != nil {
		return badRequest(c, err.Error())
	}
	data, err := h.svc.ExportAffiliateReport(c.Request().Context(), name, q)
	if err != nil {
		return mapServiceError(c, err)
	}
	span := "semua-waktu"
	if from, to := c.QueryParam("from"), c.QueryParam("to"); from != "" || to != "" {
		span = from + "_" + to
	}
	c.Response().Header().Set(echo.HeaderContentDisposition, fmt.Sprintf(`attachment; filename="laporan-affiliator-%s-%s.csv"`, name, span))
	return c.Blob(http.StatusOK, "text/csv; charset=utf-8", data)
}
