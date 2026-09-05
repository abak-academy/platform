package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"akademi-bimbel/internal/model"
	"akademi-bimbel/internal/repository"
)

func TestSubmitSession_RollsBackOnFinalizationStepFailures(t *testing.T) {
	cases := []struct {
		name    string
		table   string
		event   string
		message string
		when    func(uuid.UUID, uuid.UUID) string
	}{
		{
			name:    "graded upsert",
			table:   "exam_session_answer",
			event:   "INSERT OR UPDATE",
			message: "forced graded upsert failure",
			when: func(sessionID, _ uuid.UUID) string {
				return fmt.Sprintf("WHEN (NEW.session_id = '%s'::uuid)", sessionID)
			},
		},
		{
			name:    "terminal CAS",
			table:   "exam_session",
			event:   "UPDATE OF status",
			message: "forced session CAS failure",
			when: func(sessionID, _ uuid.UUID) string {
				return fmt.Sprintf("WHEN (NEW.id = '%s'::uuid AND NEW.status = 'submitted')", sessionID)
			},
		},
		{
			name:    "registration update",
			table:   "exam_registration",
			event:   "UPDATE OF status",
			message: "forced registration failure",
			when: func(_, registrationID uuid.UUID) string {
				return fmt.Sprintf("WHEN (NEW.id = '%s'::uuid AND NEW.status = 'submitted')", registrationID)
			},
		},
		{
			name:    "outbox insert",
			table:   "outbox",
			event:   "INSERT",
			message: "forced outbox failure",
			when: func(sessionID, _ uuid.UUID) string {
				return fmt.Sprintf("WHEN (NEW.aggregate_id = '%s'::uuid AND NEW.event_type = 'CertificateNeeded')", sessionID)
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
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
			installFailingTrigger(t, repo, tc.table, tc.event, tc.when(sessionID, registrationIDForSession(t, repo, sessionID)), tc.message)

			_, err := svc.ForceSubmitSession(ctx, sessionID.String())
			if err == nil || !strings.Contains(err.Error(), tc.message) {
				t.Fatalf("ForceSubmitSession error = %v, want %q", err, tc.message)
			}

			assertFinalizationRolledBack(t, repo, sessionID, questionID)
		})
	}
}

func TestSubmitSession_CompetingFinalizersCreateOneOutbox(t *testing.T) {
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

	var wg sync.WaitGroup
	errs := make(chan error, 2)
	wg.Add(2)
	go func() {
		defer wg.Done()
		_, err := svc.ForceSubmitSession(ctx, sessionID.String())
		errs <- err
	}()
	go func() {
		defer wg.Done()
		_, err := svc.ForceSubmitSession(ctx, sessionID.String())
		errs <- err
	}()
	wg.Wait()
	close(errs)

	var successes, alreadySubmitted int
	for err := range errs {
		switch {
		case err == nil:
			successes++
		case errors.Is(err, ErrAlreadySubmitted):
			alreadySubmitted++
		default:
			t.Fatalf("unexpected competing finalizer error: %v", err)
		}
	}
	if successes != 1 || alreadySubmitted != 1 {
		t.Fatalf("competing finalizers = %d success/%d already-submitted, want 1/1", successes, alreadySubmitted)
	}
	if got := countCertificateNeeded(t, repo, sessionID); got != 1 {
		t.Fatalf("CertificateNeeded rows = %d, want 1", got)
	}
}

func TestSubmitSession_WithSingleConnectionPoolDoesNotWaitForSecondConnection(t *testing.T) {
	_, sharedRepo := newRealDBService(t)
	ctx := context.Background()
	config := sharedRepo.Pool().Config().Copy()
	config.MaxConns = 1
	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		t.Fatalf("new single-connection pool: %v", err)
	}
	t.Cleanup(pool.Close)

	repo := repository.New(pool)
	svc := &Service{storeRepo: repo}
	sessionID, _ := seedServiceExpiryStandard(t, repo, time.Now().UTC().Truncate(time.Second))

	submitCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	if _, err := svc.ForceSubmitSession(submitCtx, sessionID.String()); err != nil {
		t.Fatalf("ForceSubmitSession with one DB connection: %v", err)
	}
}

func TestExpireExamCandidate_RevalidationNoOpsForTerminalRace(t *testing.T) {
	svc, repo := newRealDBService(t)
	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Second)
	sessionID, _ := seedServiceExpiryStandard(t, repo, now)
	if _, err := repo.Pool().Exec(ctx, `UPDATE exam_session SET status = 'submitted' WHERE id = $1`, sessionID); err != nil {
		t.Fatalf("mark session submitted: %v", err)
	}

	processed, err := svc.expireExamCandidate(ctx, now, model.ExamExpiryCandidate{
		SessionID: sessionID,
		DueAt:     now.Add(-time.Minute),
	})
	if err != nil {
		t.Fatalf("expireExamCandidate: %v", err)
	}
	if processed {
		t.Fatal("processed = true, want no-op after terminal revalidation")
	}
	if got := countCertificateNeeded(t, repo, sessionID); got != 0 {
		t.Fatalf("CertificateNeeded rows = %d, want 0", got)
	}
}

