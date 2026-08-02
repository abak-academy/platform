package handler_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"

	"akademi-bimbel/config"
	"akademi-bimbel/internal/handler"
	"akademi-bimbel/internal/infra"
	"akademi-bimbel/internal/service"

	"github.com/labstack/echo/v4"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

// schoolBulkKeyRe pins spec §D-5: no school segment in the key.
var schoolBulkKeyRe = regexp.MustCompile(`^school-bulk/[0-9a-f-]{36}-`)

// newSchoolBulkRouter mirrors routes.go's adminSchools group — the
// RBACMiddleware("schools:write") guard is the whole point of NFR-4, so these
// tests drive the real router rather than calling the handler directly.
func newSchoolBulkRouter(t *testing.T, h *handler.Handler) *echo.Echo {
	t.Helper()
	e := echo.New()
	e.HideBanner = true
	g := e.Group("/admin/schools")
	g.Use(handler.RBACMiddleware("schools:write"))
	g.POST("/bulk/presign", h.AdminPresignSchoolBulkUpload)
	g.POST("/bulk", h.AdminBulkImportSchools)
	return e
}

// claimsInjector puts claims on the context before the RBAC middleware reads
// them, standing in for the JWT middleware the real router runs first.
func claimsInjector(claims *infra.Claims) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			c.Set("claims", claims)
			return next(c)
		}
	}
}

func newSchoolBulkPresignHandler(t *testing.T) *handler.Handler {
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

func TestAdminPresignSchoolBulkUpload_SuperAdmin_200_KeyHasNoSchoolSegment(t *testing.T) {
	e := newSchoolBulkRouter(t, newSchoolBulkPresignHandler(t))
	e.Use(claimsInjector(&infra.Claims{Sub: "sa1", Role: "super_admin"}))

	req := httptest.NewRequest(http.MethodPost, "/admin/schools/bulk/presign?filename=schools.csv&content_type=text/csv", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("want 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp service.PrivateUploadURL
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if !schoolBulkKeyRe.MatchString(resp.Key) {
		t.Errorf("key %q does not match %s", resp.Key, schoolBulkKeyRe)
	}
	if resp.Method != "PUT" {
		t.Errorf("Method: want PUT, got %s", resp.Method)
	}
}

// TestAdminSchoolBulkRoutes_NonSuperAdmin_403 is the NFR-4 guard: an
// admin_school administers students within a school and must not be able to
// create schools in bulk.
func TestAdminSchoolBulkRoutes_NonSuperAdmin_403(t *testing.T) {
	schoolID := "11111111-1111-1111-1111-111111111111"
	roles := map[string]*infra.Claims{
		"admin_school": {Sub: "as1", Role: "admin_school", SchoolID: &schoolID},
		"admin_exam":   {Sub: "ae1", Role: "admin_exam"},
		"admin_store":  {Sub: "at1", Role: "admin_store"},
		"student":      {Sub: "st1", Role: "student"},
	}
	targets := []struct {
		name, path, body string
	}{
		{"presign", "/admin/schools/bulk/presign?filename=schools.csv", ""},
		{"enqueue", "/admin/schools/bulk", `{"file_key":"school-bulk/x.csv"}`},
	}

	for role, claims := range roles {
		for _, target := range targets {
			t.Run(role+"/"+target.name, func(t *testing.T) {
				e := newSchoolBulkRouter(t, newSchoolBulkPresignHandler(t))
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

func TestAdminBulkImportSchools_MissingFileKey_400(t *testing.T) {
	e := newSchoolBulkRouter(t, newSchoolBulkPresignHandler(t))
	e.Use(claimsInjector(&infra.Claims{Sub: "sa1", Role: "super_admin"}))

	req := httptest.NewRequest(http.MethodPost, "/admin/schools/bulk", strings.NewReader(`{}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("want 400, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestAdminBulkImportSchools_ForeignPrefix_404(t *testing.T) {
	e := newSchoolBulkRouter(t, newSchoolBulkPresignHandler(t))
	e.Use(claimsInjector(&infra.Claims{Sub: "sa1", Role: "super_admin"}))

	req := httptest.NewRequest(http.MethodPost, "/admin/schools/bulk",
		strings.NewReader(`{"file_key":"student-bulk/11111111-1111-1111-1111-111111111111/x.csv"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("want 404 upload_not_found, got %d: %s", rec.Code, rec.Body.String())
	}
}

// stubObjectStorage serves the S3 verbs EnqueueSchoolBulkJob issues (HEAD for
// StatObject, GET for the download) so the enqueue path can be driven
// end-to-end without a MinIO container.
func stubObjectStorage(t *testing.T, csv string) *minio.Client {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/csv")
		w.Header().Set("ETag", `"d41d8cd98f00b204e9800998ecf8427e"`)
		w.Header().Set("Last-Modified", time.Now().UTC().Format(http.TimeFormat))
		w.Header().Set("Content-Length", strconv.Itoa(len(csv)))
		if r.Method == http.MethodHead {
			w.WriteHeader(http.StatusOK)
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(csv))
	}))
	t.Cleanup(srv.Close)

	client, err := minio.New(strings.TrimPrefix(srv.URL, "http://"), &minio.Options{
		Creds:  credentials.NewStaticV4("ak", "sk", ""),
		Secure: false,
		Region: "us-east-1",
	})
	if err != nil {
		t.Fatalf("minio.New: %v", err)
	}
	return client
}

// TestAdminBulkImportSchools_Valid_202 drives the real handler -> real service
// -> real Postgres and asserts the wire shape of the 202 body (NFR-3).
func TestAdminBulkImportSchools_Valid_202(t *testing.T) {
	jobs := newAdminJobsEnv(t)
	owner := seedJobOwner(t, jobs, "schoolbulk1")

	storage := stubObjectStorage(t, "name,code\nSMA Satu,sma_satu\n")
	svc := service.NewWithStore(jobs.repo, jobs.repo, nil, nil, &service.NoopOTPProvider{}, &service.NoopEmailProvider{},
		nil, nil, storage, &config.Config{ObjectStoragePrivateBucketName: "private", ObjectStorageRegion: "us-east-1"}, nil)

	e := newSchoolBulkRouter(t, handler.New(svc))
	e.Use(claimsInjector(&infra.Claims{Sub: owner, Role: "super_admin"}))

	body, _ := json.Marshal(map[string]string{"file_key": "school-bulk/abc-schools.csv"})
	req := httptest.NewRequest(http.MethodPost, "/admin/schools/bulk", bytes.NewReader(body))
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
	if job == nil || job.Type != "school_bulk" {
		t.Fatalf("want a persisted school_bulk job, got %+v", job)
	}
}
