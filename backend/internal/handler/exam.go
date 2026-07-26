package handler

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"strconv"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"

	"akademi-bimbel/internal/model"
	"akademi-bimbel/internal/repository"
	"akademi-bimbel/internal/service"
)

func (h *Handler) AdminListTests(c echo.Context) error {
	limit := 20
	if l := c.QueryParam("limit"); l != "" {
		if n, err := strconv.Atoi(l); err == nil && n > 0 {
			limit = n
		}
	}
	filter := repository.TestFilter{
		Subject: c.QueryParam("subject"),
		Topic:   c.QueryParam("topic"),
		Cursor:  c.QueryParam("cursor"),
		Limit:   limit,
	}

	tests, nextCursor, err := h.svc.ListTests(c.Request().Context(), filter)
	if err != nil {
		return mapServiceError(c, err)
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"data":        tests,
		"next_cursor": nextCursor,
	})
}

func (h *Handler) AdminCreateTest(c echo.Context) error {
	var req struct {
		Title           string  `json:"title"`
		Subject         string  `json:"subject"`
		Topic           string  `json:"topic"`
		DurationMinutes int     `json:"duration_minutes"`
		AudioURL        *string `json:"audio_url,omitempty"`
		AudioPlayLimit  *int    `json:"audio_play_limit,omitempty"`
		SectionType     *string `json:"section_type,omitempty"`
	}
	if err := c.Bind(&req); err != nil {
		return badRequest(c, "invalid request body")
	}

	t := model.Test{
		Title:           req.Title,
		Subject:         req.Subject,
		Topic:           req.Topic,
		DurationMinutes: req.DurationMinutes,
		AudioURL:        req.AudioURL,
		AudioPlayLimit:  req.AudioPlayLimit,
		SectionType:     req.SectionType,
	}

	out, err := h.svc.CreateTest(c.Request().Context(), t)
	if err != nil {
		return mapServiceError(c, err)
	}
	return c.JSON(http.StatusCreated, out)
}

func (h *Handler) AdminGetTest(c echo.Context) error {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return badRequest(c, "invalid id")
	}
	detail, err := h.svc.GetTestDetail(c.Request().Context(), id)
	if err != nil {
		return mapServiceError(c, err)
	}
	return c.JSON(http.StatusOK, detail)
}

func (h *Handler) AdminUpdateTest(c echo.Context) error {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return badRequest(c, "invalid id")
	}
	// PATCH is partial — read existing and overlay only fields supplied.
	// (Service validateTest enforces all required fields; merging here keeps
	// the service contract intact while supporting PATCH semantics.)
	existing, err := h.svc.GetTestDetail(c.Request().Context(), id)
	if err != nil {
		return mapServiceError(c, err)
	}
	// Nullable[T] fields distinguish "absent — preserve" from "present and
	// null — clear" (a plain *T cannot: encoding/json leaves it nil either way),
	// so clearing audio/section settings via PATCH actually clears them.
	var req struct {
		Title           string           `json:"title"`
		Subject         string           `json:"subject"`
		Topic           string           `json:"topic"`
		DurationMinutes int              `json:"duration_minutes"`
		AudioURL        Nullable[string] `json:"audio_url"`
		AudioPlayLimit  Nullable[int]    `json:"audio_play_limit"`
		SectionType     Nullable[string] `json:"section_type"`
	}
	if err := c.Bind(&req); err != nil {
		return badRequest(c, "invalid request body")
	}

	t := existing.Test
	if req.Title != "" {
		t.Title = req.Title
	}
	if req.Subject != "" {
		t.Subject = req.Subject
	}
	if req.Topic != "" {
		t.Topic = req.Topic
	}
	if req.DurationMinutes > 0 {
		t.DurationMinutes = req.DurationMinutes
	}
	applyNullable(req.AudioURL, &t.AudioURL)
	applyNullable(req.AudioPlayLimit, &t.AudioPlayLimit)
	applyNullable(req.SectionType, &t.SectionType)

	out, err := h.svc.UpdateTest(c.Request().Context(), id, t)
	if err != nil {
		return mapServiceError(c, err)
	}
	return c.JSON(http.StatusOK, out)
}

