package service

import (
	"bytes"
	"context"
	"encoding/csv"
	"errors"

	"akademi-bimbel/internal/repository"
)

// ReissueStudentCredentialsBulk reissues credentials for a batch of students
// scoped to schoolID, reusing the unmodified single-student
// ReissueStudentCredentials in a loop. When all is true, studentIDs is
// ignored and every student in the school is targeted instead (paginated via
// ListStudentsBySchool). Returns the per-row report as CSV bytes.
func (s *Service) ReissueStudentCredentialsBulk(ctx context.Context, schoolID string, studentIDs []string, all bool) ([]byte, error) {
	ids := studentIDs
	if all {
		var err error
		ids, err = s.collectAllStudentIDs(ctx, schoolID)
		if err != nil {
			return nil, err
		}
	} else if len(studentIDs) > maxBulkRows {
		return nil, ErrRowLimitExceeded
	}

	rows := make([]StudentBulkResultRow, 0, len(ids))
	for _, id := range ids {
		row := StudentBulkResultRow{}

		student, err := s.storeRepo.GetStudentByID(ctx, id, schoolID)
		if err != nil {
			return nil, err
		}
		if student != nil {
			row.Name = student.Name
		}

		creds, err := s.ReissueStudentCredentials(ctx, schoolID, id)
		switch {
		case err == nil:
			row.Username = creds.Username
			row.TempPassword = creds.TempPassword
		case errors.Is(err, ErrStudentNotFound):
			row.Error = "student_not_found"
		default:
			return nil, err
		}

		rows = append(rows, row)
	}

	return BuildCredentialsResultCSV(rows), nil
}

// bulkAllRowCap mirrors maxBulkRows for the all=true pagination path. It's a
// var rather than using maxBulkRows directly so tests can lower it without
// seeding 1,000+ real students to exercise the cap.
var bulkAllRowCap = maxBulkRows

// collectAllStudentIDs paginates every non-deleted student in a school,
// erroring ErrRowLimitExceeded as soon as the collected count exceeds
// bulkAllRowCap rather than waiting for pagination to complete.
func (s *Service) collectAllStudentIDs(ctx context.Context, schoolID string) ([]string, error) {
	var ids []string
	cursor := ""
	for {
		page, nextCursor, err := s.storeRepo.ListStudentsBySchool(ctx, schoolID, repository.StudentFilter{
			Cursor: cursor,
			Limit:  100,
		})
		if err != nil {
			return nil, err
		}
		for _, r := range page {
			ids = append(ids, r.ID)
			if len(ids) > bulkAllRowCap {
				return nil, ErrRowLimitExceeded
			}
		}
		if nextCursor == "" {
			break
		}
		cursor = nextCursor
	}
	return ids, nil
}

// BuildCredentialsResultCSV writes the per-row credential reissue report as
// CSV bytes. Unlike BuildStudentBulkResultCSV, reissue doesn't re-collect
// email/status so those columns are omitted.
func BuildCredentialsResultCSV(rows []StudentBulkResultRow) []byte {
	var buf bytes.Buffer
	w := csv.NewWriter(&buf)
	_ = w.Write([]string{"name", "username", "temp_password", "error"})
	for _, r := range rows {
		_ = w.Write(csvSafeRow(r.Name, r.Username, r.TempPassword, r.Error))
	}
	w.Flush()
	return buf.Bytes()
}
