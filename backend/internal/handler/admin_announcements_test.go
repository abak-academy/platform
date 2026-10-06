package handler_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"akademi-bimbel/internal/handler"
	"akademi-bimbel/internal/model"
	"akademi-bimbel/internal/service"

	"github.com/alicebob/miniredis/v2"
	"github.com/labstack/echo/v4"
	"github.com/redis/go-redis/v9"
)

// handlerAnnounceFake is an in-memory AnnounceRepo for handler tests.
type handlerAnnounceFake struct {
	announcements map[string]*model.Announcement
	seq           int
}

func newHandlerAnnounceFake() *handlerAnnounceFake {
	return &handlerAnnounceFake{announcements: make(map[string]*model.Announcement)}
}

func (f *handlerAnnounceFake) CreateAnnouncement(_ context.Context, a *model.Announcement) error {
	f.seq++
	a.ID = fmt.Sprintf("ann-%d", f.seq)
	now := time.Now()
	a.CreatedAt = now
	a.UpdatedAt = now
	cp := *a
	f.announcements[a.ID] = &cp
	return nil
}

func (f *handlerAnnounceFake) GetAnnouncementByID(_ context.Context, id string) (*model.Announcement, error) {
	a, ok := f.announcements[id]
	if !ok {
		return nil, nil
	}
	cp := *a
	return &cp, nil
}

func (f *handlerAnnounceFake) ListAnnouncements(_ context.Context) ([]model.Announcement, error) {
	var result []model.Announcement
	for _, a := range f.announcements {
		result = append(result, *a)
	}
	return result, nil
}

func (f *handlerAnnounceFake) UpdateAnnouncement(_ context.Context, id string, a *model.Announcement) error {
	existing, ok := f.announcements[id]
	if !ok {
		return nil
	}
	existing.Title = a.Title
	existing.Message = a.Message
	existing.Type = a.Type
	existing.Recipients = a.Recipients
	existing.ScheduledAt = a.ScheduledAt
	existing.UpdatedAt = time.Now()
	return nil
}

func (f *handlerAnnounceFake) DeleteAnnouncement(_ context.Context, id string) error {
	delete(f.announcements, id)
	return nil
}

func (f *handlerAnnounceFake) ListActiveUserEmails(_ context.Context, recipients string) ([]string, error) {
	return nil, nil
}

func (f *handlerAnnounceFake) MarkAnnouncementSent(_ context.Context, id string, sentAt time.Time, recipientCount int) error {
	a, ok := f.announcements[id]
	if !ok {
		return nil
	}
	a.Status = "sent"
	a.SentAt = &sentAt
	a.RecipientCount = &recipientCount
	a.UpdatedAt = time.Now()
	return nil
}

func (f *handlerAnnounceFake) ClaimDueAnnouncements(_ context.Context, now time.Time, limit int) ([]model.Announcement, error) {
	return nil, nil
}

func (f *handlerAnnounceFake) ScheduleAnnouncementNow(_ context.Context, id string, at time.Time) error {
	a, ok := f.announcements[id]
	if !ok {
		return nil
	}
	a.Status = "scheduled"
	a.ScheduledAt = &at
	a.SentAt = nil
	a.RecipientCount = nil
	a.UpdatedAt = time.Now()
	return nil
}

// seed adds an announcement to the fake with the given status, returning its ID.
func (f *handlerAnnounceFake) seed(status string) string {
	f.seq++
	id := fmt.Sprintf("ann-seed-%d", f.seq)
	now := time.Now()
	f.announcements[id] = &model.Announcement{
		ID:        id,
		Title:     "Seed Title",
		Message:   "Seed Message",
		Type:      "announcement",
		Recipients: "all",
		Status:    status,
		CreatedBy: "admin-u1",
		CreatedAt: now,
		UpdatedAt: now,
	}
	return id
}

