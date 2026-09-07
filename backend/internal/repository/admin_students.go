package repository

import (
	"context"
	"fmt"
	"time"

	"akademi-bimbel/internal/model"

	"github.com/google/uuid"
)

// StudentRow is the student shape returned in admin school student list
// responses (no password_hash, no student-only fields beyond grade).
type StudentRow struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	// Username is nullable in the schema and really is NULL for some accounts,
	// so it must be scanned as a pointer — a plain string fails the scan and
	// 500s the whole roster because of one row.
	Username *string `json:"username"`
	Email    *string `json:"email"`
	Status   string  `json:"status"`
	Grade    *int    `json:"grade"`
	Jenjang  string  `json:"jenjang"`
	// SchoolName is the linked school's name, NULL for registrants who have no
	// school on file. UnlistedSchoolName carries what a self-registering user
	// typed when their school wasn't in the list. Operations staff use both to
	// see whose school still needs confirming.
	SchoolName         *string   `json:"school_name"`
	UnlistedSchoolName *string   `json:"unlisted_school_name"`
	CreatedAt          time.Time `json:"created_at"`
}

// StudentFilter carries optional filters for ListStudentsBySchool and
// SearchStudentsAcrossSchools.
type StudentFilter struct {
	Status   string
	Cursor   string
	Limit    int
	Q        string
	Grade    *int    // optional grade filter
	Jenjang  string  // optional jenjang filter
	SchoolID *string // optional school_id filter (cross-school search only)
	ExamID   string
	// AllSchools drops the school scope entirely, returning every student
	// including those with no school on file. It is deliberately an explicit
	// opt-in rather than "empty schoolID means all": the exam-participant and
	// bulk-credential callers share this query and must stay school-scoped.
	AllSchools bool
	// NoSchool adds a positive `AND u.school_id IS NULL` predicate, narrowing
	// to exactly the null-school bucket — the opposite of AllSchools, which
	// drops the scope instead of narrowing it. SchoolID and NoSchool are
	// never both set.
	NoSchool bool
}

// appendStudentFilterSQL appends the predicates shared by
// ListStudentsBySchool and CountStudentsBySchool — school scope, status,
// free-text q, grade, jenjang — to a query whose WHERE already contains the
// base `u.role = 'student' AND u.status != 'deleted'`. Cursor and limit are
// the caller's job (counts must ignore them), and ExamID is deliberately
// excluded: it encodes "not yet registered for this exam" for the
// participant picker, a shape no stat card should count.
func appendStudentFilterSQL(query string, filter StudentFilter, schoolID string, argNum int) (string, []any, int) {
	args := []any{}
	if !filter.AllSchools {
		query += fmt.Sprintf(` AND u.school_id = $%d`, argNum)
		args = append(args, schoolID)
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
	if filter.Grade != nil {
		query += fmt.Sprintf(` AND u.grade = $%d`, argNum)
		args = append(args, *filter.Grade)
		argNum++
	}
	if filter.Jenjang != "" {
		query += fmt.Sprintf(` AND u.jenjang = $%d`, argNum)
		args = append(args, filter.Jenjang)
		argNum++
	}
	return query, args, argNum
}

// ListStudentsBySchool returns non-deleted students scoped to a school,
// cursor-paginated (same shape as ListAdminUsers). Supports optional
// status filter, free-text search on name/username, grade filter, and
// jenjang filter.
func (r *Repository) ListStudentsBySchool(ctx context.Context, schoolID string, filter StudentFilter) ([]StudentRow, string, error) {
	if filter.Limit <= 0 {
		filter.Limit = 20
	}
	if filter.Limit > 100 {
		filter.Limit = 100
	}

	query := `SELECT u.id, u.name, u.username, u.email, u.status, u.grade, COALESCE(u.jenjang, ''),
			s.name AS school_name, u.unlisted_school_name, u.created_at
			FROM users u
			LEFT JOIN school s ON s.id = u.school_id
			WHERE u.role = 'student' AND u.status != 'deleted'`
	var cursorAt time.Time
	cursorID := uuid.Nil
	if filter.Cursor != "" {
		at, id, err := DecodeOrderCursor(filter.Cursor)
		if err != nil {
			return nil, "", err
		}
		cursorAt, cursorID = at, id
	}

	query, args, argNum := appendStudentFilterSQL(query, filter, schoolID, 1)

	if filter.ExamID != "" {
		query += fmt.Sprintf(` AND u.status = 'active'
			AND NOT EXISTS (SELECT 1 FROM exam_registration er WHERE er.student_id = u.id AND er.exam_id = $%d::uuid)`, argNum)
		args = append(args, filter.ExamID)
		argNum++
	}
	if filter.Cursor != "" {
		query += fmt.Sprintf(` AND (u.created_at < $%d OR (u.created_at = $%d AND u.id < $%d::uuid))`, argNum, argNum, argNum+1)
		args = append(args, cursorAt, cursorID)
		argNum += 2
	}

	query += fmt.Sprintf(` ORDER BY u.created_at DESC, u.id DESC LIMIT $%d`, argNum)
	args = append(args, filter.Limit+1)

	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, "", err
	}
	defer rows.Close()

	students := []StudentRow{}
	nextCursor := ""

	for rows.Next() {
		var s StudentRow
		if err := rows.Scan(&s.ID, &s.Name, &s.Username, &s.Email, &s.Status, &s.Grade, &s.Jenjang,
			&s.SchoolName, &s.UnlistedSchoolName, &s.CreatedAt); err != nil {
			return nil, "", err
		}
		students = append(students, s)
	}

	if err = rows.Err(); err != nil {
		return nil, "", err
	}

	if len(students) > filter.Limit {
		students = students[:filter.Limit]
		last := students[len(students)-1]
		nextCursor = EncodeOrderCursor(last.CreatedAt, uuid.MustParse(last.ID))
	}

	return students, nextCursor, nil
}

