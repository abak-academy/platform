package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"akademi-bimbel/internal/model"

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

// ListSchoolsByIDs returns the schools with the given ids, by name.
func (r *Repository) ListSchoolsByIDs(ctx context.Context, ids []string) ([]model.School, error) {
	schools := []model.School{}
	if len(ids) == 0 {
		return schools, nil
	}
	rows, err := r.pool.Query(ctx,
		`SELECT id, name, code, npsn, school_types, alamat, provinsi_id, kota_id, status, created_at, updated_at
		FROM school WHERE id = ANY($1::uuid[]) ORDER BY name`,
		ids,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var s model.School
		if err := rows.Scan(&s.ID, &s.Name, &s.Code, &s.NPSN, &s.SchoolTypes, &s.Alamat,
			&s.ProvinsiID, &s.KotaID, &s.Status, &s.CreatedAt, &s.UpdatedAt); err != nil {
			return nil, err
		}
		schools = append(schools, s)
	}
	return schools, rows.Err()
}

// PartnerOrderStudent is a student named on a Mitra order.
type PartnerOrderStudent struct {
	ID         uuid.UUID `json:"id"`
	Name       string    `json:"name"`
	SchoolName *string   `json:"school_name"`
}

// ListPartnerOrderStudents returns, of the given students, those a Mitra may
// order for: active students of an allowed school, or registered by the Mitra.
func (r *Repository) ListPartnerOrderStudents(ctx context.Context, ids []uuid.UUID, allowedSchoolIDs []string, partnerID string) ([]PartnerOrderStudent, error) {
	if allowedSchoolIDs == nil {
		allowedSchoolIDs = []string{}
	}
	rows, err := r.pool.Query(ctx,
		`SELECT u.id, u.name, s.name
		 FROM users u LEFT JOIN school s ON s.id = u.school_id
		 WHERE u.id = ANY($1) AND u.role = 'student' AND u.status = 'active'
		   AND (u.school_id = ANY($2::uuid[]) OR u.registered_by = $3::uuid)`,
		ids, allowedSchoolIDs, partnerID,
	)
	if err != nil {
		return nil, err
	}
	return scanPartnerOrderStudents(rows)
}

// ListOrderParticipantStudents names the students an order grants access to.
func (r *Repository) ListOrderParticipantStudents(ctx context.Context, orderID uuid.UUID) ([]PartnerOrderStudent, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT u.id, u.name, s.name
		 FROM order_participant op
		 JOIN users u ON u.id = op.student_id
		 LEFT JOIN school s ON s.id = u.school_id
		 WHERE op.order_id = $1 ORDER BY u.name`,
		orderID,
	)
	if err != nil {
		return nil, err
	}
	return scanPartnerOrderStudents(rows)
}

func scanPartnerOrderStudents(rows pgx.Rows) ([]PartnerOrderStudent, error) {
	defer rows.Close()
	out := []PartnerOrderStudent{}
	for rows.Next() {
		var s PartnerOrderStudent
		if err := rows.Scan(&s.ID, &s.Name, &s.SchoolName); err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

// PartnerDashboardCounts are the figures on a Mitra's home page.
type PartnerDashboardCounts struct {
	StudentsRegistered int `json:"students_registered"`
	OrdersPaid         int `json:"orders_paid"`
	OrdersUnpaid       int `json:"orders_unpaid"`
}

// GetPartnerDashboardCounts counts the students the Mitra registered and its
// orders: paid ones, and ones still waiting for payment.
func (r *Repository) GetPartnerDashboardCounts(ctx context.Context, partnerID string) (PartnerDashboardCounts, error) {
	var c PartnerDashboardCounts
	err := r.pool.QueryRow(ctx,
		`SELECT
		   (SELECT count(*) FROM users WHERE registered_by = $1::uuid AND role = 'student' AND status != 'deleted'),
		   (SELECT count(*) FROM orders WHERE student_id = $1::uuid AND status IN ('paid', 'processing', 'shipped', 'completed')),
		   (SELECT count(*) FROM orders WHERE student_id = $1::uuid AND status IN ('cart', 'payment_pending'))`,
		partnerID,
	).Scan(&c.StudentsRegistered, &c.OrdersPaid, &c.OrdersUnpaid)
	return c, err
}
