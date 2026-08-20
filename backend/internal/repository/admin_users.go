package repository

import (
	"context"
	"fmt"
	"time"

	"akademi-bimbel/internal/model"
)

// AdminUserRow is the trimmed account shape returned in admin list responses
// (no student-only fields, no password_hash).
type AdminUserRow struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Email     *string   `json:"email"`
	Role      string    `json:"role"`
	Status    string    `json:"status"`
	SchoolID  *string   `json:"school_id"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// AdminUserFilter carries optional filters for ListAdminUsers.
type AdminUserFilter struct {
	Role   string
	Status string
	Cursor string
	Limit  int
}

// ListAdminUsers returns admin-role users (role != 'student') that are not
// deleted, optionally filtered by role and/or status, cursor-paginated.
func (r *Repository) ListAdminUsers(ctx context.Context, filter AdminUserFilter) ([]AdminUserRow, string, error) {
	if filter.Limit <= 0 {
		filter.Limit = 20
	}
	if filter.Limit > 100 {
		filter.Limit = 100
	}

	query := `SELECT id, name, email, role, status, school_id, created_at, updated_at
		FROM users WHERE role != 'student' AND status != 'deleted'`
	args := []any{}
	argNum := 1

	if filter.Role != "" {
		query += fmt.Sprintf(` AND role = $%d`, argNum)
		args = append(args, filter.Role)
		argNum++
	}
	if filter.Status != "" {
		query += fmt.Sprintf(` AND status = $%d`, argNum)
		args = append(args, filter.Status)
		argNum++
	}
	if filter.Cursor != "" {
		query += fmt.Sprintf(` AND id < $%d::uuid`, argNum)
		args = append(args, filter.Cursor)
		argNum++
	}

	query += fmt.Sprintf(` ORDER BY created_at DESC LIMIT $%d`, argNum)
	args = append(args, filter.Limit+1)

	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, "", err
	}
	defer rows.Close()

	var accounts []AdminUserRow
	nextCursor := ""

	for rows.Next() {
		var a AdminUserRow
		if err := rows.Scan(&a.ID, &a.Name, &a.Email, &a.Role, &a.Status, &a.SchoolID, &a.CreatedAt, &a.UpdatedAt); err != nil {
			return nil, "", err
		}
		if len(accounts) < filter.Limit {
			accounts = append(accounts, a)
		} else {
			nextCursor = a.ID
		}
	}

	if err = rows.Err(); err != nil {
		return nil, "", err
	}

	return accounts, nextCursor, nil
}

// CreateAdminUser inserts a new admin user. Only admin-relevant fields are set;
// student-specific columns are left as NULL. The user is created as active with
// OTP disabled.
func (r *Repository) CreateAdminUser(ctx context.Context, u *model.User) error {
	u.Email = normalizeOptionalEmail(u.Email)
	return r.pool.QueryRow(ctx,
		`INSERT INTO users (email, name, password_hash, role, school_id, status, otp_enabled)
		 VALUES ($1, $2, $3, $4, $5, 'active', false)
		 RETURNING id, created_at, updated_at`,
		u.Email, u.Name, u.PasswordHash, u.Role, u.SchoolID,
	).Scan(&u.ID, &u.CreatedAt, &u.UpdatedAt)
}

// GetAdminUserByID returns a non-deleted user by ID. Returns nil, nil when not
// found (as opposed to a deleted user which is excluded by the WHERE clause).
func (r *Repository) GetAdminUserByID(ctx context.Context, id string) (*model.User, error) {
	u := &model.User{}
	err := r.pool.QueryRow(ctx,
		`SELECT id, email, username, phone, password_hash, role, name,
			school_id, photo_url, status, otp_enabled, auth_provider, created_at, updated_at,
			jenjang, provinsi_id, kota_id, kecamatan_id, kode_pos,
			dob, gender, grade, alamat_domisili, target_exam
		FROM users
		WHERE id = $1 AND status != 'deleted'`,
		id,
	).Scan(
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

// UpdateAdminUserRole sets the user's role. Does nothing for deleted users.
func (r *Repository) UpdateAdminUserRole(ctx context.Context, id, role string) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE users SET role = $1, updated_at = now() WHERE id = $2 AND status != 'deleted'`,
		role, id,
	)
	return err
}

// UpdateAdminUserStatus sets the user's status. Does nothing for deleted users.
func (r *Repository) UpdateAdminUserStatus(ctx context.Context, id, status string) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE users SET status = $1, updated_at = now() WHERE id = $2 AND status != 'deleted'`,
		status, id,
	)
	return err
}

// SetUserSchoolID sets or clears the school_id on a user.
func (r *Repository) SetUserSchoolID(ctx context.Context, userID string, schoolID *string) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE users SET school_id = $1, updated_at = now() WHERE id = $2 AND status != 'deleted'`,
		schoolID, userID,
	)
	return err
}
