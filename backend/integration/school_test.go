package integration_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"akademi-bimbel/internal/service"

	"github.com/stretchr/testify/require"
)

// authTokenWithSchool returns a JWT with a schoolID claim and writes the
// session key to Redis, mirroring authToken's contract.
func authTokenWithSchool(t *testing.T, env *testEnv, userID, role, schoolID string) string {
	t.Helper()
	ctx := context.Background()
	caps := service.Capabilities(role)
	tokenStr, jti, err := env.signer.SignAccess(userID, role, &schoolID, caps)
	require.NoError(t, err)
	err = env.rdb.Set(ctx, "session:access:"+jti, userID, 15*time.Minute).Err()
	require.NoError(t, err)
	return tokenStr
}

func TestSchoolCRUD_Integration(t *testing.T) {
	env := newTestEnv(t)

	// 1. Seed a school
	var schoolID string
	err := env.pool.QueryRow(t.Context(),
		`INSERT INTO school (name, code, npsn, school_types, alamat, status)
		 VALUES ($1, $2, $3, $4, $5, 'active') RETURNING id`,
		"SMAN Test", "smantest", "20000000", []string{"SMA"}, "Jl. Test No.1",
	).Scan(&schoolID)
	require.NoError(t, err)
	require.NotEmpty(t, schoolID)

	// 2. Create an admin_school account bound to the school
	superUserID := seedUser(t, env, "super_admin", "active", false)
	superToken := authToken(t, env, superUserID, "super_admin")
	createBody := map[string]interface{}{
		"email":     "schooladmin@test.com",
		"name":      "School Admin",
		"role":      "admin_school",
		"password":  "password123",
		"school_id": schoolID,
	}
	b, _ := json.Marshal(createBody)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/system/accounts", bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+superToken)
	rec := httptest.NewRecorder()
	env.server.Config.Handler.ServeHTTP(rec, req)

	require.Equal(t, http.StatusCreated, rec.Code)

	var createResp map[string]interface{}
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&createResp))
	createdAdminID := createResp["id"].(string)
	require.NotEmpty(t, createdAdminID)

	// 3. Register a student as the school admin
	adminToken := authTokenWithSchool(t, env, createdAdminID, "admin_school", schoolID)
	studentBody := map[string]interface{}{
		"name":    "Test Student",
		"jenjang": "SMA",
	}
	b, _ = json.Marshal(studentBody)
	req = httptest.NewRequest(http.MethodPost, "/api/v1/admin/students", bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+adminToken)
	rec = httptest.NewRecorder()
	env.server.Config.Handler.ServeHTTP(rec, req)

	require.Equal(t, http.StatusCreated, rec.Code)

	var regResp map[string]interface{}
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&regResp))
	require.NotEmpty(t, regResp["temp_password"])
	username := regResp["username"].(string)
	require.NotEmpty(t, username, "username must be populated")
	studentID := regResp["id"].(string)

	// 4. Student can log in with username + temp_password (FR-STU-10)
	loginBody := map[string]string{
		"identifier": username,
		"password":   regResp["temp_password"].(string),
	}
	b, _ = json.Marshal(loginBody)
	req = httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	rec = httptest.NewRecorder()
	env.server.Config.Handler.ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)

	// 5. Credential reissue returns a new password
	req = httptest.NewRequest(http.MethodGet,
		"/api/v1/admin/students/"+studentID+"/credentials", nil)
	req.Header.Set("Authorization", "Bearer "+adminToken)
	rec = httptest.NewRecorder()
	env.server.Config.Handler.ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)

	var credResp map[string]interface{}
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&credResp))
	require.NotEqual(t, regResp["temp_password"], credResp["temp_password"],
		"reissue should return a different password")
}