func (h *Handler) AdminDeleteTest(c echo.Context) error {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return badRequest(c, "invalid id")
	}
	if err := h.svc.DeleteTest(c.Request().Context(), id); err != nil {
		return mapServiceError(c, err)
	}
	return c.NoContent(http.StatusNoContent)
}

func (h *Handler) AdminListQuestions(c echo.Context) error {
	testID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return badRequest(c, "invalid id")
	}
	detail, err := h.svc.GetTestDetail(c.Request().Context(), testID)
	if err != nil {
		return mapServiceError(c, err)
	}
	return c.JSON(http.StatusOK, map[string]interface{}{
		"data":        detail.Questions,
		"next_cursor": "",
	})
}

func (h *Handler) AdminCreateQuestion(c echo.Context) error {
	testID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return badRequest(c, "invalid id")
	}

	var req questionRequest
	if err := c.Bind(&req); err != nil {
		return badRequest(c, "invalid request body")
	}

	q, err := req.toQuestion()
	if err != nil {
		return mapServiceError(c, err)
	}
	out, err := h.svc.CreateQuestionForTest(c.Request().Context(), testID, q, req.toOptions(), req.toBlanks())
	if err != nil {
		return mapServiceError(c, err)
	}
	return c.JSON(http.StatusCreated, out)
}

// AdminAttachQuestions attaches one or many bank questions to a test (FR-21).
func (h *Handler) AdminAttachQuestions(c echo.Context) error {
	testID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return badRequest(c, "invalid id")
	}

	var req struct {
		QuestionID  string   `json:"question_id"`
		QuestionIDs []string `json:"question_ids"`
	}
	if err := c.Bind(&req); err != nil {
		return badRequest(c, "invalid request body")
	}

	ids := req.QuestionIDs
	if len(ids) == 0 && req.QuestionID != "" {
		ids = []string{req.QuestionID}
	}
	if len(ids) == 0 {
		return badRequest(c, "question_id or question_ids required")
	}

	questionIDs := make([]uuid.UUID, 0, len(ids))
	for _, raw := range ids {
		id, err := uuid.Parse(raw)
		if err != nil {
			return badRequest(c, "invalid question_id")
		}
		questionIDs = append(questionIDs, id)
	}

	if err := h.svc.AttachQuestions(c.Request().Context(), testID, questionIDs); err != nil {
		return mapServiceError(c, err)
	}
	return c.NoContent(http.StatusNoContent)
}

// AdminDetachQuestion removes a question attachment from a test (FR-22).
func (h *Handler) AdminDetachQuestion(c echo.Context) error {
	testID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return badRequest(c, "invalid id")
	}
	questionID, err := uuid.Parse(c.Param("questionId"))
	if err != nil {
		return badRequest(c, "invalid question id")
	}
	if err := h.svc.DetachQuestion(c.Request().Context(), testID, questionID); err != nil {
		return mapServiceError(c, err)
	}
	return c.NoContent(http.StatusNoContent)
}

// AdminReorderTestQuestions rewrites the order of attached questions (FR-23).
func (h *Handler) AdminReorderTestQuestions(c echo.Context) error {
	testID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return badRequest(c, "invalid id")
	}

	var req struct {
		QuestionIDs []string `json:"question_ids"`
	}
	if err := c.Bind(&req); err != nil {
		return badRequest(c, "invalid request body")
	}
	if len(req.QuestionIDs) == 0 {
		return badRequest(c, "question_ids required")
	}

	questionIDs := make([]uuid.UUID, 0, len(req.QuestionIDs))
	for _, raw := range req.QuestionIDs {
		id, err := uuid.Parse(raw)
		if err != nil {
			return badRequest(c, "invalid question_id")
		}
		questionIDs = append(questionIDs, id)
	}

	if err := h.svc.ReorderTestQuestions(c.Request().Context(), testID, questionIDs); err != nil {
		return mapServiceError(c, err)
	}
	return c.NoContent(http.StatusNoContent)
}

func (h *Handler) AdminUpdateQuestion(c echo.Context) error {
	qID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return badRequest(c, "invalid id")
	}

	var req questionRequest
	if err := c.Bind(&req); err != nil {
		return badRequest(c, "invalid request body")
	}

	q, err := req.toQuestion()
	if err != nil {
		return mapServiceError(c, err)
	}
	q.ID = qID
	out, err := h.svc.SaveQuestion(c.Request().Context(), q, req.toOptions(), req.toBlanks())
	if err != nil {
		return mapServiceError(c, err)
	}
	return c.JSON(http.StatusOK, out)
}

