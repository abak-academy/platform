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

// AffiliatorListProducts lists the exam and course products an affiliator can order.
func (h *Handler) AffiliatorListProducts(c echo.Context) error {
	products, err := h.svc.ListAffiliatorProducts(c.Request().Context())
	if err != nil {
		return mapServiceError(c, err)
	}
	return c.JSON(http.StatusOK, map[string]any{"data": products})
}

type affiliatorOrderBody struct {
	SchoolID   string   `json:"school_id"`
	ProductID  string   `json:"product_id"`
	StudentIDs []string `json:"student_ids,omitempty"`
	Grade      *int     `json:"grade,omitempty"`
	All        bool     `json:"all,omitempty"`
	PromoCode  string   `json:"promo_code,omitempty"`
}

func (b affiliatorOrderBody) selector() service.ParticipantSelector {
	return service.ParticipantSelector{StudentIDs: b.StudentIDs, Grade: b.Grade, All: b.All}
}

func bindAffiliatorOrder(c echo.Context) (affiliatorOrderBody, bool) {
	var req affiliatorOrderBody
	if err := c.Bind(&req); err != nil {
		_ = badRequest(c, "invalid request body")
		return req, false
	}
	if req.SchoolID == "" || req.ProductID == "" {
		_ = badRequest(c, "school_id and product_id are required")
		return req, false
	}
	return req, true
}

// AffiliatorPreviewOrder previews an order for students of one assigned school.
func (h *Handler) AffiliatorPreviewOrder(c echo.Context) error {
	req, ok := bindAffiliatorOrder(c)
	if !ok {
		return nil
	}
	preview, err := h.svc.PreviewAffiliatorOrder(c.Request().Context(), ClaimsFromContext(c).Sub, req.SchoolID, req.ProductID, req.selector(), req.PromoCode)
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
	order, err := h.svc.CreateAffiliatorOrder(c.Request().Context(), ClaimsFromContext(c).Sub, req.SchoolID, req.ProductID, req.selector(), req.PromoCode)
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
