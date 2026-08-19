package handler_test

import (
	"bytes"
	"context"
	"encoding/csv"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"

	"akademi-bimbel/config"
	"akademi-bimbel/internal/handler"
	"akademi-bimbel/internal/service"

	"github.com/labstack/echo/v4"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

// studentBulkFolderKeyRe is option A: student-bulk/{folder-uuid}/…
var studentBulkFolderKeyRe = regexp.MustCompile(`^student-bulk/[0-9a-f-]{36}/`)

func newStudentBulkPresignHandler(t *testing.T) *handler.Handler {
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

// Minimal guard-clause tests driving Task 7's implementation. The definitive
// handler test files (admin_students_bulk_test.go / admin_jobs_test.go) are
// written in a later task with real storeRepo fixtures.

func TestAdminPresignStudentBulkUpload_NilSchoolID_403(t *testing.T) {
	env := newAdminSystemEnv(t)

	req := httptest.NewRequest(http.MethodPost, "/admin/students/bulk/presign?filename=x.csv", nil)
	rec := httptest.NewRecorder()
	c := env.e.NewContext(req, rec)
	setAdminSchoolClaimsNil(c, "u1")

	if err := env.h.AdminPresignStudentBulkUpload(c); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rec.Code != http.StatusForbidden {
		t.Errorf("want 403 for nil schoolID, got %d", rec.Code)
	}
}

func TestAdminPresignStudentBulkUpload_MissingFilename_400(t *testing.T) {
	env := newAdminSystemEnv(t)

	req := httptest.NewRequest(http.MethodPost, "/admin/students/bulk/presign", nil)
	rec := httptest.NewRecorder()
	c := env.e.NewContext(req, rec)
	setAdminSchoolClaims(c, "u1", "s1")

	if err := env.h.AdminPresignStudentBulkUpload(c); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rec.Code != http.StatusBadRequest {
		t.Errorf("want 400 for missing filename, got %d", rec.Code)
	}
}

func TestAdminPresignStudentBulkUpload_SuperAdmin_NoSchoolID_200(t *testing.T) {
	h := newStudentBulkPresignHandler(t)
	e := echo.New()
	e.HideBanner = true

	req := httptest.NewRequest(http.MethodPost, "/admin/students/bulk/presign?filename=students.csv&content_type=text/csv", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	setAdminClaims(c, "sa1")

	if err := h.AdminPresignStudentBulkUpload(c); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("want 200 without school_id, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp service.PrivateUploadURL
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if !studentBulkFolderKeyRe.MatchString(resp.Key) {
		t.Errorf("key %q does not match %s", resp.Key, studentBulkFolderKeyRe)
	}
	if strings.Contains(rec.Body.String(), "school_id is required") {
		t.Errorf("super_admin presign must not require school_id: %s", rec.Body.String())
	}
}

func TestAdminBulkImportStudents_NilSchoolID_403(t *testing.T) {
	env := newAdminSystemEnv(t)

	body := map[string]string{"file_key": "student-bulk/s1/x.csv"}
	b, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/admin/students/bulk", bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	c := env.e.NewContext(req, rec)
	setAdminSchoolClaimsNil(c, "u1")

	if err := env.h.AdminBulkImportStudents(c); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rec.Code != http.StatusForbidden {
		t.Errorf("want 403 for nil schoolID, got %d", rec.Code)
	}
}

func TestAdminBulkImportStudents_MissingFileKey_400(t *testing.T) {
	env := newAdminSystemEnv(t)

	body := map[string]string{}
	b, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/admin/students/bulk", bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	c := env.e.NewContext(req, rec)
	setAdminSchoolClaims(c, "u1", "s1")

	if err := env.h.AdminBulkImportStudents(c); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rec.Code != http.StatusBadRequest {
		t.Errorf("want 400 for missing file_key, got %d", rec.Code)
	}
}

func TestAdminBulkReissueCredentials_NilSchoolID_403(t *testing.T) {
	env := newAdminSystemEnv(t)

	body := map[string]any{"all": true}
	b, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/admin/students/bulk/credentials", bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	c := env.e.NewContext(req, rec)
	setAdminSchoolClaimsNil(c, "u1")

	if err := env.h.AdminBulkReissueCredentials(c); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rec.Code != http.StatusForbidden {
		t.Errorf("want 403 for nil schoolID, got %d", rec.Code)
	}
}

func TestAdminBulkReissueCredentials_SuperAdmin_NoSchoolID_400(t *testing.T) {
	env := newAdminSystemEnv(t)

	body := map[string]any{"all": true}
	b, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/admin/students/bulk/credentials", bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	c := env.e.NewContext(req, rec)
	setAdminClaims(c, "sa1")

	if err := env.h.AdminBulkReissueCredentials(c); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("want 400 when super_admin omits school_id on bulk credentials, got %d: %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "school_id is required") {
		t.Errorf("want school_id is required from handler, got %s", rec.Body.String())
	}
}

func TestAdminBulkReissueCredentials_BothSupplied_400(t *testing.T) {
	env := newAdminSystemEnv(t)

	body := map[string]any{"all": true, "student_ids": []string{"stu1"}}
	b, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/admin/students/bulk/credentials", bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	c := env.e.NewContext(req, rec)
	setAdminSchoolClaims(c, "u1", "s1")

	if err := env.h.AdminBulkReissueCredentials(c); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rec.Code != http.StatusBadRequest {
		t.Errorf("want 400 when both student_ids and all supplied, got %d", rec.Code)
	}
}

func TestAdminBulkReissueCredentials_NeitherSupplied_400(t *testing.T) {
	env := newAdminSystemEnv(t)

	body := map[string]any{}
	b, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/admin/students/bulk/credentials", bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	c := env.e.NewContext(req, rec)
	setAdminSchoolClaims(c, "u1", "s1")

	if err := env.h.AdminBulkReissueCredentials(c); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rec.Code != http.StatusBadRequest {
		t.Errorf("want 400 when neither student_ids nor all supplied, got %d", rec.Code)
	}
}

// TestAdminBulkReissueCredentials_HappyPath_ReturnsCSV exercises the one path
// none of the guard-clause tests above reach: a valid request that actually
// calls through to ReissueStudentCredentialsBulk and writes the CSV response.
// Uses the real-DB env from admin_jobs_test.go (same package) since this
// handler's storeRepo is a concrete *repository.Repository, not fakeable.
func TestAdminBulkReissueCredentials_HappyPath_ReturnsCSV(t *testing.T) {
	env := newAdminJobsEnv(t)
	ctx := context.Background()

	school, err := env.svc.CreateSchool(ctx, "Bulk Reissue School", "bulkreissue1", nil, nil, nil)
	if err != nil {
		t.Fatalf("CreateSchool: %v", err)
	}
	reg, err := env.svc.RegisterStudent(ctx, school.ID, "Reissue Target", "sma", nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)
	if err != nil {
		t.Fatalf("RegisterStudent: %v", err)
	}

	body := map[string]any{"student_ids": []string{reg.ID}}
	b, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/admin/students/bulk/credentials", bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	c := env.e.NewContext(req, rec)
	setAdminSchoolClaims(c, "admin1", school.ID)

	if err := env.h.AdminBulkReissueCredentials(c); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("want 200, got %d: %s", rec.Code, rec.Body.String())
	}
	if ct := rec.Header().Get(echo.HeaderContentType); ct != "text/csv" {
		t.Errorf("Content-Type: want text/csv, got %q", ct)
	}
	if cd := rec.Header().Get(echo.HeaderContentDisposition); cd != `attachment; filename="credentials.csv"` {
		t.Errorf("Content-Disposition: want attachment with filename, got %q", cd)
	}

	records, err := csv.NewReader(strings.NewReader(rec.Body.String())).ReadAll()
	if err != nil {
		t.Fatalf("read back csv body: %v", err)
	}
	if len(records) != 2 {
		t.Fatalf("want 2 records (header + 1 row), got %d", len(records))
	}
	row := records[1]
	if len(row) != 4 {
		t.Errorf("want 4 columns (name, username, temp_password, error), got %d: %+v", len(row), row)
	}
	if row[0] != "Reissue Target" || row[1] == "" || row[2] == "" || row[3] != "" {
		t.Errorf("unexpected credentials row: %+v", row)
	}
}
