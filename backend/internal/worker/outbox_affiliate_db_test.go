package worker

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"akademi-bimbel/internal/model"
	"akademi-bimbel/internal/repository"
)

// The affiliate commission step rides the shared lifecycle DB fixture
// (lifecycleDB/newLifecycleEnv from order_lifecycle_db_test.go): real orders
// via MintCart/AddItem/Checkout/HandlePaymentWebhook, then a Worker wired with
// the real service so handleOrderPaid runs the commission step for real.
func setAffiliateWorkerFlag(t *testing.T, repo *repository.Repository, value string) {
	t.Helper()
	if _, err := repo.Pool().Exec(context.Background(),
		`INSERT INTO system_config (key, value, is_secret) VALUES ('affiliate_enabled', $1, false)
		 ON CONFLICT (key) DO UPDATE SET value = $1`, value); err != nil {
		t.Fatalf("set affiliate flag %q: %v", value, err)
	}
	t.Cleanup(func() {
		_, _ = repo.Pool().Exec(context.Background(), `DELETE FROM system_config WHERE key = 'affiliate_enabled'`)
	})
}

func seedWorkerAffiliator(t *testing.T, repo *repository.Repository, status string) uuid.UUID {
	t.Helper()
	var userID uuid.UUID
	if err := repo.Pool().QueryRow(context.Background(),
		`INSERT INTO users (email, role, name) VALUES ($1, 'affiliator', 'Worker Affiliator') RETURNING id`,
		"worker-aff-"+uniqueWorkerSuffix()+"@test.local",
	).Scan(&userID); err != nil {
		t.Fatalf("seed affiliator user: %v", err)
	}
	var id uuid.UUID
	if err := repo.Pool().QueryRow(context.Background(),
		`INSERT INTO affiliators (user_id, code, status, terms_version) VALUES ($1, $2, $3, 'v1') RETURNING id`,
		userID, "WK"+strings.ToUpper(uniqueWorkerSuffix()), status,
	).Scan(&id); err != nil {
		t.Fatalf("seed affiliator: %v", err)
	}
	return id
}

func uniqueWorkerSuffix() string {
	return strings.ReplaceAll(uuid.NewString(), "-", "")[:10]
}

func bindWorkerStudent(t *testing.T, repo *repository.Repository, student, affiliatorID uuid.UUID) {
	t.Helper()
	if _, err := repo.Pool().Exec(context.Background(),
		`INSERT INTO affiliate_bindings (affiliator_id, student_id, status, source, started_at)
		 VALUES ($1, $2, 'active', 'checkout_code', now())`, affiliatorID, student,
	); err != nil {
		t.Fatalf("seed binding: %v", err)
	}
}

// pollAllWithWorker drains the outbox through w, so affiliate tests can poll
// with a worker that has the commission step wired.
func pollAllWithWorker(t *testing.T, env *lifecycleEnv, w *Worker) {
	t.Helper()
	ctx := context.Background()
	for i := 0; i < 50; i++ {
		var n int
		if err := env.repo.Pool().QueryRow(ctx, `SELECT count(*) FROM outbox WHERE processed_at IS NULL`).Scan(&n); err != nil {
			t.Fatalf("count pending outbox: %v", err)
		}
		if n == 0 {
			return
		}
		w.pollOutbox(ctx)
	}
	t.Fatal("outbox did not drain after 50 polls")
}

func workerCommissionRows(t *testing.T, repo *repository.Repository, orderID uuid.UUID) []model.AffiliateCommission {
	t.Helper()
	rows, err := repo.Pool().Query(context.Background(),
		`SELECT id, affiliator_id, student_id, order_id, order_item_id, product_type, base_amount, commission_type, commission_value,
		        level_snapshot, amount, status, wait_until, paid_at
		 FROM affiliate_commissions WHERE order_id = $1 ORDER BY order_item_id`, orderID)
	if err != nil {
		t.Fatalf("query commissions: %v", err)
	}
	defer rows.Close()

	var out []model.AffiliateCommission
	for rows.Next() {
		var c model.AffiliateCommission
		if err := rows.Scan(&c.ID, &c.AffiliatorID, &c.StudentID, &c.OrderID, &c.OrderItemID,
			&c.ProductType, &c.BaseAmount, &c.CommissionType, &c.CommissionValue, &c.LevelSnapshot, &c.Amount, &c.Status, &c.WaitUntil, &c.PaidAt); err != nil {
			t.Fatalf("scan commission: %v", err)
		}
		out = append(out, c)
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("rows: %v", err)
	}
	return out
}

