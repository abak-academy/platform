package handler

import (
	"net/http"

	"akademi-bimbel/internal/model"
	"akademi-bimbel/internal/service"
	"github.com/labstack/echo/v4"
)

func (h *Handler) ListProductGroupings(c echo.Context) error {
	groupings, err := h.svc.ListPublishedProductGroupings(c.Request().Context())
	if err != nil {
		return mapServiceError(c, err)
	}
	return c.JSON(http.StatusOK, map[string]any{"data": groupings})
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
		Name              string                          `json:"name"`
		Description       string                          `json:"description"`
		ShortName         *string                         `json:"short_name"`
		Scope             *string                         `json:"scope"`
		Subjects          []string                        `json:"subjects"`
		Tone              string                          `json:"tone"`
		Illustration      string                          `json:"illustration"`
		LogoKey           *string                         `json:"logo_key"`
		OfficialURL       *string                         `json:"official_url"`
		Content           map[string]model.EventNarrative `json:"content"`
		ParticipantPrefix *string                         `json:"participant_prefix"`
	}
	if err := c.Bind(&req); err != nil {
		return badRequest(c, "invalid request body")
	}
	grouping, err := h.svc.AdminCreateProductGrouping(c.Request().Context(), model.ProductGrouping{
		Name: req.Name, Description: req.Description, ShortName: req.ShortName, Scope: req.Scope, Subjects: req.Subjects,
		Tone: req.Tone, Illustration: req.Illustration, LogoKey: req.LogoKey, OfficialURL: req.OfficialURL,
		Content: req.Content, ParticipantPrefix: req.ParticipantPrefix,
	})
	if err != nil {
		return mapServiceError(c, err)
	}
	return c.JSON(http.StatusCreated, grouping)
}

func (h *Handler) AdminGetProductGrouping(c echo.Context) error {
	detail, err := h.svc.AdminGetProductGroupingDetail(c.Request().Context(), c.Param("id"))
	if err != nil {
		return mapServiceError(c, err)
	}
	return c.JSON(http.StatusOK, detail)
}

func (h *Handler) AdminUpdateProductGrouping(c echo.Context) error {
	var req service.ProductGroupingPatch
	if err := c.Bind(&req); err != nil {
		return badRequest(c, "invalid request body")
	}
	ctx := c.Request().Context()
	if _, err := h.svc.AdminUpdateProductGrouping(ctx, c.Param("id"), req); err != nil {
		return mapServiceError(c, err)
	}
	detail, err := h.svc.AdminGetProductGroupingDetail(ctx, c.Param("id"))
	if err != nil {
		return mapServiceError(c, err)
	}
	return c.JSON(http.StatusOK, detail)
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
	ctx := c.Request().Context()
	if _, _, err := h.svc.AdminSetGroupingProducts(ctx, c.Param("id"), req.ProductIDs); err != nil {
		return mapServiceError(c, err)
	}
	detail, err := h.svc.AdminGetProductGroupingDetail(ctx, c.Param("id"))
	if err != nil {
		return mapServiceError(c, err)
	}
	return c.JSON(http.StatusOK, detail)
}
