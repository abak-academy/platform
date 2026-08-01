package handler

import (
	"net/http"
	"time"

	"akademi-bimbel/internal/infra"
	"akademi-bimbel/internal/model"
	"akademi-bimbel/internal/repository"
	"akademi-bimbel/internal/service"
	"github.com/labstack/echo/v4"
)

func (h *Handler) ListProducts(c echo.Context) error {
	claims, _ := c.Get("claims").(*infra.Claims)
	role := ""
	if claims != nil {
		role = claims.Role
	}

	filter := repository.ProductFilter{
		Type:   c.QueryParam("type"),
		Status: c.QueryParam("status"),
		Cursor: c.QueryParam("cursor"),
	}

	products, nextCursor, err := h.svc.ListProducts(c.Request().Context(), filter, role)
	if err != nil {
		return mapServiceError(c, err)
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"data":        products,
		"next_cursor": nextCursor,
	})
}

func (h *Handler) GetProduct(c echo.Context) error {
	id := c.Param("id")
	claims, _ := c.Get("claims").(*infra.Claims)
	role := ""
	if claims != nil {
		role = claims.Role
	}

	product, err := h.svc.GetProduct(c.Request().Context(), id, role)
	if err != nil {
		return mapServiceError(c, err)
	}

	return c.JSON(http.StatusOK, product)
}

func (h *Handler) AdminListProducts(c echo.Context) error {
	claims, _ := c.Get("claims").(*infra.Claims)
	role := ""
	if claims != nil {
		role = claims.Role
	}

	filter := repository.ProductFilter{
		Type:   c.QueryParam("type"),
		Status: c.QueryParam("status"),
		Cursor: c.QueryParam("cursor"),
	}

	products, nextCursor, err := h.svc.ListProducts(c.Request().Context(), filter, role)
	if err != nil {
		return mapServiceError(c, err)
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"data":        products,
		"next_cursor": nextCursor,
	})
}

func (h *Handler) AdminCreateProduct(c echo.Context) error {
	var req struct {
		Type           string              `json:"type"`
		Name           string              `json:"name"`
		Description    string              `json:"description"`
		Price          int64               `json:"price"`
		Stock          int                 `json:"stock"`
		WeightGrams    int                 `json:"weight_grams"`
		ImageURL       string              `json:"image_url"`
		Specs          []model.ProductSpec `json:"specs"`
		AvailableFrom  *time.Time          `json:"available_from"`
		AvailableUntil *time.Time          `json:"available_until"`
		CourseIDs      []string            `json:"course_ids"`
		ExamIDs        []string            `json:"exam_ids"`
	}
	if err := c.Bind(&req); err != nil {
		return badRequest(c, "invalid request body")
	}
	if req.Type == "" || req.Name == "" {
		return badRequest(c, "type and name are required")
	}
	if err := service.ValidateSpecs(req.Specs); err != nil {
		return mapServiceError(c, err)
	}

	claims, _ := c.Get("claims").(*infra.Claims)
	role := ""
	if claims != nil {
		role = claims.Role
	}

	p := model.Product{
		Type:           req.Type,
		Name:           req.Name,
		Description:    req.Description,
		Price:          req.Price,
		Stock:          req.Stock,
		WeightGrams:    req.WeightGrams,
		ImageURL:       req.ImageURL,
		Specs:          req.Specs,
		AvailableFrom:  req.AvailableFrom,
		AvailableUntil: req.AvailableUntil,
		Status:         "draft",
	}

	var product model.Product
	var err error
	switch {
	case req.Type == "course" || len(req.CourseIDs) > 0:
		product, err = h.svc.CreateProductWithCourses(c.Request().Context(), p, req.CourseIDs, role)
	case req.Type == "exam" || len(req.ExamIDs) > 0:
		product, err = h.svc.CreateProductWithExams(c.Request().Context(), p, req.ExamIDs, role)
	default:
		product, err = h.svc.CreateProduct(c.Request().Context(), p, role)
	}
	if err != nil {
		return mapServiceError(c, err)
	}

	return c.JSON(http.StatusCreated, product)
}

func (h *Handler) AdminGetProduct(c echo.Context) error {
	id := c.Param("id")
	claims, _ := c.Get("claims").(*infra.Claims)
	role := ""
	if claims != nil {
		role = claims.Role
	}

	product, err := h.svc.GetProduct(c.Request().Context(), id, role)
	if err != nil {
		return mapServiceError(c, err)
	}

	return c.JSON(http.StatusOK, product)
}

