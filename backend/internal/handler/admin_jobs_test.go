package handler_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"akademi-bimbel/internal/handler"
	"akademi-bimbel/internal/infra"
	"akademi-bimbel/internal/model"
	"akademi-bimbel/internal/repository"
	"akademi-bimbel/internal/service"

	"github.com/labstack/echo/v4"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"
)

// AdminGetJob's only real logic is threading claims.Sub/c.Param("id") into
// svc.GetJobStatus, which touches storeRepo — a concrete *repository.Repository,
// not an interface. This package's usual fake-repo/service.New convention
// (see newAdminSystemEnv) leaves storeRepo nil, so it can't reach this handler
// at all. A real Postgres instance (mirroring internal/service/realdb_test.go's
// newRealDBService) is the only way to exercise it end-to-end; the container is
// shared across this file's tests via sync.Once.
var (
	jobEnvOnce sync.Once
	jobEnv     *adminJobsTestEnv
)

type adminJobsTestEnv struct {
	e    *echo.Echo
	h    *handler.Handler
	svc  *service.Service
	repo *repository.Repository
}

func newAdminJobsEnv(t *testing.T) *adminJobsTestEnv {
	t.Helper()
	jobEnvOnce.Do(func() {
		ctx := context.Background()
		pgContainer, err := tcpostgres.Run(ctx,
			"postgres:17-alpine",
			tcpostgres.WithDatabase("akademi_test"),
			tcpostgres.WithUsername("test"),
			tcpostgres.WithPassword("test"),
			tcpostgres.BasicWaitStrategies(),
		)
		if err != nil {
			t.Fatalf("start postgres container: %v", err)
		}
		dsn, err := pgContainer.ConnectionString(ctx, "sslmode=disable")
		if err != nil {
			t.Fatalf("connection string: %v", err)
		}
		if err := infra.RunMigrations(ctx, dsn); err != nil {
			t.Fatalf("run migrations: %v", err)
		}
		pool, err := infra.NewPool(ctx, dsn)
		if err != nil {
			t.Fatalf("new pool: %v", err)
		}
		repo := repository.New(pool)
		svc := service.NewWithStore(repo, repo, nil, nil, &service.NoopOTPProvider{}, &service.NoopEmailProvider{}, nil, nil, nil, nil, nil)
		e := echo.New()
		e.HideBanner = true
		jobEnv = &adminJobsTestEnv{e: e, h: handler.New(svc), svc: svc, repo: repo}
	})
	if jobEnv == nil {
		t.Fatal("admin jobs test env failed to initialize")
	}
	return jobEnv
}

// seedJobOwner creates a school and a student user via the real service/repo
// to satisfy job.created_by's FK to users(id).
func seedJobOwner(t *testing.T, env *adminJobsTestEnv, nis string) string {
	t.Helper()
	ctx := context.Background()
	school, err := env.svc.CreateSchool(ctx, "Job Test School "+nis, "job_"+nis, nil, nil, nil)
	if err != nil {
		t.Fatalf("CreateSchool: %v", err)
	}
	reg, err := env.svc.RegisterStudent(ctx, school.ID, "Job Owner "+nis, "sma", nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)
	if err != nil {
		t.Fatalf("RegisterStudent: %v", err)
	}
	return reg.ID
}

func setClaims(c echo.Context, sub string) {
	c.Set("claims", &infra.Claims{Sub: sub, Role: "student"})
}

func TestAdminGetJob_Owner_200(t *testing.T) {
	env := newAdminJobsEnv(t)
	owner := seedJobOwner(t, env, "gjo1")

	fileKey := "student-bulk/x/y.csv"
	job := &model.Job{Type: "student_bulk", InputURL: &fileKey, CreatedBy: owner}
	if err := env.repo.CreateJob(context.Background(), job); err != nil {
		t.Fatalf("CreateJob: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/admin/jobs/"+job.ID, nil)
	rec := httptest.NewRecorder()
	c := env.e.NewContext(req, rec)
	c.SetParamNames("id")
	c.SetParamValues(job.ID)
	setClaims(c, owner)

	if err := env.h.AdminGetJob(c); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("want 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp service.JobResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.ID != job.ID {
		t.Errorf("ID: want %s, got %s", job.ID, resp.ID)
	}
	if resp.Type != "student_bulk" {
		t.Errorf("Type: want student_bulk, got %s", resp.Type)
	}
	if resp.Status != "queued" {
		t.Errorf("Status: want queued, got %s", resp.Status)
	}
}

func TestAdminGetJob_DifferentRequester_404(t *testing.T) {
	env := newAdminJobsEnv(t)
	owner := seedJobOwner(t, env, "gjo2")
	other := seedJobOwner(t, env, "gjo3")

	fileKey := "student-bulk/x/y.csv"
	job := &model.Job{Type: "student_bulk", InputURL: &fileKey, CreatedBy: owner}
	if err := env.repo.CreateJob(context.Background(), job); err != nil {
		t.Fatalf("CreateJob: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/admin/jobs/"+job.ID, nil)
	rec := httptest.NewRecorder()
	c := env.e.NewContext(req, rec)
	c.SetParamNames("id")
	c.SetParamValues(job.ID)
	setClaims(c, other)

	if err := env.h.AdminGetJob(c); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rec.Code != http.StatusNotFound {
		t.Fatalf("want 404 for non-owner requester, got %d: %s", rec.Code, rec.Body.String())
	}

	var apiErr handler.APIError
	if err := json.Unmarshal(rec.Body.Bytes(), &apiErr); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if apiErr.Code != "job_not_found" {
		t.Errorf("code: want job_not_found, got %q", apiErr.Code)
	}
}

func TestAdminGetJob_NonexistentID_404(t *testing.T) {
	env := newAdminJobsEnv(t)
	owner := seedJobOwner(t, env, "gjo4")

	req := httptest.NewRequest(http.MethodGet, "/admin/jobs/00000000-0000-0000-0000-000000000000", nil)
	rec := httptest.NewRecorder()
	c := env.e.NewContext(req, rec)
	c.SetParamNames("id")
	c.SetParamValues("00000000-0000-0000-0000-000000000000")
	setClaims(c, owner)

	if err := env.h.AdminGetJob(c); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rec.Code != http.StatusNotFound {
		t.Fatalf("want 404 for nonexistent job id, got %d: %s", rec.Code, rec.Body.String())
	}
}
