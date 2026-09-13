package repository

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"

	"akademi-bimbel/internal/model"
	"github.com/google/uuid"
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

type SchoolOption = model.SchoolOption

type SchoolSearchFilter struct {
	Q          string
	ProvinceID string
	Category   string
	NPSN       string
	Cursor     string
	Limit      int
}

type schoolSearchCursor struct {
	Q          string `json:"q"`
	ProvinceID string `json:"province_id"`
	Category   string `json:"category"`
	Name       string `json:"name"`
	ID         string `json:"id"`
}

func encodeSchoolSearchCursor(filter SchoolSearchFilter, last SchoolOption) string {
	payload, _ := json.Marshal(schoolSearchCursor{
		Q:          filter.Q,
		ProvinceID: filter.ProvinceID,
		Category:   filter.Category,
		Name:       last.Name,
		ID:         last.ID,
	})
	return base64.RawURLEncoding.EncodeToString(payload)
}

func decodeSchoolSearchCursor(raw string, filter SchoolSearchFilter) (string, uuid.UUID, error) {
	if len(raw) > 2048 {
		return "", uuid.Nil, ErrInvalidCursor
	}
	decoded, err := base64.RawURLEncoding.DecodeString(raw)
	if err != nil {
		return "", uuid.Nil, ErrInvalidCursor
	}
	var c schoolSearchCursor
	if err := json.Unmarshal(decoded, &c); err != nil {
		return "", uuid.Nil, ErrInvalidCursor
	}
	if c.Q != filter.Q || c.ProvinceID != filter.ProvinceID || c.Category != filter.Category {
		return "", uuid.Nil, ErrInvalidCursor
	}
	id, err := uuid.Parse(c.ID)
	if err != nil {
		return "", uuid.Nil, ErrInvalidCursor
	}
	return c.Name, id, nil
}

func (r *Repository) SearchSchoolOptions(ctx context.Context, filter SchoolSearchFilter) ([]SchoolOption, string, error) {
	if filter.Limit <= 0 {
		filter.Limit = 20
	}
	if filter.Limit > 50 {
		filter.Limit = 50
	}

	if filter.NPSN != "" {
		rows, err := r.querySchoolOptions(ctx,
			`WHERE s.status = 'active' AND UPPER(BTRIM(s.npsn)) = $1 ORDER BY s.name ASC, s.id ASC LIMIT 2`,
			filter.NPSN,
		)
		if err != nil {
			return nil, "", err
		}
		if len(rows) > 1 {
			return nil, "", ErrAmbiguousSchoolIdentity
		}
		return rows, "", nil
	}

	query := `WHERE s.status = 'active' AND p.id = $1 AND LOWER(s.name) LIKE $2 ESCAPE '\'`
	args := []any{filter.ProvinceID, "%" + escapeLike(strings.ToLower(filter.Q)) + "%"}
	argNum := 3
	if filter.Category != "" {
		query += fmt.Sprintf(` AND s.category = $%d`, argNum)
		args = append(args, filter.Category)
		argNum++
	}
	if filter.Cursor != "" {
		lastName, lastID, err := decodeSchoolSearchCursor(filter.Cursor, filter)
		if err != nil {
			return nil, "", err
		}
		query += fmt.Sprintf(` AND (s.name, s.id) > ($%d, $%d::uuid)`, argNum, argNum+1)
		args = append(args, lastName, lastID.String())
		argNum += 2
	}
	query += fmt.Sprintf(` ORDER BY s.name ASC, s.id ASC LIMIT $%d`, argNum)
	args = append(args, filter.Limit+1)

	rows, err := r.querySchoolOptions(ctx, query, args...)
	if err != nil {
		return nil, "", err
	}
	nextCursor := ""
	if len(rows) > filter.Limit {
		rows = rows[:filter.Limit]
		nextCursor = encodeSchoolSearchCursor(filter, rows[len(rows)-1])
	}
	return rows, nextCursor, nil
}

func (r *Repository) querySchoolOptions(ctx context.Context, where string, args ...any) ([]SchoolOption, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT s.id, s.name, s.code, s.npsn, s.school_types, s.alamat, s.status,
			s.category, s.city_id, c.name, p.id, p.name
		FROM school s
		LEFT JOIN city c ON c.id = s.city_id
		LEFT JOIN province p ON p.id = c.province_id
		`+where,
		args...,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	options := []SchoolOption{}
	for rows.Next() {
		var o SchoolOption
		if err := rows.Scan(
			&o.ID, &o.Name, &o.Code, &o.NPSN, &o.SchoolTypes, &o.Alamat, &o.Status,
			&o.Category, &o.CityID, &o.CityName, &o.ProvinceID, &o.ProvinceName,
		); err != nil {
			return nil, err
		}
		options = append(options, o)
	}
	return options, rows.Err()
}

func (r *Repository) GetSchoolOptionByID(ctx context.Context, id string) (*SchoolOption, error) {
	rows, err := r.querySchoolOptions(ctx, `WHERE s.id = $1`, id)
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, nil
	}
	return &rows[0], nil
}

func escapeLike(s string) string {
	replacer := strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`)
	return replacer.Replace(s)
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

// UpdateSchool patches editable fields. npsnSet distinguishes an omitted NPSN
// from an explicit blank value normalized to NULL by the service.
func (r *Repository) UpdateSchool(ctx context.Context, id string, name *string, npsnSet bool, npsn, alamat *string, schoolTypes []string, code *string) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE school
		SET name = COALESCE($1, name),
			npsn = CASE WHEN $2 THEN $3 ELSE npsn END,
			alamat = COALESCE($4, alamat),
			school_types = COALESCE($5, school_types),
			code = COALESCE($6, code),
			updated_at = now()
		WHERE id = $7`,
		name, npsnSet, npsn, alamat, schoolTypes, code, id,
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

func (r *Repository) GetSchoolByNPSN(ctx context.Context, npsn string) (*model.School, error) {
	s := &model.School{}
	err := r.pool.QueryRow(ctx,
		`SELECT id, name, code, npsn, school_types, alamat, status, created_at, updated_at
		FROM school WHERE UPPER(BTRIM(npsn)) = $1`,
		npsn,
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
