package handler_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"akademi-bimbel/config"
	"akademi-bimbel/internal/handler"
	"akademi-bimbel/internal/infra"
	"akademi-bimbel/internal/service"

	"github.com/labstack/echo/v4"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

// setAdminExamClaims sets admin_exam claims on the echo context.
func setAdminExamClaims(c echo.Context, sub string) {
	c.Set("claims", &infra.Claims{Sub: sub, Role: "admin_exam"})
}

// newAdminUploadsEnvWithStorage mirrors newAdminSystemEnv but wires a real
// minio client (Region set explicitly, so PUT presigning never dials out —
// see avatar_proxy_test.go for the same trick) plus a non-nil cfg, so the
// presign tests here can assert on the actual returned key's prefix instead
// of only checking that validation didn't 400.
func newAdminUploadsEnvWithStorage(t *testing.T) *adminSystemTestEnv {
	t.Helper()
	env := newAdminSystemEnv(t)
	storage, err := minio.New("localhost:9000", &minio.Options{
		Creds:  credentials.NewStaticV4("ak", "sk", ""),
		Secure: false,
		Region: "us-east-1",
	})
	if err != nil {
		t.Fatalf("minio.New: %v", err)
	}
	svc := service.NewWithStore(newFakeRepo(), nil, nil, nil, nil, nil, nil, nil, storage, &config.Config{ObjectStorageBucketName: "bucket"})
	env.h = handler.New(svc)
	return env
}

// TestAdminUploadImage_ValidImageContentType_PassesValidation verifies that a valid
// image content type passes validation (proceeds to service call).
func TestAdminUploadImage_ValidImageContentType_PassesValidation(t *testing.T) {
	env := newAdminSystemEnv(t)
	req := httptest.NewRequest(http.MethodPost, "/admin/uploads/image?filename=test.png&content_type=image/png", nil)
	rec := httptest.NewRecorder()
	c := env.e.NewContext(req, rec)
	setAdminExamClaims(c, "exam-admin-1")

	if err := env.h.AdminUploadImage(c); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Validation passes; service call fails (no storage configured in test).
	// We're testing that validation didn't reject it (would be 400 if rejected).
	if rec.Code == http.StatusBadRequest {
		t.Errorf("content-type validation incorrectly rejected image/png")
	}
}

// TestAdminUploadImage_InvalidContentType_Returns400 verifies that non-image
// content types are rejected.
func TestAdminUploadImage_InvalidContentType_Returns400(t *testing.T) {
	env := newAdminSystemEnv(t)
	req := httptest.NewRequest(http.MethodPost, "/admin/uploads/image?filename=test.mp3&content_type=audio/mpeg", nil)
	rec := httptest.NewRecorder()
	c := env.e.NewContext(req, rec)
	setAdminExamClaims(c, "exam-admin-1")

	if err := env.h.AdminUploadImage(c); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rec.Code != http.StatusBadRequest {
		t.Errorf("want 400, got %d", rec.Code)
	}
}

// TestAdminUploadImage_MissingFilename_Returns400 verifies validation of required params.
func TestAdminUploadImage_MissingFilename_Returns400(t *testing.T) {
	env := newAdminSystemEnv(t)
	req := httptest.NewRequest(http.MethodPost, "/admin/uploads/image?content_type=image/png", nil)
	rec := httptest.NewRecorder()
	c := env.e.NewContext(req, rec)
	setAdminExamClaims(c, "exam-admin-1")

	if err := env.h.AdminUploadImage(c); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rec.Code != http.StatusBadRequest {
		t.Errorf("want 400 for missing filename, got %d", rec.Code)
	}
}

// TestAdminUploadImage_NoAuth_Returns403 verifies that unauthenticated requests fail.
func TestAdminUploadImage_NoAuth_Returns403(t *testing.T) {
	env := newAdminSystemEnv(t)
	req := httptest.NewRequest(http.MethodPost, "/admin/uploads/image?filename=test.png&content_type=image/png", nil)
	rec := httptest.NewRecorder()
	c := env.e.NewContext(req, rec)
	// No claims set

	if err := env.h.AdminUploadImage(c); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rec.Code != http.StatusForbidden {
		t.Errorf("want 403 for no auth, got %d", rec.Code)
	}
}