func TestStudentManualPasswordManagement_Integration(t *testing.T) {
	env := newTestEnv(t)
	ctx := context.Background()

	var schoolID string
	err := env.pool.QueryRow(ctx,
		`INSERT INTO school (name, code, npsn, school_types, alamat, status)
		 VALUES ($1, $2, $3, $4, $5, 'active') RETURNING id`,
		"Manual Password School", "manualpwd", "20000999", []string{"SMA"}, "Jl. Manual",
	).Scan(&schoolID)
	require.NoError(t, err)

	superID := seedUser(t, env, "super_admin", "active", false)
	superToken := authToken(t, env, superID, "super_admin")
	adminID := seedUser(t, env, "admin_school", "active", false)
	adminToken := authTokenWithSchool(t, env, adminID, "admin_school", schoolID)

	explicitPassword := "chosenPass123"
	resp, body := doJSONBody(t, env, http.MethodPost, "/api/v1/admin/students?school_id="+schoolID, map[string]any{
		"name":     "Explicit Integration",
		"jenjang":  "SMA",
		"password": explicitPassword,
	}, superToken)
	require.Equal(t, http.StatusCreated, resp.StatusCode)
	require.NotContains(t, marshalMap(body), explicitPassword)
	require.NotContains(t, body, "temp_password")
	explicitUsername := body["username"].(string)
	explicitID := body["id"].(string)
	studentAccess, studentJTI, err := env.signer.SignAccess(explicitID, service.RoleStudent, &schoolID, service.Capabilities(service.RoleStudent))
	require.NoError(t, err)
	require.NotEmpty(t, studentAccess)
	studentRefresh := "student-refresh-" + explicitID
	require.NoError(t, env.rdb.Set(ctx, "session:access:"+studentJTI, explicitID, 15*time.Minute).Err())
	require.NoError(t, env.rdb.SAdd(ctx, "user_access_sessions:"+explicitID, studentJTI).Err())
	require.NoError(t, env.rdb.Set(ctx, "session:refresh:"+studentRefresh, explicitID, 24*time.Hour).Err())
	require.NoError(t, env.rdb.SAdd(ctx, "user_refresh_sessions:"+explicitID, studentRefresh).Err())

	loginResp, _ := doJSONBody(t, env, http.MethodPost, "/api/v1/auth/login", map[string]string{
		"identifier": explicitUsername,
		"password":   explicitPassword,
	}, "")
	require.Equal(t, http.StatusOK, loginResp.StatusCode)

	resp, generated := doJSONBody(t, env, http.MethodPost, "/api/v1/admin/students?school_id="+schoolID, map[string]any{
		"name":    "Generated Integration",
		"jenjang": "SMA",
	}, superToken)
	require.Equal(t, http.StatusCreated, resp.StatusCode)
	require.NotEmpty(t, generated["temp_password"])
	loginResp, _ = doJSONBody(t, env, http.MethodPost, "/api/v1/auth/login", map[string]string{
		"identifier": generated["username"].(string),
		"password":   generated["temp_password"].(string),
	}, "")
	require.Equal(t, http.StatusOK, loginResp.StatusCode)

	forbiddenName := "Forbidden Integration"
	resp, body = doJSONBody(t, env, http.MethodPost, "/api/v1/admin/students", map[string]any{
		"name":     forbiddenName,
		"jenjang":  "SMA",
		"password": explicitPassword,
	}, adminToken)
	require.Equal(t, http.StatusForbidden, resp.StatusCode)
	require.NotContains(t, marshalMap(body), explicitPassword)
	var forbiddenCount int
	err = env.pool.QueryRow(ctx, `SELECT count(*) FROM users WHERE name = $1`, forbiddenName).Scan(&forbiddenCount)
	require.NoError(t, err)
	require.Zero(t, forbiddenCount)

	resetPassword := "resetPass123"
	resp, body = doJSONBody(t, env, http.MethodPatch, "/api/v1/admin/students/"+explicitID+"/password", map[string]string{
		"new_password": resetPassword,
	}, superToken)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	require.Equal(t, map[string]any{"message": "password updated"}, body)
	require.NotContains(t, marshalMap(body), resetPassword)
	accessExists, err := env.rdb.Exists(ctx, "session:access:"+studentJTI).Result()
	require.NoError(t, err)
	require.Zero(t, accessExists, "manual password set must revoke student access sessions")
	refreshExists, err := env.rdb.Exists(ctx, "session:refresh:"+studentRefresh).Result()
	require.NoError(t, err)
	require.Zero(t, refreshExists, "manual password set must revoke student refresh sessions")
	var passwordAuditCount int
	err = env.pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM audit_log WHERE actor_id = $1 AND target_type = 'user' AND target_id = $2 AND action = 'student.set_password'`,
		superID, explicitID,
	).Scan(&passwordAuditCount)
	require.NoError(t, err)
	require.Equal(t, 1, passwordAuditCount)

	loginResp, _ = doJSONBody(t, env, http.MethodPost, "/api/v1/auth/login", map[string]string{
		"identifier": explicitUsername,
		"password":   explicitPassword,
	}, "")
	require.Equal(t, http.StatusUnauthorized, loginResp.StatusCode)
	loginResp, _ = doJSONBody(t, env, http.MethodPost, "/api/v1/auth/login", map[string]string{
		"identifier": explicitUsername,
		"password":   resetPassword,
	}, "")
	require.Equal(t, http.StatusOK, loginResp.StatusCode)

	resp, _ = doJSONBody(t, env, http.MethodPatch, "/api/v1/admin/students/"+explicitID+"/password", map[string]string{
		"new_password": "blockedPass123",
	}, adminToken)
	require.Equal(t, http.StatusForbidden, resp.StatusCode)
	loginResp, _ = doJSONBody(t, env, http.MethodPost, "/api/v1/auth/login", map[string]string{
		"identifier": explicitUsername,
		"password":   "blockedPass123",
	}, "")
	require.Equal(t, http.StatusUnauthorized, loginResp.StatusCode)

	resp, body = doJSONBody(t, env, http.MethodPatch, "/api/v1/admin/students/"+superID+"/password", map[string]string{
		"new_password": "adminTarget123",
	}, superToken)
	require.Equal(t, http.StatusNotFound, resp.StatusCode)
	require.Equal(t, "student_not_found", body["code"])

	reissueJTI := "reissue-session"
	reissueRefresh := "reissue-refresh-" + explicitID
	require.NoError(t, env.rdb.Set(ctx, "session:access:"+reissueJTI, explicitID, 15*time.Minute).Err())
	require.NoError(t, env.rdb.SAdd(ctx, "user_access_sessions:"+explicitID, reissueJTI).Err())
	require.NoError(t, env.rdb.Set(ctx, "session:refresh:"+reissueRefresh, explicitID, 24*time.Hour).Err())
	require.NoError(t, env.rdb.SAdd(ctx, "user_refresh_sessions:"+explicitID, reissueRefresh).Err())
	resp, _ = doJSONBody(t, env, http.MethodGet, "/api/v1/admin/students/"+explicitID+"/credentials", nil, superToken)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	reissueAccessExists, err := env.rdb.Exists(ctx, "session:access:"+reissueJTI).Result()
	require.NoError(t, err)
	require.EqualValues(t, 1, reissueAccessExists, "credential reissue must preserve student access sessions")
	reissueRefreshExists, err := env.rdb.Exists(ctx, "session:refresh:"+reissueRefresh).Result()
	require.NoError(t, err)
	require.Zero(t, reissueRefreshExists, "credential reissue must revoke student refresh sessions")
}

func marshalMap(v map[string]any) string {
	b, _ := json.Marshal(v)
	return string(b)
}

// TestAdminCreateSchool_OmittedSchoolTypes_Integration reproduces the
// FR-SCH-02 blocker: omitting school_types (a spec-optional field) must not
// 500 — the NOT NULL column has no default applied when an explicit NULL is
// inserted, so nil []string must be coerced to []string{} before the INSERT.
func TestAdminCreateSchool_OmittedSchoolTypes_Integration(t *testing.T) {
	env := newTestEnv(t)

	superUserID := seedUser(t, env, "super_admin", "active", false)
	superToken := authToken(t, env, superUserID, "super_admin")

	body := map[string]interface{}{
		"name": "SMAN Omitted Types",
		"code": "smanomitted",
	}
	b, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/schools", bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+superToken)
	rec := httptest.NewRecorder()
	env.server.Config.Handler.ServeHTTP(rec, req)

	require.Equal(t, http.StatusCreated, rec.Code, "body: %s", rec.Body.String())

	var resp map[string]interface{}
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&resp))
	types, _ := resp["school_types"].([]interface{})
	require.Empty(t, types, "school_types should default to an empty array, not null")
}

// TestAdminCreateSchool_ResponseStatusActive_Integration reproduces the
// FR-SCH-02 blocker where a successful create response reported status:""
// instead of "active", even though the DB row persisted correctly.
func TestAdminCreateSchool_ResponseStatusActive_Integration(t *testing.T) {
	env := newTestEnv(t)

	superUserID := seedUser(t, env, "super_admin", "active", false)
	superToken := authToken(t, env, superUserID, "super_admin")

	body := map[string]interface{}{
		"name":         "SMAN Status Check",
		"code":         "smanstatuschk",
		"school_types": []string{"SMA"},
	}
	b, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/schools", bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+superToken)
	rec := httptest.NewRecorder()
	env.server.Config.Handler.ServeHTTP(rec, req)

	require.Equal(t, http.StatusCreated, rec.Code, "body: %s", rec.Body.String())

	var resp map[string]interface{}
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&resp))
	require.Equal(t, "active", resp["status"])
}

func TestSchoolCodeChange_Integration(t *testing.T) {
	env := newTestEnv(t)

	// Seed a school
	var schoolID string
	err := env.pool.QueryRow(t.Context(),
		`INSERT INTO school (name, code, npsn, school_types, alamat, status)
		 VALUES ($1, $2, $3, $4, $5, 'active') RETURNING id`,
		"Code Change School", "codechg", "20000001", []string{"SMA"}, "Jl. Test",
	).Scan(&schoolID)
	require.NoError(t, err)

	// Register a student
	alsUserID := seedUser(t, env, "admin_school", "active", false)
	adminToken := authTokenWithSchool(t, env, alsUserID, "admin_school", schoolID)
	studentBody := map[string]string{"name": "Stu", "jenjang": "SMA"}
	b, _ := json.Marshal(studentBody)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/students", bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+adminToken)
	rec := httptest.NewRecorder()
	env.server.Config.Handler.ServeHTTP(rec, req)
	require.Equal(t, http.StatusCreated, rec.Code)

	// Change code with students — should succeed (lock removed)
	suUserID := seedUser(t, env, "super_admin", "active", false)
	superToken := authToken(t, env, suUserID, "super_admin")
	updateBody := map[string]string{"code": "newcodechg"}
	b, _ = json.Marshal(updateBody)
	req = httptest.NewRequest(http.MethodPut, "/api/v1/admin/schools/"+schoolID, bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+superToken)
	rec = httptest.NewRecorder()
	env.server.Config.Handler.ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)
}

func TestRowScoping_Integration(t *testing.T) {
	env := newTestEnv(t)

	// Two schools
	var schoolA, schoolB string
	env.pool.QueryRow(t.Context(),
		`INSERT INTO school (name, code, npsn, school_types, alamat, status)
		 VALUES ($1, $2, $3, $4, $5, 'active') RETURNING id`,
		"School A", "schoola", "20000002", []string{"SMA"}, "Jl. A",
	).Scan(&schoolA)
	env.pool.QueryRow(t.Context(),
		`INSERT INTO school (name, code, npsn, school_types, alamat, status)
		 VALUES ($1, $2, $3, $4, $5, 'active') RETURNING id`,
		"School B", "schoolb", "20000003", []string{"SMA"}, "Jl. B",
	).Scan(&schoolB)

	// Admin A registers a student
	adminAUserID := seedUser(t, env, "admin_school", "active", false)
	tokenA := authTokenWithSchool(t, env, adminAUserID, "admin_school", schoolA)
	studentBody := map[string]string{"name": "Student A", "jenjang": "SMA"}
	b, _ := json.Marshal(studentBody)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/students", bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+tokenA)
	rec := httptest.NewRecorder()
	env.server.Config.Handler.ServeHTTP(rec, req)
	require.Equal(t, http.StatusCreated, rec.Code)
	var regResp map[string]interface{}
	json.NewDecoder(rec.Body).Decode(&regResp)
	studentID := regResp["id"].(string)

	// Admin B tries to access Admin A's student → 404
	adminBUserID := seedUser(t, env, "admin_school", "active", false)
	tokenB := authTokenWithSchool(t, env, adminBUserID, "admin_school", schoolB)
	req = httptest.NewRequest(http.MethodPatch, "/api/v1/admin/students/"+studentID,
		bytes.NewReader([]byte(`{"status":"deactivated"}`)))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+tokenB)
	rec = httptest.NewRecorder()
	env.server.Config.Handler.ServeHTTP(rec, req)
	require.Equal(t, http.StatusNotFound, rec.Code)
}