func (h *Handler) AdminDeleteQuestion(c echo.Context) error {
	qID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return badRequest(c, "invalid id")
	}
	if err := h.svc.DeleteQuestion(c.Request().Context(), qID); err != nil {
		return mapServiceError(c, err)
	}
	return c.NoContent(http.StatusNoContent)
}

// AdminListTopics returns all curated topics with a per-topic question count (FR-16).
func (h *Handler) AdminListTopics(c echo.Context) error {
	filter := repository.TopicFilter{Subject: c.QueryParam("subject")}
	items, err := h.svc.ListTopics(c.Request().Context(), filter)
	if err != nil {
		return mapServiceError(c, err)
	}
	return c.JSON(http.StatusOK, map[string]interface{}{"data": items})
}

// AdminCreateTopic creates a new topic (FR-17).
func (h *Handler) AdminCreateTopic(c echo.Context) error {
	var req struct {
		Name    string `json:"name"`
		Subject string `json:"subject"`
	}
	if err := c.Bind(&req); err != nil {
		return badRequest(c, "invalid request body")
	}

	t := model.ExamTopic{Name: req.Name, Subject: req.Subject}
	out, err := h.svc.CreateTopic(c.Request().Context(), t)
	if err != nil {
		return mapServiceError(c, err)
	}
	return c.JSON(http.StatusCreated, out)
}

// AdminUpdateTopic updates an existing topic (FR-18).
func (h *Handler) AdminUpdateTopic(c echo.Context) error {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return badRequest(c, "invalid id")
	}

	var req struct {
		Name    string `json:"name"`
		Subject string `json:"subject"`
	}
	if err := c.Bind(&req); err != nil {
		return badRequest(c, "invalid request body")
	}

	t := model.ExamTopic{Name: req.Name, Subject: req.Subject}
	out, err := h.svc.UpdateTopic(c.Request().Context(), id, t)
	if err != nil {
		return mapServiceError(c, err)
	}
	return c.JSON(http.StatusOK, out)
}

// AdminDeleteTopic deletes a topic when no question references it (FR-19).
func (h *Handler) AdminDeleteTopic(c echo.Context) error {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return badRequest(c, "invalid id")
	}
	if err := h.svc.DeleteTopic(c.Request().Context(), id); err != nil {
		return mapServiceError(c, err)
	}
	return c.NoContent(http.StatusNoContent)
}

// AdminListBankQuestions returns the bank question list with cursor pagination (FR-14).
func (h *Handler) AdminListBankQuestions(c echo.Context) error {
	limit := 20
	if l := c.QueryParam("limit"); l != "" {
		if n, err := strconv.Atoi(l); err == nil && n > 0 {
			limit = n
		}
	}
	filter := repository.QuestionFilter{
		Format:  c.QueryParam("format"),
		TopicID: c.QueryParam("topic_id"),
		Search:  c.QueryParam("search"),
		Cursor:  c.QueryParam("cursor"),
		Limit:   limit,
	}

	items, nextCursor, err := h.svc.ListBankQuestions(c.Request().Context(), filter)
	if err != nil {
		return mapServiceError(c, err)
	}
	return c.JSON(http.StatusOK, map[string]interface{}{
		"data":        items,
		"next_cursor": nextCursor,
	})
}

// AdminImportQuestions imports questions from a multipart CSV (FR-45/46).
// Expected form field: "file". Returns a per-row report with inserted count.
func (h *Handler) AdminImportQuestions(c echo.Context) error {
	file, err := c.FormFile("file")
	if err != nil {
		return badRequest(c, "file required")
	}

	src, err := file.Open()
	if err != nil {
		return c.JSON(http.StatusInternalServerError, APIError{Code: "internal", Message: "cannot open uploaded file"})
	}
	defer src.Close()

	data, err := io.ReadAll(src)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, APIError{Code: "internal", Message: "cannot read uploaded file"})
	}

	result, err := h.svc.ImportQuestionsFromCSV(c.Request().Context(), data)
	if err != nil {
		return mapServiceError(c, err)
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"inserted": result.Inserted,
		"rows":     result.Rows,
	})
}

