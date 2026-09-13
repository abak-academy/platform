package repository

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
)

type PusdatinSchoolInput struct {
	NPSN        string
	Name        string
	Alamat      *string
	SchoolTypes []string
	Category    string
	ProvinsiID  string
	KotaID      string
}

type PusdatinSchoolImage struct {
	ID          string   `json:"id"`
	NPSN        *string  `json:"npsn"`
	Name        string   `json:"name"`
	Alamat      *string  `json:"alamat"`
	SchoolTypes []string `json:"school_types"`
	Category    *string  `json:"category"`
	ProvinsiID  *string  `json:"provinsi_id"`
	KotaID      *string  `json:"kota_id"`
}

type PusdatinImportManifestRow struct {
	NPSN     string               `json:"npsn"`
	Inserted bool                 `json:"inserted"`
	Before   *PusdatinSchoolImage `json:"before,omitempty"`
	After    PusdatinSchoolImage  `json:"after"`
}

type PusdatinImportManifest struct {
	ReviewedChecksum string                      `json:"reviewed_checksum"`
	Rows             []PusdatinImportManifestRow `json:"rows"`
}

var (
	ErrMissingSchoolNPSNIndex  = errors.New("missing externally managed school NPSN index")
	ErrAmbiguousSchoolIdentity = errors.New("ambiguous school identity")
)

type PusdatinSchoolTarget struct {
	ID          string
	NPSN        *string
	Name        string
	Alamat      *string
	SchoolTypes []string
	Category    *string
	ProvinsiID  *string
	KotaID      *string
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
		`SELECT UPPER(BTRIM(npsn)), id, npsn, name, alamat, school_types, category, provinsi_id, kota_id
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
		if err := rows.Scan(&normalized, &row.ID, &row.NPSN, &row.Name, &row.Alamat, &row.SchoolTypes, &row.Category, &row.ProvinsiID, &row.KotaID); err != nil {
			return nil, err
		}
		if _, exists := out[normalized]; exists {
			return nil, ErrAmbiguousSchoolIdentity
		}
		out[normalized] = row
	}
	return out, rows.Err()
}

func (r *Repository) ApplyPusdatinSchools(ctx context.Context, rows []PusdatinSchoolInput) error {
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	for _, row := range rows {
		_, err := tx.Exec(ctx,
			`INSERT INTO school (name, code, npsn, school_types, alamat, status, category, provinsi_id, kota_id)
			VALUES ($1, $2, $3, $4, $5, 'active', $6, $7, $8)
			ON CONFLICT (UPPER(BTRIM(npsn))) WHERE npsn IS NOT NULL
			DO UPDATE SET
				name = EXCLUDED.name,
				alamat = EXCLUDED.alamat,
				school_types = EXCLUDED.school_types,
				category = EXCLUDED.category,
				provinsi_id = EXCLUDED.provinsi_id,
				kota_id = EXCLUDED.kota_id,
				updated_at = now()
			WHERE school.name IS DISTINCT FROM EXCLUDED.name
				OR school.alamat IS DISTINCT FROM EXCLUDED.alamat
				OR school.school_types IS DISTINCT FROM EXCLUDED.school_types
				OR school.category IS DISTINCT FROM EXCLUDED.category
				OR school.provinsi_id IS DISTINCT FROM EXCLUDED.provinsi_id
				OR school.kota_id IS DISTINCT FROM EXCLUDED.kota_id`,
			row.Name, "PUSDATIN-"+row.NPSN, row.NPSN, row.SchoolTypes, row.Alamat, row.Category, row.ProvinsiID, row.KotaID,
		)
		if err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}

func (r *Repository) LoadPusdatinSchoolImages(ctx context.Context, npsns []string) (map[string]PusdatinSchoolImage, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT UPPER(BTRIM(npsn)), id, npsn, name, alamat, school_types, category, provinsi_id, kota_id
		FROM school
		WHERE npsn IS NOT NULL AND UPPER(BTRIM(npsn)) = ANY($1)`,
		npsns,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := map[string]PusdatinSchoolImage{}
	for rows.Next() {
		var normalized string
		var image PusdatinSchoolImage
		if err := rows.Scan(&normalized, &image.ID, &image.NPSN, &image.Name, &image.Alamat, &image.SchoolTypes, &image.Category, &image.ProvinsiID, &image.KotaID); err != nil {
			return nil, err
		}
		out[normalized] = image
	}
	return out, rows.Err()
}

func (r *Repository) RollbackPusdatinImport(ctx context.Context, manifest PusdatinImportManifest) error {
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	for _, row := range manifest.Rows {
		current, err := loadPusdatinImageTx(ctx, tx, row.After.ID)
		if err != nil {
			return err
		}
		if current == nil || !pusdatinImageEqual(*current, row.After) {
			return fmt.Errorf("pusdatin rollback blocked: school %s changed after import", row.After.ID)
		}
		if row.Inserted {
			var refs int
			if err := tx.QueryRow(ctx, `SELECT count(*) FROM users WHERE school_id = $1`, row.After.ID).Scan(&refs); err != nil {
				return err
			}
			if refs != 0 {
				return fmt.Errorf("pusdatin rollback blocked: inserted school %s has references", row.After.ID)
			}
			if _, err := tx.Exec(ctx, `DELETE FROM school WHERE id = $1`, row.After.ID); err != nil {
				return err
			}
			continue
		}
		if row.Before == nil {
			return fmt.Errorf("pusdatin rollback blocked: missing before image for %s", row.After.ID)
		}
		if _, err := tx.Exec(ctx,
			`UPDATE school
			SET name = $1, alamat = $2, school_types = $3, category = $4, provinsi_id = $5, kota_id = $6, updated_at = now()
			WHERE id = $7`,
			row.Before.Name, row.Before.Alamat, row.Before.SchoolTypes, row.Before.Category, row.Before.ProvinsiID, row.Before.KotaID, row.After.ID,
		); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}

func loadPusdatinImageTx(ctx context.Context, tx pgx.Tx, id string) (*PusdatinSchoolImage, error) {
	var image PusdatinSchoolImage
	err := tx.QueryRow(ctx,
		`SELECT id, npsn, name, alamat, school_types, category, provinsi_id, kota_id FROM school WHERE id = $1`,
		id,
	).Scan(&image.ID, &image.NPSN, &image.Name, &image.Alamat, &image.SchoolTypes, &image.Category, &image.ProvinsiID, &image.KotaID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return &image, nil
}

func pusdatinImageEqual(a, b PusdatinSchoolImage) bool {
	return a.ID == b.ID &&
		stringPtrEqualRepo(a.NPSN, b.NPSN) &&
		a.Name == b.Name &&
		stringPtrEqualRepo(a.Alamat, b.Alamat) &&
		slicesEqual(a.SchoolTypes, b.SchoolTypes) &&
		stringPtrEqualRepo(a.Category, b.Category) &&
		stringPtrEqualRepo(a.ProvinsiID, b.ProvinsiID) &&
		stringPtrEqualRepo(a.KotaID, b.KotaID)
}

func stringPtrEqualRepo(a, b *string) bool {
	if a == nil || b == nil {
		return a == b
	}
	return *a == *b
}

func slicesEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
