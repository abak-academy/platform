package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"akademi-bimbel/internal/model"
	"akademi-bimbel/internal/repository"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func seedRevokeActor(t *testing.T, repo *repository.Repository) uuid.UUID {
	t.Helper()
	var actorID uuid.UUID
	err := repo.Pool().QueryRow(context.Background(),
		`INSERT INTO users (name, role, status, username, password_hash)
		 VALUES ($1, 'super_admin', 'active', $2, '')
		 RETURNING id`,
		"Revoke Actor "+uniqueSuffix(), "rv_actor_"+uniqueSuffix()[:5],
	).Scan(&actorID)
	require.NoError(t, err)
	return actorID
}

func seedRevokeStudent(t *testing.T, repo *repository.Repository, prefix string) uuid.UUID {
	t.Helper()
	var studentID uuid.UUID
	err := repo.Pool().QueryRow(context.Background(),
		`INSERT INTO users (name, role, status, username, password_hash, jenjang)
		 VALUES ($1, 'student', 'active', $2, '', 'sma')
		 RETURNING id`,
		"Revoke Student "+uniqueSuffix(), prefix+uniqueSuffix()[:6],
	).Scan(&studentID)
	require.NoError(t, err)
	return studentID
}

func seedRevokeExam(t *testing.T, repo *repository.Repository) uuid.UUID {
	t.Helper()
	var examID uuid.UUID
	err := repo.Pool().QueryRow(context.Background(),
		`INSERT INTO exam (title, status, timer_mode, result_config, mode)
		 VALUES ($1, 'active', 'manual', 'score_only', 'standard')
		 RETURNING id`,
		"Revoke Exam "+uniqueSuffix(),
	).Scan(&examID)
	require.NoError(t, err)
	return examID
}

func registrationStatus(t *testing.T, repo *repository.Repository, examID, studentID uuid.UUID) (string, *time.Time) {
	t.Helper()
	var status string
	var revokedAt *time.Time
	err := repo.Pool().QueryRow(context.Background(),
		`SELECT status, revoked_at FROM exam_registration WHERE exam_id = $1 AND student_id = $2`,
		examID, studentID,
	).Scan(&status, &revokedAt)
	require.NoError(t, err)
	return status, revokedAt
}

