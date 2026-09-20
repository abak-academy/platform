package service

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"akademi-bimbel/internal/model"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// ExamRevokeRowResult is the per-row outcome of RevokeExamAccess, consumed by
// the admin revoke UI to show what happened to each selected student.
type ExamRevokeRowResult struct {
	StudentID string `json:"student_id"`
	Name      string `json:"name"`
	Username  string `json:"username"`
	Status    string `json:"status"` // "revoked" | "skipped" | "failed"
	Message   string `json:"message"`
}

type RevokeExamAccessResult struct {
	RevokedCount int                   `json:"revoked_count"`
	Results      []ExamRevokeRowResult `json:"results"`
}

// ExamRevokeBulkRowResult is the per-row outcome of RevokeExamAccessBulk,
// consumed directly by the exam-revoke-bulk worker to build the result CSV.
type ExamRevokeBulkRowResult struct {
	Username string `json:"username"`
	Status   string `json:"status"` // "revoked" | "skipped" | "failed"
	Message  string `json:"message"`
}

// revokeDecision is the internal status assigned to one student during a
// revoke batch, before metadata is attached for the wire/CSV shapes.
type revokeDecision struct {
	Status  string
	Message string
}

// revokeStudentMeta is the best-effort display metadata for one revoked row.
type revokeStudentMeta struct {
	name     string
	username string
}

// RevokeExamAccess soft-revokes the exam registrations for the given students
// (mirror of GrantExamAccess). Per-row semantics: no registration for the
// exam → failed; already revoked → skipped; in-progress session → failed
// (the admin must force-submit first); otherwise → revoked. One transaction
// carries every revocation plus a single audit entry; a Redis device lock
// left by check-in is cleared best-effort after commit.
func (s *Service) RevokeExamAccess(ctx context.Context, actorID, examID string, studentIDs []uuid.UUID) (RevokeExamAccessResult, error) {
	examUUID, err := uuid.Parse(examID)
	if err != nil {
		return RevokeExamAccessResult{}, err
	}
	if len(studentIDs) == 0 {
		return RevokeExamAccessResult{}, nil
	}

	// Metadata is best-effort: a student without a users row still gets its
	// registration revoked if one exists.
	users, err := s.repo.GetUsersByIDs(ctx, studentIDs)
	if err != nil {
		return RevokeExamAccessResult{}, err
	}
	usersByID := make(map[uuid.UUID]revokeStudentMeta, len(users))
	for _, u := range users {
		meta := revokeStudentMeta{name: u.Name}
		if u.Username != nil {
			meta.username = *u.Username
		}
		if id, err := uuid.Parse(u.ID); err == nil {
			usersByID[id] = meta
		}
	}

	decisions, _, err := s.revokeExamStudents(ctx, actorID, examUUID, studentIDs)
	if err != nil {
		return RevokeExamAccessResult{}, err
	}

	result := RevokeExamAccessResult{Results: make([]ExamRevokeRowResult, 0, len(studentIDs))}
	for _, sid := range studentIDs {
		decision := decisions[sid]
		meta := usersByID[sid]
		result.Results = append(result.Results, ExamRevokeRowResult{
			StudentID: sid.String(),
			Name:      meta.name,
			Username:  meta.username,
			Status:    decision.Status,
			Message:   decision.Message,
		})
		if decision.Status == "revoked" {
			result.RevokedCount++
		}
	}
	return result, nil
}

// RevokeExamAccessBulk resolves each username exactly (case-sensitive, same
// discipline as GrantExamAccessBulk) and revokes that student's registration
// for the exam. Rows are validated in this order: blank → failed, duplicate
// username within the batch → skipped, username not found → failed, role !=
// student → failed, otherwise → the shared per-row revoke semantics.
func (s *Service) RevokeExamAccessBulk(ctx context.Context, actorID, examID string, usernames []string) ([]ExamRevokeBulkRowResult, error) {
	examUUID, err := uuid.Parse(examID)
	if err != nil {
		return nil, err
	}

	results := make([]ExamRevokeBulkRowResult, len(usernames))
	seen := make(map[string]bool, len(usernames))

	type candidate struct {
		idx       int
		username  string
		studentID uuid.UUID
	}
	var candidates []candidate
	var studentIDs []uuid.UUID

	for i, raw := range usernames {
		username := strings.TrimSpace(raw)
		if username == "" {
			results[i] = ExamRevokeBulkRowResult{Username: raw, Status: "failed", Message: "blank username"}
			continue
		}
		if seen[username] {
			results[i] = ExamRevokeBulkRowResult{Username: username, Status: "skipped", Message: "duplicate username in file"}
			continue
		}
		seen[username] = true

		user, err := s.repo.GetUserByUsername(ctx, username)
		if err != nil {
			return nil, err
		}
		if user == nil {
			results[i] = ExamRevokeBulkRowResult{Username: username, Status: "failed", Message: "username not found"}
			continue
		}
		if user.Role != "student" {
			results[i] = ExamRevokeBulkRowResult{Username: username, Status: "failed", Message: "user is not a student"}
			continue
		}

		studentID, err := uuid.Parse(user.ID)
		if err != nil {
			return nil, err
		}
		candidates = append(candidates, candidate{idx: i, username: username, studentID: studentID})
		studentIDs = append(studentIDs, studentID)
	}

	if len(candidates) == 0 {
		return results, nil
	}

	decisions, _, err := s.revokeExamStudents(ctx, actorID, examUUID, studentIDs)
	if err != nil {
		return nil, err
	}

	for _, c := range candidates {
		results[c.idx] = ExamRevokeBulkRowResult{
			Username: c.username,
			Status:   decisions[c.studentID].Status,
			Message:  decisions[c.studentID].Message,
		}
	}
	return results, nil
}

