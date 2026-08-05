package integration_test

import (
	"context"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// seedPendingOrder inserts an order in payment_pending status with one line item,
// bypassing the Midtrans checkout flow. Returns the order ID.
func seedPendingOrder(t *testing.T, env *testEnv, studentID, productName string, amount int64) string {
	t.Helper()
	ctx := context.Background()
	productID := seedProduct(t, env, "book", productName, amount)
	var orderID string
	err := env.pool.QueryRow(ctx,
		`INSERT INTO orders (student_id, status, subtotal, discount, shipping_cost, total)
		 VALUES ($1, 'payment_pending', $2, 0, 0, $2) RETURNING id`,
		studentID, amount,
	).Scan(&orderID)
	require.NoError(t, err)
	_, err = env.pool.Exec(ctx,
		`INSERT INTO order_item (order_id, product_id, product_type, name, unit_price, qty, jumlah, weight_grams)
		 VALUES ($1, $2, 'book', $3, $4, 1, $4, 0)`,
		orderID, productID, productName, amount,
	)
	require.NoError(t, err)
	return orderID
}

func TestStudentDashboard(t *testing.T) {
	env := newTestEnv(t)

	t.Run("enrolled courses show title and progress", func(t *testing.T) {
		student := seedUser(t, env, "student", "active", false)
		token := authToken(t, env, student, "student")

		course := seedCourse(t, env, "Matematika Dasar")
		section := seedSection(t, env, course)
		lesson1 := seedLesson(t, env, section)
		_ = seedLesson(t, env, section)
		seedCourseSession(t, env, student, course, "")

		markResp := env.doJSON(t, http.MethodPost, "/api/v1/courses/"+course+"/lessons/"+lesson1+"/complete", nil, token)
		require.Equal(t, http.StatusOK, markResp.StatusCode)
		markResp.Body.Close()

		resp := env.doJSON(t, http.MethodGet, "/api/v1/students/dashboard", nil, token)
		body := decodeBody(t, resp)
		require.Equal(t, http.StatusOK, resp.StatusCode, "body: %v", body)

		enrolled, ok := body["enrolled_courses"].([]any)
		require.True(t, ok, "expected enrolled_courses array, got: %v", body)
		require.Len(t, enrolled, 1)
		c := enrolled[0].(map[string]any)
		assert.Equal(t, course, c["id"])
		assert.Equal(t, "Matematika Dasar", c["title"])
		assert.Equal(t, float64(2), c["total_lessons"])
		assert.Equal(t, float64(1), c["done_lessons"])
		progress, _ := c["progress"].(float64)
		assert.InDelta(t, 0.5, progress, 1e-9)
	})

	t.Run("pending order surfaces id, product and amount", func(t *testing.T) {
		student := seedUser(t, env, "student", "active", false)
		token := authToken(t, env, student, "student")
		orderID := seedPendingOrder(t, env, student, "Buku Soal SNBT", 75000)

		resp := env.doJSON(t, http.MethodGet, "/api/v1/students/dashboard", nil, token)
		body := decodeBody(t, resp)
		require.Equal(t, http.StatusOK, resp.StatusCode, "body: %v", body)

		pending, ok := body["pending_order"].(map[string]any)
		require.True(t, ok, "expected pending_order, got: %v", body)
		assert.Equal(t, orderID, pending["id"])
		assert.Equal(t, "Buku Soal SNBT", pending["product"])
		assert.Equal(t, float64(75000), pending["amount"])
	})

	t.Run("dashboard is isolated per student", func(t *testing.T) {
		studentA := seedUser(t, env, "student", "active", false)
		studentB := seedUser(t, env, "student", "active", false)
		tokenB := authToken(t, env, studentB, "student")

		courseA := seedCourse(t, env, "Course for A")
		seedCourseSession(t, env, studentA, courseA, "")
		seedPendingOrder(t, env, studentA, "Only A's book", 50000)

		resp := env.doJSON(t, http.MethodGet, "/api/v1/students/dashboard", nil, tokenB)
		body := decodeBody(t, resp)
		require.Equal(t, http.StatusOK, resp.StatusCode, "body: %v", body)

		enrolled, _ := body["enrolled_courses"].([]any)
		assert.Empty(t, enrolled)
		assert.Nil(t, body["pending_order"])
	})
}

func TestStudentProfile(t *testing.T) {
	env := newTestEnv(t)

	student := seedUser(t, env, "student", "active", false)
	token := authToken(t, env, student, "student")

	resp := env.doJSON(t, http.MethodGet, "/api/v1/students/profile", nil, token)
	body := decodeBody(t, resp)
	require.Equal(t, http.StatusOK, resp.StatusCode, "body: %v", body)

	assert.Equal(t, student, body["id"])
	assert.Equal(t, "Test User", body["name"])
	assert.Equal(t, "student", body["role"])
	assert.Equal(t, "active", body["status"])
	assert.Equal(t, false, body["otp_enabled"])
	email, _ := body["email"].(string)
	assert.NotEmpty(t, email)
}

// Staff tokens used to pass here: the group was JWT-only, so a super_admin got
// their own record back and the web app rendered them inside the student shell.
func TestStudentRoutes_RejectStaffTokens(t *testing.T) {
	env := newTestEnv(t)

	for _, role := range []string{"super_admin", "admin_exam", "admin_school", "admin_store"} {
		t.Run(role, func(t *testing.T) {
			staff := seedUser(t, env, role, "active", false)
			token := authToken(t, env, staff, role)

			for _, call := range []struct {
				method string
				path   string
				body   map[string]any
			}{
				{http.MethodGet, "/api/v1/students/dashboard", nil},
				{http.MethodGet, "/api/v1/students/profile", nil},
				{http.MethodPatch, "/api/v1/students/profile", map[string]any{"name": "Staff"}},
				{http.MethodPatch, "/api/v1/students/photo", map[string]any{"photo_url": "avatars/x.png"}},
			} {
				resp := env.doJSON(t, call.method, call.path, call.body, token)
				body := decodeBody(t, resp)
				assert.Equal(t, http.StatusForbidden, resp.StatusCode, "%s %s body: %v", call.method, call.path, body)
				assert.Equal(t, "forbidden", body["code"], "%s %s", call.method, call.path)
			}
		})
	}
}

func TestStudentUpdateProfile(t *testing.T) {
	env := newTestEnv(t)

	t.Run("updates name", func(t *testing.T) {
		student := seedUser(t, env, "student", "active", false)
		token := authToken(t, env, student, "student")

		resp := env.doJSON(t, http.MethodPatch, "/api/v1/students/profile", map[string]any{"name": "New Name"}, token)
		body := decodeBody(t, resp)
		require.Equal(t, http.StatusOK, resp.StatusCode, "body: %v", body)
		assert.Equal(t, "New Name", body["name"])
	})

	t.Run("email collision yields 409", func(t *testing.T) {
		studentA := seedUser(t, env, "student", "active", false)
		tokenA := authToken(t, env, studentA, "student")
		studentB := seedUser(t, env, "student", "active", false)

		ctx := context.Background()
		var emailB string
		require.NoError(t, env.pool.QueryRow(ctx, `SELECT email FROM users WHERE id = $1`, studentB).Scan(&emailB))

		resp := env.doJSON(t, http.MethodPatch, "/api/v1/students/profile", map[string]any{"email": emailB}, tokenA)
		body := decodeBody(t, resp)
		assert.Equal(t, http.StatusConflict, resp.StatusCode, "body: %v", body)
	})
}