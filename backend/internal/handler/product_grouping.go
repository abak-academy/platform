package handler

import (
	"net/http"

	"akademi-bimbel/internal/model"
	"github.com/labstack/echo/v4"
)

// groupingDetailResponse is the shared shape for both the admin and
// student-facing grouping detail reads: the grouping plus its member products.
type groupingDetailResponse struct {
	model.ProductGrouping
	Products []model.Product `json:"products"`
}

func (h *Handler) ListProductGroupings(c echo.Context) error {
	groupings, err := h.svc.ListPublishedProductGroupings(c.Request().Context())
	if err != nil {
		return mapServiceError(c, err)
	}
	return c.JSON(http.StatusOK, map[string]any{"data": groupings})
}

func (h *Handler) GetProductGrouping(c echo.Context) error {
	grouping, products, err := h.svc.GetPublishedProductGrouping(c.Request().Context(), c.Param("id"))
	if err != nil {
		return mapServiceError(c, err)
	}
	return c.JSON(http.StatusOK, groupingDetailResponse{ProductGrouping: grouping, Products: products})
}

func (h *Handler) AdminListProductGroupings(c echo.Context) error {
	groupings, err := h.svc.AdminListProductGroupings(c.Request().Context())
	if err != nil {
		return mapServiceError(c, err)
	}
	return c.JSON(http.StatusOK, map[string]any{"data": groupings})
}

func (h *Handler) AdminCreateProductGrouping(c echo.Context) error {
	var req struct {
		Name        string `json:"name"`
		Description string `json:"description"`
	}
	if err := c.Bind(&req); err != nil {
		return badRequest(c, "invalid request body")
	}
	grouping, err := h.svc.AdminCreateProductGrouping(c.Request().Context(), model.ProductGrouping{
		Name:        req.Name,
		Description: req.Description,
	})
	if err != nil {
		return mapServiceError(c, err)
	}
	return c.JSON(http.StatusCreated, grouping)
}

func (h *Handler) AdminGetProductGrouping(c echo.Context) error {
	grouping, products, err := h.svc.AdminGetProductGrouping(c.Request().Context(), c.Param("id"))
	if err != nil {
		return mapServiceError(c, err)
	}
	return c.JSON(http.StatusOK, groupingDetailResponse{ProductGrouping: grouping, Products: products})
}

func (h *Handler) AdminUpdateProductGrouping(c echo.Context) error {
	var req struct {
		Name        *string `json:"name"`
		Description *string `json:"description"`
		Status      *string `json:"status"`
	}
	if err := c.Bind(&req); err != nil {
		return badRequest(c, "invalid request body")
	}
	grouping, err := h.svc.AdminUpdateProductGrouping(c.Request().Context(), c.Param("id"), req.Name, req.Description, req.Status)
	if err != nil {
		return mapServiceError(c, err)
	}
	return c.JSON(http.StatusOK, grouping)
}

func (h *Handler) AdminDeleteProductGrouping(c echo.Context) error {
	if err := h.svc.AdminDeleteProductGrouping(c.Request().Context(), c.Param("id")); err != nil {
		return mapServiceError(c, err)
	}
	return c.NoContent(http.StatusNoContent)
}

func (h *Handler) AdminReorderProductGroupings(c echo.Context) error {
	var req struct {
		GroupingIDs []string `json:"grouping_ids"`
	}
	if err := c.Bind(&req); err != nil {
		return badRequest(c, "invalid request body")
	}
	if err := h.svc.AdminReorderProductGroupings(c.Request().Context(), req.GroupingIDs); err != nil {
		return mapServiceError(c, err)
	}
	return c.JSON(http.StatusOK, map[string]string{"message": "product groupings reordered"})
}

func (h *Handler) AdminSetGroupingProducts(c echo.Context) error {
	var req struct {
		ProductIDs []string `json:"product_ids"`
	}
	if err := c.Bind(&req); err != nil {
		return badRequest(c, "invalid request body")
	}
	grouping, products, err := h.svc.AdminSetGroupingProducts(c.Request().Context(), c.Param("id"), req.ProductIDs)
	if err != nil {
		return mapServiceError(c, err)
	}
	return c.JSON(http.StatusOK, groupingDetailResponse{ProductGrouping: grouping, Products: products})
}