func TestOrderPaidAffiliate_BoundStudentCreatesPendingCommissions(t *testing.T) {
	env := newLifecycleEnv(t)
	ctx := context.Background()
	setAffiliateWorkerFlag(t, env.repo, "true")

	affiliatorID := seedWorkerAffiliator(t, env.repo, "active")
	student := env.student(t)
	bindWorkerStudent(t, env.repo, student, affiliatorID)

	course := env.course(t)
	product := env.courseProduct(t, 50000, course)
	book := env.book(t, 5)
	orderID := env.paidOrder(t, student, product, book)

	pollAllWithWorker(t, env, &Worker{repo: env.repo, commissionSvc: env.svc})

	if got := env.count(t, `SELECT count(*) FROM affiliate_commissions WHERE order_id = $1`, orderID); got != 2 {
		t.Fatalf("affiliate commissions = %d, want 2 (course + book)", got)
	}

	var paidAt time.Time
	if err := env.repo.Pool().QueryRow(ctx, `SELECT paid_at FROM orders WHERE id = $1`, orderID).Scan(&paidAt); err != nil {
		t.Fatalf("read paid_at: %v", err)
	}

	type want struct {
		productType string
		base        float64
		pct         float64
		amount      float64
		waitUntil   *time.Time
	}
	wants := []want{
		{productType: "course", base: 50000, pct: 10, amount: 5000, waitUntil: ptrTime(paidAt.AddDate(0, 0, 7))},
		{productType: "book", base: 60000, pct: 3, amount: 1800, waitUntil: nil},
	}
	for _, w := range wants {
		var base, pct, amount float64
		var level, status string
		var waitUntil *time.Time
		var affiliator, orderStudent uuid.UUID
		if err := env.repo.Pool().QueryRow(ctx,
			`SELECT base_amount, commission_value, amount, level_snapshot, status, wait_until, affiliator_id, student_id
			 FROM affiliate_commissions WHERE order_id = $1 AND product_type = $2`,
			orderID, w.productType,
		).Scan(&base, &pct, &amount, &level, &status, &waitUntil, &affiliator, &orderStudent); err != nil {
			t.Fatalf("commission for %s: %v", w.productType, err)
		}
		if base != w.base {
			t.Errorf("%s base_amount = %v, want %v", w.productType, base, w.base)
		}
		if pct != w.pct {
			t.Errorf("%s percentage = %v, want %v", w.productType, pct, w.pct)
		}
		if amount != w.amount {
			t.Errorf("%s amount = %v, want %v", w.productType, amount, w.amount)
		}
		if level != "bronze" {
			t.Errorf("%s level_snapshot = %q, want bronze", w.productType, level)
		}
		if status != "pending" {
			t.Errorf("%s status = %q, want pending", w.productType, status)
		}
		if w.waitUntil == nil && waitUntil != nil {
			t.Errorf("%s wait_until = %v, want NULL", w.productType, waitUntil)
		}
		if w.waitUntil != nil {
			if waitUntil == nil {
				t.Errorf("%s wait_until = NULL, want %v", w.productType, *w.waitUntil)
			} else if waitUntil.Sub(*w.waitUntil) > time.Second || (*w.waitUntil).Sub(*waitUntil) > time.Second {
				t.Errorf("%s wait_until = %v, want %v", w.productType, waitUntil, w.waitUntil)
			}
		}
		if affiliator != affiliatorID {
			t.Errorf("%s affiliator_id = %v, want %v", w.productType, affiliator, affiliatorID)
		}
		if orderStudent != student {
			t.Errorf("%s student_id = %v, want %v", w.productType, orderStudent, student)
		}
	}

	// Fulfilment must be untouched: the course access is granted and the mixed
	// order ends processing.
	if got := env.activeEnrollments(t, student, course); got != 1 {
		t.Errorf("active enrollments = %d, want 1", got)
	}
	if got := env.status(t, orderID); got != "processing" {
		t.Errorf("order status = %q, want processing", got)
	}
	if got := env.count(t, `SELECT count(*) FROM outbox WHERE aggregate_id = $1 AND processed_at IS NULL`, orderID); got != 0 {
		t.Errorf("unprocessed outbox events = %d, want 0", got)
	}
}

func ptrTime(t time.Time) *time.Time { return &t }

func TestOrderPaidAffiliate_ReplayedEventDoesNotDuplicateCommissions(t *testing.T) {
	env := newLifecycleEnv(t)
	setAffiliateWorkerFlag(t, env.repo, "true")

	affiliatorID := seedWorkerAffiliator(t, env.repo, "active")
	student := env.student(t)
	bindWorkerStudent(t, env.repo, student, affiliatorID)

	product := env.courseProduct(t, 50000, env.course(t))
	orderID := env.paidOrder(t, student, product)

	w := &Worker{repo: env.repo, commissionSvc: env.svc}
	pollAllWithWorker(t, env, w)

	env.exec(t,
		`INSERT INTO outbox (aggregate_type, aggregate_id, event_type, payload) VALUES ('order', $1, 'OrderPaid', $2::jsonb)`,
		orderID, `{"order_id":"`+orderID.String()+`","items":[{"product_id":"`+product.String()+`","product_type":"course","qty":1}]}`,
	)
	pollAllWithWorker(t, env, w)

	if got := env.count(t, `SELECT count(*) FROM affiliate_commissions WHERE order_id = $1`, orderID); got != 1 {
		t.Errorf("affiliate commissions after replay = %d, want 1 (ON CONFLICT DO NOTHING)", got)
	}
}

// failingCommissionProcessor proves the guard around the commission step: an
// error there must not roll back access provisioning or the outbox commit.
type failingCommissionProcessor struct{}

func (failingCommissionProcessor) CreateCommissionsForOrder(context.Context, pgx.Tx, model.Order, []model.OrderItem) error {
	return errors.New("commission boom")
}

func TestOrderPaidAffiliate_CommissionFailureDoesNotFailFulfilment(t *testing.T) {
	env := newLifecycleEnv(t)

	student := env.student(t)
	course := env.course(t)
	product := env.courseProduct(t, 50000, course)
	orderID := env.paidOrder(t, student, product)

	pollAllWithWorker(t, env, &Worker{repo: env.repo, commissionSvc: failingCommissionProcessor{}})

	if got := env.activeEnrollments(t, student, course); got != 1 {
		t.Errorf("active enrollments = %d, want 1 (access granted despite commission failure)", got)
	}
	if got := env.count(t, `SELECT count(*) FROM outbox WHERE aggregate_id = $1 AND processed_at IS NULL`, orderID); got != 0 {
		t.Errorf("unprocessed outbox events = %d, want 0 (event processed anyway)", got)
	}
	if got := env.count(t, `SELECT count(*) FROM affiliate_commissions WHERE order_id = $1`, orderID); got != 0 {
		t.Errorf("affiliate commissions = %d, want 0 (failed step left nothing)", got)
	}
}