// AdminCreateBankQuestion creates a question in the bank with no test attachment (FR-9).
func (h *Handler) AdminCreateBankQuestion(c echo.Context) error {
	var req questionRequest
	if err := c.Bind(&req); err != nil {
		return badRequest(c, "invalid request body")
	}

	q, err := req.toQuestion()
	if err != nil {
		return mapServiceError(c, err)
	}
	out, err := h.svc.CreateBankQuestion(c.Request().Context(), q, req.toOptions(), req.toBlanks())
	if err != nil {
		return mapServiceError(c, err)
	}
	return c.JSON(http.StatusCreated, out)
}

// questionRequest is the shared body for AdminCreateQuestion / AdminUpdateQuestion.
type questionRequest struct {
	Format        string          `json:"format"`
	Body          string          `json:"body"`
	Difficulty    *string         `json:"difficulty,omitempty"`
	Explanation   *string         `json:"explanation,omitempty"`
	ImageURL      *string         `json:"image_url,omitempty"`
	AudioURL      *string         `json:"audio_url,omitempty"`
	CorrectAnswer *string         `json:"correct_answer,omitempty"`
	TopicID       *string         `json:"topic_id,omitempty"`
	Options       []optionRequest `json:"options,omitempty"`
	Blanks        []blankRequest  `json:"blanks,omitempty"`
	PointCorrect  *float64        `json:"point_correct,omitempty"`
	PointWrong    *float64        `json:"point_wrong,omitempty"`
}

type optionRequest struct {
	Key       string  `json:"key"`
	Text      string  `json:"text"`
	ImageURL  *string `json:"image_url,omitempty"`
	IsCorrect bool    `json:"is_correct"`
	SortOrder int     `json:"sort_order"`
}

type blankRequest struct {
	Index         int    `json:"index"`
	CorrectAnswer string `json:"correct_answer"`
}

func (r questionRequest) toQuestion() (model.Question, error) {
	pointCorrect := 1
	if r.PointCorrect != nil {
		v := *r.PointCorrect
		if float64(int(v)) != v {
			return model.Question{}, fmt.Errorf("%w: point_correct must be an integer", service.ErrValidation)
		}
		pointCorrect = int(v)
	}
	pointWrong := 0
	if r.PointWrong != nil {
		v := *r.PointWrong
		if float64(int(v)) != v {
			return model.Question{}, fmt.Errorf("%w: point_wrong must be an integer", service.ErrValidation)
		}
		pointWrong = int(v)
	}

	var topicID *uuid.UUID
	if r.TopicID != nil && *r.TopicID != "" {
		tid, err := uuid.Parse(*r.TopicID)
		if err != nil {
			return model.Question{}, fmt.Errorf("%w: topic_id is not a valid UUID", service.ErrValidation)
		}
		topicID = &tid
	}

	return model.Question{
		Format:        r.Format,
		Body:          r.Body,
		CorrectAnswer: r.CorrectAnswer,
		Explanation:   r.Explanation,
		Difficulty:    r.Difficulty,
		ImageURL:      r.ImageURL,
		AudioURL:      r.AudioURL,
		TopicID:       topicID,
		PointCorrect:  pointCorrect,
		PointWrong:    pointWrong,
	}, nil
}

func (r questionRequest) toOptions() []model.QuestionOption {
	out := make([]model.QuestionOption, 0, len(r.Options))
	for _, o := range r.Options {
		out = append(out, model.QuestionOption{
			Key:       o.Key,
			Text:      o.Text,
			ImageURL:  o.ImageURL,
			IsCorrect: o.IsCorrect,
			SortOrder: o.SortOrder,
		})
	}
	return out
}

func (r questionRequest) toBlanks() []model.QuestionBlank {
	out := make([]model.QuestionBlank, 0, len(r.Blanks))
	for _, b := range r.Blanks {
		out = append(out, model.QuestionBlank{
			Index:         b.Index,
			CorrectAnswer: b.CorrectAnswer,
		})
	}
	return out
}

// Student registration handlers (moved from competition.go).
func (h *Handler) StudentListRegistrations(c echo.Context) error {
	claims := claimsFromContext(c)
	if claims == nil || claims.Sub == "" {
		return c.JSON(http.StatusUnauthorized, APIError{Code: "unauthorized", Message: "missing auth"})
	}
	items, err := h.svc.GetExamRegistrations(c.Request().Context(), claims.Sub)
	if err != nil {
		return mapServiceError(c, err)
	}
	return c.JSON(http.StatusOK, map[string]interface{}{"data": items})
}

