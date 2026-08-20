package handler_test

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"net/http"
	"net/url"
	"strings"
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

// student has no capabilities at all (RoleStudent: {} in rbac.go) — the exam
// token is a check-in credential (NFR-S7); a student holding another
// participant's token could check them in, so a student's own access token
// must never reach the roster response that carries it.
func TestAdminListExamRegistrations_StudentRole_Returns403(t *testing.T) {
	env := newTestEnvWithStore(t)
	student := seedUser(t, env.pool, service.RoleStudent, "Some Student")
	token := mintTokenForEnv(t, env, student.String(), service.RoleStudent)

	examID := seedExam(t, env.pool, "Roster Student RBAC Exam", false, "hidden", "classic")

	rec := getRequest(t, env.e, "/api/v1/admin/exams/"+examID.String()+"/registrations", token)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("want 403, got %d body=%s", rec.Code, rec.Body.String())
	}
}

// FR-47: the roster must carry the participant's exam token so an admin can
// help a student who lost it, without database access. Asserted against the
// real decoded response body (not the Go struct) — this project has shipped
// features dead in prod behind struct-only assertions that don't prove the
// wire shape.
func TestAdminListExamRegistrations_ExposesToken(t *testing.T) {
	env := newTestEnvWithStore(t)
	ctx := context.Background()

	admin := seedUser(t, env.pool, service.RoleSuperAdmin, "Super Admin")
	token := mintTokenForEnv(t, env, admin.String(), service.RoleSuperAdmin)

	examID := seedExam(t, env.pool, "Roster Token Exam", false, "hidden", "classic")
	student := seedUser(t, env.pool, service.RoleStudent, "Token Student")
	regID := seedRegistration(t, env.pool, student, examID)

	var wantToken string
	if err := env.pool.QueryRow(ctx, `SELECT token FROM exam_registration WHERE id = $1`, regID).Scan(&wantToken); err != nil {
		t.Fatalf("read seeded token: %v", err)
	}

	rec := getRequest(t, env.e, "/api/v1/admin/exams/"+examID.String()+"/registrations", token)
	if rec.Code != http.StatusOK {
		t.Fatalf("want 200, got %d body=%s", rec.Code, rec.Body.String())
	}

	data := decodeRosterData(t, rec.Body.Bytes())
	if len(data) != 1 {
		t.Fatalf("want 1 row, got %d", len(data))
	}
	row := data[0].(map[string]any)
	got, ok := row["token"].(string)
	if !ok {
		t.Fatalf(`response row missing string "token" key: %+v`, row)
	}
	if got != wantToken {
		t.Errorf("token: want %q (the seeded exam_registration.token), got %q", wantToken, got)
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

func TestAdminListExamRegistrations_CursorWalksBothDirections(t *testing.T) {
	env := newTestEnvWithStore(t)
	ctx := context.Background()
	admin := seedUser(t, env.pool, service.RoleSuperAdmin, "Roster Pager Admin")
	token := mintTokenForEnv(t, env, admin.String(), service.RoleSuperAdmin)
	examID := seedExam(t, env.pool, "Paged Roster Exam", false, "hidden", "classic")

	wantNumbers := map[string][]float64{
		"asc":  {1, 2, 3},
		"desc": {3, 2, 1},
	}
	for i := 0; i < 5; i++ {
		student := seedUser(t, env.pool, service.RoleStudent, "Paged Student")
		regID := seedRegistration(t, env.pool, student, examID)
		if i < 3 {
			if _, err := env.pool.Exec(ctx, `UPDATE exam_registration SET participant_number = $1 WHERE id = $2`, i+1, regID); err != nil {
				t.Fatalf("set participant number: %v", err)
			}
		}
	}

	for direction, numbered := range wantNumbers {
		t.Run(direction, func(t *testing.T) {
			cursor := ""
			seen := map[string]bool{}
			var gotNumbers []float64
			var nilCount int
			for page := 0; page < 4; page++ {
				path := "/api/v1/admin/exams/" + examID.String() + "/registrations?limit=2&sort=" + direction
				if cursor != "" {
					path += "&cursor=" + url.QueryEscape(cursor)
				}
				rec := getRequest(t, env.e, path, token)
				if rec.Code != http.StatusOK {
					t.Fatalf("page %d: want 200, got %d body=%s", page, rec.Code, rec.Body.String())
				}
				var resp struct {
					Data       []map[string]any `json:"data"`
					NextCursor string           `json:"next_cursor"`
				}
				if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
					t.Fatalf("decode response: %v", err)
				}
				if len(resp.Data) > 2 {
					t.Fatalf("page %d: got %d rows, want at most 2", page, len(resp.Data))
				}
				for _, row := range resp.Data {
					id := row["registration_id"].(string)
					if seen[id] {
						t.Fatalf("registration %s returned twice", id)
					}
					seen[id] = true
					if n, ok := row["participant_number"].(float64); ok {
						gotNumbers = append(gotNumbers, n)
					} else {
						nilCount++
					}
				}
				cursor = resp.NextCursor
				if cursor == "" {
					break
				}
			}
			if len(seen) != 5 {
				t.Fatalf("cursor walk returned %d unique rows, want 5", len(seen))
			}
			if nilCount != 2 {
				t.Fatalf("missing participant numbers: got %d, want 2", nilCount)
			}
			if len(gotNumbers) != len(numbered) {
				t.Fatalf("numbered rows: got %v, want %v", gotNumbers, numbered)
			}
			for i := range numbered {
				if gotNumbers[i] != numbered[i] {
					t.Fatalf("numbered order: got %v, want %v", gotNumbers, numbered)
				}
			}
		})
	}
}

func TestAdminListExamRegistrations_InvalidCursorReturns400(t *testing.T) {
	env := newTestEnvWithStore(t)
	admin := seedUser(t, env.pool, service.RoleSuperAdmin, "Roster Cursor Admin")
	token := mintTokenForEnv(t, env, admin.String(), service.RoleSuperAdmin)
	examID := seedExam(t, env.pool, "Roster Cursor Exam", false, "hidden", "classic")

	rec := getRequest(t, env.e, "/api/v1/admin/exams/"+examID.String()+"/registrations?limit=20&cursor=broken", token)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("want 400, got %d body=%s", rec.Code, rec.Body.String())
	}
	var apiErr struct {
		Code string `json:"code"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &apiErr); err != nil {
		t.Fatalf("decode error: %v", err)
	}
	if apiErr.Code != "invalid_cursor" {
		t.Fatalf("want invalid_cursor, got %q", apiErr.Code)
	}
}

func TestAdminExportExamRegistrations_IsCompleteScopedAndFormulaSafe(t *testing.T) {
	env := newTestEnvWithStore(t)
	ctx := context.Background()
	schoolA := seedSchool(t, env.pool)
	schoolB := seedSchool(t, env.pool)
	admin := seedUser(t, env.pool, service.RoleAdminSchool, "Roster Export Admin")
	token := mintSchoolTokenForEnv(t, env, admin.String(), service.RoleAdminSchool, schoolA)
	examID := seedExam(t, env.pool, "Roster Export Exam", false, "hidden", "classic")

	for i := 0; i < 23; i++ {
		name := "Export Student"
		if i == 0 {
			name = "=HYPERLINK(\"bad\")"
		}
		student := seedUser(t, env.pool, service.RoleStudent, name)
		if _, err := env.pool.Exec(ctx, `UPDATE users SET school_id = $1 WHERE id = $2`, schoolA, student); err != nil {
			t.Fatalf("set school: %v", err)
		}
		seedRegistration(t, env.pool, student, examID)
	}
	other := seedUser(t, env.pool, service.RoleStudent, "Other School Student")
	if _, err := env.pool.Exec(ctx, `UPDATE users SET school_id = $1 WHERE id = $2`, schoolB, other); err != nil {
		t.Fatalf("set other school: %v", err)
	}
	seedRegistration(t, env.pool, other, examID)

	rec := getRequest(t, env.e, "/api/v1/admin/exams/"+examID.String()+"/registrations/export", token)
	if rec.Code != http.StatusOK {
		t.Fatalf("want 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Header().Get("Content-Type"), "text/csv") {
		t.Fatalf("content type: got %q", rec.Header().Get("Content-Type"))
	}
	records, err := csv.NewReader(strings.NewReader(rec.Body.String())).ReadAll()
	if err != nil {
		t.Fatalf("parse CSV: %v", err)
	}
	if len(records) != 24 {
		t.Fatalf("CSV rows including header: got %d, want 24", len(records))
	}
	if got := records[0]; len(got) != 5 || got[0] != "No. Peserta" || got[1] != "Nama" {
		t.Fatalf("unexpected header: %v", got)
	}
	foundSafe := false
	for _, row := range records[1:] {
		if row[1] == "Other School Student" {
			t.Fatal("export leaked another school's student")
		}
		if row[1] == "'=HYPERLINK(\"bad\")" {
			foundSafe = true
		}
	}
	if !foundSafe {
		t.Fatal("formula-like student name was not neutralized")
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
