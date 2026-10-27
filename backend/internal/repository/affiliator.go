package repository

import (
	"context"
	"fmt"
	"time"

	"akademi-bimbel/internal/model"
)

// AffiliatorRow is the account shape returned by the affiliator admin list.
type AffiliatorRow struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Email     *string   `json:"email"`
	Status    string    `json:"status"`
	SchoolIDs []string  `json:"school_ids"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// AffiliatorFilter carries optional filters for ListAffiliators.
type AffiliatorFilter struct {
	Status string
	Cursor string
	Limit  int
}

// CreateAffiliatorUser inserts an active affiliator with its allowed schools in one statement.
func (r *Repository) CreateAffiliatorUser(ctx context.Context, u *model.User, schoolIDs []string) error {
	u.Email = normalizeOptionalEmail(u.Email)
	return r.pool.QueryRow(ctx,
		`INSERT INTO users (email, name, password_hash, role, school_ids, status, otp_enabled)
		 VALUES ($1, $2, $3, 'affiliator', $4::uuid[], 'active', false)
		 RETURNING id, created_at, updated_at`,
		u.Email, u.Name, u.PasswordHash, schoolIDs,
	).Scan(&u.ID, &u.CreatedAt, &u.UpdatedAt)
}

// PromoteSchoolAdminToAffiliator turns an admin_school account into an affiliator.
// It reports false when the account is not a non-deleted admin_school.
func (r *Repository) PromoteSchoolAdminToAffiliator(ctx context.Context, id string, schoolIDs []string) (bool, error) {
	tag, err := r.pool.Exec(ctx,
		`UPDATE users SET role = 'affiliator', school_id = NULL, school_ids = $2::uuid[], updated_at = now()
		 WHERE id = $1 AND role = 'admin_school' AND status != 'deleted'`,
		id, schoolIDs,
	)
	if err != nil {
		return false, err
	}
	return tag.RowsAffected() == 1, nil
}

// SetAffiliatorSchoolIDs replaces an affiliator's allowed schools.
// It reports false when the account is not a non-deleted affiliator.
func (r *Repository) SetAffiliatorSchoolIDs(ctx context.Context, id string, schoolIDs []string) (bool, error) {
	tag, err := r.pool.Exec(ctx,
		`UPDATE users SET school_ids = $2::uuid[], updated_at = now()
		 WHERE id = $1 AND role = 'affiliator' AND status != 'deleted'`,
		id, schoolIDs,
	)
	if err != nil {
		return false, err
	}
	return tag.RowsAffected() == 1, nil
}

// GetAffiliatorSchoolIDs returns an affiliator's allowed schools; nil when the account is not an affiliator.
func (r *Repository) GetAffiliatorSchoolIDs(ctx context.Context, id string) ([]string, error) {
	var ids []string
	err := r.pool.QueryRow(ctx,
		`SELECT school_ids::text[] FROM users WHERE id = $1 AND role = 'affiliator' AND status != 'deleted'`,
		id,
	).Scan(&ids)
	if err != nil {
		if isNotFound(err) {
			return nil, nil
		}
		return nil, err
	}
	return ids, nil
}

// ListAffiliators returns non-deleted affiliators, newest first, cursor-paginated by user id.
func (r *Repository) ListAffiliators(ctx context.Context, filter AffiliatorFilter) ([]AffiliatorRow, string, error) {
	if filter.Limit <= 0 {
		filter.Limit = 20
	}
	if filter.Limit > 100 {
		filter.Limit = 100
	}

	query := `SELECT id, name, email, status, school_ids::text[], created_at, updated_at
		FROM users WHERE role = 'affiliator' AND status != 'deleted'`
	args := []any{}
	argNum := 1

	if filter.Status != "" {
		query += fmt.Sprintf(` AND status = $%d`, argNum)
		args = append(args, filter.Status)
		argNum++
	}
	if filter.Cursor != "" {
		query += fmt.Sprintf(` AND (created_at, id) < (SELECT created_at, id FROM users WHERE id = $%d::uuid)`, argNum)
		args = append(args, filter.Cursor)
		argNum++
	}
	query += fmt.Sprintf(` ORDER BY created_at DESC, id DESC LIMIT $%d`, argNum)
	args = append(args, filter.Limit+1)

	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, "", err
	}
	defer rows.Close()

	var out []AffiliatorRow
	nextCursor := ""
	for rows.Next() {
		var a AffiliatorRow
		if err := rows.Scan(&a.ID, &a.Name, &a.Email, &a.Status, &a.SchoolIDs, &a.CreatedAt, &a.UpdatedAt); err != nil {
			return nil, "", err
		}
		if len(out) < filter.Limit {
			out = append(out, a)
		} else {
			nextCursor = out[len(out)-1].ID
			break
		}
	}
	if err := rows.Err(); err != nil {
		return nil, "", err
	}
	return out, nextCursor, nil
}
