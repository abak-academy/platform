package service

import (
	"context"
	"errors"
	"fmt"

	"akademi-bimbel/internal/model"
	"akademi-bimbel/internal/repository"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// ErrInvalidGrantStudent is returned when a student_id in the grant request
// does not exist or is not a student (FR-GRANT-06).
var ErrInvalidGrantStudent = errors.New("one or more student IDs are invalid or not students")

type GrantedStudent struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Username string `json:"username"`
}

type GrantExamAccessResult struct {
	GrantedCount    int              `json:"granted_count"`
	GrantedStudents []GrantedStudent `json:"granted_students"`
}

// GrantExamAccess creates direct exam registrations for the given students,
// bypassing the order pipeline entirely (FR-GRANT-01/02). No school-scoping
// check is performed — super_admin has none.
//
// Already-registered students are silently skipped via ON CONFLICT DO NOTHING
// (FR-GRANT-03). Returns only newly created registrations along with their
// student metadata.
func (s *Service) GrantExamAccess(ctx context.Context, actorID, examID string, studentIDs []uuid.UUID) (GrantExamAccessResult, error) {
	examUUID, err := uuid.Parse(examID)
	if err != nil {
		return GrantExamAccessResult{}, err
	}

	// Batch-validate existence + role='student' (FR-GRANT-01, FR-GRANT-06, NFR-07).
	users, err := s.repo.GetUsersByIDs(ctx, studentIDs)
	if err != nil {
		return GrantExamAccessResult{}, err
	}

	// Every requested id must be present (FR-GRANT-06).
	if len(users) != len(studentIDs) {
		return GrantExamAccessResult{}, ErrInvalidGrantStudent
	}

	// Collect distinct school_ids for audit metadata (FR-GRANT-04).
	schoolSet := make(map[string]bool)
	for _, u := range users {
		if u.SchoolID != nil {
			schoolSet[*u.SchoolID] = true
		}
	}
	schoolIDs := make([]string, 0, len(schoolSet))
	for sid := range schoolSet {
		schoolIDs = append(schoolIDs, sid)
	}

	// Convert studentIDs to strings for audit meta.
	studentIDStrs := make([]string, len(studentIDs))
	for i, id := range studentIDs {
		studentIDStrs[i] = id.String()
	}

	tx, err := s.storeRepo.BeginTx(ctx)
	if err != nil {
		return GrantExamAccessResult{}, err
	}
	defer tx.Rollback(ctx)

	var registrations []model.ExamRegistration
	for _, sid := range studentIDs {
		reg := model.ExamRegistration{
			StudentID: sid,
			ExamID:    examUUID,
			Token:     repository.GenerateExamToken(),
			Status:    "registered",
		}
		// Use RETURNING so we only capture actually-inserted rows.
		// ON CONFLICT DO NOTHING produces no output for existing rows.
		var inserted model.ExamRegistration
		err := tx.QueryRow(ctx,
			`INSERT INTO exam_registration (student_id, exam_id, token, status)
			 VALUES ($1, $2, $3, $4)
			 ON CONFLICT (student_id, exam_id) DO NOTHING
			 RETURNING id, student_id, exam_id, token, card_key, checked_in_at, attempts_used, status, created_at`,
			sid, examUUID, reg.Token, "registered",
		).Scan(
			&inserted.ID, &inserted.StudentID, &inserted.ExamID, &inserted.Token,
			&inserted.CardKey, &inserted.CheckedInAt, &inserted.AttemptsUsed,
			&inserted.Status, &inserted.CreatedAt,
		)
		if err != nil {
			// pgx.ErrNoRows means the row already existed (ON CONFLICT DO NOTHING
			// skipped it). This is not an error — silently skip per FR-GRANT-03.
			if errors.Is(err, pgx.ErrNoRows) {
				continue
			}
			return GrantExamAccessResult{}, err
		}
		registrations = append(registrations, inserted)
	}

	// Write audit log entry (FR-GRANT-04).
	actorIDStr := actorID
	if err := s.storeRepo.InsertAuditLogMeta(ctx, tx, &actorIDStr, "exam_grant", examID, "exam_grant.create", map[string]any{
		"exam_id":     examID,
		"student_ids": studentIDStrs,
		"school_ids":  schoolIDs,
	}); err != nil {
		return GrantExamAccessResult{}, fmt.Errorf("write audit log: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return GrantExamAccessResult{}, err
	}

	// Build result with student metadata matched from users slice.
	grantedStudents := make([]GrantedStudent, 0, len(registrations))
	for _, reg := range registrations {
		var student *model.User
		for j := range users {
			if users[j].ID == reg.StudentID.String() {
				student = &users[j]
				break
			}
		}
		if student == nil {
			continue
		}
		username := ""
		if student.Username != nil {
			username = *student.Username
		}
		grantedStudents = append(grantedStudents, GrantedStudent{
			ID:       student.ID,
			Name:     student.Name,
			Username: username,
		})
	}

	return GrantExamAccessResult{
		GrantedCount:    len(registrations),
		GrantedStudents: grantedStudents,
	}, nil
}