func (h *Handler) StudentGetRegistration(c echo.Context) error {
	claims := claimsFromContext(c)
	if claims == nil || claims.Sub == "" {
		return c.JSON(http.StatusUnauthorized, APIError{Code: "unauthorized", Message: "missing auth"})
	}
	id := c.Param("id")
	detail, err := h.svc.GetExamRegistration(c.Request().Context(), id, claims.Sub)
	if err != nil {
		return mapServiceError(c, err)
	}
	return c.JSON(http.StatusOK, detail)
}

func (h *Handler) StudentGetExamCard(c echo.Context) error {
	claims := claimsFromContext(c)
	if claims == nil || claims.Sub == "" {
		return c.JSON(http.StatusUnauthorized, APIError{Code: "unauthorized", Message: "missing auth"})
	}
	id := c.Param("id")
	signedURL, _, err := h.svc.GetExamCard(c.Request().Context(), id, claims.Sub)
	if err != nil {
		return mapServiceError(c, err)
	}
	// FR-30 serves the PDF from object storage via a fresh presigned GET rather
	// than streaming the bytes back through the API. The redirect is what the
	// browser's fetch follows; the presigned URL carries the download filename
	// through response-content-disposition.
	return c.Redirect(http.StatusFound, signedURL)
}

// fingerprint derives a device fingerprint from IP and User-Agent.
func fingerprint(ip, ua string) string {
	h := sha256.Sum256([]byte(ip + "|" + ua))
	return hex.EncodeToString(h[:])
}

// StudentCheckIn validates the registration token and stamps check-in. FR2-FR5.
func (h *Handler) StudentCheckIn(c echo.Context) error {
	claims := claimsFromContext(c)
	if claims == nil || claims.Sub == "" {
		return c.JSON(http.StatusUnauthorized, APIError{Code: "unauthorized", Message: "missing auth"})
	}
	var req struct {
		Token string `json:"token"`
	}
	if err := c.Bind(&req); err != nil {
		return badRequest(c, "invalid request body")
	}
	fp := fingerprint(c.RealIP(), c.Request().UserAgent())
	result, err := h.svc.CheckIn(c.Request().Context(), claims.Sub, req.Token, fp)
	if err != nil {
		return mapServiceError(c, err)
	}
	return c.JSON(http.StatusOK, result)
}

// StudentStartSession creates a new exam session. FR6-FR12.
func (h *Handler) StudentStartSession(c echo.Context) error {
	claims := claimsFromContext(c)
	if claims == nil || claims.Sub == "" {
		return c.JSON(http.StatusUnauthorized, APIError{Code: "unauthorized", Message: "missing auth"})
	}
	var req struct {
		RegistrationID string `json:"registration_id"`
	}
	if err := c.Bind(&req); err != nil {
		return badRequest(c, "invalid request body")
	}
	fp := fingerprint(c.RealIP(), c.Request().UserAgent())
	result, err := h.svc.StartSession(c.Request().Context(), claims.Sub, req.RegistrationID, fp)
	if err != nil {
		return mapServiceError(c, err)
	}
	return c.JSON(http.StatusOK, result)
}

// StudentReconnectSession returns current session state. FR13-FR14.
func (h *Handler) StudentReconnectSession(c echo.Context) error {
	claims := claimsFromContext(c)
	if claims == nil || claims.Sub == "" {
		return c.JSON(http.StatusUnauthorized, APIError{Code: "unauthorized", Message: "missing auth"})
	}
	sessionID := c.Param("id")
	result, err := h.svc.ReconnectSession(c.Request().Context(), claims.Sub, sessionID)
	if err != nil {
		return mapServiceError(c, err)
	}
	return c.JSON(http.StatusOK, result)
}

