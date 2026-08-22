package handler_test

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"
)

func TestAdminAssessment_DBBackedRoutesAndRBAC(t *testing.T) {
	env := newAdminResultsDBEnv(t)

	schoolID := seedSchool(t, env.pool)
	superID := seedUserWithSchool(t, env.pool, "super_admin", "Assessment Super", schoolID)
	adminExamID := seedUserWithSchool(t, env.pool, "admin_exam", "Assessment Exam Admin", schoolID)
	adminSchoolID := seedUserWithSchool(t, env.pool, "admin_school", "Assessment School Admin", schoolID)
	examID := seedExamWithMCQ(t, env.pool)
	studentID := seedUserWithSchool(t, env.pool, "student", "Assessment Student", schoolID)
	seedSubmittedSession(t, env.pool, studentID, examID)

	superToken := mintSuperAdminToken(t, env, superID.String())
	adminExamToken := mintAdminExamToken(t, env, adminExamID.String())
	adminSchoolToken := mintAdminToken(t, env, adminSchoolID.String(), schoolID)

	for _, tc := range []struct {
		name  string
		token string
	}{
		{"admin_exam forbidden", adminExamToken},
		{"admin_school forbidden", adminSchoolToken},
	} {
		t.Run(tc.name, func(t *testing.T) {
			rec := getRequest(t, env.e, "/api/v1/admin/exams/"+examID.String()+"/assessment", tc.token)
			if rec.Code != http.StatusForbidden {
				t.Fatalf("want 403, got %d body=%s", rec.Code, rec.Body.String())
			}
		})
	}

	t.Run("super_admin list 200 from real DB", func(t *testing.T) {
		rec := getRequest(t, env.e, "/api/v1/admin/exams/"+examID.String()+"/assessment?limit=1", superToken)
		if rec.Code != http.StatusOK {
			t.Fatalf("want 200, got %d body=%s", rec.Code, rec.Body.String())
		}
		var resp struct {
			Summary struct {
				TotalRegistered       int     `json:"total_registered"`
				CompletedParticipants int     `json:"completed_participants"`
				CompletionRate        float64 `json:"completion_rate"`
			} `json:"summary"`
			Data []struct {
				RegistrationID string   `json:"registration_id"`
				StudentName    string   `json:"student_name"`
				Status         string   `json:"status"`
				Score          *float64 `json:"score"`
				Rank           *int     `json:"rank"`
			} `json:"data"`
		}
		if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
			t.Fatalf("decode: %v", err)
		}
		if resp.Summary.TotalRegistered != 1 || resp.Summary.CompletedParticipants != 1 || resp.Summary.CompletionRate != 1 {
			t.Fatalf("summary mismatch: %+v", resp.Summary)
		}
		if len(resp.Data) != 1 || resp.Data[0].StudentName != "Assessment Student" || resp.Data[0].Status != "completed" || resp.Data[0].Score == nil || resp.Data[0].Rank == nil {
			t.Fatalf("row mismatch: %+v", resp.Data)
		}
	})

	t.Run("malformed cursor maps to validation error", func(t *testing.T) {
		rec := getRequest(t, env.e, "/api/v1/admin/exams/"+examID.String()+"/assessment?cursor=garbage", superToken)
		if rec.Code != http.StatusUnprocessableEntity {
			t.Fatalf("want 422, got %d body=%s", rec.Code, rec.Body.String())
		}
		var apiErr struct {
			Code string `json:"code"`
		}
		if err := json.Unmarshal(rec.Body.Bytes(), &apiErr); err != nil {
			t.Fatalf("decode error: %v", err)
		}
		if apiErr.Code != "validation_failed" {
			t.Fatalf("want validation_failed, got %q body=%s", apiErr.Code, rec.Body.String())
		}
	})

	t.Run("attempts validates registration belongs to exam", func(t *testing.T) {
		otherExamID := seedExamWithMCQ(t, env.pool)
		otherStudentID := seedUserWithSchool(t, env.pool, "student", "Other Assessment Student", schoolID)
		seedSubmittedSession(t, env.pool, otherStudentID, otherExamID)
		var otherRegistrationID string
		if err := env.pool.QueryRow(
			context.Background(),
			`SELECT id FROM exam_registration WHERE exam_id = $1 AND student_id = $2 LIMIT 1`,
			otherExamID, otherStudentID,
		).Scan(&otherRegistrationID); err != nil {
			t.Fatalf("lookup other registration: %v", err)
		}

		rec := getRequest(t, env.e, "/api/v1/admin/exams/"+examID.String()+"/assessment/"+otherRegistrationID+"/attempts", superToken)
		if rec.Code != http.StatusNotFound {
			t.Fatalf("want 404 for mismatched registration/exam, got %d body=%s", rec.Code, rec.Body.String())
		}
	})

	t.Run("malformed ids return 400", func(t *testing.T) {
		if rec := getRequest(t, env.e, "/api/v1/admin/exams/not-a-uuid/assessment", superToken); rec.Code != http.StatusBadRequest {
			t.Fatalf("bad exam id: want 400, got %d body=%s", rec.Code, rec.Body.String())
		}
		if rec := getRequest(t, env.e, "/api/v1/admin/exams/"+examID.String()+"/assessment/not-a-uuid/attempts", superToken); rec.Code != http.StatusBadRequest {
			t.Fatalf("bad registration id: want 400, got %d body=%s", rec.Code, rec.Body.String())
		}
	})
}
