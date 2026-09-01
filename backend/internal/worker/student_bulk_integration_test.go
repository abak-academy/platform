package worker

import (
	"context"
	"testing"

	"akademi-bimbel/internal/infra"
	"akademi-bimbel/internal/model"
	"akademi-bimbel/internal/repository"
	"akademi-bimbel/internal/service"

	"github.com/jackc/pgx/v5/pgxpool"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"
	"golang.org/x/crypto/bcrypt"
)

func TestRunStudentBulkJob_RealSuperAdminCanSetExplicitPassword(t *testing.T) {
	ctx := context.Background()
	postgres, err := tcpostgres.Run(ctx,
		"postgres:17-alpine",
		tcpostgres.WithDatabase("akademi_test"),
		tcpostgres.WithUsername("test"),
		tcpostgres.WithPassword("test"),
		tcpostgres.BasicWaitStrategies(),
	)
	if err != nil {
		t.Fatalf("start postgres container: %v", err)
	}
	t.Cleanup(func() { _ = postgres.Terminate(ctx) })

	dsn, err := postgres.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		t.Fatalf("postgres connection string: %v", err)
	}
	if err := infra.RunMigrations(ctx, dsn); err != nil {
		t.Fatalf("run migrations: %v", err)
	}
	pool, err := infra.NewPool(ctx, dsn)
	if err != nil {
		t.Fatalf("open postgres pool: %v", err)
	}
	t.Cleanup(pool.Close)

	repo := repository.New(pool)
	schoolID := seedWorkerBulkSchool(t, ctx, pool)
	actorID := seedWorkerBulkSuperAdmin(t, ctx, pool)
	inputKey := "student-bulk/namespace/upload.csv"
	job := &model.Job{Type: "student_bulk", InputURL: &inputKey, CreatedBy: actorID}
	if err := repo.CreateJob(ctx, job); err != nil {
		t.Fatalf("create job: %v", err)
	}
	claimed, err := repo.ClaimNextJob(ctx)
	if err != nil {
		t.Fatalf("claim job: %v", err)
	}
	if claimed == nil {
		t.Fatal("expected queued job to be claimed")
	}

	password := "chosenPass123"
	store := &fakeObjectStore{
		getObjectBytesFn: func(context.Context, string, string) ([]byte, error) {
			return []byte("name,school,jenjang,password\nExplicit Student,Worker Bulk School,SMA," + password + "\n"), nil
		},
	}
	svc := service.NewWithStore(repo, repo, nil, nil, &service.NoopOTPProvider{}, &service.NoopEmailProvider{}, nil, nil, nil, nil, nil)
	w := &Worker{jobRepo: repo, objectStore: store, svc: svc, privateBucket: "private-bucket"}
	w.runStudentBulkJob(ctx, *claimed)

	finished, err := repo.GetJobByID(ctx, job.ID)
	if err != nil {
		t.Fatalf("get finished job: %v", err)
	}
	if finished == nil || finished.Status != "succeeded" {
		t.Fatalf("job status = %+v, want succeeded", finished)
	}
	var hash string
	if err := pool.QueryRow(ctx, `SELECT password_hash FROM users WHERE name = 'Explicit Student'`).Scan(&hash); err != nil {
		t.Fatalf("read created student: %v", err)
	}
	if err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)); err != nil {
		t.Fatalf("explicit password was not persisted from real super_admin job: %v", err)
	}
	if len(store.deleteCalls) != 1 || store.deleteCalls[0] != "private-bucket/"+inputKey {
		t.Fatalf("input deletion calls = %v, want private-bucket/%s", store.deleteCalls, inputKey)
	}
	if schoolID == "" {
		t.Fatal("seed school id must be populated")
	}
}

func seedWorkerBulkSchool(t *testing.T, ctx context.Context, pool *pgxpool.Pool) string {
	t.Helper()
	var schoolID string
	if err := pool.QueryRow(ctx,
		`INSERT INTO school (name, code, school_types, status) VALUES ('Worker Bulk School', 'worker-bulk', ARRAY['SMA'], 'active') RETURNING id`,
	).Scan(&schoolID); err != nil {
		t.Fatalf("seed school: %v", err)
	}
	return schoolID
}

func seedWorkerBulkSuperAdmin(t *testing.T, ctx context.Context, pool *pgxpool.Pool) string {
	t.Helper()
	var actorID string
	if err := pool.QueryRow(ctx,
		`INSERT INTO users (name, username, password_hash, role, status) VALUES ('Worker Bulk Super', 'worker_bulk_super', 'unused', 'super_admin', 'active') RETURNING id`,
	).Scan(&actorID); err != nil {
		t.Fatalf("seed super admin: %v", err)
	}
	return actorID
}
