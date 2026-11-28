package handler

import (
	"encoding/json"
	"net/http"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
)

func examIDParam(c echo.Context) (uuid.UUID, bool) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		_ = badRequest(c, "invalid exam id")
		return uuid.Nil, false
	}
	return id, true
}

func (h *Handler) AdminGetCompetitionResult(c echo.Context) error {
	examID, ok := examIDParam(c)
	if !ok {
		return nil
	}
	result, err := h.svc.GetCompetitionResult(c.Request().Context(), examID)
	if err != nil {
		return mapServiceError(c, err)
	}
	return c.JSON(http.StatusOK, map[string]any{"data": result})
}

func (h *Handler) AdminPresignCompetitionResult(c echo.Context) error {
	examID, ok := examIDParam(c)
	if !ok {
		return nil
	}
	resp, err := h.svc.PresignCompetitionResultUpload(c.Request().Context(), examID, c.QueryParam("filename"))
	if err != nil {
		return mapServiceError(c, err)
	}
	return c.JSON(http.StatusOK, resp)
}

type competitionResultFileRequest struct {
	Key string `json:"key"`
}

func bindCompetitionResultFile(c echo.Context) (uuid.UUID, competitionResultFileRequest, bool) {
	var req competitionResultFileRequest
	examID, ok := examIDParam(c)
	if !ok {
		return uuid.Nil, req, false
	}
	if err := json.NewDecoder(c.Request().Body).Decode(&req); err != nil || req.Key == "" {
		_ = badRequest(c, "key is required")
		return uuid.Nil, req, false
	}
	return examID, req, true
}

func (h *Handler) AdminPreviewCompetitionResult(c echo.Context) error {
	examID, req, ok := bindCompetitionResultFile(c)
	if !ok {
		return nil
	}
	preview, err := h.svc.PreviewCompetitionResult(c.Request().Context(), examID, req.Key)
	if err != nil {
		return mapServiceError(c, err)
	}
	return c.JSON(http.StatusOK, preview)
}

func (h *Handler) AdminApplyCompetitionResult(c echo.Context) error {
	examID, req, ok := bindCompetitionResultFile(c)
	if !ok {
		return nil
	}
	actor, ok := actorFromClaims(c)
	if !ok {
		return c.JSON(http.StatusUnauthorized, APIError{Code: "unauthorized", Message: "missing auth"})
	}
	preview, result, err := h.svc.ApplyCompetitionResult(c.Request().Context(), examID, actor, req.Key)
	if err != nil {
		return mapServiceError(c, err)
	}
	if result == nil {
		return c.JSON(http.StatusUnprocessableEntity, map[string]any{
			"code": "validation_failed", "message": "file has errors", "matched": preview.Matched, "errors": preview.Errors,
		})
	}
	return c.JSON(http.StatusOK, map[string]any{"data": result})
}

func (h *Handler) setCompetitionPublished(c echo.Context, publish bool) error {
	examID, ok := examIDParam(c)
	if !ok {
		return nil
	}
	actor, ok := actorFromClaims(c)
	if !ok {
		return c.JSON(http.StatusUnauthorized, APIError{Code: "unauthorized", Message: "missing auth"})
	}
	result, err := h.svc.SetCompetitionResultPublished(c.Request().Context(), examID, publish, actor)
	if err != nil {
		return mapServiceError(c, err)
	}
	return c.JSON(http.StatusOK, map[string]any{"data": result})
}

func (h *Handler) AdminPublishCompetitionResult(c echo.Context) error {
	return h.setCompetitionPublished(c, true)
}
func (h *Handler) AdminUnpublishCompetitionResult(c echo.Context) error {
	return h.setCompetitionPublished(c, false)
}

func (h *Handler) AdminListCompetitionResultRows(c echo.Context) error {
	examID, ok := examIDParam(c)
	if !ok {
		return nil
	}
	rows, err := h.svc.ListCompetitionResultRows(c.Request().Context(), examID)
	if err != nil {
		return mapServiceError(c, err)
	}
	return c.JSON(http.StatusOK, map[string]any{"data": rows})
}

func (h *Handler) AdminDeleteCompetitionResult(c echo.Context) error {
	examID, ok := examIDParam(c)
	if !ok {
		return nil
	}
	actor, ok := actorFromClaims(c)
	if !ok {
		return c.JSON(http.StatusUnauthorized, APIError{Code: "unauthorized", Message: "missing auth"})
	}
	if err := h.svc.DeleteCompetitionResult(c.Request().Context(), examID, actor); err != nil {
		return mapServiceError(c, err)
	}
	return c.NoContent(http.StatusNoContent)
}