type adminAnnounceTestEnv struct {
	e    *echo.Echo
	h    *handler.Handler
	mr   *miniredis.Miniredis
	fake *handlerAnnounceFake
}

func newAdminAnnounceEnv(t *testing.T) *adminAnnounceTestEnv {
	t.Helper()
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("miniredis: %v", err)
	}
	t.Cleanup(mr.Close)

	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	repo := newFakeRepo()
	svc := service.New(repo, rdb, nil, nil, nil, nil)
	fake := newHandlerAnnounceFake()
	svc.SetAnnounceRepo(fake)

	e := echo.New()
	e.HideBanner = true
	return &adminAnnounceTestEnv{e: e, h: handler.New(svc), mr: mr, fake: fake}
}

// --- AdminCreateAnnouncement ---

func TestAdminCreateAnnouncement_EmptyBody_400(t *testing.T) {
	env := newAdminAnnounceEnv(t)
	body := bytes.NewReader([]byte(`{}`))
	req := httptest.NewRequest(http.MethodPost, "/admin/notifications", body)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	c := env.e.NewContext(req, rec)
	setAdminClaims(c, "admin-u1")

	err := env.h.AdminCreateAnnouncement(c)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("want 400, got %d body=%s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	json.NewDecoder(rec.Body).Decode(&resp)
	if resp["code"] != "invalid_request" {
		t.Errorf("want code=invalid_request, got %v", resp["code"])
	}
}

func TestAdminCreateAnnouncement_MissingTitle_400(t *testing.T) {
	env := newAdminAnnounceEnv(t)
	payload := map[string]string{"message": "hello"}
	b, _ := json.Marshal(payload)
	body := bytes.NewReader(b)
	req := httptest.NewRequest(http.MethodPost, "/admin/notifications", body)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	c := env.e.NewContext(req, rec)
	setAdminClaims(c, "admin-u1")

	err := env.h.AdminCreateAnnouncement(c)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("want 400, got %d body=%s", rec.Code, rec.Body.String())
	}
}