// StudentSaveAnswers upserts answers for a session. FR15-FR16.
func (h *Handler) StudentSaveAnswers(c echo.Context) error {
	claims := claimsFromContext(c)
	if claims == nil || claims.Sub == "" {
		return c.JSON(http.StatusUnauthorized, APIError{Code: "unauthorized", Message: "missing auth"})
	}
	sessionID := c.Param("id")
	var req struct {
		Answers []service.AnswerInput `json:"answers"`
	}
	if err := c.Bind(&req); err != nil {
		return badRequest(c, "invalid request body")
	}
	if err := h.svc.SaveAnswers(c.Request().Context(), claims.Sub, sessionID, req.Answers); err != nil {
		return mapServiceError(c, err)
	}
	return c.JSON(http.StatusOK, map[string]string{"status": "ok"})
}

// StudentSubmitSession grades and submits the session. FR17-FR20.
func (h *Handler) StudentSubmitSession(c echo.Context) error {
	claims := claimsFromContext(c)
	if claims == nil || claims.Sub == "" {
		return c.JSON(http.StatusUnauthorized, APIError{Code: "unauthorized", Message: "missing auth"})
	}
	sessionID := c.Param("id")
	result, err := h.svc.SubmitSession(c.Request().Context(), claims.Sub, sessionID)
	if err != nil {
		return mapServiceError(c, err)
	}
	return c.JSON(http.StatusOK, result)
}

// StudentAdvanceSection closes the active section and promotes the next (FR-10).
func (h *Handler) StudentAdvanceSection(c echo.Context) error {
	claims := claimsFromContext(c)
	if claims == nil || claims.Sub == "" {
		return c.JSON(http.StatusUnauthorized, APIError{Code: "unauthorized", Message: "missing auth"})
	}
	sessionID := c.Param("id")
	testID := c.Param("testId")
	result, err := h.svc.AdvanceSection(c.Request().Context(), claims.Sub, sessionID, testID)
	if err != nil {
		return mapServiceError(c, err)
	}
	return c.JSON(http.StatusOK, result)
}

// StudentLogViolation records an integrity event. FR21-FR22.
func (h *Handler) StudentLogViolation(c echo.Context) error {
	claims := claimsFromContext(c)
	if claims == nil || claims.Sub == "" {
		return c.JSON(http.StatusUnauthorized, APIError{Code: "unauthorized", Message: "missing auth"})
	}
	sessionID := c.Param("id")
	var req struct {
		ViolationType string `json:"violation_type"`
	}
	if err := c.Bind(&req); err != nil {
		return badRequest(c, "invalid request body")
	}
	if err := h.svc.LogViolation(c.Request().Context(), claims.Sub, sessionID, req.ViolationType); err != nil {
		return mapServiceError(c, err)
	}
	return c.JSON(http.StatusOK, map[string]string{"status": "ok"})
}

// AdminReopenSession extends a session's deadline. FR23.
func (h *Handler) AdminReopenSession(c echo.Context) error {
	sessionID := c.Param("id")
	var req struct {
		ExtendMinutes int `json:"extend_minutes"`
	}
	if err := c.Bind(&req); err != nil {
		return badRequest(c, "invalid request body")
	}
	if err := h.svc.ReopenSession(c.Request().Context(), sessionID, req.ExtendMinutes); err != nil {
		return mapServiceError(c, err)
	}
	return c.JSON(http.StatusOK, map[string]string{"status": "ok"})
}

// AdminForceSubmitSession grades and submits an in-progress session. FR24.
func (h *Handler) AdminForceSubmitSession(c echo.Context) error {
	sessionID := c.Param("id")
	result, err := h.svc.ForceSubmitSession(c.Request().Context(), sessionID)
	if err != nil {
		return mapServiceError(c, err)
	}
	return c.JSON(http.StatusOK, result)
}

// StudentGetSessionResult returns the gated result view for the caller's own session
// (FR-S5-20..24).
func (h *Handler) StudentGetSessionResult(c echo.Context) error {
	claims := claimsFromContext(c)
	if claims == nil || claims.Sub == "" {
		return c.JSON(http.StatusUnauthorized, APIError{Code: "unauthorized", Message: "missing auth"})
	}
	sessionID := c.Param("id")
	result, err := h.svc.GetSessionResult(c.Request().Context(), claims.Sub, sessionID)
	if err != nil {
		return mapServiceError(c, err)
	}
	return c.JSON(http.StatusOK, result)
}

