package handler_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"akademi-bimbel/config"
	"akademi-bimbel/internal/handler"
	"akademi-bimbel/internal/infra"
	"akademi-bimbel/internal/repository"
	"akademi-bimbel/internal/server"
	"akademi-bimbel/internal/service"

	"github.com/alicebob/miniredis/v2"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/labstack/echo/v4"
	"github.com/redis/go-redis/v9"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"
)

// webhookSigTestHexKey is a fixed 32-byte AES key used only to encrypt the
// biteship_webhook_secret fixture in these tests — not a real secret.
const webhookSigTestHexKey = "4c73650bf4ff750b4676412888747ed94bef9d8bba9880270276f9e180270d94"

// newWebhookSignatureTestEnv builds a real-DB-backed router exercising the
// production route table (server.RegisterRoutesForTest), so these tests hit
// the HTTP boundary exactly like a real webhook call — not the service layer
// directly, which is the whole point of this file (D-6 / regression guard
// for 841cd84).
func newWebhookSignatureTestEnv(t *testing.T) (*echo.Echo, *repository.Repository) {
	t.Helper()
	ctx := context.Background()

	pgContainer, err := tcpostgres.Run(ctx,
		"postgres:17-alpine",
		tcpostgres.WithDatabase("akademi_webhook_sig_test"),
		tcpostgres.WithUsername("test"),
		tcpostgres.WithPassword("test"),
		tcpostgres.BasicWaitStrategies(),
	)
	if err != nil {
		t.Fatalf("start postgres container: %v", err)
	}
	t.Cleanup(func() {
		if err := pgContainer.Terminate(ctx); err != nil {
			t.Logf("terminate postgres container: %v", err)
		}
	})

	dsn, err := pgContainer.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		t.Fatalf("connection string: %v", err)
	}
	if err := infra.RunMigrations(ctx, dsn); err != nil {
		t.Fatalf("run migrations: %v", err)
	}

	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatalf("open pool: %v", err)
	}
	t.Cleanup(pool.Close)

	repo := repository.New(pool)

	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("miniredis: %v", err)
	}
	t.Cleanup(mr.Close)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})

	cfg := &config.Config{
		JWTSecret:           "test-secret",
		AccessTokenTTL:      15 * time.Minute,
		RefreshTokenTTL:     168 * time.Hour,
		ConfigEncryptionKey: webhookSigTestHexKey,
	}
	signer := infra.NewJWTSigner(cfg.JWTSecret, cfg.AccessTokenTTL)

	svc := service.NewWithStore(
		repo, repo, rdb, signer,
		&service.NoopOTPProvider{}, &service.NoopEmailProvider{},
		&service.NoopPaymentClient{}, &service.NoopLogisticsClient{},
		nil, cfg,
		nil,
	)

	h := handler.New(svc)
	e := echo.New()
	e.HideBanner = true
	server.RegisterRoutesForTest(e, h, svc, signer)

	return e, repo
}

// seedShippedOrderForSigTest creates a minimal physical order already marked
// shipped via Biteship, so the webhook path can locate it by
// biteshipOrderID. It mirrors createShippableOrderForHandler's pattern of
// building a real cart via MintCart/AddItem then landing state with SQL.
func seedShippedOrderForSigTest(t *testing.T, svc *service.Service, repo *repository.Repository, biteshipOrderID string) uuid.UUID {
	t.Helper()
	ctx := context.Background()

	var productID string
	if err := repo.Pool().QueryRow(ctx,
		`INSERT INTO product (type, name, price, stock, status, weight_grams)
		 VALUES ('book', $1, 50000, 10, 'published', 500) RETURNING id`,
		"Webhook Sig Test Book "+uuid.New().String(),
	).Scan(&productID); err != nil {
		t.Fatalf("create product: %v", err)
	}

	var studentID string
	if err := repo.Pool().QueryRow(ctx,
		`INSERT INTO users (name, role, status, username, password_hash)
		 VALUES ('Webhook Sig Test Student', 'student', 'active', $1, '') RETURNING id`,
		"webhooksigtest_"+uuid.New().String(),
	).Scan(&studentID); err != nil {
		t.Fatalf("insert student: %v", err)
	}

	order, _, err := svc.MintCart(ctx, studentID)
	if err != nil {
		t.Fatalf("MintCart: %v", err)
	}
	if err := svc.AddItem(ctx, studentID, order.ID.String(), productID, 1); err != nil {
		t.Fatalf("AddItem: %v", err)
	}
	if err := repo.SetShippedBiteship(ctx, order.ID, "WB-SIGTEST-1", biteshipOrderID); err != nil {
		t.Fatalf("seed shipped order: %v", err)
	}
	return order.ID
}

