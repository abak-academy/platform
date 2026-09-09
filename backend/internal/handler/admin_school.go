package handler

import (
	"net/http"
	"strconv"

	"akademi-bimbel/internal/service"

	"github.com/labstack/echo/v4"
)

// AdminListSchools returns the full list of schools, cursor-paginated and
// keyset-ordered by name, with optional q (name/code/npsn) and status filters.
func (h *Handler) AdminListSchools(c echo.Context) error {
	cursor := c.QueryParam("cursor")
	q := c.QueryParam("q")
	statusFilter := c.QueryParam("status")

	limit := 20
	if l := c.QueryParam("limit"); l != "" {
		if n, err := strconv.Atoi(l); err == nil && n > 0 {
			limit = n
			if limit > 100 {
				limit = 100
			}
		}
	}

	schools, nextCursor, counts, err := h.svc.AdminListSchools(c.Request().Context(), service.AdminListSchoolsParams{
		Limit:  limit,
		Cursor: cursor,
		Q:      q,
		Status: statusFilter,
	})
	if err != nil {
		return mapServiceError(c, err)
	}

	return c.JSON(http.StatusOK, map[string]any{
		"data":        schools,
		"next_cursor": nextCursor,
		"total":       counts.Total,
		"active":      counts.Active,
		"students":    counts.Students,
	})
}

// AdminListSchoolOptions returns every active school (id/name/code) for
// picker dropdowns, unpaginated — see Service.SchoolOptions.
func (h *Handler) AdminListSchoolOptions(c echo.Context) error {
	options, err := h.svc.SchoolOptions(c.Request().Context())
	if err != nil {
		return mapServiceError(c, err)
	}
	return c.JSON(http.StatusOK, map[string]any{"data": options})
}

// AdminCreateSchool creates a new school.
func (h *Handler) AdminCreateSchool(c echo.Context) error {
	var req struct {
		Name        string   `json:"name"`
		Code        string   `json:"code"`
		NPSN        *string  `json:"npsn"`
		SchoolTypes []string `json:"school_types"`
		Alamat      *string  `json:"alamat"`
	}
	if err := c.Bind(&req); err != nil {
		return badRequest(c, "invalid request body")
	}
	if req.Code == "" {
		return badRequest(c, "code is required")
	}

	school, err := h.svc.CreateSchool(c.Request().Context(), req.Name, req.Code, req.NPSN, req.SchoolTypes, req.Alamat)
	if err != nil {
		return mapServiceError(c, err)
	}
	return c.JSON(http.StatusCreated, school)
}

// AdminUpdateSchool updates school fields.
func (h *Handler) AdminUpdateSchool(c echo.Context) error {
	id := c.Param("id")

	var req struct {
		Name        *string  `json:"name"`
		Code        *string  `json:"code"`
		NPSN        *string  `json:"npsn"`
		SchoolTypes []string `json:"school_types"`
		Alamat      *string  `json:"alamat"`
	}
	if err := c.Bind(&req); err != nil {
		return badRequest(c, "invalid request body")
	}

	school, err := h.svc.UpdateSchool(c.Request().Context(), id, req.Name, req.NPSN, req.Alamat, req.SchoolTypes, req.Code)
	if err != nil {
		return mapServiceError(c, err)
	}
	return c.JSON(http.StatusOK, school)
}

// AdminChangeSchoolStatus toggles a school's active/deactivated status.
func (h *Handler) AdminChangeSchoolStatus(c echo.Context) error {
	id := c.Param("id")

	var req struct {
		Status string `json:"status"`
	}
	if err := c.Bind(&req); err != nil {
		return badRequest(c, "invalid request body")
	}
	if req.Status != "active" && req.Status != "deactivated" {
		return badRequest(c, "status must be active or deactivated")
	}

	school, err := h.svc.ChangeSchoolStatus(c.Request().Context(), id, req.Status)
	if err != nil {
		return mapServiceError(c, err)
	}
	return c.JSON(http.StatusOK, school)
}