// AdminListGradingSessions returns the grading queue for an exam (FR-S5-16).
func (h *Handler) AdminListGradingSessions(c echo.Context) error {
	examID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return badRequest(c, "invalid id")
	}
	items, err := h.svc.ListGradingSessions(c.Request().Context(), examID)
	if err != nil {
		return mapServiceError(c, err)
	}
	return c.JSON(http.StatusOK, map[string]interface{}{"data": items})
}

// AdminGetSessionEssays returns the essay answers of a session for grading (FR-S5-17).
func (h *Handler) AdminGetSessionEssays(c echo.Context) error {
	sessionID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return badRequest(c, "invalid id")
	}
	items, err := h.svc.GetSessionEssays(c.Request().Context(), sessionID)
	if err != nil {
		return mapServiceError(c, err)
	}
	return c.JSON(http.StatusOK, map[string]interface{}{"data": items})
}

// AdminGradeEssay grades one essay answer and recomputes the session total (FR-S5-12..14).
func (h *Handler) AdminGradeEssay(c echo.Context) error {
	sessionID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return badRequest(c, "invalid id")
	}
	claims := claimsFromContext(c)
	if claims == nil || claims.Sub == "" {
		return c.JSON(http.StatusUnauthorized, APIError{Code: "unauthorized", Message: "missing auth"})
	}
	graderID, err := uuid.Parse(claims.Sub)
	if err != nil {
		return badRequest(c, "invalid grader id")
	}

	var req struct {
		QuestionID string  `json:"question_id"`
		Score      float64 `json:"score"`
		Comment    *string `json:"comment,omitempty"`
	}
	if err := c.Bind(&req); err != nil {
		return badRequest(c, "invalid request body")
	}
	questionID, err := uuid.Parse(req.QuestionID)
	if err != nil {
		return badRequest(c, "invalid question_id")
	}

	total, err := h.svc.GradeEssayAnswer(c.Request().Context(), sessionID, questionID, req.Score, req.Comment, graderID)
	if err != nil {
		return mapServiceError(c, err)
	}
	return c.JSON(http.StatusOK, map[string]interface{}{"status": "ok", "score": total})
}

// AdminGetExamLeaderboard returns cursor-paginated leaderboard for an exam.
func (h *Handler) AdminGetExamLeaderboard(c echo.Context) error {
	examID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return badRequest(c, "invalid id")
	}
	limit := 20
	if l := c.QueryParam("limit"); l != "" {
		if n, err := strconv.Atoi(l); err == nil && n > 0 {
			limit = n
		}
	}
	cursor := c.QueryParam("cursor")

	entries, nextCursor, err := h.svc.AdminGetLeaderboard(c.Request().Context(), examID, cursor, limit)
	if err != nil {
		return mapServiceError(c, err)
	}
	return c.JSON(http.StatusOK, map[string]interface{}{
		"data":        entries,
		"next_cursor": nextCursor,
	})
}

// AdminGetExamAnalytics returns exam analytics (completion rate, avg score, distribution).
func (h *Handler) AdminGetExamAnalytics(c echo.Context) error {
	examID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return badRequest(c, "invalid id")
	}
	analytics, err := h.svc.GetExamAnalytics(c.Request().Context(), examID)
	if err != nil {
		return mapServiceError(c, err)
	}
	return c.JSON(http.StatusOK, analytics)
}

// AdminGetExamCertificatePreview streams a preview certificate PDF. It is a POST
// (not GET) because the optional JSON body carrying an unsaved layout lets the
// editor preview a change before saving it — a body a real browser can send,
// unlike a GET body which only some HTTP clients support; when absent, the
// exam's saved (or default) layout is used.
func (h *Handler) AdminGetExamCertificatePreview(c echo.Context) error {
	examID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return badRequest(c, "invalid id")
	}
	template := c.QueryParam("template")

	var body struct {
		Layout *service.Layout `json:"layout"`
	}
	if err := c.Bind(&body); err != nil {
		return badRequest(c, "invalid request body")
	}

	pdf, err := h.svc.GetCertificatePreviewWithLayout(c.Request().Context(), examID, template, body.Layout)
	if err != nil {
		return mapServiceError(c, err)
	}
	c.Response().Header().Set("Content-Type", "application/pdf")
	return c.Stream(http.StatusOK, "application/pdf", bytes.NewReader(pdf))
}

