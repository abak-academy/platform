package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// PartnerStudentRow is the student shape a Mitra sees: enough to pick the
// student for an order, and nothing that identifies or contacts the person.
type PartnerStudentRow struct {
	ID         string  `json:"id"`
	Name       string  `json:"name"`
	Username   *string `json:"username"`
	SchoolName *string `json:"school_name"`
	Grade      *int    `json:"grade"`
	Jenjang    string  `json:"jenjang"`
	Status     string  `json:"status"`
	// Activated is false until the student replaces the temporary password.
	Activated      bool      `json:"activated"`
	RegisteredByMe bool      `json:"registered_by_me"`
	CreatedAt      time.Time `json:"created_at"`
}

// PartnerStudentFilter narrows a Mitra's student list. SchoolID and Q are
// optional; RegisteredByMe restricts the list to the students this Mitra
// registered.
type PartnerStudentFilter struct {
	SchoolID       string
	Status         string
	Q              string
	RegisteredByMe bool
	Cursor         string
	Limit          int
}

// ListPartnerStudents returns the students of the allowed schools together with
// the students the Mitra registered, newest first. An empty school list leaves
// only the registered ones.
func (r *Repository) ListPartnerStudents(ctx context.Context, partnerID string, schoolIDs []string, filter PartnerStudentFilter) ([]PartnerStudentRow, string, error) {
	if filter.Limit <= 0 {
		filter.Limit = 20
	}
	if filter.Limit > 100 {
		filter.Limit = 100
	}
	if schoolIDs == nil {
		schoolIDs = []string{}
	}

	query := `SELECT u.id, u.name, u.username, COALESCE(s.name, u.unlisted_school_name), u.grade,
			COALESCE(u.jenjang, ''), u.status, NOT u.password_temporary, (u.registered_by = $1::uuid), u.created_at
		FROM users u
		LEFT JOIN school s ON s.id = u.school_id
		WHERE u.role = 'student' AND u.status != 'deleted'
		  AND (u.school_id = ANY($2::uuid[]) OR u.registered_by = $1::uuid)`
	args := []any{partnerID, schoolIDs}
	argNum := 3

	if filter.RegisteredByMe {
		query += ` AND u.registered_by = $1::uuid`
	}
	if filter.SchoolID != "" {
		query += fmt.Sprintf(` AND u.school_id = $%d::uuid`, argNum)
		args = append(args, filter.SchoolID)
		argNum++
	}
	if filter.Status != "" {
		query += fmt.Sprintf(` AND u.status = $%d`, argNum)
		args = append(args, filter.Status)
		argNum++
	}
	if filter.Q != "" {
		query += fmt.Sprintf(` AND u.name ILIKE $%d`, argNum)
		args = append(args, "%"+filter.Q+"%")
		argNum++
	}
	if filter.Cursor != "" {
		at, id, err := DecodeOrderCursor(filter.Cursor)
		if err != nil {
			return nil, "", err
		}
		query += fmt.Sprintf(` AND (u.created_at < $%d OR (u.created_at = $%d AND u.id < $%d::uuid))`, argNum, argNum, argNum+1)
		args = append(args, at, id)
		argNum += 2
	}
	query += fmt.Sprintf(` ORDER BY u.created_at DESC, u.id DESC LIMIT $%d`, argNum)
	args = append(args, filter.Limit+1)

	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, "", err
	}
	defer rows.Close()

	students := []PartnerStudentRow{}
	for rows.Next() {
		var s PartnerStudentRow
		var registeredByMe *bool
		if err := rows.Scan(&s.ID, &s.Name, &s.Username, &s.SchoolName, &s.Grade, &s.Jenjang,
			&s.Status, &s.Activated, &registeredByMe, &s.CreatedAt); err != nil {
			return nil, "", err
		}
		s.RegisteredByMe = registeredByMe != nil && *registeredByMe
		students = append(students, s)
	}
	if err := rows.Err(); err != nil {
		return nil, "", err
	}

	nextCursor := ""
	if len(students) > filter.Limit {
		students = students[:filter.Limit]
		last := students[len(students)-1]
		nextCursor = EncodeOrderCursor(last.CreatedAt, uuid.MustParse(last.ID))
	}
	return students, nextCursor, nil
}

// StudentRegistration is who registered a student and whether the student has
// replaced the temporary password.
type StudentRegistration struct {
	Username          *string
	RegisteredBy      *string
	PasswordTemporary bool
}

// GetStudentRegistration returns nil when there is no such student.
func (r *Repository) GetStudentRegistration(ctx context.Context, id string) (*StudentRegistration, error) {
	var reg StudentRegistration
	err := r.pool.QueryRow(ctx,
		`SELECT username, registered_by::text, password_temporary
		 FROM users WHERE id = $1 AND role = 'student' AND status = 'active'`, id,
	).Scan(&reg.Username, &reg.RegisteredBy, &reg.PasswordTemporary)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &reg, nil
}

// ReissueStudentCredentialsByRegistrar replaces the password of a student the
// Mitra registered, only while the student has not replaced the temporary one.
// The conditions sit in the UPDATE itself, so a student who sets a password at
// the same moment cannot be overtaken. It reports whether a row changed.
func (r *Repository) ReissueStudentCredentialsByRegistrar(ctx context.Context, id, partnerID, hash string) (bool, error) {
	tag, err := r.pool.Exec(ctx,
		`UPDATE users SET password_hash = $1, password_temporary = true, updated_at = now()
		 WHERE id = $2 AND role = 'student' AND status = 'active'
		   AND registered_by = $3::uuid AND password_temporary = true`,
		hash, id, partnerID)
	if err != nil {
		return false, err
	}
	return tag.RowsAffected() > 0, nil
}
