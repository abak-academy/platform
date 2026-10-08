package integration_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"akademi-bimbel/config"
	"akademi-bimbel/internal/handler"
	"akademi-bimbel/internal/infra"
	"akademi-bimbel/internal/model"
	"akademi-bimbel/internal/repository"
	"akademi-bimbel/internal/server"
	"akademi-bimbel/internal/service"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"
	tcredis "github.com/testcontainers/testcontainers-go/modules/redis"
)

type testEnv struct {
	pool    *pgxpool.Pool
	rdb     *redis.Client
	svc     *service.Service
	signer  *infra.JWTSigner
	server  *httptest.Server
	otpCode *service.RecordingOTPProvider
}

// One Postgres and one Redis container serve the whole package. Migrations run
// once into akademi_test, which then acts purely as a CREATE DATABASE template:
// each test gets a private, already-migrated database for the cost of a file
// copy instead of a container start plus a full migration replay.
//
// RunMigrations closes its own pool (infra/migrate.go), and nothing pools against
// the template afterwards — CREATE DATABASE ... TEMPLATE refuses while any other
// session is connected to the source.
const templateDB = "akademi_test"

var (
	sharedInfraOnce sync.Once
	sharedMaintDSN  string
	sharedRedisAddr string
	sharedInfraErr  error
	testDatabaseSeq atomic.Int64
)

// withDatabase rewrites the database name in a postgres URL, keeping credentials,
// host, port and query string intact.
func withDatabase(dsn, name string) (string, error) {
	u, err := url.Parse(dsn)
	if err != nil {
		return "", err
	}
	u.Path = "/" + name
	return u.String(), nil
}

func sharedTestInfra(t *testing.T) (maintDSN, redisAddr string) {
	t.Helper()
	sharedInfraOnce.Do(func() {
		ctx := context.Background()

		pgContainer, err := tcpostgres.Run(ctx,
			"postgres:17-alpine",
			tcpostgres.WithDatabase(templateDB),
			tcpostgres.WithUsername("test"),
			tcpostgres.WithPassword("test"),
			tcpostgres.BasicWaitStrategies(),
		)
		if err != nil {
			sharedInfraErr = fmt.Errorf("start postgres container: %w", err)
			return
		}

		templateDSN, err := pgContainer.ConnectionString(ctx, "sslmode=disable")
		if err != nil {
			sharedInfraErr = fmt.Errorf("postgres connection string: %w", err)
			return
		}
		if err := infra.RunMigrations(ctx, templateDSN); err != nil {
			sharedInfraErr = fmt.Errorf("run migrations: %w", err)
			return
		}
		if sharedMaintDSN, err = withDatabase(templateDSN, "postgres"); err != nil {
			sharedInfraErr = fmt.Errorf("maintenance dsn: %w", err)
			return
		}

		redisContainer, err := tcredis.Run(ctx, "redis:7-alpine")
		if err != nil {
			sharedInfraErr = fmt.Errorf("start redis container: %w", err)
			return
		}
		addr, err := redisContainer.ConnectionString(ctx)
		if err != nil {
			sharedInfraErr = fmt.Errorf("redis connection string: %w", err)
			return
		}
		sharedRedisAddr = strings.TrimPrefix(addr, "redis://")
	})
	if sharedInfraErr != nil {
		t.Fatalf("shared test infra: %v", sharedInfraErr)
	}
	return sharedMaintDSN, sharedRedisAddr
}

