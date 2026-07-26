package handler_test

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"akademi-bimbel/internal/service"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

// ---------------------------------------------------------------------------
// AdminListExamRegistrations tests (FR-32 admin participant roster)
// ---------------------------------------------------------------------------

// mintSchoolTokenForEnv mints an access token carrying a school_id, so the
// middleware populates claims.SchoolID (needed to exercise admin_school
// tenant scoping on the roster).
func mintSchoolTokenForEnv(t *testing.T, env *testEnvWithStore, userID, role, schoolID string) string {
	t.Helper()
	rdb := redis.NewClient(&redis.Options{Addr: env.mr.Addr()})
	t.Cleanup(func() { rdb.Close() })
	tokenString, jti, err := env.signer.SignAccess(userID, role, &schoolID, []string{})
	if err != nil {
		t.Fatalf("SignAccess: %v", err)
	}
	if err := rdb.Set(context.Background(), "session:access:"+jti, userID, 15*time.Minute).Err(); err != nil {
		t.Fatalf("redis set session: %v", err)
	}
	return tokenString
}

func TestAdminListExamRegistrations_NoToken_Returns401(t *testing.T) {
	env := newTestEnvWithStore(t)
	rec := getRequest(t, env.e, "/api/v1/admin/exams/00000000-0000-0000-0000-000000000000/registrations", "")
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("want 401, got %d body=%s", rec.Code, rec.Body.String())
	}
}

// admin_store has no products(exam) capability at all (see rbac.go) — the
// read-only roster endpoint must reject it same as the write-gated group.
func TestAdminListExamRegistrations_RoleWithoutReadCapability_Returns403(t *testing.T) {
	env := newTestEnvWithStore(t)
	admin := seedUser(t, env.pool, service.RoleAdminStore, "Store Admin")
	token := mintTokenForEnv(t, env, admin.String(), service.RoleAdminStore)

	examID := seedExam(t, env.pool, "Roster RBAC Exam", false, "hidden", "classic")

	rec := getRequest(t, env.e, "/api/v1/admin/exams/"+examID.String()+"/registrations", token)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("want 403, got %d body=%s", rec.Code, rec.Body.String())
	}
}

// An admin_school whose token carries no school_id cannot be scoped, so the
// roster must refuse rather than fall back to an all-schools view.
func TestAdminListExamRegistrations_AdminSchoolNilSchool_Returns403(t *testing.T) {
	env := newTestEnvWithStore(t)
	admin := seedUser(t, env.pool, service.RoleAdminSchool, "Unscoped School Admin")
	token := mintTokenForEnv(t, env, admin.String(), service.RoleAdminSchool) // nil school_id

	examID := seedExam(t, env.pool, "Roster Nil-School Exam", false, "hidden", "classic")

	rec := getRequest(t, env.e, "/api/v1/admin/exams/"+examID.String()+"/registrations", token)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("want 403, got %d body=%s", rec.Code, rec.Body.String())
	}
}

// admin_school only has products(exam):read and must see its OWN school's
// participants — and MUST NOT see another school's students registered to the
// same exam (tenant isolation; the pre-fix endpoint leaked cross-school PII).
func TestAdminListExamRegistrations_AdminSchool_ScopedToOwnSchool(t *testing.T) {
	env := newTestEnvWithStore(t)
	ctx := context.Background()

	schoolA := seedSchool(t, env.pool)
	schoolB := seedSchool(t, env.pool)

	admin := seedUser(t, env.pool, service.RoleAdminSchool, "School A Admin")
	token := mintSchoolTokenForEnv(t, env, admin.String(), service.RoleAdminSchool, schoolA)

	examID := seedExam(t, env.pool, "Shared Roster Exam", false, "hidden", "classic")

	studentA := seedUser(t, env.pool, service.RoleStudent, "Student A")
	studentB := seedUser(t, env.pool, service.RoleStudent, "Student B")
	for id, school := range map[uuid.UUID]string{studentA: schoolA, studentB: schoolB} {
		if _, err := env.pool.Exec(ctx, `UPDATE users SET school_id = $1 WHERE id = $2`, school, id); err != nil {
			t.Fatalf("set user school: %v", err)
		}
	}
	regA := seedRegistration(t, env.pool, studentA, examID)
	seedRegistration(t, env.pool, studentB, examID)
	if _, err := env.pool.Exec(ctx, `UPDATE exam_registration SET participant_number = 1 WHERE id = $1`, regA); err != nil {
		t.Fatalf("set participant_number: %v", err)
	}

	rec := getRequest(t, env.e, "/api/v1/admin/exams/"+examID.String()+"/registrations", token)
	if rec.Code != http.StatusOK {
		t.Fatalf("want 200, got %d body=%s", rec.Code, rec.Body.String())
	}

	data := decodeRosterData(t, rec.Body.Bytes())
	if len(data) != 1 {
		t.Fatalf("admin_school must see only its own school's 1 student, got %d rows", len(data))
	}
	row := data[0].(map[string]any)
	if row["student_id"] != studentA.String() {
		t.Errorf("student_id: want school-A student %s, got %v (cross-school leak)", studentA.String(), row["student_id"])
	}
}

// super_admin is a global exam manager and sees the full cross-school roster.
func TestAdminListExamRegistrations_SuperAdmin_SeesAllSchools(t *testing.T) {
	env := newTestEnvWithStore(t)
	ctx := context.Background()

	schoolA := seedSchool(t, env.pool)
	schoolB := seedSchool(t, env.pool)

	admin := seedUser(t, env.pool, service.RoleSuperAdmin, "Super Admin")
	token := mintTokenForEnv(t, env, admin.String(), service.RoleSuperAdmin)

	examID := seedExam(t, env.pool, "Super Roster Exam", false, "hidden", "classic")
	studentA := seedUser(t, env.pool, service.RoleStudent, "Student A")
	studentB := seedUser(t, env.pool, service.RoleStudent, "Student B")
	for id, school := range map[uuid.UUID]string{studentA: schoolA, studentB: schoolB} {
		if _, err := env.pool.Exec(ctx, `UPDATE users SET school_id = $1 WHERE id = $2`, school, id); err != nil {
			t.Fatalf("set user school: %v", err)
		}
	}
	seedRegistration(t, env.pool, studentA, examID)
	seedRegistration(t, env.pool, studentB, examID)

	rec := getRequest(t, env.e, "/api/v1/admin/exams/"+examID.String()+"/registrations", token)
	if rec.Code != http.StatusOK {
		t.Fatalf("want 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	if data := decodeRosterData(t, rec.Body.Bytes()); len(data) != 2 {
		t.Fatalf("super_admin must see both schools' students, got %d rows", len(data))
	}
}

func decodeRosterData(t *testing.T, body []byte) []any {
	t.Helper()
	var resp map[string]any
	if err := json.Unmarshal(body, &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	data, ok := resp["data"].([]any)
	if !ok {
		t.Fatalf("data is not an array: %T", resp["data"])
	}
	return data
}
