package handler

import (
	"errors"
	"net/http"

	"akademi-bimbel/internal/infra"
	"akademi-bimbel/internal/repository"
	"akademi-bimbel/internal/service"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
)

type questionBundleRequest struct {
	Template service.QuestionBundleTemplate `json:"template"`
}

func questionBundleClaims(c echo.Context) (*infra.Claims, uuid.UUID, error) {
	claims, ok := c.Get("claims").(*infra.Claims)
	if !ok || claims == nil || claims.Sub == "" {
		return nil, uuid.Nil, service.ErrInvalidCredentials
	}
	actor, err := uuid.Parse(claims.Sub)
	if err != nil {
		return nil, uuid.Nil, service.ErrInvalidCredentials
	}
	return claims, actor, nil
}

func questionBundleTarget(c echo.Context) (uuid.UUID, string, error) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return uuid.Nil, "", err
	}
	variant := c.Param("variant")
	if variant != "naskah" && variant != "kunci" {
		return uuid.Nil, "", service.ErrValidation
	}
	return id, variant, nil
}

func (h *Handler) AdminRequestTestQuestionBundle(c echo.Context) error {
	testID, variant, err := questionBundleTarget(c)
	if err != nil {
		return badRequest(c, "invalid question bundle target")
	}
	claims, actor, err := questionBundleClaims(c)
	if err != nil {
		return mapServiceError(c, err)
	}
	var req questionBundleRequest
	if err := c.Bind(&req); err != nil {
		return badRequest(c, "invalid request body")
	}
	state, err := h.svc.RequestQuestionBundle(c.Request().Context(), actor, claims.Role, testID, variant, req.Template)
	if err != nil {
		return mapQuestionBundleError(c, err)
	}
	status := http.StatusAccepted
	if state.Status == "ready" {
		status = http.StatusOK
	}
	return c.JSON(status, state)
}

func (h *Handler) AdminGetTestQuestionBundle(c echo.Context) error {
	testID, variant, err := questionBundleTarget(c)
	if err != nil {
		return badRequest(c, "invalid question bundle target")
	}
	claims, _, err := questionBundleClaims(c)
	if err != nil {
		return mapServiceError(c, err)
	}
	state, err := h.svc.GetQuestionBundleState(c.Request().Context(), claims.Role, testID, variant)
	if err != nil {
		return mapQuestionBundleError(c, err)
	}
	return c.JSON(http.StatusOK, state)
}

func (h *Handler) AdminDownloadTestQuestionBundle(c echo.Context) error {
	testID, variant, err := questionBundleTarget(c)
	if err != nil {
		return badRequest(c, "invalid question bundle target")
	}
	claims, actor, err := questionBundleClaims(c)
	if err != nil {
		return mapServiceError(c, err)
	}
	url, err := h.svc.GetQuestionBundleDownloadURL(c.Request().Context(), actor, claims.Role, testID, variant)
	if err != nil {
		return mapQuestionBundleError(c, err)
	}
	return c.JSON(http.StatusOK, map[string]string{"url": url})
}

func mapQuestionBundleError(c echo.Context, err error) error {
	if errors.Is(err, repository.ErrNotFound) {
		return c.JSON(http.StatusNotFound, map[string]string{"error": "question bundle owner not found"})
	}
	return mapServiceError(c, err)
}
