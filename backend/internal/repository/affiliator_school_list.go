package repository

import "context"

// AffiliatorSchoolQuery filters the affiliator-to-school assignments. Q matches the
// affiliator name or email and the school name, code or NPSN; AffiliatorID and
// SchoolID are exact. Empty means no filter.
type AffiliatorSchoolQuery struct {
	Q            string
	AffiliatorID string
	SchoolID     string
	Limit        int
	Offset       int
}

// AffiliatorSchoolRow is one school assigned to one affiliator.
type AffiliatorSchoolRow struct {
	AffiliatorID     string  `json:"affiliator_id"`
	AffiliatorName   string  `json:"affiliator_name"`
	AffiliatorEmail  *string `json:"affiliator_email"`
	AffiliatorStatus string  `json:"affiliator_status"`
	SchoolID         string  `json:"school_id"`
	SchoolName       string  `json:"school_name"`
	SchoolCode       string  `json:"school_code"`
	SchoolNPSN       *string `json:"school_npsn"`
	SchoolStatus     string  `json:"school_status"`
	StudentCount     int     `json:"student_count"`
}

// ListAffiliatorSchools returns the assignments sorted by school then affiliator,
// with the total that matches the filters. A school id kept in an affiliator's
// list that no longer exists is left out, since there is nothing to show for it.
func (r *Repository) ListAffiliatorSchools(ctx context.Context, q AffiliatorSchoolQuery) ([]AffiliatorSchoolRow, int, error) {
	limit, offset := q.Limit, q.Offset
	if limit <= 0 {
		limit = 20
	}
	if limit > 10000 {
		limit = 10000
	}
	if offset < 0 {
		offset = 0
	}

	rows, err := r.pool.Query(ctx, `
		WITH r AS (
			SELECT a.id::text AS affiliator_id, a.name AS affiliator_name, a.email AS affiliator_email, a.status AS affiliator_status,
				s.id::text AS school_id, s.name AS school_name, s.code AS school_code, s.npsn AS school_npsn, s.status AS school_status,
				(SELECT COUNT(*) FROM users st WHERE st.role = 'student' AND st.status != 'deleted' AND st.school_id = s.id) AS student_count
			FROM users a
			CROSS JOIN LATERAL unnest(a.school_ids) AS assigned(school_id)
			JOIN school s ON s.id = assigned.school_id
			WHERE a.role = 'affiliator' AND a.status != 'deleted'
			  AND ($1 = '' OR a.name ILIKE '%' || $1 || '%' OR a.email ILIKE '%' || $1 || '%'
			       OR s.name ILIKE '%' || $1 || '%' OR s.code ILIKE '%' || $1 || '%' OR s.npsn ILIKE '%' || $1 || '%')
			  AND ($2 = '' OR a.id = NULLIF($2, '')::uuid)
			  AND ($3 = '' OR s.id = NULLIF($3, '')::uuid)
		)
		SELECT r.*, COUNT(*) OVER () FROM r
		ORDER BY r.school_name, r.affiliator_name, r.affiliator_id
		LIMIT $4 OFFSET $5`,
		q.Q, q.AffiliatorID, q.SchoolID, limit, offset,
	)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	out := []AffiliatorSchoolRow{}
	total := 0
	for rows.Next() {
		var a AffiliatorSchoolRow
		if err := rows.Scan(&a.AffiliatorID, &a.AffiliatorName, &a.AffiliatorEmail, &a.AffiliatorStatus,
			&a.SchoolID, &a.SchoolName, &a.SchoolCode, &a.SchoolNPSN, &a.SchoolStatus, &a.StudentCount, &total); err != nil {
			return nil, 0, err
		}
		out = append(out, a)
	}
	return out, total, rows.Err()
}
