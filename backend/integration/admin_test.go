package integration_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAdmin(t *testing.T) {
	env := newTestEnv(t)
	ctx := context.Background()

	// FR-INT-21: Admin confirm transitions and persists status.
	t.Run("FR-INT-21 admin confirm persists paid status", func(t *testing.T) {
		adminID := seedUser(t, env, "super_admin", "active", false)
		adminToken := authToken(t, env, adminID, "super_admin")

		// Seed a payment_pending order directly; AdminConfirmOrder sets status to "paid".
		studentID := seedUser(t, env, "student", "active", false)
		var orderID string
		err := env.pool.QueryRow(ctx,
			`INSERT INTO orders (student_id, status, subtotal, total)
			 VALUES ($1, 'payment_pending', 50000, 50000) RETURNING id`,
			studentID,
		).Scan(&orderID)
		require.NoError(t, err)

		key := fmt.Sprintf("confirm-%d", time.Now().UnixNano())
		// FR-25/FR-26: confirm now requires a payment_proof_url under payment_proof/.
		bodyBytes, err := json.Marshal(map[string]string{
			"payment_proof_url": "payment_proof/" + adminID + "/proof.jpg",
		})
		require.NoError(t, err)
		req, err := http.NewRequest(http.MethodPost,
			env.server.URL+"/api/v1/admin/orders/"+orderID+"/confirm", bytes.NewReader(bodyBytes))
		require.NoError(t, err)
		req.Header.Set("Authorization", "Bearer "+adminToken)
		req.Header.Set("Idempotency-Key", key)
		req.Header.Set("Content-Type", "application/json")
		resp, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		drainClose(resp)

		require.Equal(t, http.StatusOK, resp.StatusCode)

		var status string
		require.NoError(t, env.pool.QueryRow(ctx,
			`SELECT status FROM orders WHERE id=$1`, orderID,
		).Scan(&status))
		assert.Equal(t, "paid", status, "order status must be 'paid' after admin confirm")
	})

	// FR-INT-22: Admin ship writes tracking_number + shipped_at.
	t.Run("FR-INT-22 admin ship writes tracking_number and shipped_at", func(t *testing.T) {
		adminID := seedUser(t, env, "super_admin", "active", false)
		adminToken := authToken(t, env, adminID, "super_admin")

		// Seed an order already in "paid" status (shippable).
		studentID := seedUser(t, env, "student", "active", false)
		var orderID string
		err := env.pool.QueryRow(ctx,
			`INSERT INTO orders (student_id, status, subtotal, total)
			 VALUES ($1, 'paid', 50000, 50000) RETURNING id`,
			studentID,
		).Scan(&orderID)
		require.NoError(t, err)

		// /ship auto-books at the carrier and takes no body; manual waybill entry
		// moved to /ship-manual, which is what this case has always exercised.
		resp := env.doJSON(t, http.MethodPost,
			"/api/v1/admin/orders/"+orderID+"/ship-manual",
			map[string]string{"tracking_number": "JNE-123456"},
			adminToken,
		)
		drainClose(resp)
		require.Equal(t, http.StatusOK, resp.StatusCode)

		var trackingNumber string
		var shippedAt *time.Time
		require.NoError(t, env.pool.QueryRow(ctx,
			`SELECT tracking_number, shipped_at FROM orders WHERE id=$1`, orderID,
		).Scan(&trackingNumber, &shippedAt))
		assert.Equal(t, "JNE-123456", trackingNumber, "tracking_number must be persisted")
		assert.NotNil(t, shippedAt, "shipped_at must be set")
	})

	// FR-INT-23: Non-admin is forbidden on admin order routes.
	t.Run("FR-INT-23 student token gets 403 on admin order route", func(t *testing.T) {
		studentID := seedUser(t, env, "student", "active", false)
		studentToken := authToken(t, env, studentID, "student")

		// Seed any order so the route exists.
		var orderID string
		err := env.pool.QueryRow(ctx,
			`INSERT INTO orders (student_id, status, subtotal, total)
			 VALUES ($1, 'payment_pending', 10000, 10000) RETURNING id`,
			studentID,
		).Scan(&orderID)
		require.NoError(t, err)

		req, err := http.NewRequest(http.MethodPost,
			env.server.URL+"/api/v1/admin/orders/"+orderID+"/confirm", nil)
		require.NoError(t, err)
		req.Header.Set("Authorization", "Bearer "+studentToken)
		req.Header.Set("Idempotency-Key", "student-confirm-key")
		resp, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		body := decodeBody(t, resp)

		assert.Equal(t, http.StatusForbidden, resp.StatusCode)
		assert.Equal(t, "forbidden", body["code"], "error code must be 'forbidden'")
	})

	// BUG-A: Empty list endpoints return [] not null.
	t.Run("empty list endpoints return [] not null", func(t *testing.T) {
		adminID := seedUser(t, env, "super_admin", "active", false)
		adminToken := authToken(t, env, adminID, "super_admin")

		// Products list — no RBAC, just JWT.
		resp := env.doJSON(t, http.MethodGet, "/api/v1/admin/products", nil, adminToken)
		body := decodeBody(t, resp)
		require.Equal(t, http.StatusOK, resp.StatusCode)
		data, ok := body["data"].([]any)
		require.True(t, ok, "products data must be an array (not null), got: %v", body["data"])
		require.Empty(t, data)

		// Promo codes list — super_admin has promos:write via wildcard, no seed data.
		resp = env.doJSON(t, http.MethodGet, "/api/v1/admin/promo-codes", nil, adminToken)
		body = decodeBody(t, resp)
		require.Equal(t, http.StatusOK, resp.StatusCode)
		data, ok = body["data"].([]any)
		require.True(t, ok, "promo-codes data must be an array (not null), got: %v", body["data"])
		require.Empty(t, data)

		// Courses list — no courses seeded.
		resp = env.doJSON(t, http.MethodGet, "/api/v1/admin/courses", nil, adminToken)
		body = decodeBody(t, resp)
		require.Equal(t, http.StatusOK, resp.StatusCode)
		data, ok = body["data"].([]any)
		require.True(t, ok, "courses data must be an array (not null), got: %v", body["data"])
		require.Empty(t, data)

		// Tests list — no tests seeded.
		resp = env.doJSON(t, http.MethodGet, "/api/v1/admin/tests", nil, adminToken)
		body = decodeBody(t, resp)
		require.Equal(t, http.StatusOK, resp.StatusCode)
		data, ok = body["data"].([]any)
		require.True(t, ok, "tests data must be an array (not null), got: %v", body["data"])
		require.Empty(t, data)

		// Exams list — no exams seeded.
		resp = env.doJSON(t, http.MethodGet, "/api/v1/admin/exams", nil, adminToken)
		body = decodeBody(t, resp)
		require.Equal(t, http.StatusOK, resp.StatusCode)
		data, ok = body["data"].([]any)
		require.True(t, ok, "exams data must be an array (not null), got: %v", body["data"])
		require.Empty(t, data)

		// Sections list — seed a course with no sections, then list its sections.
		courseID := seedCourse(t, env, "Empty Sections Course")
		resp = env.doJSON(t, http.MethodGet, "/api/v1/admin/courses/"+courseID+"/sections", nil, adminToken)
		body = decodeBody(t, resp)
		require.Equal(t, http.StatusOK, resp.StatusCode)
		data, ok = body["data"].([]any)
		require.True(t, ok, "sections data must be an array (not null), got: %v", body["data"])
		require.Empty(t, data)

		// Questions list — seed a test with no questions, then list its questions.
		testID := seedTest(t, env, "Empty Q Test", "math", "algebra", 30)
		resp = env.doJSON(t, http.MethodGet, "/api/v1/admin/tests/"+testID+"/questions", nil, adminToken)
		body = decodeBody(t, resp)
		require.Equal(t, http.StatusOK, resp.StatusCode)
		data, ok = body["data"].([]any)
		require.True(t, ok, "questions data must be an array (not null), got: %v", body["data"])
		require.Empty(t, data)

		// Exam leaderboard — seed an exam with no sessions, then fetch its leaderboard.
		examID := seedExam(t, env, "Empty Leaderboard Exam", "score_only", nil)
		resp = env.doJSON(t, http.MethodGet, "/api/v1/admin/exams/"+examID+"/leaderboard", nil, adminToken)
		body = decodeBody(t, resp)
		require.Equal(t, http.StatusOK, resp.StatusCode)
		data, ok = body["data"].([]any)
		require.True(t, ok, "exam leaderboard data must be an array (not null), got: %v", body["data"])
		require.Empty(t, data)
	})

	// FR-INT-24: Validate promo computes discount against a real DB row.
	t.Run("FR-INT-24 validate promo computes discount against real DB row", func(t *testing.T) {
		discountPct := 10.0
		seedPromo(t, env, "SAVE10", discountPct)

		resp := env.doJSON(t, http.MethodPost, "/api/v1/promo-codes/validate",
			map[string]any{"code": "SAVE10", "subtotal": 100000.0},
			"", // no auth required
		)
		body := decodeBody(t, resp)
		require.Equal(t, http.StatusOK, resp.StatusCode)

		discount, _ := body["discount"].(float64)
		finalTotal, _ := body["final_total"].(float64)
		assert.Equal(t, 10000.0, discount, "discount must be 10% of 100000")
		assert.Equal(t, 90000.0, finalTotal, "final_total must be subtotal - discount")
	})

	// FR-INT-25: Admin create promo persists; used_count increment documented gap.
	t.Run("FR-INT-25 admin create promo persists row", func(t *testing.T) {
		adminID := seedUser(t, env, "super_admin", "active", false)
		adminToken := authToken(t, env, adminID, "super_admin")

		discountPct := 15.0
		resp := env.doJSON(t, http.MethodPost, "/api/v1/admin/promo-codes",
			map[string]any{
				"code":             "ADMIN15",
				"discount_percent": discountPct,
			},
			adminToken,
		)
		body := decodeBody(t, resp)
		require.Equal(t, http.StatusCreated, resp.StatusCode)

		promoID, ok := body["id"].(string)
		require.True(t, ok, "response must contain promo id, got: %v", body)
		require.NotEmpty(t, promoID)

		// Verify the row persisted in DB.
		var code string
		var usedCount int
		require.NoError(t, env.pool.QueryRow(ctx,
			`SELECT code, used_count FROM promo_code WHERE id=$1`, promoID,
		).Scan(&code, &usedCount))
		assert.Equal(t, "ADMIN15", code)
		assert.Equal(t, 0, usedCount, "used_count must start at 0")
	})

	t.Run("FR-INT-25 used_count increment on checkout", func(t *testing.T) {
		studentID := seedUser(t, env, "student", "active", false)
		token := authToken(t, env, studentID, "student")
		productID := seedProduct(t, env, "book", "Buku Promo Test", 100000)
		promoID := seedPromo(t, env, "PROMO25", 10.0)

		orderResp := env.doJSON(t, http.MethodPost, "/api/v1/orders", nil, token)
		require.Equal(t, http.StatusCreated, orderResp.StatusCode)
		orderID := decodeBody(t, orderResp)["id"].(string)

		drainClose(env.doJSON(t, http.MethodPost, "/api/v1/orders/"+orderID+"/items",
			map[string]any{"product_id": productID, "qty": 1}, token))

		provinceID, cityID, districtID := seedRegionIDs(t, env)
		patchResp := env.doJSON(t, http.MethodPatch, "/api/v1/orders/"+orderID,
			map[string]any{
				"promo_code":    "PROMO25",
				"courier":       "JNE",
				"service":       "REG",
				"shipping_cost": 15000.0,
				"province_id":   provinceID,
				"city_id":       cityID,
				"district_id":   districtID,
				"kode_pos":      "12345",
			}, token)
		require.Equal(t, http.StatusOK, patchResp.StatusCode, "PATCH promo: %v", decodeBody(t, patchResp))

		coResp := checkoutWithKey(t, env, orderID, token, fmt.Sprintf("idemp-promo25-%d", time.Now().UnixNano()))
		coBody := decodeBody(t, coResp)
		require.Equal(t, http.StatusOK, coResp.StatusCode, "checkout failed: %v", coBody)

		var usedCount int
		require.NoError(t, env.pool.QueryRow(ctx,
			`SELECT used_count FROM promo_code WHERE id=$1`, promoID,
		).Scan(&usedCount))
		assert.Equal(t, 1, usedCount, "used_count must be 1 after checkout with promo")
	})
}
