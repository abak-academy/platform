package service

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/uuid"
)

// ExamGrantBulkRowResult is the per-row outcome of GrantExamAccessBulk,
// consumed directly by the exam-grant-bulk worker to build the result CSV.
type ExamGrantBulkRowResult struct {
	Username string `json:"username"`
	Status   string `json:"status"` // "granted" | "skipped" | "failed"
	Message  string `json:"message"`
}

// GrantExamAccessBulk resolves each username exactly (case-sensitive; no
// fuzzy matching — usernames are uniquely indexed on the raw column, see
// db/migrations/0002_identity.up.sql) and grants exam access through the same
// insertExamRegistrationRow business logic/transaction discipline as
// GrantExamAccess. Rows are validated in this order: blank -> failed,
// duplicate username within the batch -> skipped, username not found ->
// failed, role != student -> failed, already registered -> skipped,
// otherwise -> granted.
func (s *Service) GrantExamAccessBulk(ctx context.Context, actorID, examID string, usernames []string) ([]ExamGrantBulkRowResult, error) {
	examUUID, err := uuid.Parse(examID)
	if err != nil {
		return nil, err
	}

	results := make([]ExamGrantBulkRowResult, len(usernames))
	seen := make(map[string]bool, len(usernames))

	type candidate struct {
		idx       int
		username  string
		studentID uuid.UUID
	}
	var candidates []candidate

	for i, raw := range usernames {
		username := strings.TrimSpace(raw)
		if username == "" {
			results[i] = ExamGrantBulkRowResult{Username: raw, Status: "failed", Message: "blank username"}
			continue
		}
		if seen[username] {
			results[i] = ExamGrantBulkRowResult{Username: username, Status: "skipped", Message: "duplicate username in file"}
			continue
		}
		seen[username] = true

		user, err := s.repo.GetUserByUsername(ctx, username)
		if err != nil {
			return nil, err
		}
		if user == nil {
			results[i] = ExamGrantBulkRowResult{Username: username, Status: "failed", Message: "username not found"}
			continue
		}
		if user.Role != "student" {
			results[i] = ExamGrantBulkRowResult{Username: username, Status: "failed", Message: "user is not a student"}
			continue
		}

		studentID, err := uuid.Parse(user.ID)
		if err != nil {
			return nil, err
		}
		candidates = append(candidates, candidate{idx: i, username: username, studentID: studentID})
	}

	if len(candidates) == 0 {
		return results, nil
	}

	tx, err := s.storeRepo.BeginTx(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	var grantedStudentIDs []string
	for _, c := range candidates {
		inserted, err := insertExamRegistrationRow(ctx, tx, c.studentID, examUUID)
		if err != nil {
			return nil, err
		}
		if inserted == nil {
			results[c.idx] = ExamGrantBulkRowResult{Username: c.username, Status: "skipped", Message: "already registered"}
			continue
		}
		results[c.idx] = ExamGrantBulkRowResult{Username: c.username, Status: "granted"}
		grantedStudentIDs = append(grantedStudentIDs, c.studentID.String())
	}

	if len(grantedStudentIDs) > 0 {
		actorIDStr := actorID
		if err := s.storeRepo.InsertAuditLogMeta(ctx, tx, &actorIDStr, "exam_grant", examID, "exam_grant.create", map[string]any{
			"exam_id":     examID,
			"student_ids": grantedStudentIDs,
		}); err != nil {
			return nil, fmt.Errorf("write audit log: %w", err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}

	return results, nil
}
