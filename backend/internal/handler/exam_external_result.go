package handler

import (
	"encoding/json"
	"errors"
	"net/http"

	"akademi-bimbel/internal/service"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
)

func externalErr(c echo.Context, err error) error {
	if errors.Is(err, service.ErrExternalRoundNotFound) {
		return c.JSON(http.StatusNotFound, APIError{Code: "external_round_not_found", Message: err.Error()})
	}
	return mapServiceError(c, err)
}

func (h *Handler) AdminListExternalRounds(c echo.Context) error {
	examID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return badRequest(c, "invalid exam id")
	}
	rounds, err := h.svc.ListExternalRounds(c.Request().Context(), examID)
	if err != nil {
		return externalErr(c, err)
	}
	return c.JSON(http.StatusOK, map[string]any{"data": rounds})
}

func (h *Handler) AdminPresignExternalRound(c echo.Context) error {
	examID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return badRequest(c, "invalid exam id")
	}
	resp, err := h.svc.PresignExternalRoundUpload(c.Request().Context(), examID, c.QueryParam("filename"))
	if err != nil {
		return externalErr(c, err)
	}
	return c.JSON(http.StatusOK, resp)
}

type externalRoundFileRequest struct {
	Key        string `json:"key"`
	RoundLabel string `json:"round_label"`
}

func bindExternalRoundFile(c echo.Context) (uuid.UUID, externalRoundFileRequest, error) {
	var req externalRoundFileRequest
	examID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return uuid.Nil, req, badRequest(c, "invalid exam id")
	}
	if err := json.NewDecoder(c.Request().Body).Decode(&req); err != nil || req.Key == "" {
		return uuid.Nil, req, badRequest(c, "key and round_label are required")
	}
	return examID, req, nil
}

func (h *Handler) AdminPreviewExternalRound(c echo.Context) error {
	examID, req, err := bindExternalRoundFile(c)
	if err != nil || examID == uuid.Nil {
		return err
	}
	preview, err := h.svc.PreviewExternalRound(c.Request().Context(), examID, req.Key, req.RoundLabel)
	if err != nil {
		return externalErr(c, err)
	}
	return c.JSON(http.StatusOK, preview)
}

func (h *Handler) AdminApplyExternalRound(c echo.Context) error {
	examID, req, err := bindExternalRoundFile(c)
	if err != nil || examID == uuid.Nil {
		return err
	}
	actor, ok := actorFromClaims(c)
	if !ok {
		return c.JSON(http.StatusUnauthorized, APIError{Code: "unauthorized", Message: "missing auth"})
	}
	preview, round, err := h.svc.ApplyExternalRound(c.Request().Context(), examID, actor, req.Key, req.RoundLabel)
	if err != nil {
		return externalErr(c, err)
	}
	if round == nil {
		return c.JSON(http.StatusUnprocessableEntity, map[string]any{
			"code": "validation_failed", "message": "file has errors", "matched": preview.Matched, "errors": preview.Errors,
		})
	}
	return c.JSON(http.StatusOK, map[string]any{"data": round})
}

func (h *Handler) setExternalPublished(c echo.Context, publish bool) error {
	roundID, err := uuid.Parse(c.Param("round_id"))
	if err != nil {
		return badRequest(c, "invalid round id")
	}
	actor, ok := actorFromClaims(c)
	if !ok {
		return c.JSON(http.StatusUnauthorized, APIError{Code: "unauthorized", Message: "missing auth"})
	}
	round, err := h.svc.SetExternalRoundPublished(c.Request().Context(), roundID, publish, actor)
	if err != nil {
		return externalErr(c, err)
	}
	return c.JSON(http.StatusOK, map[string]any{"data": round})
}

func (h *Handler) AdminPublishExternalRound(c echo.Context) error {
	return h.setExternalPublished(c, true)
}
func (h *Handler) AdminUnpublishExternalRound(c echo.Context) error {
	return h.setExternalPublished(c, false)
}

func (h *Handler) AdminListExternalResults(c echo.Context) error {
	roundID, err := uuid.Parse(c.Param("round_id"))
	if err != nil {
		return badRequest(c, "invalid round id")
	}
	rows, err := h.svc.ListExternalResults(c.Request().Context(), roundID)
	if err != nil {
		return externalErr(c, err)
	}
	return c.JSON(http.StatusOK, map[string]any{"data": rows})
}

func (h *Handler) AdminDeleteExternalRound(c echo.Context) error {
	roundID, err := uuid.Parse(c.Param("round_id"))
	if err != nil {
		return badRequest(c, "invalid round id")
	}
	actor, ok := actorFromClaims(c)
	if !ok {
		return c.JSON(http.StatusUnauthorized, APIError{Code: "unauthorized", Message: "missing auth"})
	}
	if err := h.svc.DeleteExternalRound(c.Request().Context(), roundID, actor); err != nil {
		return externalErr(c, err)
	}
	return c.NoContent(http.StatusNoContent)
}