// TestAdminUploadImage_WithClaims_ProceedsToService verifies that valid
// claims proceed through handler validation (RBAC check is in middleware).
func TestAdminUploadImage_WithClaims_ProceedsToService(t *testing.T) {
	env := newAdminSystemEnv(t)
	req := httptest.NewRequest(http.MethodPost, "/admin/uploads/image?filename=test.png&content_type=image/png", nil)
	rec := httptest.NewRecorder()
	c := env.e.NewContext(req, rec)
	c.Set("claims", &infra.Claims{Sub: "user-1", Role: "student"})

	if err := env.h.AdminUploadImage(c); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Handler proceeds (RBAC is enforced in middleware via routes.go).
	// Service call fails (no storage), but validation passed.
	if rec.Code == http.StatusBadRequest {
		t.Errorf("handler validation incorrectly rejected valid request")
	}
}

// TestAdminUploadAudio_ValidAudioContentType_PassesValidation verifies that a valid
// audio content type passes validation (proceeds to service call).
func TestAdminUploadAudio_ValidAudioContentType_PassesValidation(t *testing.T) {
	env := newAdminSystemEnv(t)
	req := httptest.NewRequest(http.MethodPost, "/admin/uploads/audio?filename=test.mp3&content_type=audio/mpeg", nil)
	rec := httptest.NewRecorder()
	c := env.e.NewContext(req, rec)
	setAdminExamClaims(c, "exam-admin-1")

	if err := env.h.AdminUploadAudio(c); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Validation passes; service call fails (no storage configured in test).
	// We're testing that validation didn't reject it (would be 400 if rejected).
	if rec.Code == http.StatusBadRequest {
		t.Errorf("content-type validation incorrectly rejected audio/mpeg")
	}
}

// TestAdminUploadAudio_InvalidContentType_Returns400 verifies that non-audio
// content types are rejected.
func TestAdminUploadAudio_InvalidContentType_Returns400(t *testing.T) {
	env := newAdminSystemEnv(t)
	req := httptest.NewRequest(http.MethodPost, "/admin/uploads/audio?filename=test.png&content_type=image/png", nil)
	rec := httptest.NewRecorder()
	c := env.e.NewContext(req, rec)
	setAdminExamClaims(c, "exam-admin-1")

	if err := env.h.AdminUploadAudio(c); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rec.Code != http.StatusBadRequest {
		t.Errorf("want 400, got %d", rec.Code)
	}
}

// TestAdminUploadAudio_NoAuth_Returns403 verifies that unauthenticated requests fail.
func TestAdminUploadAudio_NoAuth_Returns403(t *testing.T) {
	env := newAdminSystemEnv(t)
	req := httptest.NewRequest(http.MethodPost, "/admin/uploads/audio?filename=test.mp3&content_type=audio/mpeg", nil)
	rec := httptest.NewRecorder()
	c := env.e.NewContext(req, rec)
	// No claims set

	if err := env.h.AdminUploadAudio(c); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rec.Code != http.StatusForbidden {
		t.Errorf("want 403 for no auth, got %d", rec.Code)
	}
}

// TestAdminUploadImage_KeyUnderQuestionPrefix verifies the signed key routes
// to question/, not avatars/ — question images share the same admin upload
// pipeline as listening audio, both of which are content the student read-
// proxy must serve from a non-avatars prefix.
func TestAdminUploadImage_KeyUnderQuestionPrefix(t *testing.T) {
	env := newAdminUploadsEnvWithStorage(t)
	req := httptest.NewRequest(http.MethodPost, "/admin/uploads/image?filename=test.png&content_type=image/png", nil)
	rec := httptest.NewRecorder()
	c := env.e.NewContext(req, rec)
	setAdminExamClaims(c, "exam-admin-1")

	if err := env.h.AdminUploadImage(c); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("want 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	json.NewDecoder(rec.Body).Decode(&resp)
	key, _ := resp["key"].(string)
	if !strings.HasPrefix(key, "question/") {
		t.Errorf("want key under question/, got %q", key)
	}
}

// TestAdminUploadAudio_KeyUnderQuestionPrefix verifies listening audio also
// routes to question/ (see TestAdminUploadImage_KeyUnderQuestionPrefix).
func TestAdminUploadAudio_KeyUnderQuestionPrefix(t *testing.T) {
	env := newAdminUploadsEnvWithStorage(t)
	req := httptest.NewRequest(http.MethodPost, "/admin/uploads/audio?filename=test.mp3&content_type=audio/mpeg", nil)
	rec := httptest.NewRecorder()
	c := env.e.NewContext(req, rec)
	setAdminExamClaims(c, "exam-admin-1")

	if err := env.h.AdminUploadAudio(c); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("want 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	json.NewDecoder(rec.Body).Decode(&resp)
	key, _ := resp["key"].(string)
	if !strings.HasPrefix(key, "question/") {
		t.Errorf("want key under question/, got %q", key)
	}
}
