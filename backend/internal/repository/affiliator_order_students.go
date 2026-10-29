package repository

import (
	"context"

	"github.com/google/uuid"
)

// AffiliatorOrderStudent is a student an affiliator may buy for, with the school
// shown on the order preview (empty for a student without a school).
type AffiliatorOrderStudent struct {
	ID         uuid.UUID `json:"student_id"`
	Name       string    `json:"name"`
	SchoolID   *string   `json:"school_id"`
	SchoolName *string   `json:"school_name"`
}

// ListAffiliatorOrderStudents returns the non-deleted students among ids that sit
// in one of schoolIDs, or have no school and were registered by affiliatorID.
// Ids outside that scope are simply absent from the result.
func (r *Repository) ListAffiliatorOrderStudents(ctx context.Context, ids []uuid.UUID, schoolIDs []string, affiliatorID string) ([]AffiliatorOrderStudent, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT u.id, u.name, u.school_id::text, s.name
		FROM users u
		LEFT JOIN school s ON s.id = u.school_id
		WHERE u.id = ANY($1::uuid[]) AND u.role = 'student' AND u.status != 'deleted'
		  AND (u.school_id = ANY($2::uuid[]) OR (u.school_id IS NULL AND u.registered_by = $3::uuid))
		ORDER BY s.name NULLS LAST, u.name, u.id`, ids, schoolIDs, affiliatorID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []AffiliatorOrderStudent{}
	for rows.Next() {
		var st AffiliatorOrderStudent
		if err := rows.Scan(&st.ID, &st.Name, &st.SchoolID, &st.SchoolName); err != nil {
			return nil, err
		}
		out = append(out, st)
	}
	return out, rows.Err()
}