func newTestEnv(t *testing.T) *testEnv {
	t.Helper()
	ctx := context.Background()

	maintDSN, redisAddr := sharedTestInfra(t)

	dbName := fmt.Sprintf("it_%d", testDatabaseSeq.Add(1))
	maint, err := pgxpool.New(ctx, maintDSN)
	require.NoError(t, err)
	_, err = maint.Exec(ctx, "CREATE DATABASE "+pgx.Identifier{dbName}.Sanitize()+" TEMPLATE "+templateDB)
	maint.Close()
	require.NoError(t, err)

	pgDSN, err := withDatabase(maintDSN, dbName)
	require.NoError(t, err)

	// The package is sequential (no t.Parallel), so one Redis keyspace is enough
	// as long as each test starts from empty.
	rdb := infra.NewRedis(redisAddr, "")
	require.NoError(t, rdb.FlushAll(ctx).Err())
	t.Cleanup(func() { _ = rdb.Close() })

	// --- Wiring ---
	pool, err := infra.NewPool(ctx, pgDSN)
	require.NoError(t, err)
	t.Cleanup(func() { pool.Close() })

	repo := repository.New(pool)
	require.NoError(t, repo.UpsertSystemConfig(ctx, "biteship_services", "jne:reg,jne:yes,tiki:ons", false))
	signer := infra.NewJWTSigner("test-secret-32-bytes-0000000000!", 15*time.Minute)

	cfg := &config.Config{
		Env:                "test",
		JWTSecret:          "test-secret-32-bytes-0000000000!",
		AccessTokenTTL:     15 * time.Minute,
		RefreshTokenTTL:    24 * time.Hour,
		OTPTTL:             5 * time.Minute,
		WorkerPollInterval: 5 * time.Second,
		CORSOrigins:        []string{"*"},
		OTPSecret:          "integration-otp-secret",
	}
	otpCode := service.NewRecordingOTPProvider()

	svc := service.NewWithStore(
		repo, repo,
		rdb,
		signer,
		otpCode,
		&service.NoopEmailProvider{},
		&service.NoopPaymentClient{},
		stubLogisticsClient{},
		nil,
		cfg,
		nil,
	)

	h := handler.New(svc)
	e := server.New(h, svc, signer, *cfg)

	ts := httptest.NewServer(e)
	t.Cleanup(ts.Close)

	return &testEnv{
		pool:    pool,
		rdb:     rdb,
		svc:     svc,
		signer:  signer,
		server:  ts,
		otpCode: otpCode,
	}
}

// doJSON sends a JSON request to the test server and returns the response.
func (env *testEnv) doJSON(t *testing.T, method, path string, body any, token string) *http.Response {
	t.Helper()
	var buf bytes.Buffer
	if body != nil {
		require.NoError(t, json.NewEncoder(&buf).Encode(body))
	}
	req, err := http.NewRequest(method, env.server.URL+path, &buf)
	require.NoError(t, err)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	return resp
}

// seedUser inserts a user row directly via SQL and returns the userID.
func seedUser(t *testing.T, env *testEnv, role, status string, otpEnabled bool) string {
	t.Helper()
	ctx := context.Background()
	email := fmt.Sprintf("test-%d@example.com", time.Now().UnixNano())
	// bcrypt of "password123"
	const passwordHash = "$2a$10$N9qo8uLOickgx2ZMRZoMyeIjZAgcfl7p92ldGxad68LJZdL17lhWy"
	var userID string
	err := env.pool.QueryRow(ctx,
		`INSERT INTO users (email, password_hash, role, name, status, otp_enabled)
		 VALUES ($1, $2, $3, $4, $5, $6) RETURNING id`,
		email, passwordHash, role, "Test User", status, otpEnabled,
	).Scan(&userID)
	require.NoError(t, err)
	return userID
}

// authToken mints an access token and writes the Redis session key so JWTMiddleware passes.
func authToken(t *testing.T, env *testEnv, userID, role string) string {
	t.Helper()
	ctx := context.Background()
	caps := service.Capabilities(role)
	tokenStr, jti, err := env.signer.SignAccess(userID, role, nil, caps)
	require.NoError(t, err)
	err = env.rdb.Set(ctx, "session:access:"+jti, userID, 15*time.Minute).Err()
	require.NoError(t, err)
	return tokenStr
}

// seedProduct inserts a product row and returns its ID.
func seedProduct(t *testing.T, env *testEnv, productType, name string, price int64) string {
	t.Helper()
	ctx := context.Background()
	var id string
	err := env.pool.QueryRow(ctx,
		`INSERT INTO product (type, name, price, stock, status)
		 VALUES ($1, $2, $3, 100, 'published') RETURNING id`,
		productType, name, price,
	).Scan(&id)
	require.NoError(t, err)
	return id
}

// seedCourse inserts a course row and returns its ID.
func seedCourse(t *testing.T, env *testEnv, title string) string {
	t.Helper()
	ctx := context.Background()
	var id string
	err := env.pool.QueryRow(ctx,
		`INSERT INTO course (title, level, subject, instructor_name)
		 VALUES ($1, 'beginner', 'general', 'Instructor') RETURNING id`,
		title,
	).Scan(&id)
	require.NoError(t, err)
	return id
}

// linkProductCourse links a product to a course via product_course join table.
func linkProductCourse(t *testing.T, env *testEnv, productID, courseID string) {
	t.Helper()
	ctx := context.Background()
	_, err := env.pool.Exec(ctx,
		`INSERT INTO product_course (product_id, course_id) VALUES ($1, $2)`,
		productID, courseID,
	)
	require.NoError(t, err)
}