func (h *Handler) AdminUpdateProduct(c echo.Context) error {
	id := c.Param("id")
	var req struct {
		Name           string               `json:"name"`
		Description    string               `json:"description"`
		Price          int64                `json:"price"`
		Stock          int                  `json:"stock"`
		WeightGrams    *int                 `json:"weight_grams"`
		ImageURL       *string              `json:"image_url"`
		Specs          *[]model.ProductSpec `json:"specs"`  // pointer: absent preserves existing specs, present replaces them
		Status         Nullable[string]     `json:"status"` // published ↔ hidden visibility flip only; absent preserves existing
		AvailableFrom  Nullable[time.Time]  `json:"available_from"` // absent preserves; null clears; value sets
		AvailableUntil Nullable[time.Time]  `json:"available_until"`
		CourseIDs      []string             `json:"course_ids"`
		ExamIDs        []string             `json:"exam_ids"`
	}
	if err := c.Bind(&req); err != nil {
		return badRequest(c, "invalid request body")
	}
	if req.Specs != nil {
		if err := service.ValidateSpecs(*req.Specs); err != nil {
			return mapServiceError(c, err)
		}
	}

	claims, _ := c.Get("claims").(*infra.Claims)
	role := ""
	if claims != nil {
		role = claims.Role
	}

	status := req.Status.Value
	if !req.Status.Set {
		existing, err := h.svc.GetProduct(c.Request().Context(), id, role)
		if err != nil {
			return mapServiceError(c, err)
		}
		status = existing.Status
	}

	p := model.Product{
		Name:              req.Name,
		Description:       req.Description,
		Price:             req.Price,
		Stock:             req.Stock,
		Status:            status,
		WeightGramsSet:    req.WeightGrams != nil,
		ImageURLSet:       req.ImageURL != nil,
		AvailableFromSet:  req.AvailableFrom.Set,
		AvailableUntilSet: req.AvailableUntil.Set,
	}
	if req.WeightGrams != nil {
		p.WeightGrams = *req.WeightGrams
	}
	if req.ImageURL != nil {
		p.ImageURL = *req.ImageURL
	}
	if req.Specs != nil {
		p.Specs = *req.Specs
	} else {
		existing, err := h.svc.GetProduct(c.Request().Context(), id, role)
		if err != nil {
			return mapServiceError(c, err)
		}
		p.Specs = existing.Specs
	}
	// Set → overlay (value or explicit clear); absent → preserved in the service.
	if req.AvailableFrom.Set && req.AvailableFrom.Valid {
		v := req.AvailableFrom.Value
		p.AvailableFrom = &v
	}
	if req.AvailableUntil.Set && req.AvailableUntil.Valid {
		v := req.AvailableUntil.Value
		p.AvailableUntil = &v
	}

	var product model.Product
	var err error
	switch {
	case req.CourseIDs != nil:
		product, err = h.svc.UpdateProductWithCourses(c.Request().Context(), id, p, req.CourseIDs, role)
	case req.ExamIDs != nil:
		product, err = h.svc.UpdateProductWithExams(c.Request().Context(), id, p, req.ExamIDs, role)
	default:
		product, err = h.svc.UpdateProduct(c.Request().Context(), id, p, role)
	}
	if err != nil {
		return mapServiceError(c, err)
	}

	return c.JSON(http.StatusOK, product)
}

func (h *Handler) AdminPublishProduct(c echo.Context) error {
	id := c.Param("id")
	claims, _ := c.Get("claims").(*infra.Claims)
	role := ""
	if claims != nil {
		role = claims.Role
	}

	err := h.svc.PublishProduct(c.Request().Context(), id, role)
	if err != nil {
		return mapServiceError(c, err)
	}

	return c.JSON(http.StatusOK, map[string]string{
		"message": "product published",
	})
}

func (h *Handler) AdminDeleteProduct(c echo.Context) error {
	id := c.Param("id")
	claims, _ := c.Get("claims").(*infra.Claims)
	role := ""
	if claims != nil {
		role = claims.Role
	}

	err := h.svc.DeleteProduct(c.Request().Context(), id, role)
	if err != nil {
		return mapServiceError(c, err)
	}

	return c.NoContent(http.StatusNoContent)
}