// StudentAdminCounts summarizes the current filtered student set for the
// admin student list's stat cards, so "Total"/"Aktif"/"Nonaktif" reflect the
// full filtered result set in the DB rather than only the rows loaded onto
// the client so far — the same stat-cards-from-a-single-page bug previously
// fixed on the schools list (see CountSchoolsAdmin and
// docs/backlog/student-list-stats-and-search-debounce.md).
type StudentAdminCounts struct {
	Total       int `json:"total"`
	Active      int `json:"active"`
	Deactivated int `json:"deactivated"`
}

// CountStudentsAdmin returns total/active/deactivated counts for the same
// school scope and status/q/grade/jenjang filters ListStudentsBySchool
// applies, ignoring cursor, limit, and the exam-eligibility filter.
// (CountStudentsBySchool is the unrelated per-school student_count used by
// the school rows themselves.)
func (r *Repository) CountStudentsAdmin(ctx context.Context, schoolID string, filter StudentFilter) (StudentAdminCounts, error) {
	query := `SELECT COUNT(*),
		COUNT(*) FILTER (WHERE u.status = 'active'),
		COUNT(*) FILTER (WHERE u.status = 'deactivated')
		FROM users u
		WHERE u.role = 'student' AND u.status != 'deleted'`

	query, args, _ := appendStudentFilterSQL(query, filter, schoolID, 1)

	var counts StudentAdminCounts
	err := r.pool.QueryRow(ctx, query, args...).Scan(&counts.Total, &counts.Active, &counts.Deactivated)
	return counts, err
}

// CreateStudent inserts a new user with role='student', otp_enabled=false,
// must_change_password=true (the admin register path always issues a
// credential the student does not know yet), and scans back id, created_at,
// updated_at.
func (r *Repository) CreateStudent(ctx context.Context, u *model.User) error {
	u.Email = normalizeOptionalEmail(u.Email)
	u.MustChangePassword = true
	return r.pool.QueryRow(ctx,
		`INSERT INTO users (
			email, username, password_hash, role, name,
			school_id, status, otp_enabled, must_change_password,
			jenjang, provinsi_id, kota_id, kecamatan_id, kode_pos,
			dob, gender, grade, alamat_domisili, target_exam
		) VALUES (
			$1, $2, $3, 'student', $4,
			$5, 'active', false, true,
			$6, $7, $8, $9, $10,
			$11, $12, $13, $14, $15
		) RETURNING id, created_at, updated_at`,
		u.Email, u.Username, u.PasswordHash, u.Name,
		u.SchoolID,
		u.Jenjang, u.ProvinsiID, u.KotaID, u.KecamatanID, u.KodePos,
		u.DOB, u.Gender, u.Grade, u.AlamatDomisili, u.TargetExam,
	).Scan(&u.ID, &u.CreatedAt, &u.UpdatedAt)
}