// linkProductExam links a product to an exam via product_exam join table.
func linkProductExam(t *testing.T, env *testEnv, productID, examID string) {
	t.Helper()
	ctx := context.Background()
	_, err := env.pool.Exec(ctx,
		`INSERT INTO product_exam (product_id, exam_id) VALUES ($1, $2)`,
		productID, examID,
	)
	require.NoError(t, err)
}

// seedRegionIDs returns a valid province/city/district ID triple from the
// seeded region tables, for PatchCart calls that select a courier.
func seedRegionIDs(t *testing.T, env *testEnv) (provinceID, cityID, districtID string) {
	t.Helper()
	ctx := context.Background()

	require.NoError(t, env.pool.QueryRow(ctx,
		`SELECT id FROM province LIMIT 1`,
	).Scan(&provinceID), "must have at least one province seeded")

	require.NoError(t, env.pool.QueryRow(ctx,
		`SELECT id FROM city WHERE province_id = $1 LIMIT 1`, provinceID,
	).Scan(&cityID), "must have at least one city for this province")

	require.NoError(t, env.pool.QueryRow(ctx,
		`SELECT id FROM district WHERE city_id = $1 LIMIT 1`, cityID,
	).Scan(&districtID), "must have at least one district for this city")

	return provinceID, cityID, districtID
}

// seedPromo inserts a promo code and returns its ID.
func seedPromo(t *testing.T, env *testEnv, code string, discountPercent float64) string {
	t.Helper()
	ctx := context.Background()
	var id string
	err := env.pool.QueryRow(ctx,
		`INSERT INTO promo_code (code, discount_percent) VALUES ($1, $2) RETURNING id`,
		code, discountPercent,
	).Scan(&id)
	require.NoError(t, err)
	return id
}

// stubLogisticsClient stands in for a carrier API across the integration suite.
// NoopLogisticsClient used to serve this role by returning hardcoded quotes, but
// that made it impossible to tell a configured carrier from an unconfigured one
// in production — it now reports shipping unavailable instead. The fixed quotes
// live here, where they are unambiguously test data.
type stubLogisticsClient struct{}

func (stubLogisticsClient) GetRates(context.Context, service.ShippingQuoteRequest) ([]service.CourierRate, error) {
	return []service.CourierRate{
		{Courier: "JNE", Service: "REG", EstimatedDays: 3, Price: 15000, CourierCode: "jne", ServiceCode: "reg"},
		{Courier: "TIKI", Service: "ONS", EstimatedDays: 1, Price: 25000, CourierCode: "tiki", ServiceCode: "ons"},
	}, nil
}

func (stubLogisticsClient) CreateOrder(context.Context, service.CreateShipmentRequest) (service.Shipment, error) {
	return service.Shipment{}, service.ErrShippingUnavailable
}

func (stubLogisticsClient) TrackWaybill(context.Context, string, string) (service.WaybillTracking, error) {
	return service.WaybillTracking{}, service.ErrShippingUnavailable
}

func (stubLogisticsClient) CancelOrder(context.Context, string, string) error {
	return service.ErrShippingUnavailable
}

func (stubLogisticsClient) GetOrder(context.Context, string) (service.Shipment, error) {
	return service.Shipment{}, service.ErrShippingUnavailable
}

func relationProduct(t *testing.T, env *testEnv, kind string) (string, []string) {
	t.Helper()
	ctx := context.Background()
	ids := make([]string, 5)
	for i := range ids {
		if kind == "exam" {
			exam, err := env.svc.CreateExam(ctx, model.Exam{Title: "Relation exam", Mode: "standard"})
			require.NoError(t, err)
			ids[i] = exam.ID.String()
		} else {
			course, err := env.svc.CreateCourse(ctx, "Relation course", "SMA", "Math", "Instructor", service.RoleSuperAdmin)
			require.NoError(t, err)
			ids[i] = course.ID.String()
		}
	}
	p := model.Product{Type: kind, Name: "Original", Price: 1000, Status: "published", WeightGrams: 100, ImageURL: "old.jpg"}
	var product model.Product
	var err error
	if kind == "exam" {
		product, err = env.svc.CreateProductWithExams(ctx, p, ids[:2], service.RoleSuperAdmin)
	} else {
		product, err = env.svc.CreateProductWithCourses(ctx, p, ids[:2], service.RoleSuperAdmin)
	}
	require.NoError(t, err)
	return product.ID, ids
}

func persistedRelations(t *testing.T, env *testEnv, id, kind string) []string {
	t.Helper()
	product, err := env.svc.GetProduct(context.Background(), id, service.RoleSuperAdmin)
	require.NoError(t, err)
	if kind == "exam" {
		return product.ExamIDs
	}
	return product.CourseIDs
}