// assertShippingWebhookWroteNothing confirms the webhook attempt left the
// order's shipment_status untouched and created no shipment event row —
// the "nothing written" half of the rejection contract, asserted rather
// than assumed.
func assertShippingWebhookWroteNothing(t *testing.T, repo *repository.Repository, orderID uuid.UUID) {
	t.Helper()
	ctx := context.Background()

	order, err := repo.GetOrderByID(ctx, orderID)
	if err != nil {
		t.Fatalf("GetOrderByID: %v", err)
	}
	if order.ShipmentStatus != nil {
		t.Errorf("want shipment_status untouched, got %v", *order.ShipmentStatus)
	}

	events, err := repo.ListShipmentEvents(ctx, orderID)
	if err != nil {
		t.Fatalf("ListShipmentEvents: %v", err)
	}
	if len(events) != 0 {
		t.Errorf("want 0 shipment events, got %d", len(events))
	}
}

// webhookErrorBody is the shape of handler.APIError, decoded from the
// response so tests can assert on the "code" field rather than trusting
// only the HTTP status.
type webhookErrorBody struct {
	Code string `json:"code"`
}

// TestPaymentWebhookRejectsBadSignature is the handler-level regression
// guard for the no-op signature verifier shipped in 841cd84: a well-formed
// Midtrans body carrying a wrong signature_key must be rejected 401
// invalid_signature through the real HTTP route, not just at the service
// layer (which is all the coverage that bypass ever had).
func TestPaymentWebhookRejectsBadSignature(t *testing.T) {
	e, _ := newWebhookSignatureTestEnv(t)

	body := []byte(`{
		"order_id": "11111111-1111-1111-1111-111111111111",
		"transaction_id": "trx-sigtest-1",
		"transaction_status": "settlement",
		"gross_amount": "50000.00",
		"status_code": "200",
		"signature_key": "wrong-signature-key"
	}`)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/webhooks/payment", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusUnauthorized, rec.Body.String())
	}

	var got webhookErrorBody
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode response body: %v (body=%s)", err, rec.Body.String())
	}
	if got.Code != "invalid_signature" {
		t.Errorf("code = %q, want %q", got.Code, "invalid_signature")
	}
}