func TestAdminCreateAnnouncement_HappyPath_201(t *testing.T) {
	env := newAdminAnnounceEnv(t)
	payload := map[string]string{
		"title":      "Test Announcement",
		"message":    "This is a test",
		"type":       "announcement",
		"recipients": "all",
		"status":     "draft",
	}
	b, _ := json.Marshal(payload)
	body := bytes.NewReader(b)
	req := httptest.NewRequest(http.MethodPost, "/admin/notifications", body)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	c := env.e.NewContext(req, rec)
	setAdminClaims(c, "admin-u1")

	err := env.h.AdminCreateAnnouncement(c)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rec.Code != http.StatusCreated {
		t.Fatalf("want 201, got %d body=%s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	json.NewDecoder(rec.Body).Decode(&resp)
	if resp["id"] == "" || resp["id"] == nil {
		t.Error("want non-empty id in response")
	}
	if resp["title"] != "Test Announcement" {
		t.Errorf("want title=%q, got %v", "Test Announcement", resp["title"])
	}
}

func TestAdminCreateAnnouncement_InvalidType_400(t *testing.T) {
	env := newAdminAnnounceEnv(t)
	payload := map[string]string{
		"title":      "Test",
		"message":    "msg",
		"type":       "invalid_type",
		"recipients": "all",
		"status":     "draft",
	}
	b, _ := json.Marshal(payload)
	body := bytes.NewReader(b)
	req := httptest.NewRequest(http.MethodPost, "/admin/notifications/announcements", body)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	c := env.e.NewContext(req, rec)
	setAdminClaims(c, "admin-u1")

	err := env.h.AdminCreateAnnouncement(c)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("want 400, got %d body=%s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	json.NewDecoder(rec.Body).Decode(&resp)
	if resp["code"] != "invalid_request" {
		t.Errorf("want code=invalid_request, got %v", resp["code"])
	}
}

// --- AdminListAnnouncements ---

func TestAdminListAnnouncements_HappyPath_200(t *testing.T) {
	env := newAdminAnnounceEnv(t)
	env.fake.seed("draft")
	env.fake.seed("sent")

	req := httptest.NewRequest(http.MethodGet, "/admin/notifications", nil)
	rec := httptest.NewRecorder()
	c := env.e.NewContext(req, rec)

	err := env.h.AdminListAnnouncements(c)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("want 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	json.NewDecoder(rec.Body).Decode(&resp)
	data, ok := resp["data"].([]any)
	if !ok {
		t.Fatalf("want data array, got %T", resp["data"])
	}
	if len(data) != 2 {
		t.Errorf("want 2 announcements, got %d", len(data))
	}
}

// --- AdminUpdateAnnouncement ---

func TestAdminUpdateAnnouncement_NotFound_404(t *testing.T) {
	env := newAdminAnnounceEnv(t)
	payload := map[string]string{"title": "Updated"}
	b, _ := json.Marshal(payload)
	body := bytes.NewReader(b)
	req := httptest.NewRequest(http.MethodPatch, "/admin/notifications/nonexistent", body)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	c := env.e.NewContext(req, rec)
	c.SetParamNames("id")
	c.SetParamValues("nonexistent")

	err := env.h.AdminUpdateAnnouncement(c)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rec.Code != http.StatusNotFound {
		t.Fatalf("want 404, got %d body=%s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	json.NewDecoder(rec.Body).Decode(&resp)
	if resp["code"] != "announcement_not_found" {
		t.Errorf("want code=announcement_not_found, got %v", resp["code"])
	}
}

func TestAdminUpdateAnnouncement_Immutable_409(t *testing.T) {
	env := newAdminAnnounceEnv(t)
	id := env.fake.seed("sent")

	payload := map[string]string{"title": "Updated"}
	b, _ := json.Marshal(payload)
	body := bytes.NewReader(b)
	req := httptest.NewRequest(http.MethodPatch, "/admin/notifications/"+id, body)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	c := env.e.NewContext(req, rec)
	c.SetParamNames("id")
	c.SetParamValues(id)

	err := env.h.AdminUpdateAnnouncement(c)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rec.Code != http.StatusConflict {
		t.Fatalf("want 409, got %d body=%s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	json.NewDecoder(rec.Body).Decode(&resp)
	if resp["code"] != "announcement_immutable" {
		t.Errorf("want code=announcement_immutable, got %v", resp["code"])
	}
}

func TestAdminUpdateAnnouncement_HappyPath_200(t *testing.T) {
	env := newAdminAnnounceEnv(t)
	id := env.fake.seed("draft")

	payload := map[string]string{"title": "Updated Title"}
	b, _ := json.Marshal(payload)
	body := bytes.NewReader(b)
	req := httptest.NewRequest(http.MethodPatch, "/admin/notifications/"+id, body)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	c := env.e.NewContext(req, rec)
	c.SetParamNames("id")
	c.SetParamValues(id)

	err := env.h.AdminUpdateAnnouncement(c)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("want 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	json.NewDecoder(rec.Body).Decode(&resp)
	if resp["title"] != "Updated Title" {
		t.Errorf("want title=Updated Title, got %v", resp["title"])
	}
}

// --- AdminDeleteAnnouncement ---

func TestAdminDeleteAnnouncement_NotFound_404(t *testing.T) {
	env := newAdminAnnounceEnv(t)
	req := httptest.NewRequest(http.MethodDelete, "/admin/notifications/nonexistent", nil)
	rec := httptest.NewRecorder()
	c := env.e.NewContext(req, rec)
	c.SetParamNames("id")
	c.SetParamValues("nonexistent")

	err := env.h.AdminDeleteAnnouncement(c)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rec.Code != http.StatusNotFound {
		t.Fatalf("want 404, got %d body=%s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	json.NewDecoder(rec.Body).Decode(&resp)
	if resp["code"] != "announcement_not_found" {
		t.Errorf("want code=announcement_not_found, got %v", resp["code"])
	}
}

func TestAdminDeleteAnnouncement_Immutable_409(t *testing.T) {
	env := newAdminAnnounceEnv(t)
	id := env.fake.seed("sent")

	req := httptest.NewRequest(http.MethodDelete, "/admin/notifications/"+id, nil)
	rec := httptest.NewRecorder()
	c := env.e.NewContext(req, rec)
	c.SetParamNames("id")
	c.SetParamValues(id)

	err := env.h.AdminDeleteAnnouncement(c)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rec.Code != http.StatusConflict {
		t.Fatalf("want 409, got %d body=%s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	json.NewDecoder(rec.Body).Decode(&resp)
	if resp["code"] != "announcement_immutable" {
		t.Errorf("want code=announcement_immutable, got %v", resp["code"])
	}
}

func TestAdminDeleteAnnouncement_HappyPath_200(t *testing.T) {
	env := newAdminAnnounceEnv(t)
	id := env.fake.seed("draft")

	req := httptest.NewRequest(http.MethodDelete, "/admin/notifications/"+id, nil)
	rec := httptest.NewRecorder()
	c := env.e.NewContext(req, rec)
	c.SetParamNames("id")
	c.SetParamValues(id)

	err := env.h.AdminDeleteAnnouncement(c)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("want 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	json.NewDecoder(rec.Body).Decode(&resp)
	if resp["message"] != "announcement deleted" {
		t.Errorf("want message=announcement deleted, got %v", resp["message"])
	}
}

// --- AdminSendAnnouncement ---

func TestAdminSendAnnouncement_NotFound_404(t *testing.T) {
	env := newAdminAnnounceEnv(t)
	req := httptest.NewRequest(http.MethodPost, "/admin/notifications/nonexistent/send", nil)
	rec := httptest.NewRecorder()
	c := env.e.NewContext(req, rec)
	c.SetParamNames("id")
	c.SetParamValues("nonexistent")

	err := env.h.AdminSendAnnouncement(c)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rec.Code != http.StatusNotFound {
		t.Fatalf("want 404, got %d body=%s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	json.NewDecoder(rec.Body).Decode(&resp)
	if resp["code"] != "announcement_not_found" {
		t.Errorf("want code=announcement_not_found, got %v", resp["code"])
	}
}

func TestAdminSendAnnouncement_Immutable_409(t *testing.T) {
	env := newAdminAnnounceEnv(t)
	id := env.fake.seed("sent")

	req := httptest.NewRequest(http.MethodPost, "/admin/notifications/"+id+"/send", nil)
	rec := httptest.NewRecorder()
	c := env.e.NewContext(req, rec)
	c.SetParamNames("id")
	c.SetParamValues(id)

	err := env.h.AdminSendAnnouncement(c)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rec.Code != http.StatusConflict {
		t.Fatalf("want 409, got %d body=%s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	json.NewDecoder(rec.Body).Decode(&resp)
	if resp["code"] != "announcement_immutable" {
		t.Errorf("want code=announcement_immutable, got %v", resp["code"])
	}
}

func TestAdminSendAnnouncement_HappyPath_200(t *testing.T) {
	env := newAdminAnnounceEnv(t)
	id := env.fake.seed("draft")

	req := httptest.NewRequest(http.MethodPost, "/admin/notifications/"+id+"/send", nil)
	rec := httptest.NewRecorder()
	c := env.e.NewContext(req, rec)
	c.SetParamNames("id")
	c.SetParamValues(id)

	err := env.h.AdminSendAnnouncement(c)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("want 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	json.NewDecoder(rec.Body).Decode(&resp)
	if resp["id"] == "" || resp["id"] == nil {
		t.Error("want non-empty id in response")
	}
}