// GetStudentByID returns a student by ID. When schoolID is non-empty, the
// row must belong to that school (including NULL-school students, which
// never match a real school id). When schoolID is empty, any student id
// matches — used by super_admin acting on a roster that is not school-filtered.
// Returns nil, nil when not found.
func (r *Repository) GetStudentByID(ctx context.Context, id, schoolID string) (*model.User, error) {
	u := &model.User{}
	query := `SELECT id, email, username, phone, password_hash, role, name,
			school_id, photo_url, status, otp_enabled, auth_provider, created_at, updated_at,
			jenjang, provinsi_id, kota_id, kecamatan_id, kode_pos,
			dob, gender, grade, alamat_domisili, target_exam
		FROM users
		WHERE id = $1 AND role = 'student' AND status != 'deleted'`
	args := []any{id}
	if schoolID != "" {
		query += ` AND school_id = $2`
		args = append(args, schoolID)
	}
	err := r.pool.QueryRow(ctx, query, args...).Scan(
		&u.ID, &u.Email, &u.Username, &u.Phone, &u.PasswordHash, &u.Role, &u.Name,
		&u.SchoolID, &u.PhotoURL, &u.Status, &u.OTPEnabled, &u.AuthProvider, &u.CreatedAt, &u.UpdatedAt,
		&u.Jenjang, &u.ProvinsiID, &u.KotaID, &u.KecamatanID, &u.KodePos,
		&u.DOB, &u.Gender, &u.Grade, &u.AlamatDomisili, &u.TargetExam,
	)
	if err != nil {
		if isNotFound(err) {
			return nil, nil
		}
		return nil, err
	}
	return u, nil
}

// UpdateStudentStatus sets the status of a student. schoolID empty skips the
// school predicate (super_admin, id-only). Non-empty keeps the school bound.
func (r *Repository) UpdateStudentStatus(ctx context.Context, id, schoolID, status string) error {
	query := `UPDATE users SET status = $1, updated_at = now()
			WHERE id = $2 AND role = 'student' AND status != 'deleted'`
	args := []any{status, id}
	if schoolID != "" {
		query += ` AND school_id = $3`
		args = append(args, schoolID)
	}
	_, err := r.pool.Exec(ctx, query, args...)
	return err
}

// CrossSchoolStudentRow extends StudentRow with school info for cross-school
// search results. Used by SearchStudentsAcrossSchools.
type CrossSchoolStudentRow struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	// Nullable for the same reason as StudentRow.Username — see there.
	Username *string `json:"username"`
	Email    *string `json:"email"`
	Status   string  `json:"status"`
	Grade    *int    `json:"grade"`
	Jenjang  string  `json:"jenjang"`
	// SchoolID/SchoolName are nullable: a student can have no school on file,
	// in which case UnlistedSchoolName carries the free-text name they typed
	// at registration.
	SchoolID           *string   `json:"school_id"`
	SchoolName         *string   `json:"school_name"`
	UnlistedSchoolName *string   `json:"unlisted_school_name"`
	CreatedAt          time.Time `json:"created_at"`
}

