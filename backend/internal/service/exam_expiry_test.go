package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	"akademi-bimbel/internal/model"
	"akademi-bimbel/internal/repository"
)

func TestExpiryPageMadeProgress(t *testing.T) {
	tests := []struct {
		name    string
		results []ExamExpiryResult
		want    bool
	}{
		{name: "empty page", want: false},
		{name: "only no-ops", results: []ExamExpiryResult{{}}, want: false},
		{name: "processed candidate", results: []ExamExpiryResult{{Processed: true}}, want: true},
		{name: "skipped failed candidate", results: []ExamExpiryResult{{Err: errors.New("failed")}}, want: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := expiryPageMadeProgress(tt.results); got != tt.want {
				t.Fatalf("expiryPageMadeProgress() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestExpireExamSessions_NonPositivePageSizeReturnsEmpty(t *testing.T) {
	svc := &Service{}
	results, err := svc.ExpireExamSessions(context.Background(), time.Now(), 0)
	if err != nil {
		t.Fatalf("ExpireExamSessions: %v", err)
	}
	if len(results) != 0 {
		t.Fatalf("results = %#v, want empty", results)
	}
}

func TestExpireExamSessions_StandardFinalizesThroughSharedSubmit(t *testing.T) {
	svc, repo := newRealDBService(t)
	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Second)

	sessionID, questionID := seedServiceExpiryStandard(t, repo, now)
	answer := "a"
	if err := repo.SaveAnswersTx(ctx, sessionID, []model.ExamSessionAnswer{{
		QuestionID: questionID,
		Answer:     &answer,
	}}, nil); err != nil {
		t.Fatalf("SaveAnswersTx: %v", err)
	}

	results, err := svc.ExpireExamSessions(ctx, now, 5)
	if err != nil {
		t.Fatalf("ExpireExamSessions: %v", err)
	}
	if !expiryResultSucceeded(results, sessionID) {
		t.Fatalf("expiry results did not include successful session %s: %#v", sessionID, results)
	}

	var status, regStatus string
	var score float64
	if err := repo.Pool().QueryRow(ctx,
		`SELECT s.status, COALESCE(s.score, -1), r.status
		FROM exam_session s
		JOIN exam_registration r ON r.id = s.registration_id
		WHERE s.id = $1`,
		sessionID,
	).Scan(&status, &score, &regStatus); err != nil {
		t.Fatalf("query finalized session: %v", err)
	}
	if status != "submitted" || regStatus != "submitted" || score != 4 {
		t.Fatalf("finalized state = status:%q reg:%q score:%v, want submitted/submitted/4", status, regStatus, score)
	}
	var outboxCount int
	if err := repo.Pool().QueryRow(ctx,
		`SELECT count(*) FROM outbox WHERE aggregate_id = $1 AND event_type = 'CertificateNeeded'`,
		sessionID,
	).Scan(&outboxCount); err != nil {
		t.Fatalf("query outbox: %v", err)
	}
	if outboxCount != 1 {
		t.Fatalf("CertificateNeeded rows = %d, want 1", outboxCount)
	}
}

func TestExpireExamSessions_ExpiredNonFinalSectionAdvancesOnce(t *testing.T) {
	svc, repo := newRealDBService(t)
	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Second)

	sessionID, firstTestID, secondTestID := seedServiceExpirySectioned(t, repo, now)
	results, err := svc.ExpireExamSessions(ctx, now, 5)
	if err != nil {
		t.Fatalf("ExpireExamSessions: %v", err)
	}
	if !expiryResultSucceeded(results, sessionID) {
		t.Fatalf("expiry results did not include successful section session %s: %#v", sessionID, results)
	}

	rows, err := repo.Pool().Query(ctx,
		`SELECT test_id, status FROM exam_session_section WHERE session_id = $1 ORDER BY sort_order`,
		sessionID,
	)
	if err != nil {
		t.Fatalf("query sections: %v", err)
	}
	defer rows.Close()
	statuses := map[uuid.UUID]string{}
	for rows.Next() {
		var testID uuid.UUID
		var status string
		if err := rows.Scan(&testID, &status); err != nil {
			t.Fatalf("scan section: %v", err)
		}
		statuses[testID] = status
	}
	if statuses[firstTestID] != "submitted" || statuses[secondTestID] != "active" {
		t.Fatalf("section statuses = %#v, want first submitted and second active", statuses)
	}

	results, err = svc.ExpireExamSessions(ctx, now, 5)
	if err != nil {
		t.Fatalf("second ExpireExamSessions: %v", err)
	}
	if expiryResultSucceeded(results, sessionID) {
		t.Fatalf("second expiry should be idempotent no-op for no-longer-due section: %#v", results)
	}
}

func TestExpireExamSessions_FinalSectionFinalizesSession(t *testing.T) {
	svc, repo := newRealDBService(t)
	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Second)

	sessionID, testID, questionID := seedServiceExpiryFinalSection(t, repo, now)
	answer := "a"
	if err := repo.SaveAnswersTx(ctx, sessionID, []model.ExamSessionAnswer{{
		QuestionID: questionID,
		Answer:     &answer,
	}}, nil); err != nil {
		t.Fatalf("SaveAnswersTx: %v", err)
	}

	results, err := svc.ExpireExamSessions(ctx, now, 5)
	if err != nil {
		t.Fatalf("ExpireExamSessions: %v", err)
	}
	if !expiryResultSucceeded(results, sessionID) {
		t.Fatalf("expiry results did not include successful final section %s: %#v", sessionID, results)
	}

	var sessionStatus, sectionStatus string
	var score float64
	if err := repo.Pool().QueryRow(ctx,
		`SELECT s.status, COALESCE(s.score, -1), ss.status
		FROM exam_session s
		JOIN exam_session_section ss ON ss.session_id = s.id
		WHERE s.id = $1 AND ss.test_id = $2`,
		sessionID, testID,
	).Scan(&sessionStatus, &score, &sectionStatus); err != nil {
		t.Fatalf("query final section state: %v", err)
	}
	if sessionStatus != "submitted" || sectionStatus != "submitted" || score != 4 {
		t.Fatalf("final section state = session:%q section:%q score:%v, want submitted/submitted/4", sessionStatus, sectionStatus, score)
	}
}

func expiryResultSucceeded(results []ExamExpiryResult, sessionID uuid.UUID) bool {
	for _, res := range results {
		if res.Candidate.SessionID == sessionID && res.Err == nil && res.Processed {
			return true
		}
	}
	return false
}

func seedServiceExpiryFinalSection(t *testing.T, repo *repository.Repository, now time.Time) (uuid.UUID, uuid.UUID, uuid.UUID) {
	t.Helper()
	ctx := context.Background()
	studentID := seedServiceExpiryStudent(t, repo, "final")
	testID, questionID := seedServiceExpiryMCQ(t, repo, "final-section")
	grace := 5
	exam := &model.Exam{
		Title:              "Expiry IELTS Final " + uniqueSuffix(),
		Mode:               "ielts",
		TimerMode:          "per_test",
		GraceWindowMinutes: &grace,
		ResultConfig:       "hidden",
	}
	if err := repo.CreateExam(ctx, exam); err != nil {
		t.Fatalf("CreateExam: %v", err)
	}
	if _, err := repo.Pool().Exec(ctx,
		`INSERT INTO exam_test (exam_id, test_id, sort_order) VALUES ($1, $2, 1)`,
		exam.ID, testID,
	); err != nil {
		t.Fatalf("insert final section exam_test: %v", err)
	}
	regID := seedServiceExpiryRegistration(t, repo, studentID, exam.ID)
	sessionID := seedServiceExpirySession(t, repo, regID, studentID, exam.ID, now.Add(-10*time.Minute), nil)
	if _, err := repo.Pool().Exec(ctx,
		`INSERT INTO exam_session_section
			(session_id, test_id, sort_order, duration_minutes, status, started_at)
		VALUES ($1, $2, 1, 20, 'active', $3)`,
		sessionID, testID, now.Add(-30*time.Minute),
	); err != nil {
		t.Fatalf("insert final session section: %v", err)
	}
	return sessionID, testID, questionID
}

func seedServiceExpiryStandard(t *testing.T, repo *repository.Repository, now time.Time) (uuid.UUID, uuid.UUID) {
	t.Helper()
	ctx := context.Background()
	studentID := seedServiceExpiryStudent(t, repo, "std")
	testID, questionID := seedServiceExpiryMCQ(t, repo, "standard")
	duration := 30
	grace := 5
	exam := &model.Exam{
		Title:              "Expiry Standard " + uniqueSuffix(),
		Mode:               "standard",
		TimerMode:          "overall",
		DurationMinutes:    &duration,
		GraceWindowMinutes: &grace,
		ResultConfig:       "hidden",
	}
	if err := repo.CreateExam(ctx, exam); err != nil {
		t.Fatalf("CreateExam: %v", err)
	}
	if _, err := repo.Pool().Exec(ctx,
		`INSERT INTO exam_test (exam_id, test_id, sort_order) VALUES ($1, $2, 1)`,
		exam.ID, testID,
	); err != nil {
		t.Fatalf("insert exam_test: %v", err)
	}
	regID := seedServiceExpiryRegistration(t, repo, studentID, exam.ID)
	return seedServiceExpirySession(t, repo, regID, studentID, exam.ID, now.Add(-45*time.Minute), nil), questionID
}

func seedServiceExpirySectioned(t *testing.T, repo *repository.Repository, now time.Time) (uuid.UUID, uuid.UUID, uuid.UUID) {
	t.Helper()
	ctx := context.Background()
	studentID := seedServiceExpiryStudent(t, repo, "sec")
	firstTestID, _ := seedServiceExpiryMCQ(t, repo, "section-one")
	secondTestID, _ := seedServiceExpiryMCQ(t, repo, "section-two")
	grace := 5
	exam := &model.Exam{
		Title:              "Expiry IELTS " + uniqueSuffix(),
		Mode:               "ielts",
		TimerMode:          "per_test",
		GraceWindowMinutes: &grace,
		ResultConfig:       "hidden",
	}
	if err := repo.CreateExam(ctx, exam); err != nil {
		t.Fatalf("CreateExam: %v", err)
	}
	if _, err := repo.Pool().Exec(ctx,
		`INSERT INTO exam_test (exam_id, test_id, sort_order) VALUES ($1, $2, 1), ($1, $3, 2)`,
		exam.ID, firstTestID, secondTestID,
	); err != nil {
		t.Fatalf("insert sectioned exam_test: %v", err)
	}
	regID := seedServiceExpiryRegistration(t, repo, studentID, exam.ID)
	sessionID := seedServiceExpirySession(t, repo, regID, studentID, exam.ID, now.Add(-10*time.Minute), nil)
	if _, err := repo.Pool().Exec(ctx,
		`INSERT INTO exam_session_section
			(session_id, test_id, sort_order, duration_minutes, status, started_at)
		VALUES ($1, $2, 1, 20, 'active', $4),
		       ($1, $3, 2, 20, 'pending', NULL)`,
		sessionID, firstTestID, secondTestID, now.Add(-30*time.Minute),
	); err != nil {
		t.Fatalf("insert service expiry sections: %v", err)
	}
	return sessionID, firstTestID, secondTestID
}

func seedServiceExpiryStudent(t *testing.T, repo *repository.Repository, prefix string) uuid.UUID {
	t.Helper()
	ctx := context.Background()
	var studentID uuid.UUID
	suffix := uniqueSuffix()
	if err := repo.Pool().QueryRow(ctx,
		`INSERT INTO users (name, username, jenjang, role, status, auth_provider)
		VALUES ($1, $2, 'sd', 'student', 'active', 'password')
		RETURNING id`,
		"Expiry "+prefix+" "+suffix, "exp_"+prefix+"_"+suffix,
	).Scan(&studentID); err != nil {
		t.Fatalf("insert expiry student: %v", err)
	}
	return studentID
}

func seedServiceExpiryMCQ(t *testing.T, repo *repository.Repository, prefix string) (uuid.UUID, uuid.UUID) {
	t.Helper()
	ctx := context.Background()
	test := &model.Test{Title: "Expiry Test " + prefix + " " + uniqueSuffix(), Subject: "math", Topic: "expiry", DurationMinutes: 20}
	if err := repo.CreateTest(ctx, test); err != nil {
		t.Fatalf("CreateTest: %v", err)
	}
	var questionID uuid.UUID
	if err := repo.Pool().QueryRow(ctx,
		`INSERT INTO question (format, body, correct_answer, point_correct, point_wrong)
		VALUES ('mcq', $1, 'a', 4, 0)
		RETURNING id`,
		"question "+prefix,
	).Scan(&questionID); err != nil {
		t.Fatalf("insert expiry question: %v", err)
	}
	if _, err := repo.Pool().Exec(ctx,
		`INSERT INTO question_option (question_id, key, text, is_correct, sort_order)
		VALUES ($1, 'a', 'A', true, 1), ($1, 'b', 'B', false, 2)`,
		questionID,
	); err != nil {
		t.Fatalf("insert expiry options: %v", err)
	}
	if _, err := repo.Pool().Exec(ctx,
		`INSERT INTO test_question (test_id, question_id, sort_order) VALUES ($1, $2, 1)`,
		test.ID, questionID,
	); err != nil {
		t.Fatalf("insert expiry test_question: %v", err)
	}
	return test.ID, questionID
}

func seedServiceExpiryRegistration(t *testing.T, repo *repository.Repository, studentID, examID uuid.UUID) uuid.UUID {
	t.Helper()
	ctx := context.Background()
	var regID uuid.UUID
	if err := repo.Pool().QueryRow(ctx,
		`INSERT INTO exam_registration (student_id, exam_id, token, status)
		VALUES ($1, $2, $3, 'in_progress')
		RETURNING id`,
		studentID, examID, uuid.NewString(),
	).Scan(&regID); err != nil {
		t.Fatalf("insert expiry registration: %v", err)
	}
	return regID
}

func seedServiceExpirySession(t *testing.T, repo *repository.Repository, regID, studentID, examID uuid.UUID, startedAt time.Time, extendedUntil *time.Time) uuid.UUID {
	t.Helper()
	ctx := context.Background()
	var sessionID uuid.UUID
	if err := repo.Pool().QueryRow(ctx,
		`INSERT INTO exam_session (registration_id, student_id, exam_id, started_at, extended_until, status)
		VALUES ($1, $2, $3, $4, $5, 'in_progress')
		RETURNING id`,
		regID, studentID, examID, startedAt, extendedUntil,
	).Scan(&sessionID); err != nil {
		t.Fatalf("insert expiry session: %v", err)
	}
	return sessionID
}
