package handler

import (
	"errors"
	"net/http"

	"akademi-bimbel/internal/infra"
	"akademi-bimbel/internal/service"
	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
)

// errScopeDone is a sentinel: the response has already been written by
// resolveSchoolScope; the caller should return nil immediately.
var errScopeDone = errors.New("scope response written")

// scopeHandled returns true when err indicates the resolver already wrote
// the response. The caller should return nil. Real errors (e.g. DB failure in
// SchoolExists) return false — the caller should use mapServiceError.
func scopeHandled(err error) bool {
	return errors.Is(err, errScopeDone)
}

// resolveSchoolScope resolves the target school ID based on the actor's role.
//
// super_admin reads optional ?school_id=. Omitting it returns "" (not a 400) —
// "not scoped to any school". Handlers that cannot run without a school must
// check schoolID == "" themselves. A present school_id is still validated
// (UUID + exists).
//
// Other roles use their JWT school_id (rejecting mismatched query params).
// On a scope error, writes the response and returns ("", errScopeDone).
// On a system error (e.g. SchoolExists failure) returns ("", err) for
// mapServiceError.
func (h *Handler) resolveSchoolScope(c echo.Context, claims *infra.Claims) (string, error) {
	if claims.Role == "super_admin" {
		return h.resolveGlobalSchoolParam(c)
	}

	if claims.SchoolID == nil {
		c.JSON(http.StatusForbidden, APIError{Code: "forbidden", Message: "missing school scope"})
		return "", errScopeDone
	}

	if sid := c.QueryParam("school_id"); sid != "" && sid != *claims.SchoolID {
		c.JSON(http.StatusForbidden, APIError{Code: "forbidden", Message: "cannot widen school scope"})
		return "", errScopeDone
	}

	return *claims.SchoolID, nil
}

// resolveGlobalSchoolParam reads ?school_id= for a role that may read across
// every school: empty means "not scoped to any school", a present value must
// parse as a UUID and reference an existing school.
func (h *Handler) resolveGlobalSchoolParam(c echo.Context) (string, error) {
	sid := c.QueryParam("school_id")
	if sid == "" {
		return "", nil
	}
	if _, err := uuid.Parse(sid); err != nil {
		c.JSON(http.StatusBadRequest, APIError{Code: "invalid_request", Message: "school_id must be a valid UUID"})
		return "", errScopeDone
	}
	exists, err := h.svc.SchoolExists(c.Request().Context(), sid)
	if err != nil {
		return "", err
	}
	if !exists {
		c.JSON(http.StatusNotFound, APIError{Code: "not_found", Message: "school not found"})
		return "", errScopeDone
	}
	return sid, nil
}

// resolveResultsSchoolScope is resolveSchoolScope with admin_exam also treated
// as unscoped: it manages the exam catalogue globally and owns no school, so
// results default to every school. Deliberately narrow — widening
// resolveSchoolScope itself would also unscope /admin/dashboard/exam, which
// admin_exam reaches via sessions:read.
func (h *Handler) resolveResultsSchoolScope(c echo.Context, claims *infra.Claims) (string, error) {
	if claims.Role == service.RoleAdminExam {
		return h.resolveGlobalSchoolParam(c)
	}
	return h.resolveSchoolScope(c, claims)
}