// certificateDesignRequest is the PUT body for AdminUpdateExamCertificateDesign:
// the full certificate design triplet (FR-17/FR-18/FR-26-29), replaced wholesale
// — unlike AdminUpdateExam's PATCH, there is no partial-overlay semantics here.
type certificateDesignRequest struct {
	Template      string         `json:"template"`
	BackgroundKey *string        `json:"background_key"`
	Layout        service.Layout `json:"layout"`
}

// AdminGetExamCertificateDesign returns the admin editor's read model: template,
// a presigned background URL (never the raw key, FR-18), and the resolved layout
// — the built-in default when nothing has been saved yet (FR-29).
func (h *Handler) AdminGetExamCertificateDesign(c echo.Context) error {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return badRequest(c, "invalid id")
	}
	resp, err := h.svc.GetCertificateDesign(c.Request().Context(), id)
	if err != nil {
		return mapServiceError(c, err)
	}
	return c.JSON(http.StatusOK, resp)
}

// AdminUpdateExamCertificateDesign persists the certificate design triplet the
// editor saves: template, background object key (never a URL, FR-18), and layout
// (validated server-side against Task 3's rules — the editor is not the security
// boundary). It overlays only these three fields onto the existing exam and
// reuses UpdateExam's staleness-bump wiring (FR-14/C3), mirroring AdminUpdateExam.
func (h *Handler) AdminUpdateExamCertificateDesign(c echo.Context) error {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return badRequest(c, "invalid id")
	}

	existing, err := h.svc.GetExam(c.Request().Context(), id)
	if err != nil {
		return mapServiceError(c, err)
	}

	var req certificateDesignRequest
	if err := c.Bind(&req); err != nil {
		return badRequest(c, "invalid request body")
	}

	// This endpoint always carries a full Layout (an omitted `layout` key binds
	// the zero value), so it must always be validated here — unlike the general
	// validateExam gate, which skips layout validation for a template-only
	// design blob (e.g. AdminUpdateExam's plain certificate_template PATCH).
	if err := service.ValidateLayout(req.Layout); err != nil {
		return mapServiceError(c, err)
	}

	raw, err := service.MarshalCertificateDesign(req.Template, req.BackgroundKey, req.Layout)
	if err != nil {
		return badRequest(c, "invalid layout")
	}

	overlay := existing.Exam
	overlay.CertificateDesign = raw

	out, err := h.svc.UpdateExam(c.Request().Context(), id, overlay)
	if err != nil {
		return mapServiceError(c, err)
	}
	return c.JSON(http.StatusOK, out)
}

// AdminGetSessionMonitor returns the session monitor payload for an exam: exam summary,
// one row per registrant with derived status, and recent violations. FR-1.
func (h *Handler) AdminGetSessionMonitor(c echo.Context) error {
	examID := c.QueryParam("exam_id")
	resp, err := h.svc.GetSessionMonitor(c.Request().Context(), examID)
	if err != nil {
		return mapServiceError(c, err)
	}
	return c.JSON(http.StatusOK, resp)
}

// AdminGetSessionViolations returns the violation log for a session, newest-first. FR-8.
func (h *Handler) AdminGetSessionViolations(c echo.Context) error {
	sessionID := c.Param("id")
	items, err := h.svc.GetSessionViolations(c.Request().Context(), sessionID)
	if err != nil {
		return mapServiceError(c, err)
	}
	return c.JSON(http.StatusOK, map[string]interface{}{"data": items})
}

// StudentGetSessionLeaderboard returns the exam leaderboard scoped to the caller's session.
func (h *Handler) StudentGetSessionLeaderboard(c echo.Context) error {
	claims := claimsFromContext(c)
	if claims == nil || claims.Sub == "" {
		return c.JSON(http.StatusUnauthorized, APIError{Code: "unauthorized", Message: "missing auth"})
	}
	limit := 20
	if l := c.QueryParam("limit"); l != "" {
		if n, err := strconv.Atoi(l); err == nil && n > 0 {
			limit = n
		}
	}
	cursor := c.QueryParam("cursor")
	sessionID := c.Param("id")

	entries, nextCursor, err := h.svc.StudentGetSessionLeaderboard(c.Request().Context(), claims.Sub, sessionID, cursor, limit)
	if err != nil {
		return mapServiceError(c, err)
	}
	return c.JSON(http.StatusOK, map[string]interface{}{
		"data":        entries,
		"next_cursor": nextCursor,
	})
}
