package worker

import (
	"context"
	"encoding/json"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"

	"akademi-bimbel/internal/infra"
	"akademi-bimbel/internal/repository"
	"akademi-bimbel/internal/service"
)

var (
	lifecycleOnce sync.Once
	lifecycleRepo *repository.Repository
)

func lifecycleDB(t *testing.T) *repository.Repository {
	t.Helper()
	lifecycleOnce.Do(func() {
		ctx := context.Background()
		pg, err := tcpostgres.Run(ctx,
			"postgres:17-alpine",
			tcpostgres.WithDatabase("akademi_test"),
			tcpostgres.WithUsername("test"),
			tcpostgres.WithPassword("test"),
			tcpostgres.BasicWaitStrategies(),
		)
		if err != nil {
			t.Fatalf("start postgres container: %v", err)
		}
		dsn, err := pg.ConnectionString(ctx, "sslmode=disable")
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
		lifecycleRepo = repository.New(pool)
	})
	if lifecycleRepo == nil {
		t.Fatal("lifecycle database unavailable")
	}
	return lifecycleRepo
}

type lifecyclePayments struct {
	creates atomic.Int32
	paid    atomic.Bool
}

func (p *lifecyclePayments) CreatePayment(_ context.Context, req service.PaymentRequest) (service.PaymentResponse, error) {
	p.creates.Add(1)
	return service.PaymentResponse{GatewayRef: "life-" + req.OrderID, ExpiresAt: time.Now().Add(time.Hour)}, nil
}

func (p *lifecyclePayments) QueryStatus(context.Context, string) (service.PaymentStatus, error) {
	return service.PaymentStatus{Paid: p.paid.Load()}, nil
}

func (p *lifecyclePayments) VerifySignature([]byte, string) bool { return true }

type lifecycleEnv struct {
	repo *repository.Repository
	svc  *service.Service
	pay  *lifecyclePayments
	w    *Worker
}

// stock is this branch's own helper: the guard branch removed it as unused, and
// the expiry tests here need it.
func (e *lifecycleEnv) stock(t *testing.T, productID uuid.UUID) int {
	t.Helper()
	var n int
	if err := e.repo.Pool().QueryRow(context.Background(), `SELECT stock FROM product WHERE id = $1`, productID).Scan(&n); err != nil {
		t.Fatalf("stock: %v", err)
	}
	return n
}

func newLifecycleEnv(t *testing.T) *lifecycleEnv {
	t.Helper()
	repo := lifecycleDB(t)
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("miniredis: %v", err)
	}
	t.Cleanup(mr.Close)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	pay := &lifecyclePayments{}
	svc := service.NewWithStore(repo, repo, rdb, nil, &service.NoopOTPProvider{}, &service.NoopEmailProvider{},
		pay, &service.NoopLogisticsClient{}, nil, nil, nil)
	return &lifecycleEnv{repo: repo, svc: svc, pay: pay, w: &Worker{repo: repo}}
}

func (e *lifecycleEnv) exec(t *testing.T, sql string, args ...any) {
	t.Helper()
	if _, err := e.repo.Pool().Exec(context.Background(), sql, args...); err != nil {
		t.Fatalf("exec %q: %v", sql, err)
	}
}

func (e *lifecycleEnv) student(t *testing.T) uuid.UUID {
	t.Helper()
	var id uuid.UUID
	if err := e.repo.Pool().QueryRow(context.Background(),
		`INSERT INTO users (name, role, status, username, password_hash, unlisted_school_name, grade, dob)
		 VALUES ('Lifecycle Student', 'student', 'active', $1, '', 'SMA Test', 12, '2008-01-01') RETURNING id`,
		"life_"+uuid.NewString()[:12],
	).Scan(&id); err != nil {
		t.Fatalf("insert student: %v", err)
	}
	return id
}

func (e *lifecycleEnv) admin(t *testing.T) string {
	t.Helper()
	var id string
	if err := e.repo.Pool().QueryRow(context.Background(),
		`INSERT INTO users (name, role, status, username, password_hash)
		 VALUES ('Lifecycle Admin', 'super_admin', 'active', $1, '') RETURNING id`,
		"lifeadmin_"+uuid.NewString()[:12],
	).Scan(&id); err != nil {
		t.Fatalf("insert admin: %v", err)
	}
	return id
}

