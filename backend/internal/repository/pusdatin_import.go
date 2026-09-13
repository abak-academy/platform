package repository

import (
	"context"
	"errors"

	"akademi-bimbel/internal/model"
	"github.com/jackc/pgx/v5"
)

var ErrMissingSchoolNPSNIndex = errors.New("missing externally managed school NPSN index")

type PusdatinSchoolTarget struct {
	ID          string
	NPSN        *string
	Name        string
	Alamat      *string
	SchoolTypes []string
	Category    *string
	CityID      *string
}

func (r *Repository) VerifySchoolNPSNImportIndex(ctx context.Context) error {
	var ok bool
	err := r.pool.QueryRow(ctx,
		`SELECT EXISTS(
			SELECT 1
			FROM pg_index i
			JOIN pg_class c ON c.oid = i.indexrelid
			JOIN pg_class t ON t.oid = i.indrelid
			WHERE t.relname = 'school'
				AND c.relname = 'uq_school_npsn_normalized'
				AND i.indisunique
				AND i.indisvalid
				AND i.indisready
				AND pg_get_indexdef(i.indexrelid) ILIKE '%upper(btrim(npsn))%'
				AND pg_get_expr(i.indpred, i.indrelid) ILIKE '%npsn IS NOT NULL%'
		)`,
	).Scan(&ok)
	if err != nil {
		return err
	}
	if !ok {
		return ErrMissingSchoolNPSNIndex
	}
	return nil
}

func (r *Repository) LoadPusdatinSchoolTargets(ctx context.Context, npsns []string) (map[string]PusdatinSchoolTarget, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT UPPER(BTRIM(npsn)), id, npsn, name, alamat, school_types, category, city_id
		FROM school
		WHERE npsn IS NOT NULL AND UPPER(BTRIM(npsn)) = ANY($1)`,
		npsns,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := map[string]PusdatinSchoolTarget{}
	for rows.Next() {
		var normalized string
		var row PusdatinSchoolTarget
		if err := rows.Scan(&normalized, &row.ID, &row.NPSN, &row.Name, &row.Alamat, &row.SchoolTypes, &row.Category, &row.CityID); err != nil {
			return nil, err
		}
		if _, exists := out[normalized]; exists {
			return nil, ErrAmbiguousSchoolIdentity
		}
		out[normalized] = row
	}
	return out, rows.Err()
}

func (r *Repository) ApplyPusdatinSchools(ctx context.Context, rows []model.PusdatinTransformedSchool) error {
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	for _, row := range rows {
		_, err := tx.Exec(ctx,
			`INSERT INTO school (name, code, npsn, school_types, alamat, status, category, city_id)
			VALUES ($1, $2, $3, $4, $5, 'active', $6, $7)
			ON CONFLICT (UPPER(BTRIM(npsn))) WHERE npsn IS NOT NULL
			DO UPDATE SET
				name = EXCLUDED.name,
				alamat = EXCLUDED.alamat,
				school_types = EXCLUDED.school_types,
				category = EXCLUDED.category,
				city_id = EXCLUDED.city_id,
				updated_at = now()
			WHERE school.name IS DISTINCT FROM EXCLUDED.name
				OR school.alamat IS DISTINCT FROM EXCLUDED.alamat
				OR school.school_types IS DISTINCT FROM EXCLUDED.school_types
				OR school.category IS DISTINCT FROM EXCLUDED.category
				OR school.city_id IS DISTINCT FROM EXCLUDED.city_id`,
			row.Name, "PUSDATIN-"+row.NPSN, row.NPSN, row.SchoolTypes, row.Alamat, row.Category, row.CityID,
		)
		if err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}