func TestExpireExamSessions_RollsBackSectionPromotionFailure(t *testing.T) {
	svc, repo := newRealDBService(t)
	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Second)
	sessionID, firstTestID, secondTestID := seedServiceExpirySectioned(t, repo, now)
	installFailingTrigger(t, repo, "exam_session_section", "UPDATE OF started_at", fmt.Sprintf("WHEN (NEW.session_id = '%s'::uuid AND NEW.status = 'active')", sessionID), "forced section promotion failure")

	results, err := svc.ExpireExamSessions(ctx, now, 5)
	if err != nil {
		t.Fatalf("ExpireExamSessions: %v", err)
	}
	if !expiryResultFailedWith(results, sessionID, "forced section promotion failure") {
		t.Fatalf("expiry results = %#v, want section promotion failure", results)
	}

	assertSectionStatus(t, repo, sessionID, firstTestID, "active")
	assertSectionStatus(t, repo, sessionID, secondTestID, "pending")
	assertSessionAndRegistrationStatus(t, repo, sessionID, "in_progress", "in_progress")
}

func TestExpireExamSessions_RollsBackFinalSectionFailure(t *testing.T) {
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
	installFailingTrigger(t, repo, "exam_session", "UPDATE OF status", fmt.Sprintf("WHEN (NEW.id = '%s'::uuid AND NEW.status = 'submitted')", sessionID), "forced finalization failure")

	results, err := svc.ExpireExamSessions(ctx, now, 5)
	if err != nil {
		t.Fatalf("ExpireExamSessions: %v", err)
	}
	if !expiryResultFailedWith(results, sessionID, "forced finalization failure") {
		t.Fatalf("expiry results = %#v, want finalization failure", results)
	}

	assertSectionStatus(t, repo, sessionID, testID, "active")
	assertFinalizationRolledBack(t, repo, sessionID, questionID)
}

func TestExpireExamSessions_ContinuesAfterCandidateError(t *testing.T) {
	svc, repo := newRealDBService(t)
	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Second)
	failedSessionID, failedQuestionID := seedServiceExpiryStandard(t, repo, now.Add(-time.Minute))
	okSessionID, okQuestionID := seedServiceExpiryStandard(t, repo, now)
	for _, item := range []struct {
		sessionID  uuid.UUID
		questionID uuid.UUID
	}{
		{failedSessionID, failedQuestionID},
		{okSessionID, okQuestionID},
	} {
		answer := "a"
		if err := repo.SaveAnswersTx(ctx, item.sessionID, []model.ExamSessionAnswer{{
			QuestionID: item.questionID,
			Answer:     &answer,
		}}, nil); err != nil {
			t.Fatalf("SaveAnswersTx %s: %v", item.sessionID, err)
		}
	}
	installFailingTrigger(t, repo, "exam_session", "UPDATE OF status", fmt.Sprintf("WHEN (NEW.id = '%s'::uuid AND NEW.status = 'submitted')", failedSessionID), "forced one candidate failure")

	results, err := svc.ExpireExamSessions(ctx, now, 5)
	if err != nil {
		t.Fatalf("ExpireExamSessions: %v", err)
	}

	var sawFailure, sawSuccess bool
	for _, res := range results {
		if res.Candidate.SessionID == failedSessionID && res.Err != nil {
			sawFailure = true
		}
		if res.Candidate.SessionID == okSessionID && res.Err == nil && res.Processed {
			sawSuccess = true
		}
	}
	if !sawFailure || !sawSuccess {
		t.Fatalf("results = %#v, want one failed candidate and one later success", results)
	}
	assertSessionAndRegistrationStatus(t, repo, failedSessionID, "in_progress", "in_progress")
	assertSessionAndRegistrationStatus(t, repo, okSessionID, "submitted", "submitted")
	if got := countCertificateNeeded(t, repo, failedSessionID); got != 0 {
		t.Fatalf("failed session CertificateNeeded rows = %d, want 0", got)
	}
	if got := countCertificateNeeded(t, repo, okSessionID); got != 1 {
		t.Fatalf("ok session CertificateNeeded rows = %d, want 1", got)
	}
}

func TestExpireExamSessions_DrainsPastFailedFullPage(t *testing.T) {
	svc, repo := newRealDBService(t)
	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Second)
	failedSessionID, _ := seedServiceExpiryStandard(t, repo, now.Add(-2*time.Minute))
	firstOKSessionID, _ := seedServiceExpiryStandard(t, repo, now.Add(-time.Minute))
	secondOKSessionID, _ := seedServiceExpiryStandard(t, repo, now)
	installFailingTrigger(t, repo, "exam_session", "UPDATE OF status", fmt.Sprintf("WHEN (NEW.id = '%s'::uuid AND NEW.status = 'submitted')", failedSessionID), "forced poison candidate")

	results, err := svc.ExpireExamSessions(ctx, now, 2)
	if err != nil {
		t.Fatalf("ExpireExamSessions: %v", err)
	}
	if !expiryResultFailedWith(results, failedSessionID, "forced poison candidate") {
		t.Fatalf("results = %#v, want poison candidate failure", results)
	}
	if !expiryResultSucceeded(results, firstOKSessionID) || !expiryResultSucceeded(results, secondOKSessionID) {
		t.Fatalf("results = %#v, want both later candidates processed", results)
	}
	assertSessionAndRegistrationStatus(t, repo, failedSessionID, "in_progress", "in_progress")
	assertSessionAndRegistrationStatus(t, repo, firstOKSessionID, "submitted", "submitted")
	assertSessionAndRegistrationStatus(t, repo, secondOKSessionID, "submitted", "submitted")
}