func (e *lifecycleEnv) course(t *testing.T) uuid.UUID {
	t.Helper()
	var id uuid.UUID
	if err := e.repo.Pool().QueryRow(context.Background(),
		`INSERT INTO course (title, level, subject, instructor_name) VALUES ($1, 'sma', 'math', 'Teacher') RETURNING id`, "Course "+uuid.NewString(),
	).Scan(&id); err != nil {
		t.Fatalf("insert course: %v", err)
	}
	return id
}

func (e *lifecycleEnv) courseProduct(t *testing.T, price int, courseID uuid.UUID) uuid.UUID {
	t.Helper()
	var id uuid.UUID
	if err := e.repo.Pool().QueryRow(context.Background(),
		`INSERT INTO product (type, name, price, stock, status) VALUES ('course', $1, $2, 0, 'published') RETURNING id`,
		"Course product "+uuid.NewString(), price,
	).Scan(&id); err != nil {
		t.Fatalf("insert course product: %v", err)
	}
	e.exec(t, `INSERT INTO product_course (product_id, course_id) VALUES ($1, $2)`, id, courseID)
	return id
}

func (e *lifecycleEnv) book(t *testing.T, stock int) uuid.UUID {
	t.Helper()
	var id uuid.UUID
	if err := e.repo.Pool().QueryRow(context.Background(),
		`INSERT INTO product (type, name, price, stock, status, weight_grams) VALUES ('book', $1, 30000, $2, 'published', 300) RETURNING id`,
		"Book "+uuid.NewString(), stock,
	).Scan(&id); err != nil {
		t.Fatalf("insert book: %v", err)
	}
	return id
}

// pendingOrder checks out a cart of the given products (book qty 2) so the
// order is payment_pending with its stock taken.
func (e *lifecycleEnv) pendingOrder(t *testing.T, student uuid.UUID, products ...uuid.UUID) uuid.UUID {
	t.Helper()
	ctx := context.Background()
	order, _, err := e.svc.MintCart(ctx, student.String())
	if err != nil {
		t.Fatalf("MintCart: %v", err)
	}
	physical := false
	for _, p := range products {
		var typ string
		if err := e.repo.Pool().QueryRow(ctx, `SELECT type FROM product WHERE id = $1`, p).Scan(&typ); err != nil {
			t.Fatalf("product type: %v", err)
		}
		qty := 1
		if typ == "book" {
			qty, physical = 2, true
		}
		if err := e.svc.AddItem(ctx, student.String(), order.ID.String(), p.String(), qty); err != nil {
			t.Fatalf("AddItem: %v", err)
		}
	}
	if physical {
		e.exec(t, `UPDATE orders SET shipping_cost = 18000, selected_courier = 'JNE' WHERE id = $1`, order.ID)
	}
	if _, err := e.svc.Checkout(ctx, student.String(), order.ID.String(), "life-"+uuid.NewString(), ""); err != nil {
		t.Fatalf("Checkout: %v", err)
	}
	return order.ID
}

func (e *lifecycleEnv) expireDeadline(t *testing.T, orderID uuid.UUID) {
	t.Helper()
	e.exec(t, `UPDATE orders SET payment_expires_at = now() - interval '1 minute' WHERE id = $1`, orderID)
}

func (e *lifecycleEnv) status(t *testing.T, orderID uuid.UUID) string {
	t.Helper()
	var s string
	if err := e.repo.Pool().QueryRow(context.Background(), `SELECT status FROM orders WHERE id = $1`, orderID).Scan(&s); err != nil {
		t.Fatalf("order status: %v", err)
	}
	return s
}

func (e *lifecycleEnv) count(t *testing.T, sql string, args ...any) int {
	t.Helper()
	var n int
	if err := e.repo.Pool().QueryRow(context.Background(), sql, args...).Scan(&n); err != nil {
		t.Fatalf("count %q: %v", sql, err)
	}
	return n
}

func (e *lifecycleEnv) activeEnrollments(t *testing.T, student, course uuid.UUID) int {
	t.Helper()
	return e.count(t, `SELECT count(*) FROM course_session WHERE student_id = $1 AND course_id = $2 AND status = 'active'`, student, course)
}

// TODO: uncertain: this payload shape is hand-written, not captured from the provider.
func (e *lifecycleEnv) settlementPayload(orderID uuid.UUID) []byte {
	b, _ := json.Marshal(map[string]string{
		"transaction_status": "settlement",
		"order_id":           orderID.String(),
		"transaction_id":     "tx-" + orderID.String(),
		"payment_type":       "bank_transfer",
	})
	return b
}
