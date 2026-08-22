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

// examGrantBulkKeyRe pins the spec's key shape: exam-grant-bulk/{examID}/{uuid}-{filename}.
var examGrantBulkKeyRe = regexp.MustCompile(`^exam-grant-bulk/[0-9a-f-]{36}/[0-9a-f-]{36}-`)

const testExamID = "11111111-1111-1111-1111-111111111111"

// newExamGrantBulkRouter mirrors routes.go's adminExamGrants group — the
// RBACMiddleware("exam-grants:write") guard (super_admin only) is the point
// of this route, so these tests drive the real router rather than calling
// the handler directly.
func newExamGrantBulkRouter(t *testing.T, h *handler.Handler) *echo.Echo {
	t.Helper()
	e := echo.New()
	e.HideBanner = true
	g := e.Group("/admin/exam-grants")
	g.Use(handler.RBACMiddleware("exam-grants:write"))
	g.POST("/bulk/presign", h.AdminPresignExamGrantBulkUpload)
	g.POST("/bulk", h.AdminEnqueueExamGrantBulk)
	return e
}

func newExamGrantBulkPresignHandler(t *testing.T) *handler.Handler {
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

func TestAdminPresignExamGrantBulkUpload_SuperAdmin_200_KeyHasExamSegment(t *testing.T) {
	e := newExamGrantBulkRouter(t, newExamGrantBulkPresignHandler(t))
	e.Use(claimsInjector(&infra.Claims{Sub: "sa1", Role: "super_admin"}))

	req := httptest.NewRequest(http.MethodPost,
		"/admin/exam-grants/bulk/presign?exam_id="+testExamID+"&filename=grants.csv&content_type=text/csv", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("want 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp service.PrivateUploadURL
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if !examGrantBulkKeyRe.MatchString(resp.Key) {
		t.Errorf("key %q does not match %s", resp.Key, examGrantBulkKeyRe)
	}
	if !strings.Contains(resp.Key, testExamID) {
		t.Errorf("key %q does not encode exam id %s", resp.Key, testExamID)
	}
	if resp.Method != "PUT" {
		t.Errorf("Method: want PUT, got %s", resp.Method)
	}
}

func TestAdminPresignExamGrantBulkUpload_MissingExamID_400(t *testing.T) {
	e := newExamGrantBulkRouter(t, newExamGrantBulkPresignHandler(t))
	e.Use(claimsInjector(&infra.Claims{Sub: "sa1", Role: "super_admin"}))

	req := httptest.NewRequest(http.MethodPost, "/admin/exam-grants/bulk/presign?filename=grants.csv", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("want 400, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestAdminPresignExamGrantBulkUpload_MissingFilename_400(t *testing.T) {
	e := newExamGrantBulkRouter(t, newExamGrantBulkPresignHandler(t))
	e.Use(claimsInjector(&infra.Claims{Sub: "sa1", Role: "super_admin"}))

	req := httptest.NewRequest(http.MethodPost, "/admin/exam-grants/bulk/presign?exam_id="+testExamID, nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("want 400, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestAdminPresignExamGrantBulkUpload_InvalidExamID_400(t *testing.T) {
	e := newExamGrantBulkRouter(t, newExamGrantBulkPresignHandler(t))
	e.Use(claimsInjector(&infra.Claims{Sub: "sa1", Role: "super_admin"}))

	req := httptest.NewRequest(http.MethodPost, "/admin/exam-grants/bulk/presign?exam_id=not-a-uuid&filename=grants.csv", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("want 400 for malformed exam_id, got %d: %s", rec.Code, rec.Body.String())
	}
}

// TestAdminExamGrantBulkRoutes_NonSuperAdmin_403 is the super-admin-only
// guard: admin_school and admin_exam manage rosters/content, not cross-school
// bulk grants, and must not reach either bulk route.
func TestAdminExamGrantBulkRoutes_NonSuperAdmin_403(t *testing.T) {
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
		{"presign", "/admin/exam-grants/bulk/presign?exam_id=" + testExamID + "&filename=grants.csv", ""},
		{"enqueue", "/admin/exam-grants/bulk", `{"exam_id":"` + testExamID + `","file_key":"exam-grant-bulk/` + testExamID + `/x.csv"}`},
	}

	for role, claims := range roles {
		for _, target := range targets {
			t.Run(role+"/"+target.name, func(t *testing.T) {
				e := newExamGrantBulkRouter(t, newExamGrantBulkPresignHandler(t))
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

func TestAdminEnqueueExamGrantBulk_MissingExamID_400(t *testing.T) {
	e := newExamGrantBulkRouter(t, newExamGrantBulkPresignHandler(t))
	e.Use(claimsInjector(&infra.Claims{Sub: "sa1", Role: "super_admin"}))

	req := httptest.NewRequest(http.MethodPost, "/admin/exam-grants/bulk",
		strings.NewReader(`{"file_key":"exam-grant-bulk/`+testExamID+`/x.csv"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("want 400, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestAdminEnqueueExamGrantBulk_MissingFileKey_400(t *testing.T) {
	e := newExamGrantBulkRouter(t, newExamGrantBulkPresignHandler(t))
	e.Use(claimsInjector(&infra.Claims{Sub: "sa1", Role: "super_admin"}))

	req := httptest.NewRequest(http.MethodPost, "/admin/exam-grants/bulk",
		strings.NewReader(`{"exam_id":"`+testExamID+`"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("want 400, got %d: %s", rec.Code, rec.Body.String())
	}
}

// TestAdminEnqueueExamGrantBulk_PrefixMismatch_404 pins the "validate key
// prefix matches exam_id" requirement: a file_key under a different exam's
// folder (or a foreign job type's prefix entirely) must not be enqueued
// against this exam_id.
func TestAdminEnqueueExamGrantBulk_PrefixMismatch_404(t *testing.T) {
	otherExamID := "33333333-3333-3333-3333-333333333333"
	cases := []string{
		"exam-grant-bulk/" + otherExamID + "/x.csv",
		"student-bulk/11111111-1111-1111-1111-111111111111/x.csv",
	}

	for _, fileKey := range cases {
		t.Run(fileKey, func(t *testing.T) {
			e := newExamGrantBulkRouter(t, newExamGrantBulkPresignHandler(t))
			e.Use(claimsInjector(&infra.Claims{Sub: "sa1", Role: "super_admin"}))

			body, _ := json.Marshal(map[string]string{"exam_id": testExamID, "file_key": fileKey})
			req := httptest.NewRequest(http.MethodPost, "/admin/exam-grants/bulk", bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			rec := httptest.NewRecorder()
			e.ServeHTTP(rec, req)

			if rec.Code != http.StatusNotFound {
				t.Fatalf("want 404 upload_not_found, got %d: %s", rec.Code, rec.Body.String())
			}
		})
	}
}

func TestAdminEnqueueExamGrantBulk_InvalidExamID_400(t *testing.T) {
	e := newExamGrantBulkRouter(t, newExamGrantBulkPresignHandler(t))
	e.Use(claimsInjector(&infra.Claims{Sub: "sa1", Role: "super_admin"}))

	body, _ := json.Marshal(map[string]string{"exam_id": "not-a-uuid", "file_key": "exam-grant-bulk/not-a-uuid/x.csv"})
	req := httptest.NewRequest(http.MethodPost, "/admin/exam-grants/bulk", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("want 400 for malformed exam_id, got %d: %s", rec.Code, rec.Body.String())
	}
}

// TestAdminEnqueueExamGrantBulk_Valid_202 drives the real handler -> real
// service -> real Postgres and asserts the wire shape of the 202 body, plus
// the persisted job's type and exam-scoped file key.
func TestAdminEnqueueExamGrantBulk_Valid_202(t *testing.T) {
	jobs := newAdminJobsEnv(t)
	owner := seedJobOwner(t, jobs, "examgrantbulk1")
	exam, err := jobs.svc.CreateExam(context.Background(), model.Exam{Title: "Exam Grant Bulk Test", Mode: ""})
	if err != nil {
		t.Fatalf("CreateExam: %v", err)
	}

	storage := stubObjectStorage(t, "username\nandi123\n")
	svc := service.NewWithStore(jobs.repo, jobs.repo, nil, nil, &service.NoopOTPProvider{}, &service.NoopEmailProvider{},
		nil, nil, storage, &config.Config{ObjectStoragePrivateBucketName: "private", ObjectStorageRegion: "us-east-1"}, nil)

	e := newExamGrantBulkRouter(t, handler.New(svc))
	e.Use(claimsInjector(&infra.Claims{Sub: owner, Role: "super_admin"}))

	examID := exam.ID.String()
	fileKey := "exam-grant-bulk/" + examID + "/abc-grants.csv"
	body, _ := json.Marshal(map[string]string{"exam_id": examID, "file_key": fileKey})
	req := httptest.NewRequest(http.MethodPost, "/admin/exam-grants/bulk", bytes.NewReader(body))
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
	if job == nil || job.Type != "exam_grant_bulk" {
		t.Fatalf("want a persisted exam_grant_bulk job, got %+v", job)
	}
	if job.InputURL == nil || *job.InputURL != fileKey {
		t.Fatalf("want input_url %q, got %+v", fileKey, job.InputURL)
	}
}