func TestRevokeExamAccess_Integration(t *testing.T) {
	svc, repo := newRealDBService(t)
	ctx := context.Background()

	actorID := seedRevokeActor(t, repo)
	examID := seedRevokeExam(t, repo)

	studentIDs := make([]uuid.UUID, 3)
	for i := range studentIDs {
		studentIDs[i] = seedRevokeStudent(t, repo, "rv_stu_")
	}

	if _, err := svc.GrantExamAccess(ctx, actorID.String(), examID.String(), studentIDs); err != nil {
		t.Fatalf("GrantExamAccess: %v", err)
	}

	actorStr := actorID.String()

	t.Run("revoke stamps status/revoked_at/revoked_by and writes audit", func(t *testing.T) {
		result, err := svc.RevokeExamAccess(ctx, actorStr, examID.String(), []uuid.UUID{studentIDs[0]})
		require.NoError(t, err)
		require.Equal(t, 1, result.RevokedCount)
		require.Len(t, result.Results, 1)
		require.Equal(t, "revoked", result.Results[0].Status)
		require.Equal(t, studentIDs[0].String(), result.Results[0].StudentID)

		status, revokedAt := registrationStatus(t, repo, examID, studentIDs[0])
		require.Equal(t, "revoked", status)
		require.NotNil(t, revokedAt)

		var auditCount int
		require.NoError(t, repo.Pool().QueryRow(ctx,
			`SELECT COUNT(*) FROM audit_log WHERE actor_id = $1 AND action = 'exam_grant.revoke'`,
			actorID,
		).Scan(&auditCount))
		require.Equal(t, 1, auditCount)
	})

	t.Run("mixed batch — skipped for already revoked, failed for unregistered", func(t *testing.T) {
		unregistered := seedRevokeStudent(t, repo, "rv_none_")

		result, err := svc.RevokeExamAccess(ctx, actorStr, examID.String(), []uuid.UUID{studentIDs[0], studentIDs[1], unregistered})
		require.NoError(t, err)
		require.Equal(t, 1, result.RevokedCount)

		byID := map[string]ExamRevokeRowResult{}
		for _, r := range result.Results {
			byID[r.StudentID] = r
		}
		require.Equal(t, "skipped", byID[studentIDs[0].String()].Status)
		require.Equal(t, "already revoked", byID[studentIDs[0].String()].Message)
		require.Equal(t, "revoked", byID[studentIDs[1].String()].Status)
		require.Equal(t, "failed", byID[unregistered.String()].Status)
		require.Equal(t, "not registered for this exam", byID[unregistered.String()].Message)

		status, _ := registrationStatus(t, repo, examID, studentIDs[1])
		require.Equal(t, "revoked", status)
	})

	t.Run("in-progress session blocks revoke until submitted", func(t *testing.T) {
		blocked := studentIDs[2]
		var regID uuid.UUID
		require.NoError(t, repo.Pool().QueryRow(ctx,
			`SELECT id FROM exam_registration WHERE exam_id = $1 AND student_id = $2`,
			examID, blocked,
		).Scan(&regID))
		_, err := repo.Pool().Exec(ctx,
			`INSERT INTO exam_session (registration_id, student_id, exam_id, attempt_number, started_at, status)
			 VALUES ($1, $2, $3, 1, now(), 'in_progress')`,
			regID, blocked, examID,
		)
		require.NoError(t, err)

		result, err := svc.RevokeExamAccess(ctx, actorStr, examID.String(), []uuid.UUID{blocked})
		require.NoError(t, err)
		require.Equal(t, "failed", result.Results[0].Status)
		require.Equal(t, "has an in-progress session: force-submit first", result.Results[0].Message)
		status, _ := registrationStatus(t, repo, examID, blocked)
		require.Equal(t, "registered", status)

		_, err = repo.Pool().Exec(ctx,
			`UPDATE exam_session SET status = 'submitted', submitted_at = now() WHERE registration_id = $1`,
			regID,
		)
		require.NoError(t, err)

		result, err = svc.RevokeExamAccess(ctx, actorStr, examID.String(), []uuid.UUID{blocked})
		require.NoError(t, err)
		require.Equal(t, "revoked", result.Results[0].Status)
	})
}

func TestRegrantReactivatesRevokedRegistration(t *testing.T) {
	svc, repo := newRealDBService(t)
	ctx := context.Background()

	actorID := seedRevokeActor(t, repo)
	examID := seedRevokeExam(t, repo)
	studentID := seedRevokeStudent(t, repo, "rv_re_")
	actorStr := actorID.String()

	_, err := svc.GrantExamAccess(ctx, actorStr, examID.String(), []uuid.UUID{studentID})
	require.NoError(t, err)

	var originalNumber *int
	require.NoError(t, repo.Pool().QueryRow(ctx,
		`SELECT participant_number FROM exam_registration WHERE exam_id = $1 AND student_id = $2`,
		examID, studentID,
	).Scan(&originalNumber))
	require.NotNil(t, originalNumber)

	_, err = svc.RevokeExamAccess(ctx, actorStr, examID.String(), []uuid.UUID{studentID})
	require.NoError(t, err)
	status, _ := registrationStatus(t, repo, examID, studentID)
	require.Equal(t, "revoked", status)

	grant, err := svc.GrantExamAccess(ctx, actorStr, examID.String(), []uuid.UUID{studentID})
	require.NoError(t, err)
	require.Equal(t, 1, grant.GrantedCount)

	status, revokedAt := registrationStatus(t, repo, examID, studentID)
	require.Equal(t, "registered", status)
	require.Nil(t, revokedAt)

	var nowNumber int
	require.NoError(t, repo.Pool().QueryRow(ctx,
		`SELECT participant_number FROM exam_registration WHERE exam_id = $1 AND student_id = $2`,
		examID, studentID,
	).Scan(&nowNumber))
	require.Equal(t, *originalNumber, nowNumber)
}

