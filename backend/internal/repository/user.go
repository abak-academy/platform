package repository

import (
	"context"
	"strings"
	"time"

	"akademi-bimbel/internal/model"

	"github.com/google/uuid"
)

func normalizeEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}

// normalizeOptionalEmail treats missing / whitespace-only as SQL NULL so
// blank emails do not occupy idx_users_email_active (empty string is NOT NULL).
func normalizeOptionalEmail(email *string) *string {
	if email == nil {
		return nil
	}
	n := normalizeEmail(*email)
	if n == "" {
		return nil
	}
	return &n
}

func (r *Repository) CreateUser(ctx context.Context, u *model.User) error {
	u.Email = normalizeOptionalEmail(u.Email)
	if u.AuthProvider == "" {
		u.AuthProvider = "password"
	}
	return r.pool.QueryRow(ctx,
		`INSERT INTO users (
			email, username, phone, password_hash, role, name,
			school_id, photo_url, status, otp_enabled, auth_provider,
			jenjang, provinsi_id, kota_id, kecamatan_id, kode_pos,
			dob, gender, grade, alamat_domisili, target_exam
		) VALUES (
			$1, $2, $3, $4, $5, $6,
			$7, $8, $9, $10, $11,
			$12, $13, $14, $15, $16,
			$17, $18, $19, $20, $21
		) RETURNING id, created_at, updated_at`,
		u.Email, u.Username, u.Phone, u.PasswordHash, u.Role, u.Name,
		u.SchoolID, u.PhotoURL, u.Status, u.OTPEnabled, u.AuthProvider,
		u.Jenjang, u.ProvinsiID, u.KotaID, u.KecamatanID, u.KodePos,
		u.DOB, u.Gender, u.Grade, u.AlamatDomisili, u.TargetExam,
	).Scan(&u.ID, &u.CreatedAt, &u.UpdatedAt)
}

func (r *Repository) GetUserByEmail(ctx context.Context, email string) (*model.User, error) {
	email = normalizeEmail(email)
	u := &model.User{}
	err := r.pool.QueryRow(ctx,
		`SELECT id, email, username, phone, password_hash, role, name,
			school_id, photo_url, status, otp_enabled, auth_provider, created_at, updated_at,
			jenjang, provinsi_id, kota_id, kecamatan_id, kode_pos,
			unlisted_school_name, dob, gender, grade, alamat_domisili, target_exam
		FROM users
		WHERE email = $1 AND status != 'deleted'`,
		email,
	).Scan(
		&u.ID, &u.Email, &u.Username, &u.Phone, &u.PasswordHash, &u.Role, &u.Name,
		&u.SchoolID, &u.PhotoURL, &u.Status, &u.OTPEnabled, &u.AuthProvider, &u.CreatedAt, &u.UpdatedAt,
		&u.Jenjang, &u.ProvinsiID, &u.KotaID, &u.KecamatanID, &u.KodePos,
		&u.UnlistedSchoolName, &u.DOB, &u.Gender, &u.Grade, &u.AlamatDomisili, &u.TargetExam,
	)
	if err != nil {
		if isNotFound(err) {
			return nil, nil
		}
		return nil, err
	}
	return u, nil
}

func (r *Repository) GetUserByUsername(ctx context.Context, username string) (*model.User, error) {
	u := &model.User{}
	err := r.pool.QueryRow(ctx,
		`SELECT id, email, username, phone, password_hash, role, name,
			school_id, photo_url, status, otp_enabled, auth_provider, created_at, updated_at,
			jenjang, provinsi_id, kota_id, kecamatan_id, kode_pos,
			unlisted_school_name, dob, gender, grade, alamat_domisili, target_exam
		FROM users
		WHERE username = $1 AND status != 'deleted'`,
		username,
	).Scan(
		&u.ID, &u.Email, &u.Username, &u.Phone, &u.PasswordHash, &u.Role, &u.Name,
		&u.SchoolID, &u.PhotoURL, &u.Status, &u.OTPEnabled, &u.AuthProvider, &u.CreatedAt, &u.UpdatedAt,
		&u.Jenjang, &u.ProvinsiID, &u.KotaID, &u.KecamatanID, &u.KodePos,
		&u.UnlistedSchoolName, &u.DOB, &u.Gender, &u.Grade, &u.AlamatDomisili, &u.TargetExam,
	)
	if err != nil {
		if isNotFound(err) {
			return nil, nil
		}
		return nil, err
	}
	return u, nil
}

func (r *Repository) GetUserByID(ctx context.Context, id string) (*model.User, error) {
	u := &model.User{}
	err := r.pool.QueryRow(ctx,
		`SELECT id, email, username, phone, password_hash, role, name,
			school_id, photo_url, status, otp_enabled, auth_provider, created_at, updated_at,
			jenjang, provinsi_id, kota_id, kecamatan_id, kode_pos,
			unlisted_school_name, dob, gender, grade, alamat_domisili, target_exam
		FROM users
		WHERE id = $1`,
		id,
	).Scan(
		&u.ID, &u.Email, &u.Username, &u.Phone, &u.PasswordHash, &u.Role, &u.Name,
		&u.SchoolID, &u.PhotoURL, &u.Status, &u.OTPEnabled, &u.AuthProvider, &u.CreatedAt, &u.UpdatedAt,
		&u.Jenjang, &u.ProvinsiID, &u.KotaID, &u.KecamatanID, &u.KodePos,
		&u.UnlistedSchoolName, &u.DOB, &u.Gender, &u.Grade, &u.AlamatDomisili, &u.TargetExam,
	)
	if err != nil {
		if isNotFound(err) {
			return nil, nil
		}
		return nil, err
	}
	return u, nil
}