// revokeExamStudents assigns a revoke decision to every studentID and commits
// the revocations inside one transaction (single audit entry, shared by the
// manual and CSV bulk paths). Returns the per-student decisions and the
// revoked studentID → registrationID map for device-lock cleanup.
func (s *Service) revokeExamStudents(ctx context.Context, actorID string, examUUID uuid.UUID, studentIDs []uuid.UUID) (map[uuid.UUID]revokeDecision, map[uuid.UUID]uuid.UUID, error) {
	decisions := make(map[uuid.UUID]revokeDecision, len(studentIDs))

	statuses, err := s.storeRepo.GetExamRegistrationStatuses(ctx, examUUID, studentIDs)
	if err != nil {
		return nil, nil, err
	}

	var candidates []uuid.UUID
	for _, sid := range studentIDs {
		status, ok := statuses[sid]
		switch {
		case !ok:
			decisions[sid] = revokeDecision{Status: "failed", Message: "not registered for this exam"}
		case status == "revoked":
			decisions[sid] = revokeDecision{Status: "skipped", Message: "already revoked"}
		default:
			candidates = append(candidates, sid)
		}
	}
	if len(candidates) == 0 {
		return decisions, map[uuid.UUID]uuid.UUID{}, nil
	}

	inProgress, err := s.storeRepo.CountInProgressSessions(ctx, examUUID, candidates)
	if err != nil {
		return nil, nil, err
	}
	blocked := make(map[uuid.UUID]bool, len(inProgress))
	for _, sid := range inProgress {
		blocked[sid] = true
	}

	var revocable []uuid.UUID
	for _, sid := range candidates {
		if blocked[sid] {
			decisions[sid] = revokeDecision{Status: "failed", Message: "has an in-progress session: force-submit first"}
			continue
		}
		revocable = append(revocable, sid)
	}
	if len(revocable) == 0 {
		return decisions, map[uuid.UUID]uuid.UUID{}, nil
	}

	tx, err := s.storeRepo.BeginTx(ctx)
	if err != nil {
		return nil, nil, err
	}
	defer tx.Rollback(ctx)

	revoked, err := s.storeRepo.RevokeExamRegistrationsTx(ctx, tx, examUUID, revocable, actorID)
	if err != nil {
		return nil, nil, err
	}

	if len(revoked) > 0 {
		revokedIDs := make([]string, 0, len(revoked))
		for sid := range revoked {
			revokedIDs = append(revokedIDs, sid.String())
		}
		actorIDStr := actorID
		if err := s.storeRepo.InsertAuditLogMeta(ctx, tx, &actorIDStr, "exam_grant", examUUID.String(), "exam_grant.revoke", map[string]any{
			"exam_id":     examUUID.String(),
			"student_ids": revokedIDs,
		}); err != nil {
			return nil, nil, fmt.Errorf("write audit log: %w", err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, nil, err
	}

	for _, sid := range revocable {
		if regID, ok := revoked[sid]; ok {
			decisions[sid] = revokeDecision{Status: "revoked"}
			s.clearExamDeviceLock(ctx, regID)
		} else {
			// The UPDATE predicate matched on the pre-read status; losing the
			// race means something revoked it concurrently — report that.
			decisions[sid] = revokeDecision{Status: "skipped", Message: "already revoked"}
		}
	}
	return decisions, revoked, nil
}

// clearExamDeviceLock drops the check-in device fingerprint left in Redis for
// a revoked registration. Best-effort hygiene — the lock's TTL expires on its
// own, so a failed delete only logs.
func (s *Service) clearExamDeviceLock(ctx context.Context, regID uuid.UUID) {
	if s.rdb == nil {
		return
	}
	if err := s.rdb.Del(ctx, "exam:device:"+regID.String()).Err(); err != nil {
		slog.Error("clear exam device lock", "registration_id", regID, "error", err)
	}
}

// revokeOrderExamRegistrationsTx revokes the exam registrations created by a
// refunded order — the exam-side twin of RevokeEnrollmentsByOrder: exams
// linked to each exam-type order item × the order's participants (or the
// buyer for self-purchases). Runs inside the refund's transaction.
func (s *Service) revokeOrderExamRegistrationsTx(ctx context.Context, tx pgx.Tx, orderID, buyer uuid.UUID, items []model.OrderItem, actorID string) error {
	examsByProduct := make(map[uuid.UUID][]uuid.UUID)
	for _, item := range items {
		if item.ProductType != "exam" {
			continue
		}
		exams, err := s.storeRepo.GetExamsByProductID(ctx, item.ProductID)
		if err != nil {
			return err
		}
		if len(exams) == 0 {
			continue
		}
		examIDs := make([]uuid.UUID, 0, len(exams))
		for _, exam := range exams {
			examIDs = append(examIDs, exam.ID)
		}
		examsByProduct[item.ProductID] = examIDs
	}
	if len(examsByProduct) == 0 {
		return nil
	}

	participants, err := s.storeRepo.GetOrderParticipants(ctx, orderID)
	if err != nil {
		return err
	}
	if len(participants) == 0 {
		participants = []uuid.UUID{buyer}
	}

	return s.storeRepo.RevokeExamRegistrationsByOrderTx(ctx, tx, examsByProduct, participants, actorID)
}
