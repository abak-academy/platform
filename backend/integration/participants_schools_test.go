package integration_test

import (
	"context"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// seedE4School inserts an active school row and returns its id.
func seedE4School(t *testing.T, env *testEnv, name, code string, types []string) string {
	t.Helper()
	var id string
	err := env.pool.QueryRow(context.Background(),
		`INSERT INTO school (name, code, npsn, school_types, alamat, status)
		 VALUES ($1, $2, $3, $4, $5, 'active') RETURNING id`,
		name, code, "20999999", types, "Jl. Contoh No. 1",
	).Scan(&id)
	require.NoError(t, err)
	return id
}

// seedE4Student inserts an active student. schoolID nil leaves users.school_id
// NULL, which is the FB-12 shape: the student typed a school name that is not
// in the registry and it lives in unlisted_school_name instead.
func seedE4Student(t *testing.T, env *testEnv, name string, schoolID, unlistedSchoolName *string, jenjang string) string {
	t.Helper()
	// bcrypt of "password123", same fixed hash seedUser uses.
	const passwordHash = "$2a$10$N9qo8uLOickgx2ZMRZoMyeIjZAgcfl7p92ldGxad68LJZdL17lhWy"
	email := fmt.Sprintf("e4-%d@example.test", time.Now().UnixNano())
	var id string
	err := env.pool.QueryRow(context.Background(),
		`INSERT INTO users (email, password_hash, role, name, status, otp_enabled, school_id, unlisted_school_name, jenjang)
		 VALUES ($1, $2, 'student', $3, 'active', false, $4, $5, $6) RETURNING id`,
		email, passwordHash, name, schoolID, unlistedSchoolName, jenjang,
	).Scan(&id)
	require.NoError(t, err)
	return id
}

// readSchoolColumns reads the three school-related user columns straight out of
// Postgres. Every FB-14 assertion below goes through this rather than trusting
// the response body the service just built.
func readSchoolColumns(t *testing.T, env *testEnv, userID string) (schoolID, unlisted, jenjang *string) {
	t.Helper()
	err := env.pool.QueryRow(context.Background(),
		`SELECT school_id::text, unlisted_school_name, jenjang FROM users WHERE id = $1`, userID,
	).Scan(&schoolID, &unlisted, &jenjang)
	require.NoError(t, err)
	return schoolID, unlisted, jenjang
}

// TestNoSchoolStudentIsAFirstClassParticipant drives the real grant-picker
// handler against a real database: the FB-12 backend half (nullable school on
// the wire, the school_id=none facet) and FR-8's regression claim that a
// school-less student can still be granted an exam and appear on the roster.
func TestNoSchoolStudentIsAFirstClassParticipant(t *testing.T) {
	env := newTestEnv(t)

	suffix := fmt.Sprintf("E4x%d", time.Now().UnixNano())
	schoolID := seedE4School(t, env, "SMAN Terdaftar "+suffix, "sman-"+suffix, []string{"SMA"})

	listedStudent := seedE4Student(t, env, "Bagas Terdaftar "+suffix, &schoolID, nil, "SMA")
	unlistedName := "SMA Swasta Belum Terdaftar"
	noSchoolStudent := seedE4Student(t, env, "Citra Tanpa Sekolah "+suffix, nil, &unlistedName, "SMA")

	superAdmin := seedUser(t, env, "super_admin", "active", false)
	superToken := authToken(t, env, superAdmin, "super_admin")

	// rowsByID indexes a search response body by student id.
	rowsByID := func(t *testing.T, body map[string]any) map[string]map[string]any {
		t.Helper()
		data, ok := body["data"].([]any)
		require.True(t, ok, "expected data array, got: %v", body)
		out := map[string]map[string]any{}
		for _, r := range data {
			row, ok := r.(map[string]any)
			require.True(t, ok, "expected object rows, got: %T", r)
			out[row["id"].(string)] = row
		}
		return out
	}

	t.Run("unfiltered search returns both, with null school fields on the no-school row", func(t *testing.T) {
		resp := env.doJSON(t, http.MethodGet, "/api/v1/admin/exam-grants/students/search?q="+suffix, nil, superToken)
		body := decodeBody(t, resp)
		require.Equal(t, http.StatusOK, resp.StatusCode, "body: %v", body)

		rows := rowsByID(t, body)
		require.Contains(t, rows, noSchoolStudent, "FR-1: the LEFT JOIN must not drop a NULL-school student")
		require.Contains(t, rows, listedStudent)

		// FR-3 on the real wire: the keys are present and null, not absent.
		noSchool := rows[noSchoolStudent]
		sid, ok := noSchool["school_id"]
		require.True(t, ok, "school_id key must be present on the wire: %v", noSchool)
		assert.Nil(t, sid)
		sname, ok := noSchool["school_name"]
		require.True(t, ok, "school_name key must be present on the wire: %v", noSchool)
		assert.Nil(t, sname)
		assert.Equal(t, unlistedName, noSchool["unlisted_school_name"])

		listed := rows[listedStudent]
		assert.Equal(t, schoolID, listed["school_id"])
		assert.Equal(t, "SMAN Terdaftar "+suffix, listed["school_name"])
		uname, ok := listed["unlisted_school_name"]
		require.True(t, ok, "unlisted_school_name key must be present on the wire: %v", listed)
		assert.Nil(t, uname)
	})

	t.Run("school_id=none selects the no-school student and excludes school-bearing ones", func(t *testing.T) {
		resp := env.doJSON(t, http.MethodGet, "/api/v1/admin/exam-grants/students/search?q="+suffix+"&school_id=none", nil, superToken)
		body := decodeBody(t, resp)
		require.Equal(t, http.StatusOK, resp.StatusCode, "body: %v", body)

		rows := rowsByID(t, body)
		assert.Contains(t, rows, noSchoolStudent)
		assert.NotContains(t, rows, listedStudent, "FR-4: none must be a real IS NULL predicate")
	})

	t.Run("granting the no-school student succeeds and lands them on the roster", func(t *testing.T) {
		examID := seedExam(t, env, "Ujian Lintas Sekolah "+suffix, "score_only", nil)

		grantResp := env.doJSON(t, http.MethodPost, "/api/v1/admin/exam-grants", map[string]any{
			"exam_id":     examID,
			"student_ids": []string{noSchoolStudent},
		}, superToken)
		grantBody := decodeBody(t, grantResp)
		require.Equal(t, http.StatusCreated, grantResp.StatusCode, "body: %v", grantBody)

		rosterResp := env.doJSON(t, http.MethodGet, "/api/v1/admin/exams/"+examID+"/registrations", nil, superToken)
		rosterBody := decodeBody(t, rosterResp)
		require.Equal(t, http.StatusOK, rosterResp.StatusCode, "body: %v", rosterBody)

		entries, ok := rosterBody["data"].([]any)
		require.True(t, ok, "expected roster data array, got: %v", rosterBody)
		found := false
		for _, e := range entries {
			row := e.(map[string]any)
			if row["student_id"] == noSchoolStudent {
				found = true
				assert.Equal(t, "Citra Tanpa Sekolah "+suffix, row["student_name"])
				// FB-28: a grant-path registration with a NULL participant_number
				// kills certificate generation for that student.
				assert.NotNil(t, row["participant_number"], "grant must allocate a participant_number")
			}
		}
		assert.True(t, found, "FR-8: a school-less student must appear on the roster after a grant")
	})
}

// TestProfileSchoolRoundTripAgainstDB is FR-13: listed -> unlisted -> listed,
// including the jenjang the old school does not offer. Every claim is read back
// out of the users table, because the response body is built by the same call
// that is under test.
func TestProfileSchoolRoundTripAgainstDB(t *testing.T) {
	env := newTestEnv(t)

	suffix := fmt.Sprintf("E4r%d", time.Now().UnixNano())
	// SMA-only school: the differing-jenjang case needs a school that would
	// reject "SMK" if the old school were still being validated against.
	smaSchool := seedE4School(t, env, "SMAN Asal "+suffix, "smanasal-"+suffix, []string{"SMA"})

	student := seedE4Student(t, env, "Dewi Pindah "+suffix, &smaSchool, nil, "SMA")
	token := authToken(t, env, student, "student")

	const unlistedName = "SMK Bina Mandiri Belum Terdaftar"

	t.Run("listed to unlisted with a jenjang the old school does not offer", func(t *testing.T) {
		// Exactly the body the profile form sends for the unlisted intent
		// (web/app/(student)/profile/page.tsx).
		resp := env.doJSON(t, http.MethodPatch, "/api/v1/students/profile", map[string]any{
			"school_id":            "",
			"unlisted_school_name": unlistedName,
			"jenjang":              "SMK",
		}, token)
		body := decodeBody(t, resp)
		require.Equal(t, http.StatusOK, resp.StatusCode, "body: %v", body)

		dbSchoolID, dbUnlisted, dbJenjang := readSchoolColumns(t, env, student)
		assert.Nil(t, dbSchoolID, "users.school_id must actually be cleared, not left by COALESCE")
		require.NotNil(t, dbUnlisted)
		assert.Equal(t, unlistedName, *dbUnlisted)
		require.NotNil(t, dbJenjang)
		assert.Equal(t, "SMK", *dbJenjang)

		got := decodeBody(t, env.doJSON(t, http.MethodGet, "/api/v1/students/profile", nil, token))
		sid, ok := got["school_id"]
		require.True(t, ok, "school_id key must be present on the wire: %v", got)
		assert.Nil(t, sid)
		assert.Equal(t, unlistedName, got["unlisted_school_name"])
		assert.Equal(t, "SMK", got["jenjang"])
	})

	t.Run("back to the listed school clears the free-text name", func(t *testing.T) {
		resp := env.doJSON(t, http.MethodPatch, "/api/v1/students/profile", map[string]any{
			"school_id":            smaSchool,
			"unlisted_school_name": "",
			"jenjang":              "SMA",
		}, token)
		body := decodeBody(t, resp)
		require.Equal(t, http.StatusOK, resp.StatusCode, "body: %v", body)

		dbSchoolID, dbUnlisted, dbJenjang := readSchoolColumns(t, env, student)
		require.NotNil(t, dbSchoolID)
		assert.Equal(t, smaSchool, *dbSchoolID)
		assert.Nil(t, dbUnlisted, "invariant 3: the two school columns are never both set")
		require.NotNil(t, dbJenjang)
		assert.Equal(t, "SMA", *dbJenjang)
	})

	t.Run("a submission mentioning neither field leaves both columns alone", func(t *testing.T) {
		resp := env.doJSON(t, http.MethodPatch, "/api/v1/students/profile", map[string]any{
			"name": "Dewi Pindah Baru " + suffix,
		}, token)
		body := decodeBody(t, resp)
		require.Equal(t, http.StatusOK, resp.StatusCode, "body: %v", body)

		dbSchoolID, dbUnlisted, _ := readSchoolColumns(t, env, student)
		require.NotNil(t, dbSchoolID)
		assert.Equal(t, smaSchool, *dbSchoolID)
		assert.Nil(t, dbUnlisted)
		assert.Equal(t, "Dewi Pindah Baru "+suffix, body["name"])
	})
}