// SearchStudentsAcrossSchools searches active students across all schools with
// optional filters. When filter.SchoolID is set, narrows to that school. Joins
// school to include school_name in each result row. Cursor-paginated with a
// bounded default limit (FR-SEARCH-01/03).
func (r *Repository) SearchStudentsAcrossSchools(ctx context.Context, filter StudentFilter) ([]CrossSchoolStudentRow, string, error) {
	if filter.Limit <= 0 {
		filter.Limit = 20
	}
	if filter.Limit > 100 {
		filter.Limit = 100
	}

	query := `SELECT u.id, u.name, u.username, u.email, u.status, u.grade, COALESCE(u.jenjang, ''), u.created_at, u.school_id, s.name, u.unlisted_school_name
			FROM users u LEFT JOIN school s ON u.school_id = s.id
			WHERE u.role = 'student' AND u.status != 'deleted'`
	args := []any{}
	argNum := 1
	var cursorAt time.Time
	cursorID := uuid.Nil
	if filter.Cursor != "" {
		at, id, err := DecodeOrderCursor(filter.Cursor)
		if err != nil {
			return nil, "", err
		}
		cursorAt, cursorID = at, id
	}

	if filter.SchoolID != nil {
		query += fmt.Sprintf(` AND u.school_id = $%d`, argNum)
		args = append(args, *filter.SchoolID)
		argNum++
	}
	if filter.NoSchool {
		query += ` AND u.school_id IS NULL`
	}
	if filter.Q != "" {
		query += fmt.Sprintf(` AND (u.name ILIKE $%d OR u.username ILIKE $%d)`, argNum, argNum+1)
		args = append(args, "%"+filter.Q+"%", "%"+filter.Q+"%")
		argNum += 2
	}
	if filter.Grade != nil {
		query += fmt.Sprintf(` AND u.grade = $%d`, argNum)
		args = append(args, *filter.Grade)
		argNum++
	}
	if filter.Jenjang != "" {
		query += fmt.Sprintf(` AND u.jenjang = $%d`, argNum)
		args = append(args, filter.Jenjang)
		argNum++
	}
	if filter.ExamID != "" {
		query += fmt.Sprintf(` AND u.status = 'active'
			AND NOT EXISTS (SELECT 1 FROM exam_registration er WHERE er.student_id = u.id AND er.exam_id = $%d::uuid)`, argNum)
		args = append(args, filter.ExamID)
		argNum++
	}
	if filter.Cursor != "" {
		query += fmt.Sprintf(` AND (u.created_at < $%d OR (u.created_at = $%d AND u.id < $%d::uuid))`, argNum, argNum, argNum+1)
		args = append(args, cursorAt, cursorID)
		argNum += 2
	}

	query += fmt.Sprintf(` ORDER BY u.created_at DESC, u.id DESC LIMIT $%d`, argNum)
	args = append(args, filter.Limit+1)

	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, "", err
	}
	defer rows.Close()

	students := []CrossSchoolStudentRow{}
	nextCursor := ""
	for rows.Next() {
		var s CrossSchoolStudentRow
		if err := rows.Scan(&s.ID, &s.Name, &s.Username, &s.Email, &s.Status, &s.Grade, &s.Jenjang, &s.CreatedAt, &s.SchoolID, &s.SchoolName, &s.UnlistedSchoolName); err != nil {
			return nil, "", err
		}
		students = append(students, s)
	}

	if err = rows.Err(); err != nil {
		return nil, "", err
	}

	if len(students) > filter.Limit {
		students = students[:filter.Limit]
		last := students[len(students)-1]
		nextCursor = EncodeOrderCursor(last.CreatedAt, uuid.MustParse(last.ID))
	}

	return students, nextCursor, nil
}

// ResetStudentPasswordHash overwrites the password hash and flags the
// credential for forced change at next login — the admin reissue/set paths
// always issue a temporary credential. schoolID empty skips the school
// predicate (super_admin, id-only).
func (r *Repository) ResetStudentPasswordHash(ctx context.Context, id, schoolID, hash string) error {
	query := `UPDATE users SET password_hash = $1, must_change_password = true, updated_at = now()
			WHERE id = $2 AND role = 'student' AND status != 'deleted'`
	args := []any{hash, id}
	if schoolID != "" {
		query += ` AND school_id = $3`
		args = append(args, schoolID)
	}
	_, err := r.pool.Exec(ctx, query, args...)
	return err
}