func TestCreateExamRegistration_RevivesRevokedRowAndStaysIdempotent(t *testing.T) {
	_, repo := newRealDBService(t)
	ctx := context.Background()

	examID := seedRevokeExam(t, repo)
	studentID := seedRevokeStudent(t, repo, "rv_ob_")

	newReg := func() model.ExamRegistration {
		return model.ExamRegistration{
			StudentID: studentID,
			ExamID:    examID,
			Token:     "TOK" + uniqueSuffix(),
			Status:    "registered",
		}
	}

	tx, err := repo.BeginTx(ctx)
	require.NoError(t, err)
	require.NoError(t, repo.CreateExamRegistration(ctx, tx, newReg()))
	require.NoError(t, tx.Commit(ctx))

	var firstToken string
	var firstNumber int
	require.NoError(t, repo.Pool().QueryRow(ctx,
		`SELECT token, participant_number FROM exam_registration WHERE exam_id = $1 AND student_id = $2`,
		examID, studentID,
	).Scan(&firstToken, &firstNumber))

	_, err = repo.Pool().Exec(ctx,
		`UPDATE exam_registration SET status = 'revoked', revoked_at = now() WHERE exam_id = $1 AND student_id = $2`,
		examID, studentID,
	)
	require.NoError(t, err)

	tx, err = repo.BeginTx(ctx)
	require.NoError(t, err)
	require.NoError(t, repo.CreateExamRegistration(ctx, tx, newReg()))
	require.NoError(t, tx.Commit(ctx))

	var status string
	var revokedAt *time.Time
	var newToken string
	var sameNumber int
	require.NoError(t, repo.Pool().QueryRow(ctx,
		`SELECT status, revoked_at, token, participant_number FROM exam_registration WHERE exam_id = $1 AND student_id = $2`,
		examID, studentID,
	).Scan(&status, &revokedAt, &newToken, &sameNumber))
	require.Equal(t, "registered", status)
	require.Nil(t, revokedAt)
	require.NotEqual(t, firstToken, newToken)
	require.Equal(t, firstNumber, sameNumber)

	tx, err = repo.BeginTx(ctx)
	require.NoError(t, err)
	require.NoError(t, repo.CreateExamRegistration(ctx, tx, newReg()))
	require.NoError(t, tx.Commit(ctx))
	var tokenAfterRedelivery string
	require.NoError(t, repo.Pool().QueryRow(ctx,
		`SELECT token FROM exam_registration WHERE exam_id = $1 AND student_id = $2`,
		examID, studentID,
	).Scan(&tokenAfterRedelivery))
	require.Equal(t, newToken, tokenAfterRedelivery)
}

func TestRefundOrder_RevokesExamRegistrations(t *testing.T) {
	svc, repo := newRealDBService(t)
	ctx := context.Background()

	actorID := seedRevokeActor(t, repo)
	examID := seedRevokeExam(t, repo)
	buyer := seedRevokeStudent(t, repo, "rv_buyer_")
	participant := seedRevokeStudent(t, repo, "rv_part_")

	var productID string
	require.NoError(t, repo.Pool().QueryRow(ctx,
		`INSERT INTO product (type, name, price, stock, status) VALUES ('exam', $1, 100000, 0, 'published') RETURNING id`,
		"Refund Product "+uniqueSuffix(),
	).Scan(&productID))
	_, err := repo.Pool().Exec(ctx, `INSERT INTO product_exam (product_id, exam_id) VALUES ($1, $2)`, productID, examID)
	require.NoError(t, err)

	if _, err := svc.GrantExamAccess(ctx, actorID.String(), examID.String(), []uuid.UUID{buyer, participant}); err != nil {
		t.Fatalf("GrantExamAccess: %v", err)
	}

	order, _, err := svc.MintCart(ctx, buyer.String())
	require.NoError(t, err)
	require.NoError(t, svc.AddItem(ctx, buyer.String(), order.ID.String(), productID, 1))
	_, err = repo.Pool().Exec(ctx,
		`INSERT INTO order_participant (order_id, student_id) VALUES ($1, $2)`,
		order.ID, participant,
	)
	require.NoError(t, err)
	_, err = repo.Pool().Exec(ctx, `UPDATE orders SET status = 'paid' WHERE id = $1`, order.ID)
	require.NoError(t, err)

	require.NoError(t, svc.AdminRefundOrder(ctx, actorID.String(), order.ID.String(), "refund_proof/"+uniqueSuffix()+".png"))

	status, _ := registrationStatus(t, repo, examID, participant)
	require.Equal(t, "revoked", status)
	status, _ = registrationStatus(t, repo, examID, buyer)
	require.Equal(t, "registered", status)
}

