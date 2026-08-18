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
// super_admin reads from ?school_id= query param (validated to exist).
// Other roles use their JWT school_id (rejecting mismatched query params).
// On success, returns (schoolID, nil). On a scope error, writes the response
// and returns ("", errScopeDone). On a system error (e.g. SchoolExists failure)
// returns ("", err) for the caller to handle via mapServiceError.
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

// resolveGlobalSchoolParam validates the ?school_id= query param for a role
// allowed to read across every school: empty param is the caller's problem to
// reject, present param must parse as a UUID and reference an existing school.
// On success returns (schoolID, nil); on a scope error, writes the response
// and returns ("", errScopeDone).
func (h *Handler) resolveGlobalSchoolParam(c echo.Context) (string, error) {
	sid := c.QueryParam("school_id")
	if sid == "" {
		c.JSON(http.StatusBadRequest, APIError{Code: "invalid_request", Message: "school_id is required"})
		return "", errScopeDone
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

// resolveSchoolScopeOptional is resolveSchoolScope with the school made
// optional for super_admin and admin_exam: omitting school_id returns ""
// instead of a 400, meaning "not scoped to any school".
//
// Only endpoints that genuinely work without a school may use this — the
// student roster (registrants are not all school pupils; university students
// and the general public sign up too), student registration (an operator
// can confirm the school after the fact), and admin results (list/detail/export
// default to every school; the repository's filter is optional). Every other
// caller keeps resolveSchoolScope, where a missing school is a client error.
//
// admin_school is unaffected: its scope still rides on the JWT and cannot be
// widened.
func (h *Handler) resolveSchoolScopeOptional(c echo.Context, claims *infra.Claims) (string, error) {
	if claims.Role == "super_admin" || claims.Role == service.RoleAdminExam {
		if c.QueryParam("school_id") == "" {
			return "", nil
		}
		return h.resolveGlobalSchoolParam(c)
	}
	return h.resolveSchoolScope(c, claims)
}
