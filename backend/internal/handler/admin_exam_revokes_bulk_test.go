package handler_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"

	"akademi-bimbel/config"
	"akademi-bimbel/internal/handler"
	"akademi-bimbel/internal/infra"
	"akademi-bimbel/internal/model"
	"akademi-bimbel/internal/service"

	"github.com/labstack/echo/v4"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

// examRevokeBulkKeyRe pins the key shape: exam-revoke-bulk/{examID}/{uuid}-{filename}.
var examRevokeBulkKeyRe = regexp.MustCompile(`^exam-revoke-bulk/[0-9a-f-]{36}/[0-9a-f-]{36}-`)

// newExamRevokeRouter mirrors routes.go's adminExamGrants group — the
// RBACMiddleware("exam-grants:write") guard (super_admin only) is the point
// of these routes, so these tests drive the real router rather than calling
// the handlers directly.
func newExamRevokeRouter(t *testing.T, h *handler.Handler) *echo.Echo {
	t.Helper()
	e := echo.New()
	e.HideBanner = true
	g := e.Group("/admin/exam-grants")
	g.Use(handler.RBACMiddleware("exam-grants:write"))
	g.POST("/revoke", h.AdminRevokeExamAccess)
	g.POST("/revoke/bulk/presign", h.AdminPresignExamRevokeBulkUpload)
	g.POST("/revoke/bulk", h.AdminEnqueueExamRevokeBulk)
	return e
}

func newExamRevokePresignHandler(t *testing.T) *handler.Handler {
	t.Helper()
	storage, err := minio.New("localhost:9000", &minio.Options{
		Creds:  credentials.NewStaticV4("ak", "sk", ""),
		Secure: false,
		Region: "us-east-1",
	})
	if err != nil {
		t.Fatalf("minio.New: %v", err)
	}
	svc := service.NewWithStore(newFakeRepo(), nil, nil, nil, nil, nil, nil, nil, storage,
		&config.Config{ObjectStoragePrivateBucketName: "private", ObjectStorageRegion: "us-east-1"}, nil)
	return handler.New(svc)
}