func TestRefundOrder_SelfPurchase_RevokesBuyer(t *testing.T) {
	svc, repo := newRealDBService(t)
	ctx := context.Background()

	actorID := seedRevokeActor(t, repo)
	examID := seedRevokeExam(t, repo)
	buyer := seedRevokeStudent(t, repo, "rv_self_")

	var productID string
	require.NoError(t, repo.Pool().QueryRow(ctx,
		`INSERT INTO product (type, name, price, stock, status) VALUES ('exam', $1, 100000, 0, 'published') RETURNING id`,
		"Self Refund Product "+uniqueSuffix(),
	).Scan(&productID))
	_, err := repo.Pool().Exec(ctx, `INSERT INTO product_exam (product_id, exam_id) VALUES ($1, $2)`, productID, examID)
	require.NoError(t, err)

	if _, err := svc.GrantExamAccess(ctx, actorID.String(), examID.String(), []uuid.UUID{buyer}); err != nil {
		t.Fatalf("GrantExamAccess: %v", err)
	}

	order, _, err := svc.MintCart(ctx, buyer.String())
	require.NoError(t, err)
	require.NoError(t, svc.AddItem(ctx, buyer.String(), order.ID.String(), productID, 1))
	_, err = repo.Pool().Exec(ctx, `UPDATE orders SET status = 'paid' WHERE id = $1`, order.ID)
	require.NoError(t, err)

	require.NoError(t, svc.AdminRefundOrder(ctx, actorID.String(), order.ID.String(), "refund_proof/"+uniqueSuffix()+".png"))

	status, _ := registrationStatus(t, repo, examID, buyer)
	require.Equal(t, "revoked", status)
}

func TestRevokedRegistration_ExcludedFromStudentListCountAndEligibility(t *testing.T) {
	svc, repo := newRealDBService(t)
	ctx := context.Background()

	actorID := seedRevokeActor(t, repo)
	examID := seedRevokeExam(t, repo)
	revoked := seedRevokeStudent(t, repo, "rv_ex_1_")
	live := seedRevokeStudent(t, repo, "rv_ex_2_")
	actorStr := actorID.String()

	if _, err := svc.GrantExamAccess(ctx, actorStr, examID.String(), []uuid.UUID{revoked, live}); err != nil {
		t.Fatalf("GrantExamAccess: %v", err)
	}
	if _, err := svc.RevokeExamAccess(ctx, actorStr, examID.String(), []uuid.UUID{revoked}); err != nil {
		t.Fatalf("RevokeExamAccess: %v", err)
	}

	t.Run("student list hides revoked", func(t *testing.T) {
		items, err := svc.GetExamRegistrations(ctx, revoked.String())
		require.NoError(t, err)
		require.Empty(t, items)
		items, err = svc.GetExamRegistrations(ctx, live.String())
		require.NoError(t, err)
		require.Len(t, items, 1)
	})

	t.Run("registration_count excludes revoked", func(t *testing.T) {
		exams, _, err := svc.ListExams(ctx, repository.ExamFilter{})
		require.NoError(t, err)
		var count *int
		for i := range exams {
			if exams[i].ID == examID {
				count = &exams[i].RegistrationCount
				break
			}
		}
		require.NotNil(t, count, "exam must appear in the admin list")
		require.Equal(t, 1, *count)
	})

	t.Run("bulk-order eligibility counts revoked as eligible", func(t *testing.T) {
		already, err := repo.FilterAlreadyRegistered(ctx, examID, []uuid.UUID{revoked, live})
		require.NoError(t, err)
		require.Equal(t, []uuid.UUID{live}, already)
	})

	t.Run("grant search offers revoked student again", func(t *testing.T) {
		rows, _, err := svc.SearchStudentsAcrossSchools(ctx, "", nil, false, nil, "", 50, "", examID.String())
		require.NoError(t, err)
		ids := map[string]bool{}
		for _, r := range rows {
			ids[r.ID] = true
		}
		require.True(t, ids[revoked.String()], "revoked student must reappear as grantable")
		require.False(t, ids[live.String()], "live student must stay filtered out")
	})
}

