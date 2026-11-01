package handler

import (
	"fmt"
	"net/http"
	"time"

	"akademi-bimbel/internal/model"
	"akademi-bimbel/internal/service"
	"github.com/labstack/echo/v4"
)

func unitSalesRange(c echo.Context) (*time.Time, *time.Time, error) {
	from, to, err := parseDayRange(c.QueryParam("from"), c.QueryParam("to"))
	if err != nil {
		return nil, nil, err
	}
	if from != nil && to != nil && !from.Before(*to) {
		return nil, nil, fmt.Errorf("from must not be after to")
	}
	return from, to, nil
}

func unitSalesDate(raw string) *string {
	if raw == "" {
		return nil
	}
	return &raw
}

func (h *Handler) AdminUnitSales(c echo.Context) error {
	from, to, err := unitSalesRange(c)
	if err != nil {
		return badRequest(c, err.Error())
	}
	types, err := h.svc.AdminUnitSales(c.Request().Context(), from, to)
	if err != nil {
		return mapServiceError(c, err)
	}
	return c.JSON(http.StatusOK, model.UnitSalesResponse{From: unitSalesDate(c.QueryParam("from")), To: unitSalesDate(c.QueryParam("to")), Types: types})
}

func (h *Handler) AdminUnitSalesExport(c echo.Context) error {
	typ := c.QueryParam("type")
	if typ == "" {
		return badRequest(c, "type is required")
	}
	if err := validateOrderProductType(typ); err != nil {
		return badRequest(c, err.Error())
	}
	from, to, err := unitSalesRange(c)
	if err != nil {
		return badRequest(c, err.Error())
	}
	types, err := h.svc.AdminUnitSales(c.Request().Context(), from, to)
	if err != nil {
		return mapServiceError(c, err)
	}
	for _, row := range types {
		if row.Type != typ {
			continue
		}
		data, err := service.UnitSalesCSV(row.Products)
		if err != nil {
			return mapServiceError(c, err)
		}
		start, end := c.QueryParam("from"), c.QueryParam("to")
		if start == "" {
			start = "semua-waktu"
		}
		if end == "" {
			end = "semua-waktu"
		}
		c.Response().Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="unit-terjual-%s-%s_%s.csv"`, typ, start, end))
		return c.Blob(http.StatusOK, "text/csv; charset=utf-8", data)
	}
	return badRequest(c, "unknown product type")
}
