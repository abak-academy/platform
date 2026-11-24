package handler_test

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"akademi-bimbel/internal/service"
	"github.com/google/uuid"
)

// setSchoolLocation sets a school's provinsi_id/kota_id directly — seedSchool
// leaves both NULL, and these tests need real seeded region ids (see
// db/migrations/0029_seed_province_city_district.up.sql) to exercise the
// registrant-diversity filter.
func setSchoolLocation(t *testing.T, env *testEnvWithStore, schoolID, provinsiID, kotaID string) {
	t.Helper()
	if _, err := env.pool.Exec(context.Background(),
		`UPDATE school SET provinsi_id = $1, kota_id = $2 WHERE id = $3`,
		provinsiID, kotaID, schoolID,
	); err != nil {
		t.Fatalf("set school location: %v", err)
	}
}

// Opsi A's whole point is that the exam list never pays for a
// COUNT(DISTINCT …) per row unless the admin actually asked for diversity
// numbers — so the endpoint must reject a request that supplies none of
// province_id/city_id/school_id rather than silently computing an unscoped
// (expensive) result.
func TestAdminListExamRegistrantDiversity_NoFilterReturns400(t *testing.T) {
	env := newTestEnvWithStore(t)
	admin := seedUser(t, env.pool, service.RoleSuperAdmin, "Diversity Admin")
	token := mintTokenForEnv(t, env, admin.String(), service.RoleSuperAdmin)

	rec := getRequest(t, env.e, "/api/v1/admin/exams/registrant-diversity", token)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("want 400, got %d body=%s", rec.Code, rec.Body.String())
	}
}

func TestAdminListExamRegistrantDiversity_InvalidSchoolIDReturns400(t *testing.T) {
	env := newTestEnvWithStore(t)
	admin := seedUser(t, env.pool, service.RoleSuperAdmin, "Diversity Admin")
	token := mintTokenForEnv(t, env, admin.String(), service.RoleSuperAdmin)

	rec := getRequest(t, env.e, "/api/v1/admin/exams/registrant-diversity?school_id=not-a-uuid", token)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("want 400, got %d body=%s", rec.Code, rec.Body.String())
	}
}

