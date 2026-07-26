package repository

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

// TestMigration0042_CertificateDesign proves 0042 folds certificate_template and
// certificate_background_key into the certificate_design JSON blob (preserving any
// pre-existing layout/signature_key already in there), drops the two columns and the
// chk_certificate_template constraint, and that .down.sql reverses all of it — a
// pre-existing (template + background_key + layout) design must resolve to the same
// effective values after the up migration (FR-26).
func TestMigration0042_CertificateDesign(t *testing.T) {
	ctx := context.Background()
	pool := newMigration0025Pool(t)

	// Bring the DB to exactly the pre-0042 schema (0041 is the last migration before it).
	applyMigrationsUpTo(t, pool, "0041_drop_certificate_number_seq.up.sql")

	assertColumnExists := func(table, column string, want bool, msg string) {
		t.Helper()
		var exists bool
		require.NoError(t, pool.QueryRow(ctx,
			`SELECT EXISTS(SELECT 1 FROM information_schema.columns WHERE table_name = $1 AND column_name = $2)`,
			table, column,
		).Scan(&exists))
		require.Equal(t, want, exists, msg)
	}
	assertConstraintExists := func(name string, want bool, msg string) {
		t.Helper()
		var exists bool
		require.NoError(t, pool.QueryRow(ctx,
			`SELECT EXISTS(SELECT 1 FROM pg_constraint WHERE conname = $1)`, name,
		).Scan(&exists))
		require.Equal(t, want, exists, msg)
	}

	// Pre-0042: old shape.
	assertColumnExists("exam", "certificate_template", true, "certificate_template must exist before 0042")
	assertColumnExists("exam", "certificate_background_key", true, "certificate_background_key must exist before 0042")
	assertColumnExists("exam", "certificate_layout", true, "certificate_layout must exist before 0042")
	assertColumnExists("exam", "certificate_design", false, "certificate_design must not exist before 0042")
	assertConstraintExists("chk_certificate_template", true, "chk_certificate_template must exist before 0042")

	// Seed exams covering: (a) a fully-designed exam (template+bg key+layout+signature_key),
	// (b) an exam with only a template and NULL layout, to prove NULL-seeding works.
	preExistingLayout := `{"page":{"width_mm":297,"height_mm":210},"background":{"kind":"builtin","ref":"modern"},"fields":[{"id":"title","x_mm":10,"y_mm":10,"w_mm":50,"align":"left","visible":true}],"signature_key":"certificates/sig/admin.png"}`
	var designedExamID uuid.UUID
	require.NoError(t, pool.QueryRow(ctx,
		`INSERT INTO exam (title, certificate_template, certificate_background_key, certificate_layout)
		VALUES ($1, 'modern', $2, $3) RETURNING id`,
		"Fully Designed Exam", "avatars/admin/custom-bg.png", preExistingLayout,
	).Scan(&designedExamID))

	var bareExamID uuid.UUID
	require.NoError(t, pool.QueryRow(ctx,
		`INSERT INTO exam (title, certificate_template) VALUES ($1, 'classic') RETURNING id`,
		"Bare Template Exam",
	).Scan(&bareExamID))

	// Apply 0042 up.
	applyMigrationFile(t, pool, "0042_certificate_design.up.sql")

	assertColumnExists("exam", "certificate_template", false, "certificate_template must be dropped by 0042")
	assertColumnExists("exam", "certificate_background_key", false, "certificate_background_key must be dropped by 0042")
	assertColumnExists("exam", "certificate_layout", false, "certificate_layout (old name) must be gone (renamed)")
	assertColumnExists("exam", "certificate_design", true, "certificate_design must exist after 0042")
	assertConstraintExists("chk_certificate_template", false, "chk_certificate_template must be dropped by 0042")

	// The fully-designed exam's blob must carry ALL of: folded template, folded
	// background_key, AND the pre-existing layout fields/signature_key untouched —
	// this is the "preserve existing designs" guarantee (FR-26).
	var designedRaw []byte
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT certificate_design FROM exam WHERE id = $1`, designedExamID,
	).Scan(&designedRaw))
	var designed struct {
		Template      string `json:"template"`
		BackgroundKey string `json:"background_key"`
		SignatureKey  string `json:"signature_key"`
		Background    struct {
			Kind string `json:"kind"`
			Ref  string `json:"ref"`
		} `json:"background"`
		Fields []struct {
			ID string `json:"id"`
		} `json:"fields"`
	}
	require.NoError(t, json.Unmarshal(designedRaw, &designed))
	require.Equal(t, "modern", designed.Template, "template must be folded in")
	require.Equal(t, "avatars/admin/custom-bg.png", designed.BackgroundKey, "background_key must be folded in")
	require.Equal(t, "certificates/sig/admin.png", designed.SignatureKey, "pre-existing signature_key must survive the fold")
	require.Equal(t, "modern", designed.Background.Ref, "pre-existing background.ref must survive the fold")
	require.Len(t, designed.Fields, 1, "pre-existing layout fields must survive the fold")
	require.Equal(t, "title", designed.Fields[0].ID)

	// The bare exam (NULL certificate_layout, no background key) must have its
	// certificate_design seeded to a non-NULL object carrying just the template —
	// jsonb_set on a NULL target is a no-op, hence the explicit '{}' seed in the up
	// migration.
	var bareRaw []byte
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT certificate_design FROM exam WHERE id = $1`, bareExamID,
	).Scan(&bareRaw))
	require.NotNil(t, bareRaw, "certificate_design must not be NULL after folding a NULL certificate_layout")
	var bare struct {
		Template string `json:"template"`
	}
	require.NoError(t, json.Unmarshal(bareRaw, &bare))
	require.Equal(t, "classic", bare.Template)

	// Apply 0042 down and verify the columns/constraint are restored with the
	// original effective values.
	applyMigrationFile(t, pool, "0042_certificate_design.down.sql")

	assertColumnExists("exam", "certificate_template", true, "certificate_template must be restored by down")
	assertColumnExists("exam", "certificate_background_key", true, "certificate_background_key must be restored by down")
	assertColumnExists("exam", "certificate_layout", true, "certificate_layout must be restored by down")
	assertColumnExists("exam", "certificate_design", false, "certificate_design must be gone after down (renamed back)")
	assertConstraintExists("chk_certificate_template", true, "chk_certificate_template must be restored by down")

	var restoredTemplate string
	var restoredKey *string
	var restoredLayout []byte
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT certificate_template, certificate_background_key, certificate_layout FROM exam WHERE id = $1`, designedExamID,
	).Scan(&restoredTemplate, &restoredKey, &restoredLayout))
	require.Equal(t, "modern", restoredTemplate)
	require.NotNil(t, restoredKey)
	require.Equal(t, "avatars/admin/custom-bg.png", *restoredKey)
	var restoredLayoutDecoded struct {
		SignatureKey string `json:"signature_key"`
		Fields       []struct {
			ID string `json:"id"`
		} `json:"fields"`
	}
	require.NoError(t, json.Unmarshal(restoredLayout, &restoredLayoutDecoded))
	require.Equal(t, "certificates/sig/admin.png", restoredLayoutDecoded.SignatureKey)
	require.Len(t, restoredLayoutDecoded.Fields, 1)

	// The narrowed-back certificate_template CHECK constraint (down restores the
	// widened classic|modern|elegant|custom set from 0035, since that's the state
	// immediately before 0042 ran) still rejects garbage.
	_, err := pool.Exec(ctx,
		`INSERT INTO exam (title, certificate_template) VALUES ($1, 'bogus')`,
		"Post-down Bogus Exam",
	)
	require.Error(t, err, "invalid certificate_template must be rejected again after down restores the CHECK constraint")
}
