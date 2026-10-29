package handler

import (
	"net/http"
	"strconv"

	"akademi-bimbel/internal/repository"
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
	rows, next, total, err := h.svc.ListAffiliators(c.Request().Context(), c.QueryParam("status"), c.QueryParam("q"), limit, c.QueryParam("cursor"))
	if err != nil {
		return mapServiceError(c, err)
	}
	return c.JSON(http.StatusOK, map[string]any{"data": rows, "next_cursor": next, "total": total})
}

// AdminListPromotableAdmins lists the admin_school accounts that can become affiliators.
func (h *Handler) AdminListPromotableAdmins(c echo.Context) error {
	limit := 20
	if l := c.QueryParam("limit"); l != "" {
		if n, err := strconv.Atoi(l); err == nil && n > 0 {
			limit = min(n, 100)
		}
	}
	rows, next, total, err := h.svc.ListPromotableAdmins(c.Request().Context(), c.QueryParam("q"), limit, c.QueryParam("cursor"))
	if err != nil {
		return mapServiceError(c, err)
	}
	return c.JSON(http.StatusOK, map[string]any{"data": rows, "next_cursor": next, "total": total})
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

// AdminPresignAffiliatorBulkUpload issues a presigned PUT URL to the private
// bucket for an affiliator-bulk CSV upload (super admin only by route).
func (h *Handler) AdminPresignAffiliatorBulkUpload(c echo.Context) error {
	filename := c.QueryParam("filename")
	if filename == "" {
		return badRequest(c, "filename is required")
	}
	resp, err := h.svc.GeneratePresignedAffiliatorBulkUploadURL(c.Request().Context(), filename, c.QueryParam("content_type"))
	if err != nil {
		return mapServiceError(c, err)
	}
	return c.JSON(http.StatusOK, resp)
}

// AdminBulkImportAffiliators enqueues an async affiliator_bulk job from an
// already-uploaded CSV.
func (h *Handler) AdminBulkImportAffiliators(c echo.Context) error {
	var req struct {
		FileKey string `json:"file_key"`
	}
	if err := c.Bind(&req); err != nil {
		return badRequest(c, "invalid request body")
	}
	if req.FileKey == "" {
		return badRequest(c, "file_key is required")
	}
	jobID, err := h.svc.EnqueueAffiliatorBulkJob(c.Request().Context(), ClaimsFromContext(c).Sub, req.FileKey)
	if err != nil {
		return mapServiceError(c, err)
	}
	return c.JSON(http.StatusAccepted, map[string]string{"job_id": jobID})
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

// AffiliatorListProducts lists the exam and course products an affiliator can order.
func (h *Handler) AffiliatorListProducts(c echo.Context) error {
	products, err := h.svc.ListAffiliatorProducts(c.Request().Context())
	if err != nil {
		return mapServiceError(c, err)
	}
	return c.JSON(http.StatusOK, map[string]any{"data": products})
}

type affiliatorOrderBody struct {
	ProductID  string   `json:"product_id"`
	StudentIDs []string `json:"student_ids"`
	PromoCode  string   `json:"promo_code,omitempty"`
}

func bindAffiliatorOrder(c echo.Context) (affiliatorOrderBody, bool) {
	var req affiliatorOrderBody
	if err := c.Bind(&req); err != nil {
		_ = badRequest(c, "invalid request body")
		return req, false
	}
	if req.ProductID == "" {
		_ = badRequest(c, "product_id is required")
		return req, false
	}
	return req, true
}

// AffiliatorPreviewOrder previews an order for chosen students of any assigned school.
func (h *Handler) AffiliatorPreviewOrder(c echo.Context) error {
	req, ok := bindAffiliatorOrder(c)
	if !ok {
		return nil
	}
	preview, err := h.svc.PreviewAffiliatorOrder(c.Request().Context(), ClaimsFromContext(c).Sub, req.ProductID, req.StudentIDs, req.PromoCode)
	if err != nil {
		return mapServiceError(c, err)
	}
	return c.JSON(http.StatusOK, preview)
}

// AffiliatorCreateOrder creates an order the affiliator pays for on the students' behalf.
func (h *Handler) AffiliatorCreateOrder(c echo.Context) error {
	req, ok := bindAffiliatorOrder(c)
	if !ok {
		return nil
	}
	order, err := h.svc.CreateAffiliatorOrder(c.Request().Context(), ClaimsFromContext(c).Sub, req.ProductID, req.StudentIDs, req.PromoCode)
	if err != nil {
		return mapServiceError(c, err)
	}
	return c.JSON(http.StatusCreated, order)
}

// AffiliatorCheckoutOrder checks out one of the affiliator's own orders.
func (h *Handler) AffiliatorCheckoutOrder(c echo.Context) error {
	key := c.Request().Header.Get("Idempotency-Key")
	if key == "" {
		return badRequest(c, "Idempotency-Key header is required")
	}
	result, err := h.svc.CheckoutAffiliatorOrder(c.Request().Context(), ClaimsFromContext(c).Sub, c.Param("id"), key)
	if err != nil {
		return mapServiceError(c, err)
	}
	return c.JSON(http.StatusOK, result)
}

// AdminGetAffiliator returns one affiliator with lifetime totals and recent orders.
func (h *Handler) AdminGetAffiliator(c echo.Context) error {
	detail, err := h.svc.GetAffiliatorDetail(c.Request().Context(), c.Param("id"))
	if err != nil {
		return mapServiceError(c, err)
	}
	return c.JSON(http.StatusOK, detail)
}

// AffiliatorDashboard returns the signed-in affiliator's dashboard figures.
func (h *Handler) AffiliatorDashboard(c echo.Context) error {
	from, to, err := parseDayRange(c.QueryParam("from"), c.QueryParam("to"))
	if err != nil {
		return badRequest(c, err.Error())
	}
	d, err := h.svc.AffiliatorDashboard(c.Request().Context(), ClaimsFromContext(c).Sub, from, to)
	if err != nil {
		return mapServiceError(c, err)
	}
	return c.JSON(http.StatusOK, d)
}

// AffiliatorListOrders lists the signed-in affiliator's orders.
func (h *Handler) AffiliatorListOrders(c echo.Context) error {
	limit := 20
	if l := c.QueryParam("limit"); l != "" {
		if n, err := strconv.Atoi(l); err == nil && n > 0 {
			limit = min(n, 100)
		}
	}
	rows, next, err := h.svc.ListAffiliatorOrders(c.Request().Context(), ClaimsFromContext(c).Sub, c.QueryParam("status"), limit, c.QueryParam("cursor"))
	if err != nil {
		return mapServiceError(c, err)
	}
	return c.JSON(http.StatusOK, map[string]any{"data": rows, "next_cursor": next})
}

func affiliatorSchoolQuery(c echo.Context) repository.AffiliatorSchoolQuery {
	q := repository.AffiliatorSchoolQuery{
		Q:            c.QueryParam("q"),
		AffiliatorID: c.QueryParam("affiliator_id"),
		SchoolID:     c.QueryParam("school_id"),
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
	return q
}

// AdminListAffiliatorSchools lists which schools each affiliator has, paged.
func (h *Handler) AdminListAffiliatorSchools(c echo.Context) error {
	q := affiliatorSchoolQuery(c)
	rows, total, err := h.svc.ListAffiliatorSchools(c.Request().Context(), q)
	if err != nil {
		return mapServiceError(c, err)
	}
	return c.JSON(http.StatusOK, map[string]any{"data": rows, "total": total, "limit": q.Limit, "offset": q.Offset})
}

// AdminExportAffiliatorSchools serves the same list, unpaged, as a CSV attachment.
func (h *Handler) AdminExportAffiliatorSchools(c echo.Context) error {
	data, err := h.svc.ExportAffiliatorSchools(c.Request().Context(), affiliatorSchoolQuery(c))
	if err != nil {
		return mapServiceError(c, err)
	}
	c.Response().Header().Set(echo.HeaderContentDisposition, `attachment; filename="sekolah-affiliator.csv"`)
	return c.Blob(http.StatusOK, "text/csv; charset=utf-8", data)
}
