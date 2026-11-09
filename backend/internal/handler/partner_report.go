package handler

import (
	"net/http"

	"akademi-bimbel/internal/repository"

	"github.com/labstack/echo/v4"
)

func partnerReportFilter(c echo.Context) (repository.PartnerReportFilter, error) {
	from, to, err := parseDayRange(c.QueryParam("from"), c.QueryParam("to"))
	if err != nil {
		return repository.PartnerReportFilter{}, err
	}
	return repository.PartnerReportFilter{
		PartnerID: c.QueryParam("partner_id"),
		Status:    c.QueryParam("status"),
		Q:         c.QueryParam("q"),
		From:      from,
		To:        to,
	}, nil
}

// AdminPartnerReport backs GET /admin/partner-reports/orders.
func (h *Handler) AdminPartnerReport(c echo.Context) error {
	filter, err := partnerReportFilter(c)
	if err != nil {
		return badRequest(c, err.Error())
	}
	page, perPage, err := adminQueuePaging(c)
	if err != nil {
		return badRequest(c, err.Error())
	}
	report, err := h.svc.AdminPartnerReport(c.Request().Context(), filter, page, perPage)
	if err != nil {
		return mapServiceError(c, err)
	}
	return c.JSON(http.StatusOK, report)
}

// AdminPartnerReportExport backs GET /admin/partner-reports/orders/export.
func (h *Handler) AdminPartnerReportExport(c echo.Context) error {
	filter, err := partnerReportFilter(c)
	if err != nil {
		return badRequest(c, err.Error())
	}
	data, err := h.svc.AdminPartnerReportCSV(c.Request().Context(), filter)
	if err != nil {
		return mapServiceError(c, err)
	}
	return csvDownload(c, "partner-report-orders.csv", data)
}
