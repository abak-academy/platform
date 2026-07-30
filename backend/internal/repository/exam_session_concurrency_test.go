package repository

import (
	"context"
	"errors"
	"runtime"
	"sync"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"

	"akademi-bimbel/internal/infra"
)

// newConcurrencyTestPool is newGradingTestPool with an explicit, generous MaxConns:
// the tests in this file hold N transactions open simultaneously behind a start
// barrier, which would deadlock against the pgxpool default (max(4, NumCPU())) once
// N exceeds that count on a small runner.
func newConcurrencyTestPool(t *testing.T, maxConns int32) *pgxpool.Pool {
	t.Helper()
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
	t.Cleanup(func() { _ = pgContainer.Terminate(ctx) })

	dsn, err := pgContainer.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		t.Fatalf("connection string: %v", err)
	}

	if err := infra.RunMigrations(ctx, dsn); err != nil {
		t.Fatalf("run migrations: %v", err)
	}

	cfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		t.Fatalf("parse pool config: %v", err)
	}
	cfg.MaxConns = maxConns

	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		t.Fatalf("open pool: %v", err)
	}
	t.Cleanup(pool.Close)

	return pool
}

// raceCreateExamSessionTx fires n goroutines at the same registration through a
// shared sync.WaitGroup start barrier: every goroutine opens its own transaction and
// blocks until all n are past BeginTx, then they are released together so the
// UPDATE ... WHERE attempts_used < ceiling statements genuinely overlap inside
// Postgres. This is the only test in the package that observes real transaction
// overlap — TestCreateExamSessionTx_SecondCallRejected and
// TestCreateExamSessionTx_MaxAttemptsTwo are two sequential, fully-committed
// transactions each and cannot distinguish an atomic row guard from a broken
// check-then-act one (see Center's mutation finding in review_result.json).
func raceCreateExamSessionTx(t *testing.T, pool *pgxpool.Pool, n int, maxAttempts *int) (successCount int, attemptNumbers []int, nonAttemptErrs []error) {
	t.Helper()
	ctx := context.Background()
	repo := New(pool)

	reg, _ := seedGuardRegistration(t, pool)
	if n == 0 {
		t.Fatal("n must be > 0")
	}

	var ready sync.WaitGroup
	ready.Add(n)
	start := make(chan struct{})
	var done sync.WaitGroup
	done.Add(n)

	successes := make([]bool, n)
	attempts := make([]int, n)
	errs := make([]error, n)

	for i := 0; i < n; i++ {
		go func(i int) {
			defer done.Done()

			tx, err := repo.BeginTx(ctx)
			if err != nil {
				errs[i] = err
				ready.Done()
				return
			}

			ready.Done()
			<-start

			sess, err := repo.CreateExamSessionTx(ctx, tx, reg, maxAttempts)
			if err != nil {
				errs[i] = err
				_ = tx.Rollback(ctx)
				return
			}
			if commitErr := tx.Commit(ctx); commitErr != nil {
				errs[i] = commitErr
				return
			}
			successes[i] = true
			attempts[i] = sess.AttemptNumber
		}(i)
	}

	ready.Wait()
	close(start)
	done.Wait()

	for i := 0; i < n; i++ {
		if successes[i] {
			successCount++
			attemptNumbers = append(attemptNumbers, attempts[i])
			continue
		}
		if !errors.Is(errs[i], ErrNoAttemptsLeft) {
			nonAttemptErrs = append(nonAttemptErrs, errs[i])
		}
	}
	return successCount, attemptNumbers, nonAttemptErrs
}

// TestCreateExamSessionTx_ConcurrentStarts_SingleAttempt races n goroutines against
// one registration with max_attempts=nil (single-attempt, FR18). Exactly one must
// succeed with attempt_number 1; every other goroutine must get ErrNoAttemptsLeft —
// the atomic `WHERE attempts_used < ceiling` guard has to serialize the winner even
// though every goroutine issues its UPDATE inside the same overlapping window.
func TestCreateExamSessionTx_ConcurrentStarts_SingleAttempt(t *testing.T) {
	n := 10
	if maxProcs := runtime.GOMAXPROCS(0); maxProcs < 2 {
		t.Skip("needs real parallelism to race goroutines")
	}
	pool := newConcurrencyTestPool(t, int32(n+4))

	successCount, attemptNumbers, nonAttemptErrs := raceCreateExamSessionTx(t, pool, n, nil)

	for _, err := range nonAttemptErrs {
		t.Errorf("unexpected error: %v", err)
	}
	if successCount != 1 {
		t.Fatalf("successful starts: want 1, got %d (attempt_numbers=%v)", successCount, attemptNumbers)
	}
	if len(attemptNumbers) != 1 || attemptNumbers[0] != 1 {
		t.Errorf("winner attempt_number: want [1], got %v", attemptNumbers)
	}
}

// TestCreateExamSessionTx_ConcurrentStarts_MaxAttemptsTwo races n goroutines against
// one registration with max_attempts=2. Exactly two must succeed, with attempt_number
// {1,2} and no duplicate — same guard, retested at the wider ceiling.
func TestCreateExamSessionTx_ConcurrentStarts_MaxAttemptsTwo(t *testing.T) {
	n := 10
	if maxProcs := runtime.GOMAXPROCS(0); maxProcs < 2 {
		t.Skip("needs real parallelism to race goroutines")
	}
	pool := newConcurrencyTestPool(t, int32(n+4))

	successCount, attemptNumbers, nonAttemptErrs := raceCreateExamSessionTx(t, pool, n, intptr(2))

	for _, err := range nonAttemptErrs {
		t.Errorf("unexpected error: %v", err)
	}
	if successCount != 2 {
		t.Fatalf("successful starts: want 2, got %d (attempt_numbers=%v)", successCount, attemptNumbers)
	}

	seen := map[int]bool{}
	for _, a := range attemptNumbers {
		if seen[a] {
			t.Errorf("duplicate attempt_number %d among winners: %v", a, attemptNumbers)
		}
		seen[a] = true
	}
	if !seen[1] || !seen[2] {
		t.Errorf("attempt_numbers: want {1,2}, got %v", attemptNumbers)
	}
}