func TestAdminPresignExamRevokeBulkUpload_SuperAdmin_200_KeyHasExamSegment(t *testing.T) {
	e := newExamRevokeRouter(t, newExamRevokePresignHandler(t))
	e.Use(claimsInjector(&infra.Claims{Sub: "sa1", Role: "super_admin"}))

	req := httptest.NewRequest(http.MethodPost,
		"/admin/exam-grants/revoke/bulk/presign?exam_id="+testExamID+"&filename=revokes.csv&content_type=text/csv", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("want 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp service.PrivateUploadURL
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if !examRevokeBulkKeyRe.MatchString(resp.Key) {
		t.Errorf("key %q does not match %s", resp.Key, examRevokeBulkKeyRe)
	}
	if !strings.Contains(resp.Key, testExamID) {
		t.Errorf("key %q does not encode exam id %s", resp.Key, testExamID)
	}
	if resp.Method != "PUT" {
		t.Errorf("Method: want PUT, got %s", resp.Method)
	}
}

func TestAdminPresignExamRevokeBulkUpload_MissingParams_400(t *testing.T) {
	cases := []struct{ name, path string }{
		{"missing exam_id", "/admin/exam-grants/revoke/bulk/presign?filename=revokes.csv"},
		{"missing filename", "/admin/exam-grants/revoke/bulk/presign?exam_id=" + testExamID},
		{"invalid exam_id", "/admin/exam-grants/revoke/bulk/presign?exam_id=not-a-uuid&filename=revokes.csv"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			e := newExamRevokeRouter(t, newExamRevokePresignHandler(t))
			e.Use(claimsInjector(&infra.Claims{Sub: "sa1", Role: "super_admin"}))

			req := httptest.NewRequest(http.MethodPost, tc.path, nil)
			rec := httptest.NewRecorder()
			e.ServeHTTP(rec, req)

			if rec.Code != http.StatusBadRequest {
				t.Fatalf("want 400, got %d: %s", rec.Code, rec.Body.String())
			}
		})
	}
}

func TestAdminRevokeExamAccess_MissingStudentIDs_400(t *testing.T) {
	e := newExamRevokeRouter(t, newExamRevokePresignHandler(t))
	e.Use(claimsInjector(&infra.Claims{Sub: "sa1", Role: "super_admin"}))

	req := httptest.NewRequest(http.MethodPost, "/admin/exam-grants/revoke",
		strings.NewReader(`{"exam_id":"`+testExamID+`"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("want 400, got %d: %s", rec.Code, rec.Body.String())
	}
}

// TestAdminExamRevokeRoutes_NonSuperAdmin_403 is the super-admin-only guard:
// only the role that grants exam access may revoke it.
func TestAdminExamRevokeRoutes_NonSuperAdmin_403(t *testing.T) {
	schoolID := "22222222-2222-2222-2222-222222222222"
	roles := map[string]*infra.Claims{
		"admin_school": {Sub: "as1", Role: "admin_school", SchoolID: &schoolID},
		"admin_exam":   {Sub: "ae1", Role: "admin_exam"},
		"admin_store":  {Sub: "at1", Role: "admin_store"},
		"student":      {Sub: "st1", Role: "student"},
	}
	targets := []struct {
		name, path, body string
	}{
		{"revoke", "/admin/exam-grants/revoke", `{"exam_id":"` + testExamID + `","student_ids":["44444444-4444-4444-4444-444444444444"]}`},
		{"presign", "/admin/exam-grants/revoke/bulk/presign?exam_id=" + testExamID + "&filename=revokes.csv", ""},
		{"enqueue", "/admin/exam-grants/revoke/bulk", `{"exam_id":"` + testExamID + `","file_key":"exam-revoke-bulk/` + testExamID + `/x.csv"}`},
	}

	for role, claims := range roles {
		for _, target := range targets {
			t.Run(role+"/"+target.name, func(t *testing.T) {
				e := newExamRevokeRouter(t, newExamRevokePresignHandler(t))
				e.Use(claimsInjector(claims))

				req := httptest.NewRequest(http.MethodPost, target.path, strings.NewReader(target.body))
				req.Header.Set("Content-Type", "application/json")
				rec := httptest.NewRecorder()
				e.ServeHTTP(rec, req)

				if rec.Code != http.StatusForbidden {
					t.Fatalf("want 403, got %d: %s", rec.Code, rec.Body.String())
				}
			})
		}
	}
}

// TestAdminExamRevokeRoutes_NoClaims_401 pins the unauthorized side of the
// RBAC middleware before any capability check runs.
func TestAdminExamRevokeRoutes_NoClaims_401(t *testing.T) {
	e := newExamRevokeRouter(t, newExamRevokePresignHandler(t))

	req := httptest.NewRequest(http.MethodPost, "/admin/exam-grants/revoke", strings.NewReader("{}"))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("want 401, got %d: %s", rec.Code, rec.Body.String())
	}
}

// TestAdminEnqueueExamRevokeBulk_PrefixMismatch_404 pins the "validate key
// prefix matches exam_id" requirement for the revoke-side enqueue.
func TestAdminEnqueueExamRevokeBulk_PrefixMismatch_404(t *testing.T) {
	otherExamID := "33333333-3333-3333-3333-333333333333"
	for _, fileKey := range []string{
		"exam-revoke-bulk/" + otherExamID + "/x.csv",
		"exam-grant-bulk/" + otherExamID + "/x.csv",
	} {
		e := newExamRevokeRouter(t, newExamRevokePresignHandler(t))
		e.Use(claimsInjector(&infra.Claims{Sub: "sa1", Role: "super_admin"}))

		body, _ := json.Marshal(map[string]string{"exam_id": testExamID, "file_key": fileKey})
		req := httptest.NewRequest(http.MethodPost, "/admin/exam-grants/revoke/bulk", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)

		if rec.Code != http.StatusNotFound {
			t.Fatalf("file_key %s: want 404 upload_not_found, got %d: %s", fileKey, rec.Code, rec.Body.String())
		}
	}
}

// TestAdminEnqueueExamRevokeBulk_Valid_202 drives the real handler -> real
// service -> real Postgres and asserts the persisted job's type and key.
func TestAdminEnqueueExamRevokeBulk_Valid_202(t *testing.T) {
	jobs := newAdminJobsEnv(t)
	owner := seedJobOwner(t, jobs, "examrevokebulk")
	exam, err := jobs.svc.CreateExam(context.Background(), model.Exam{Title: "Exam Revoke Bulk Test", Mode: ""})
	if err != nil {
		t.Fatalf("CreateExam: %v", err)
	}

	storage := stubObjectStorage(t, "username\nandi123\n")
	svc := service.NewWithStore(jobs.repo, jobs.repo, nil, nil, &service.NoopOTPProvider{}, &service.NoopEmailProvider{},
		nil, nil, storage, &config.Config{ObjectStoragePrivateBucketName: "private", ObjectStorageRegion: "us-east-1"}, nil)

	e := newExamRevokeRouter(t, handler.New(svc))
	e.Use(claimsInjector(&infra.Claims{Sub: owner, Role: "super_admin"}))

	examID := exam.ID.String()
	fileKey := "exam-revoke-bulk/" + examID + "/abc-revokes.csv"
	body, _ := json.Marshal(map[string]string{"exam_id": examID, "file_key": fileKey})
	req := httptest.NewRequest(http.MethodPost, "/admin/exam-grants/revoke/bulk", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusAccepted {
		t.Fatalf("want 202, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp["job_id"] == "" {
		t.Fatalf("want a job_id in the 202 body, got %v", resp)
	}

	job, err := jobs.repo.GetJobByID(context.Background(), resp["job_id"])
	if err != nil {
		t.Fatalf("GetJobByID: %v", err)
	}
	if job == nil || job.Type != "exam_revoke_bulk" {
		t.Fatalf("want a persisted exam_revoke_bulk job, got %+v", job)
	}
	if job.InputURL == nil || *job.InputURL != fileKey {
		t.Fatalf("want input_url %q, got %+v", fileKey, job.InputURL)
	}
}
