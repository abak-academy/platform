package repository

import (
	"context"
	"fmt"

	"akademi-bimbel/internal/model"
)

// SchoolAdminRow is the school row returned in admin list responses,
// embedding School with a computed student_count.
type SchoolAdminRow struct {
	model.School
	StudentCount int `json:"student_count"`
}

// SchoolAdminFilter carries optional filters and pagination for
// ListSchoolsAdmin / CountSchoolsAdmin. The two share buildSchoolFilterSQL so
// the WHERE clause used to page can never drift from the one used to count.
type SchoolAdminFilter struct {
	Q      string // matches name/code/npsn, case-insensitive substring
	Status string // "active" or "deactivated"; empty means no filter
	Cursor string
	Limit  int
}

// buildSchoolFilterSQL returns the shared WHERE clause (q/status only —
// no cursor, no LIMIT) plus its args, so ListSchoolsAdmin and
// CountSchoolsAdmin filter identically. argNum is the next free placeholder.
func buildSchoolFilterSQL(filter SchoolAdminFilter, argNum int) (string, []any, int) {
	where := ""
	args := []any{}

	if filter.Q != "" {
		where += fmt.Sprintf(` AND (s.name ILIKE $%d OR s.code ILIKE $%d OR s.npsn ILIKE $%d)`, argNum, argNum, argNum)
		args = append(args, "%"+filter.Q+"%")
		argNum++
	}
	if filter.Status != "" {
		where += fmt.Sprintf(` AND s.status = $%d`, argNum)
		args = append(args, filter.Status)
		argNum++
	}

	return where, args, argNum
}

// ListSchoolsAdmin returns schools cursor-paginated, ordered by name then id.
// The cursor is the composite (name, id) keyset of the last row on the
// previous page — not a bare id — so "load more" walks the same ORDER BY
// the query uses instead of skipping/duplicating rows (see
// docs/backlog/school-bulk-list-pagination.md). Each row carries a computed
// student_count from the users table.
func (r *Repository) ListSchoolsAdmin(ctx context.Context, filter SchoolAdminFilter) ([]SchoolAdminRow, string, error) {
	if filter.Limit <= 0 {
		filter.Limit = 20
	}
	if filter.Limit > 100 {
		filter.Limit = 100
	}

	query := `SELECT s.id, s.name, s.code, s.npsn, s.school_types, s.alamat,
		s.status, s.created_at, s.updated_at,
		(SELECT COUNT(*) FROM users WHERE school_id = s.id AND role = 'student' AND status != 'deleted') AS student_count
		FROM school s WHERE 1=1`
	args := []any{}
	argNum := 1

	filterWhere, filterArgs, nextArgNum := buildSchoolFilterSQL(filter, argNum)
	query += filterWhere
	args = append(args, filterArgs...)
	argNum = nextArgNum

	if filter.Cursor != "" {
		cursorName, cursorID, err := DecodeNameCursor(filter.Cursor)
		if err != nil {
			return nil, "", err
		}
		query += fmt.Sprintf(` AND (s.name, s.id) > ($%d, $%d::uuid)`, argNum, argNum+1)
		args = append(args, cursorName, cursorID.String())
		argNum += 2
	}

	query += fmt.Sprintf(` ORDER BY s.name ASC, s.id ASC LIMIT $%d`, argNum)
	args = append(args, filter.Limit+1)

	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, "", err
	}
	defer rows.Close()

	schools := []SchoolAdminRow{}
	hasMore := false

	for rows.Next() {
		var s SchoolAdminRow
		if err := rows.Scan(
			&s.ID, &s.Name, &s.Code, &s.NPSN, &s.SchoolTypes, &s.Alamat,
			&s.Status, &s.CreatedAt, &s.UpdatedAt, &s.StudentCount,
		); err != nil {
			return nil, "", err
		}
		if len(schools) < filter.Limit {
			schools = append(schools, s)
		} else {
			// This (limit+1)-th row only proves a next page exists; it must not
			// become the cursor itself — the predicate is strictly greater-than,
			// so pointing at this row's own values would skip it. The cursor is
			// the last row actually returned, below.
			hasMore = true
		}
	}

	if err = rows.Err(); err != nil {
		return nil, "", err
	}

	nextCursor := ""
	if hasMore && len(schools) > 0 {
		last := schools[len(schools)-1]
		nextCursor = EncodeNameCursor(last.Name, last.ID)
	}

	return schools, nextCursor, nil
}

// SchoolAdminCounts summarizes the current filtered school set for the admin
// list's stat cards, so "Total"/"Active" reflect the full filtered result
// set in the DB rather than only the rows loaded onto the client so far.
type SchoolAdminCounts struct {
	Total    int `json:"total"`
	Active   int `json:"active"`
	Students int `json:"students"`
}

// CountSchoolsAdmin returns total/active/student counts for the same q/status
// filters ListSchoolsAdmin applies, ignoring cursor and limit.
func (r *Repository) CountSchoolsAdmin(ctx context.Context, filter SchoolAdminFilter) (SchoolAdminCounts, error) {
	query := `SELECT COUNT(*),
		COUNT(*) FILTER (WHERE s.status = 'active'),
		COALESCE(SUM((SELECT COUNT(*) FROM users WHERE school_id = s.id AND role = 'student' AND status != 'deleted')), 0)
		FROM school s WHERE 1=1`

	filterWhere, args, _ := buildSchoolFilterSQL(filter, 1)
	query += filterWhere

	var counts SchoolAdminCounts
	err := r.pool.QueryRow(ctx, query, args...).Scan(&counts.Total, &counts.Active, &counts.Students)
	return counts, err
}

