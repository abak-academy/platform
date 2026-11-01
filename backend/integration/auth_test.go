package integration_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// decodeBody decodes response JSON into a map; closes the body.
func decodeBody(t *testing.T, resp *http.Response) map[string]any {
	t.Helper()
	defer resp.Body.Close()
	var m map[string]any
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&m))
	return m
}

// parseJTI extracts the jti (RegisteredClaims.ID) from a JWT access token.
func parseJTI(t *testing.T, env *testEnv, token string) string {
	t.Helper()
	claims, err := env.signer.ParseAccess(token)
	require.NoError(t, err)
	return claims.RegisteredClaims.ID
}

// TestAuthFlow exercises FR-INT-01..06 against a single shared env (one container pair).
func TestAuthFlow(t *testing.T) {
	env := newTestEnv(t)
	ctx := context.Background()

	t.Run("FR-INT-01 register persists user and signals OTP", func(t *testing.T) {
		email := fmt.Sprintf("reg-%d@example.com", time.Now().UnixNano())
		resp := env.doJSON(t, http.MethodPost, "/api/v1/auth/register", map[string]any{
			"email":    email,
			"password": "password123",
			"name":     "Test Register",
		}, "")
		body := decodeBody(t, resp)

		require.Equal(t, http.StatusCreated, resp.StatusCode)
		assert.Equal(t, true, body["otp_required"])
		pendingToken, ok := body["pending_token"].(string)
		require.True(t, ok)
		assert.NotEmpty(t, pendingToken)

		var count int
		err := env.pool.QueryRow(ctx, `SELECT COUNT(*) FROM users WHERE email=$1 AND status != 'deleted'`, email).Scan(&count)
		require.NoError(t, err)
		assert.Equal(t, 1, count)
	})

	t.Run("FR-INT-02 duplicate email rejected", func(t *testing.T) {
		email := fmt.Sprintf("dup-%d@example.com", time.Now().UnixNano())
		const passwordHash = "$2a$10$N9qo8uLOickgx2ZMRZoMyeIjZAgcfl7p92ldGxad68LJZdL17lhWy"
		_, err := env.pool.Exec(ctx,
			`INSERT INTO users (email, password_hash, role, name, status, otp_enabled)
			 VALUES ($1, $2, 'student', 'Existing User', 'active', false)`,
			email, passwordHash,
		)
		require.NoError(t, err)

		resp := env.doJSON(t, http.MethodPost, "/api/v1/auth/register", map[string]any{
			"email":    email,
			"password": "password123",
			"name":     "Duplicate User",
		}, "")
		body := decodeBody(t, resp)

		require.Equal(t, http.StatusConflict, resp.StatusCode)
		assert.Equal(t, "email_taken", body["code"])

		var count int
		err = env.pool.QueryRow(ctx, `SELECT COUNT(*) FROM users WHERE email=$1`, email).Scan(&count)
		require.NoError(t, err)
		assert.Equal(t, 1, count)
	})

	t.Run("FR-INT-03 OTP verify activates session and returns tokens", func(t *testing.T) {
		email := fmt.Sprintf("otp-%d@example.com", time.Now().UnixNano())
		resp := env.doJSON(t, http.MethodPost, "/api/v1/auth/register", map[string]any{
			"email":    email,
			"password": "password123",
			"name":     "OTP User",
		}, "")
		regBody := decodeBody(t, resp)
		require.Equal(t, http.StatusCreated, resp.StatusCode)

		pendingToken := regBody["pending_token"].(string)

		// With hashed storage the raw code only crossed the provider boundary.
		// The env is package-scoped, so this test's code is the latest one.
		codes := env.otpCode.Codes
		require.NotEmpty(t, codes)
		code := codes[len(codes)-1]
		require.NotEmpty(t, code)

		resp2 := env.doJSON(t, http.MethodPost, "/api/v1/auth/otp/verify", map[string]any{
			"pending_token": pendingToken,
			"code":          code,
		}, "")
		body2 := decodeBody(t, resp2)
		require.Equal(t, http.StatusOK, resp2.StatusCode)

		accessToken, _ := body2["access_token"].(string)
		refreshToken, _ := body2["refresh_token"].(string)
		require.NotEmpty(t, accessToken)
		require.NotEmpty(t, refreshToken)

		jti := parseJTI(t, env, accessToken)
		exists, err := env.rdb.Exists(ctx, "session:access:"+jti).Result()
		require.NoError(t, err)
		assert.EqualValues(t, 1, exists)
	})

	t.Run("FR-INT-04 login writes Redis session", func(t *testing.T) {
		// Insert user directly with a known password hash so we can log in.
		// bcrypt cost-10 hash for "pass-int04".
		const knownHash = "$2a$10$lmfsViKHNqS8yVbOr1M7geunAOhZRsiPYB1A99ioPQmi.z0dQmkXe"
		email := fmt.Sprintf("login-%d@example.com", time.Now().UnixNano())
		_, err := env.pool.Exec(ctx,
			`INSERT INTO users (email, password_hash, role, name, status, otp_enabled)
			 VALUES ($1, $2, 'student', 'Login User', 'active', false)`,
			email, knownHash,
		)
		require.NoError(t, err)

		resp := env.doJSON(t, http.MethodPost, "/api/v1/auth/login", map[string]any{
			"identifier": email,
			"password":   "password123",
		}, "")
		body := decodeBody(t, resp)
		require.Equal(t, http.StatusOK, resp.StatusCode)

		accessToken, _ := body["access_token"].(string)
		refreshToken, _ := body["refresh_token"].(string)
		require.NotEmpty(t, accessToken)
		require.NotEmpty(t, refreshToken)

		jti := parseJTI(t, env, accessToken)

		exists, err := env.rdb.Exists(ctx, "session:access:"+jti).Result()
		require.NoError(t, err)
		assert.EqualValues(t, 1, exists, "session:access:<jti> must exist")

		exists, err = env.rdb.Exists(ctx, "session:refresh:"+refreshToken).Result()
		require.NoError(t, err)
		assert.EqualValues(t, 1, exists, "session:refresh:<token> must exist")
	})

	t.Run("FR-INT-05 logout revokes session; follow-up 401", func(t *testing.T) {
		userID := seedUser(t, env, "student", "active", false)
		token := authToken(t, env, userID, "student")
		jti := parseJTI(t, env, token)

		resp := env.doJSON(t, http.MethodPost, "/api/v1/auth/logout", nil, token)
		resp.Body.Close()
		assert.True(t, resp.StatusCode == http.StatusOK || resp.StatusCode == http.StatusNoContent,
			"logout should be 200 or 204, got %d", resp.StatusCode)

		exists, err := env.rdb.Exists(ctx, "session:access:"+jti).Result()
		require.NoError(t, err)
		assert.EqualValues(t, 0, exists, "session:access key must be deleted after logout")

		resp2 := env.doJSON(t, http.MethodGet, "/api/v1/auth/me", nil, token)
		resp2.Body.Close()
		assert.Equal(t, http.StatusUnauthorized, resp2.StatusCode)
	})

	t.Run("FR-INT-06 GET /me returns user from DB", func(t *testing.T) {
		userID := seedUser(t, env, "student", "active", false)
		token := authToken(t, env, userID, "student")

		var expectedEmail string
		err := env.pool.QueryRow(ctx, `SELECT email FROM users WHERE id=$1`, userID).Scan(&expectedEmail)
		require.NoError(t, err)

		resp := env.doJSON(t, http.MethodGet, "/api/v1/auth/me", nil, token)
		body := decodeBody(t, resp)

		require.Equal(t, http.StatusOK, resp.StatusCode)
		assert.Equal(t, expectedEmail, body["email"])
		assert.Equal(t, "student", body["role"])
	})
}