// TestShippingWebhookRejectsBadSecret is the handler-level regression guard
// for FR-C-11: the shipping webhook must fail closed through the real HTTP
// route for a wrong header, a missing header, and — the exact fail-open
// shape 841cd84 shipped — a header that "matches" only because no secret is
// configured. All three must be 401 invalid_signature with nothing written.
func TestShippingWebhookRejectsBadSecret(t *testing.T) {
	e, repo := newWebhookSignatureTestEnv(t)
	svc := service.NewWithStore(repo, repo, nil, nil, &service.NoopOTPProvider{}, &service.NoopEmailProvider{}, &service.NoopPaymentClient{}, &service.NoopLogisticsClient{}, nil, nil, nil)

	post := func(t *testing.T, header string, setHeader bool, body []byte) *httptest.ResponseRecorder {
		t.Helper()
		req := httptest.NewRequest(http.MethodPost, "/api/v1/webhooks/shipping", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		if setHeader {
			req.Header.Set("X-Biteship-Signature", header)
		}
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)
		return rec
	}

	assertRejected := func(t *testing.T, rec *httptest.ResponseRecorder) {
		t.Helper()
		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusUnauthorized, rec.Body.String())
		}
		var got webhookErrorBody
		if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
			t.Fatalf("decode response body: %v (body=%s)", err, rec.Body.String())
		}
		if got.Code != "invalid_signature" {
			t.Errorf("code = %q, want %q", got.Code, "invalid_signature")
		}
	}

	// Runs first, before any subtest configures biteship_webhook_secret:
	// getWebhookSecret must still return "" here, so this is the true
	// unset-secret case, not just an empty string this test happened to set.
	t.Run("correct_looking_header_against_empty_configured_secret", func(t *testing.T) {
		// biteship_webhook_secret is intentionally left unset for this
		// order — getWebhookSecret returns "", and a header value that
		// also happens to be empty is exactly the shape
		// subtle.ConstantTimeCompare("", "") would wrongly accept, which
		// is the bug this repo shipped once (841cd84).
		orderID := seedShippedOrderForSigTest(t, svc, repo, "webhooksig-empty-secret")

		body := shipWebhookSigTestBody("webhooksig-empty-secret")
		rec := post(t, "", true, body)
		assertRejected(t, rec)
		assertShippingWebhookWroteNothing(t, repo, orderID)
	})

	// From here on, biteship_webhook_secret is configured for the rest of
	// this test's subtests.
	if err := repo.UpsertSystemConfig(context.Background(), "biteship_webhook_secret", encryptOrFail(t, "correct-secret"), true); err != nil {
		t.Fatalf("seed webhook secret: %v", err)
	}

	t.Run("bad_header_against_configured_secret", func(t *testing.T) {
		orderID := seedShippedOrderForSigTest(t, svc, repo, "webhooksig-bad-header")

		body := shipWebhookSigTestBody("webhooksig-bad-header")
		rec := post(t, "totally-wrong-signature", true, body)
		assertRejected(t, rec)
		assertShippingWebhookWroteNothing(t, repo, orderID)
	})

	t.Run("absent_header", func(t *testing.T) {
		orderID := seedShippedOrderForSigTest(t, svc, repo, "webhooksig-absent-header")

		body := shipWebhookSigTestBody("webhooksig-absent-header")
		rec := post(t, "", false, body)
		assertRejected(t, rec)
		assertShippingWebhookWroteNothing(t, repo, orderID)
	})
}

func shipWebhookSigTestBody(orderID string) []byte {
	body, _ := json.Marshal(map[string]any{
		"order_id":   orderID,
		"status":     "confirmed",
		"updated_at": time.Now(),
	})
	return body
}

func encryptOrFail(t *testing.T, plaintext string) string {
	t.Helper()
	encrypted, err := service.EncryptConfigValue(webhookSigTestHexKey, plaintext)
	if err != nil {
		t.Fatalf("encrypt webhook secret: %v", err)
	}
	return encrypted
}

// TestShippingWebhookAcceptsInstallProbe is the HTTP-boundary half of the
// installation handshake. Biteship validates the URL by POSTing an empty body
// and will not install a webhook that answers anything but 2xx; before the
// carve-out this returned 500 ("parse shipping webhook payload: unexpected
// end of JSON input"), which is how the install failed in staging.
func TestShippingWebhookAcceptsInstallProbe(t *testing.T) {
	e, repo := newWebhookSignatureTestEnv(t)

	if err := repo.UpsertSystemConfig(context.Background(), "biteship_webhook_secret", encryptOrFail(t, "correct-secret"), true); err != nil {
		t.Fatalf("seed webhook secret: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/v1/webhooks/shipping", bytes.NewReader(nil))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Biteship-Signature", "correct-secret")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}
}