// SchoolOption is the minimal shape used to populate school picker
// dropdowns — deliberately excludes student_count (a correlated subquery per
// row) and other fields the pickers never render. SchoolTypes is included
// because the student-registration picker constrains its jenjang options to
// the selected school's types; unlike student_count it's a plain column, not
// a subquery, so it's cheap to carry here.
type SchoolOption struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Code        string   `json:"code"`
	SchoolTypes []string `json:"school_types"`
}

// ListSchoolOptions returns every active school (id, name, code, school_types),
// ordered by name, for use in picker dropdowns. Unlike ListSchoolsAdmin this
// is not paginated: pickers need the full active registry to let users select
// any school, not just the first page (see school-bulk-list-pagination
// backlog, "picker" gap — GET /admin/schools with no cursor/limit was
// silently truncating every picker in the app to 20 alphabetically-first
// schools).
func (r *Repository) ListSchoolOptions(ctx context.Context) ([]SchoolOption, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT id, name, code, school_types FROM school WHERE status = 'active' ORDER BY name ASC`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	options := []SchoolOption{}
	for rows.Next() {
		var o SchoolOption
		if err := rows.Scan(&o.ID, &o.Name, &o.Code, &o.SchoolTypes); err != nil {
			return nil, err
		}
		options = append(options, o)
	}
	return options, rows.Err()
}

// GetSchoolByID returns a school by ID. Returns nil, nil when not found.
func (r *Repository) GetSchoolByID(ctx context.Context, id string) (*model.School, error) {
	s := &model.School{}
	err := r.pool.QueryRow(ctx,
		`SELECT id, name, code, npsn, school_types, alamat, status, created_at, updated_at
		FROM school WHERE id = $1`,
		id,
	).Scan(
		&s.ID, &s.Name, &s.Code, &s.NPSN, &s.SchoolTypes, &s.Alamat,
		&s.Status, &s.CreatedAt, &s.UpdatedAt,
	)
	if err != nil {
		if isNotFound(err) {
			return nil, nil
		}
		return nil, err
	}
	return s, nil
}

// SchoolCodeExists checks whether a given code already exists in the school table.
// excludeID optionally excludes a specific school ID (for update checks).
func (r *Repository) SchoolCodeExists(ctx context.Context, code string, excludeID *string) (bool, error) {
	query := `SELECT EXISTS(SELECT 1 FROM school WHERE code = $1`
	args := []any{code}
	if excludeID != nil {
		query += ` AND id != $2`
		args = append(args, *excludeID)
	}
	query += `)`

	var exists bool
	err := r.pool.QueryRow(ctx, query, args...).Scan(&exists)
	return exists, err
}

// CreateSchool inserts a new school with status='active' and scans back
// id, created_at, updated_at.
func (r *Repository) CreateSchool(ctx context.Context, s *model.School) error {
	return r.pool.QueryRow(ctx,
		`INSERT INTO school (name, code, npsn, school_types, alamat, status)
		VALUES ($1, $2, $3, $4, $5, 'active')
		RETURNING id, created_at, updated_at`,
		s.Name, s.Code, s.NPSN, s.SchoolTypes, s.Alamat,
	).Scan(&s.ID, &s.CreatedAt, &s.UpdatedAt)
}

// UpdateSchool patches editable fields using COALESCE. Nil pointer arguments
// leave the corresponding column unchanged.
func (r *Repository) UpdateSchool(ctx context.Context, id string, name, npsn, alamat *string, schoolTypes []string, code *string) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE school
		SET name = COALESCE($1, name),
			npsn = COALESCE($2, npsn),
			alamat = COALESCE($3, alamat),
			school_types = COALESCE($4, school_types),
			code = COALESCE($5, code),
			updated_at = now()
		WHERE id = $6`,
		name, npsn, alamat, schoolTypes, code, id,
	)
	return err
}

// UpdateSchoolStatus sets the status of a school.
func (r *Repository) UpdateSchoolStatus(ctx context.Context, id, status string) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE school SET status = $1, updated_at = now() WHERE id = $2`,
		status, id,
	)
	return err
}

// GetSchoolByNameCI returns a school by its name (case-insensitive),
// or nil, nil when not found.
func (r *Repository) GetSchoolByNameCI(ctx context.Context, name string) (*model.School, error) {
	s := &model.School{}
	err := r.pool.QueryRow(ctx,
		`SELECT id, name, code, npsn, school_types, alamat, status, created_at, updated_at
		FROM school WHERE LOWER(name) = LOWER($1)`,
		name,
	).Scan(
		&s.ID, &s.Name, &s.Code, &s.NPSN, &s.SchoolTypes, &s.Alamat,
		&s.Status, &s.CreatedAt, &s.UpdatedAt,
	)
	if err != nil {
		if isNotFound(err) {
			return nil, nil
		}
		return nil, err
	}
	return s, nil
}

// CountStudentsBySchool returns the number of non-deleted students for a school.
func (r *Repository) CountStudentsBySchool(ctx context.Context, schoolID string) (int, error) {
	var count int
	err := r.pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM users WHERE school_id = $1 AND role = 'student' AND status != 'deleted'`,
		schoolID,
	).Scan(&count)
	return count, err
}