func (r *Repository) UpdatePasswordHash(ctx context.Context, userID, hash string) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE users SET password_hash = $1, updated_at = now() WHERE id = $2`,
		hash, userID,
	)
	return err
}

// ActivateUser transitions a pending_verification user to active in a single
// UPDATE, so verification is atomic (no read-modify-write).
func (r *Repository) ActivateUser(ctx context.Context, userID string) (bool, error) {
	tag, err := r.pool.Exec(ctx,
		`UPDATE users SET status = 'active', otp_enabled = false, updated_at = now()
		WHERE id = $1 AND status = 'pending_verification'`,
		userID,
	)
	return tag.RowsAffected() == 1, err
}

// UpdateUserProfile patches the editable profile fields. nil args leave most
// columns unchanged via COALESCE. school_id/unlisted_school_name are the
// exception: COALESCE can never write NULL into them, so applySchool gates
// them explicitly — false leaves both untouched, true writes schoolID/
// unlistedSchoolName verbatim (including NULL when the pointer is nil).
func (r *Repository) UpdateUserProfile(ctx context.Context, userID string, name, email, username, phone, address, targetExam *string, grade *int, dob *time.Time, applySchool bool, schoolID *string, unlistedSchoolName *string, jenjang *string, provinsiID, kotaID, kecamatanID, kodePos *string) error {
	email = normalizeOptionalEmail(email)
	_, err := r.pool.Exec(ctx,
		`UPDATE users
		SET name = COALESCE($1, name),
		    email = COALESCE($2, email),
		    username = COALESCE($3, username),
		    phone = COALESCE($4, phone),
		    alamat_domisili = COALESCE($5, alamat_domisili),
		    target_exam = COALESCE($6, target_exam),
		    grade = COALESCE($7, grade),
		    dob = COALESCE($8, dob),
		    school_id = CASE WHEN $9 THEN $10::uuid ELSE school_id END,
		    unlisted_school_name = CASE WHEN $9 THEN $11 ELSE unlisted_school_name END,
		    jenjang = COALESCE($12, jenjang),
		    provinsi_id = COALESCE($13, provinsi_id),
		    kota_id = COALESCE($14, kota_id),
		    kecamatan_id = COALESCE($15, kecamatan_id),
		    kode_pos = COALESCE($16, kode_pos),
		    updated_at = now()
		WHERE id = $17`,
		name, email, username, phone, address, targetExam, grade, dob, applySchool, schoolID, unlistedSchoolName, jenjang, provinsiID, kotaID, kecamatanID, kodePos, userID,
	)
	return err
}

// UpdateUserPhoto sets the user's avatar URL.
func (r *Repository) UpdateUserPhoto(ctx context.Context, userID, photoURL string) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE users SET photo_url = $1, updated_at = now() WHERE id = $2`,
		photoURL, userID,
	)
	return err
}

// ListSchools returns active schools ordered by name.
func (r *Repository) ListSchools(ctx context.Context) ([]*model.School, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT id, name, code, school_types FROM school WHERE status = 'active' ORDER BY name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	schools := []*model.School{}
	for rows.Next() {
		s := &model.School{}
		if err := rows.Scan(&s.ID, &s.Name, &s.Code, &s.SchoolTypes); err != nil {
			return nil, err
		}
		schools = append(schools, s)
	}
	return schools, rows.Err()
}

// GetUsersByIDs returns only users with role='student' for the given IDs.
// Used by the direct exam grant path to batch-validate existence + role
// (no school-boundary filter — super_admin has none).
func (r *Repository) GetUsersByIDs(ctx context.Context, ids []uuid.UUID) ([]model.User, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT id, email, username, phone, password_hash, role, name,
			school_id, photo_url, status, otp_enabled, auth_provider, created_at, updated_at,
			jenjang, provinsi_id, kota_id, kecamatan_id, kode_pos,
			unlisted_school_name, dob, gender, grade, alamat_domisili, target_exam
		FROM users
		WHERE id = ANY($1) AND role = 'student'`,
		ids,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var users []model.User
	for rows.Next() {
		var u model.User
		if err := rows.Scan(
			&u.ID, &u.Email, &u.Username, &u.Phone, &u.PasswordHash, &u.Role, &u.Name,
			&u.SchoolID, &u.PhotoURL, &u.Status, &u.OTPEnabled, &u.AuthProvider, &u.CreatedAt, &u.UpdatedAt,
			&u.Jenjang, &u.ProvinsiID, &u.KotaID, &u.KecamatanID, &u.KodePos,
			&u.UnlistedSchoolName, &u.DOB, &u.Gender, &u.Grade, &u.AlamatDomisili, &u.TargetExam,
		); err != nil {
			return nil, err
		}
		users = append(users, u)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if users == nil {
		users = []model.User{}
	}
	return users, nil
}

func (r *Repository) TombstoneUser(ctx context.Context, userID string) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE users
		SET status = 'deleted', name = '[deleted user]',
		    email = NULL, phone = NULL, alamat_domisili = NULL,
		    updated_at = now()
		WHERE id = $1`,
		userID,
	)
	return err
}