// With a province_id filter, the endpoint must return, per exam, the
// registrant count and distinct province/city/school counts scoped to that
// province — excluding registrants from a school outside it.
func TestAdminListExamRegistrantDiversity_ReturnsPerExamCountsScopedToFilter(t *testing.T) {
	env := newTestEnvWithStore(t)
	ctx := context.Background()

	schoolIn := seedSchool(t, env.pool)
	setSchoolLocation(t, env, schoolIn, "11", "1101")
	schoolOut := seedSchool(t, env.pool)
	setSchoolLocation(t, env, schoolOut, "12", "1201")

	admin := seedUser(t, env.pool, service.RoleSuperAdmin, "Diversity Admin")
	token := mintTokenForEnv(t, env, admin.String(), service.RoleSuperAdmin)

	examID := seedExam(t, env.pool, "Registrant Diversity Exam", false, "hidden", "classic")

	inStudent := seedUser(t, env.pool, service.RoleStudent, "In Province Student")
	outStudent := seedUser(t, env.pool, service.RoleStudent, "Out Of Province Student")
	for id, school := range map[uuid.UUID]string{inStudent: schoolIn, outStudent: schoolOut} {
		if _, err := env.pool.Exec(ctx, `UPDATE users SET school_id = $1 WHERE id = $2`, school, id); err != nil {
			t.Fatalf("set user school: %v", err)
		}
	}
	seedRegistration(t, env.pool, inStudent, examID)
	seedRegistration(t, env.pool, outStudent, examID)

	rec := getRequest(t, env.e, "/api/v1/admin/exams/registrant-diversity?province_id=11", token)
	if rec.Code != http.StatusOK {
		t.Fatalf("want 200, got %d body=%s", rec.Code, rec.Body.String())
	}

	var resp struct {
		Data []struct {
			ExamID            string `json:"exam_id"`
			RegistrationCount int    `json:"registration_count"`
			ProvinceCount     int    `json:"province_count"`
			CityCount         int    `json:"city_count"`
			SchoolCount       int    `json:"school_count"`
		} `json:"data"`
		NextCursor string `json:"next_cursor"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(resp.Data) != 1 {
		t.Fatalf("want 1 exam row, got %d body=%s", len(resp.Data), rec.Body.String())
	}
	row := resp.Data[0]
	if row.ExamID != examID.String() {
		t.Fatalf("exam_id: want %s, got %s", examID.String(), row.ExamID)
	}
	if row.RegistrationCount != 1 {
		t.Fatalf("registration_count: want 1 (province-11 student only), got %d", row.RegistrationCount)
	}
	if row.ProvinceCount != 1 || row.CityCount != 1 || row.SchoolCount != 1 {
		t.Fatalf("diversity counts: want 1/1/1, got %d/%d/%d", row.ProvinceCount, row.CityCount, row.SchoolCount)
	}
}

// Feature 1/Opsi B: a province_id filter on the counts endpoint (the stat
// cards) and the roster endpoint (the table) must agree — proves the
// province_id/city_id/school_id query params this task adds to both
// handlers are actually wired through, not just supported at the repo layer.
func TestAdminExamRegistrationCountsAndRoster_ProvinceFilterStayConsistent(t *testing.T) {
	env := newTestEnvWithStore(t)
	ctx := context.Background()

	schoolIn := seedSchool(t, env.pool)
	setSchoolLocation(t, env, schoolIn, "11", "1101")
	schoolOut := seedSchool(t, env.pool)
	setSchoolLocation(t, env, schoolOut, "12", "1201")

	admin := seedUser(t, env.pool, service.RoleSuperAdmin, "Province Filter Admin")
	token := mintTokenForEnv(t, env, admin.String(), service.RoleSuperAdmin)

	examID := seedExam(t, env.pool, "Province Filter Exam", false, "hidden", "classic")

	inStudent := seedUser(t, env.pool, service.RoleStudent, "In Province Student")
	outStudent := seedUser(t, env.pool, service.RoleStudent, "Out Of Province Student")
	for id, school := range map[uuid.UUID]string{inStudent: schoolIn, outStudent: schoolOut} {
		if _, err := env.pool.Exec(ctx, `UPDATE users SET school_id = $1 WHERE id = $2`, school, id); err != nil {
			t.Fatalf("set user school: %v", err)
		}
	}
	seedRegistration(t, env.pool, inStudent, examID)
	seedRegistration(t, env.pool, outStudent, examID)

	countsRec := getRequest(t, env.e, "/api/v1/admin/exams/"+examID.String()+"/registrations/counts?province_id=11", token)
	if countsRec.Code != http.StatusOK {
		t.Fatalf("counts: want 200, got %d body=%s", countsRec.Code, countsRec.Body.String())
	}
	var counts map[string]float64
	if err := json.Unmarshal(countsRec.Body.Bytes(), &counts); err != nil {
		t.Fatalf("decode counts: %v", err)
	}
	if counts["total"] != 1 || counts["province_count"] != 1 || counts["city_count"] != 1 || counts["school_count"] != 1 {
		t.Fatalf("filtered counts: got %v", counts)
	}

	rosterRec := getRequest(t, env.e, "/api/v1/admin/exams/"+examID.String()+"/registrations?province_id=11", token)
	if rosterRec.Code != http.StatusOK {
		t.Fatalf("roster: want 200, got %d body=%s", rosterRec.Code, rosterRec.Body.String())
	}
	rosterData := decodeRosterData(t, rosterRec.Body.Bytes())
	if len(rosterData) != 1 {
		t.Fatalf("filtered roster: want 1 row (matching the filtered counts' total), got %d", len(rosterData))
	}
	if got := rosterData[0].(map[string]any)["student_id"]; got != inStudent.String() {
		t.Fatalf("filtered roster student_id: want %s, got %v", inStudent.String(), got)
	}
}