func installFailingTrigger(t *testing.T, repo *repository.Repository, table, event, when, message string) {
	t.Helper()
	ctx := context.Background()
	name := "fail_" + strings.ReplaceAll(uuid.NewString(), "-", "")
	if _, err := repo.Pool().Exec(ctx, fmt.Sprintf(
		`CREATE FUNCTION %s() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION '%s'; END; $$`,
		name, message,
	)); err != nil {
		t.Fatalf("create trigger function: %v", err)
	}
	if _, err := repo.Pool().Exec(ctx, fmt.Sprintf(
		`CREATE TRIGGER %s BEFORE %s ON %s FOR EACH ROW %s EXECUTE FUNCTION %s()`,
		name, event, table, when, name,
	)); err != nil {
		t.Fatalf("create trigger: %v", err)
	}
	t.Cleanup(func() {
		_, _ = repo.Pool().Exec(context.Background(), fmt.Sprintf(`DROP TRIGGER IF EXISTS %s ON %s`, name, table))
		_, _ = repo.Pool().Exec(context.Background(), fmt.Sprintf(`DROP FUNCTION IF EXISTS %s()`, name))
	})
}

func expiryResultFailedWith(results []ExamExpiryResult, sessionID uuid.UUID, message string) bool {
	for _, res := range results {
		if res.Candidate.SessionID == sessionID && res.Err != nil && strings.Contains(res.Err.Error(), message) {
			return true
		}
	}
	return false
}

func registrationIDForSession(t *testing.T, repo *repository.Repository, sessionID uuid.UUID) uuid.UUID {
	t.Helper()
	var registrationID uuid.UUID
	if err := repo.Pool().QueryRow(context.Background(),
		`SELECT registration_id FROM exam_session WHERE id = $1`,
		sessionID,
	).Scan(&registrationID); err != nil {
		t.Fatalf("query registration id: %v", err)
	}
	return registrationID
}

func assertFinalizationRolledBack(t *testing.T, repo *repository.Repository, sessionID, questionID uuid.UUID) {
	t.Helper()
	assertSessionAndRegistrationStatus(t, repo, sessionID, "in_progress", "in_progress")
	if got := countCertificateNeeded(t, repo, sessionID); got != 0 {
		t.Fatalf("CertificateNeeded rows = %d, want 0", got)
	}

	var answer string
	var score *float64
	var gradedAt *time.Time
	if err := repo.Pool().QueryRow(context.Background(),
		`SELECT answer, score, graded_at FROM exam_session_answer WHERE session_id = $1 AND question_id = $2`,
		sessionID, questionID,
	).Scan(&answer, &score, &gradedAt); err != nil {
		t.Fatalf("query answer: %v", err)
	}
	if answer != "a" || score != nil || gradedAt != nil {
		t.Fatalf("answer state = answer:%q score:%v graded_at:%v, want original ungraded answer", answer, score, gradedAt)
	}
}

func assertSessionAndRegistrationStatus(t *testing.T, repo *repository.Repository, sessionID uuid.UUID, wantSession, wantRegistration string) {
	t.Helper()
	var status, regStatus string
	if err := repo.Pool().QueryRow(context.Background(),
		`SELECT s.status, r.status
		FROM exam_session s
		JOIN exam_registration r ON r.id = s.registration_id
		WHERE s.id = $1`,
		sessionID,
	).Scan(&status, &regStatus); err != nil {
		t.Fatalf("query status: %v", err)
	}
	if status != wantSession || regStatus != wantRegistration {
		t.Fatalf("status = session:%q registration:%q, want %q/%q", status, regStatus, wantSession, wantRegistration)
	}
}

func assertSectionStatus(t *testing.T, repo *repository.Repository, sessionID, testID uuid.UUID, want string) {
	t.Helper()
	var status string
	if err := repo.Pool().QueryRow(context.Background(),
		`SELECT status FROM exam_session_section WHERE session_id = $1 AND test_id = $2`,
		sessionID, testID,
	).Scan(&status); err != nil {
		t.Fatalf("query section: %v", err)
	}
	if status != want {
		t.Fatalf("section %s status = %q, want %q", testID, status, want)
	}
}

func countCertificateNeeded(t *testing.T, repo *repository.Repository, sessionID uuid.UUID) int {
	t.Helper()
	var count int
	if err := repo.Pool().QueryRow(context.Background(),
		`SELECT count(*) FROM outbox WHERE aggregate_id = $1 AND event_type = 'CertificateNeeded'`,
		sessionID,
	).Scan(&count); err != nil {
		t.Fatalf("query CertificateNeeded count: %v", err)
	}
	return count
}
