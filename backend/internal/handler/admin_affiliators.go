package handler

import (
	"net/http"
	"strconv"

	"akademi-bimbel/internal/service"

	"github.com/labstack/echo/v4"
)

// AdminListAffiliators lists affiliators, filterable by status, cursor-paginated.
func (h *Handler) AdminListAffiliators(c echo.Context) error {
	limit := 20
	if l := c.QueryParam("limit"); l != "" {
		if n, err := strconv.Atoi(l); err == nil && n > 0 {
			limit = min(n, 100)
		}
	}
	rows, next, err := h.svc.ListAffiliators(c.Request().Context(), c.QueryParam("status"), limit, c.QueryParam("cursor"))
	if err != nil {
		return mapServiceError(c, err)
	}
	return c.JSON(http.StatusOK, map[string]any{"data": rows, "next_cursor": next})
}

// AdminCreateAffiliator creates one affiliator account.
func (h *Handler) AdminCreateAffiliator(c echo.Context) error {
	var req service.AffiliatorInput
	if err := c.Bind(&req); err != nil {
		return badRequest(c, "invalid request body")
	}
	acc, err := h.svc.CreateAffiliator(c.Request().Context(), ClaimsFromContext(c).Sub, req)
	if err != nil {
		return mapServiceError(c, err)
	}
	return c.JSON(http.StatusCreated, acc)
}

// AdminBulkCreateAffiliators creates many affiliator accounts and reports each row.
func (h *Handler) AdminBulkCreateAffiliators(c echo.Context) error {
	var req struct {
		Rows []service.AffiliatorInput `json:"rows"`
	}
	if err := c.Bind(&req); err != nil {
		return badRequest(c, "invalid request body")
	}
	results, err := h.svc.BulkCreateAffiliators(c.Request().Context(), ClaimsFromContext(c).Sub, req.Rows)
	if err != nil {
		return mapServiceError(c, err)
	}
	return c.JSON(http.StatusOK, map[string]any{"results": results})
}

// AdminPromoteAffiliator turns an admin_school account into an affiliator.
func (h *Handler) AdminPromoteAffiliator(c echo.Context) error {
	var req struct {
		SchoolIDs []string `json:"school_ids"`
	}
	if err := c.Bind(&req); err != nil {
		return badRequest(c, "invalid request body")
	}
	if err := h.svc.PromoteToAffiliator(c.Request().Context(), ClaimsFromContext(c).Sub, c.Param("id"), req.SchoolIDs); err != nil {
		return mapServiceError(c, err)
	}
	return c.JSON(http.StatusOK, map[string]string{"message": "affiliator promoted"})
}

// AdminSetAffiliatorSchools replaces the schools an affiliator may access.
func (h *Handler) AdminSetAffiliatorSchools(c echo.Context) error {
	var req struct {
		SchoolIDs []string `json:"school_ids"`
	}
	if err := c.Bind(&req); err != nil {
		return badRequest(c, "invalid request body")
	}
	if err := h.svc.SetAffiliatorSchools(c.Request().Context(), ClaimsFromContext(c).Sub, c.Param("id"), req.SchoolIDs); err != nil {
		return mapServiceError(c, err)
	}
	return c.JSON(http.StatusOK, map[string]string{"message": "schools updated"})
}

// AffiliatorMe returns the signed-in affiliator's profile, schools and active state.
func (h *Handler) AffiliatorMe(c echo.Context) error {
	resp, err := h.svc.GetAffiliatorProfile(c.Request().Context(), ClaimsFromContext(c).Sub)
	if err != nil {
		return mapServiceError(c, err)
	}
	return c.JSON(http.StatusOK, resp)
}