func TestCheckInAndStartSession_RejectRevokedRegistration(t *testing.T) {
	svc, repo := newRealDBService(t)
	ctx := context.Background()

	actorID := seedRevokeActor(t, repo)
	examID := seedRevokeExam(t, repo)
	studentID := seedRevokeStudent(t, repo, "rv_gate_")

	if _, err := svc.GrantExamAccess(ctx, actorID.String(), examID.String(), []uuid.UUID{studentID}); err != nil {
		t.Fatalf("GrantExamAccess: %v", err)
	}
	if _, err := svc.RevokeExamAccess(ctx, actorID.String(), examID.String(), []uuid.UUID{studentID}); err != nil {
		t.Fatalf("RevokeExamAccess: %v", err)
	}

	var token, regID string
	require.NoError(t, repo.Pool().QueryRow(ctx,
		`SELECT token, id FROM exam_registration WHERE exam_id = $1 AND student_id = $2`,
		examID, studentID,
	).Scan(&token, &regID))

	_, err := svc.CheckIn(ctx, studentID.String(), token, "fp-revoke-test")
	require.True(t, errors.Is(err, ErrRegistrationRevoked), "CheckIn: want ErrRegistrationRevoked, got %v", err)

	_, err = svc.StartSession(ctx, studentID.String(), regID, "fp-revoke-test")
	require.True(t, errors.Is(err, ErrRegistrationRevoked), "StartSession: want ErrRegistrationRevoked, got %v", err)

	_, err = svc.GetExamRegistration(ctx, regID, studentID.String())
	require.True(t, errors.Is(err, ErrRegistrationRevoked), "GetExamRegistration: want ErrRegistrationRevoked, got %v", err)
}

func TestRevokeExamAccessBulk_Integration(t *testing.T) {
	svc, repo := newRealDBService(t)
	ctx := context.Background()

	actorID := seedRevokeActor(t, repo)
	examID := seedRevokeExam(t, repo)

	usernames := []string{"rvb_" + uniqueSuffix(), "rvb_" + uniqueSuffix(), "rvb_" + uniqueSuffix()}
	studentIDs := make([]uuid.UUID, len(usernames))
	for i, username := range usernames {
		var studentID uuid.UUID
		require.NoError(t, repo.Pool().QueryRow(ctx,
			`INSERT INTO users (name, role, status, username, password_hash, jenjang)
			 VALUES ($1, 'student', 'active', $2, '', 'sma')
			 RETURNING id`,
			"Bulk Revoke Student "+uniqueSuffix(), username,
		).Scan(&studentID))
		studentIDs[i] = studentID
	}

	if _, err := svc.GrantExamAccess(ctx, actorID.String(), examID.String(), studentIDs); err != nil {
		t.Fatalf("GrantExamAccess: %v", err)
	}

	results, err := svc.RevokeExamAccessBulk(ctx, actorID.String(), examID.String(),
		[]string{"", usernames[0], usernames[0], "nope_" + uniqueSuffix()[:8]})
	require.NoError(t, err)
	require.Len(t, results, 4)
	require.Equal(t, "failed", results[0].Status)
	require.Equal(t, "blank username", results[0].Message)
	require.Equal(t, "revoked", results[1].Status)
	require.Equal(t, "skipped", results[2].Status)
	require.Equal(t, "duplicate username in file", results[2].Message)
	require.Equal(t, "failed", results[3].Status)
	require.Equal(t, "username not found", results[3].Message)

	status, _ := registrationStatus(t, repo, examID, studentIDs[0])
	require.Equal(t, "revoked", status)
	status, _ = registrationStatus(t, repo, examID, studentIDs[1])
	require.Equal(t, "registered", status)

	// Second pass: usernames[0] is already revoked, usernames[1] revokes now.
	results, err = svc.RevokeExamAccessBulk(ctx, actorID.String(), examID.String(), usernames[:2])
	require.NoError(t, err)
	require.Len(t, results, 2)
	require.Equal(t, "skipped", results[0].Status)
	require.Equal(t, "revoked", results[1].Status)

	// Third pass: everything is revoked → all skipped.
	results, err = svc.RevokeExamAccessBulk(ctx, actorID.String(), examID.String(), usernames[:2])
	require.NoError(t, err)
	require.Len(t, results, 2)
	require.Equal(t, "skipped", results[0].Status)
	require.Equal(t, "skipped", results[1].Status)
}
