package handler

import (
	"net/http"

	"github.com/labstack/echo/v4"
)

type affiliateJobRunRequest struct {
	Scope   string  `json:"scope"`
	Quarter *string `json:"quarter"`
}

// AdminGetAffiliate backs GET /admin/affiliates/:id — the detail tabs of the
// super admin console.
func (h *Handler) AdminGetAffiliate(c echo.Context) error {
	detail, err := h.svc.GetAffiliateAdminDetail(c.Request().Context(), c.Param("id"))
	if err != nil {
		return mapServiceError(c, err)
	}
	return c.JSON(http.StatusOK, detail)
}

// AdminRunAffiliateJobs backs POST /admin/affiliate-jobs/run. Scope and
// quarter validation lives in the service; the job sentinels are mapped by
// the parent's mapServiceError cases.
func (h *Handler) AdminRunAffiliateJobs(c echo.Context) error {
	var req affiliateJobRunRequest
	if err := c.Bind(&req); err != nil {
		return badRequest(c, "invalid request body")
	}
	res, err := h.svc.RunAffiliateJob(c.Request().Context(), req.Scope, req.Quarter)
	if err != nil {
		return mapServiceError(c, err)
	}
	return c.JSON(http.StatusOK, res)
}
