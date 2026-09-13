package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"strings"
	"testing"

	"akademi-bimbel/internal/model"
	"akademi-bimbel/internal/repository"
)

func TestPusdatinImport_DryRunApplyAndIdempotentRepeat(t *testing.T) {
	svc, repo := newRealDBService(t)
	ctx := context.Background()
	suffix := uniqueSuffix()

	var cityID, provinceID, cityName, provinceName string
	if err := repo.Pool().QueryRow(ctx,
		`SELECT c.id, c.province_id, c.name, p.name FROM city c JOIN province p ON p.id = c.province_id ORDER BY c.id LIMIT 1`,
	).Scan(&cityID, &provinceID, &cityName, &provinceName); err != nil {
		t.Fatalf("load city: %v", err)
	}

	existingNPSN := "8" + strings.ToUpper(suffix[:7])
	if _, err := svc.CreateSchool(ctx, "Old Pusdatin "+suffix, "old_"+suffix, &existingNPSN, []string{"SMP"}, stringPtr("Old Address")); err != nil {
		t.Fatalf("seed existing school: %v", err)
	}
	newNPSN := "9" + strings.ToUpper(suffix[:7])
	csvData := pusdatinApplyCSV(existingNPSN, newNPSN, cityName)
	sum := sha256.Sum256([]byte(csvData))
	opts := model.PusdatinTransformOptions{
		ExpectedSourceSHA256: hex.EncodeToString(sum[:]),
		Cities: []model.PusdatinCityReference{{
			ID: cityID, Name: cityName, ProvinceID: provinceID, ProvinceName: provinceName,
		}},
	}

	preview, err := svc.DryRunPusdatinImport(ctx, strings.NewReader(csvData), opts)
	if err != nil {
		t.Fatalf("DryRunPusdatinImport: %v", err)
	}
	if preview.Counts.Inserted != 1 || preview.Counts.Updated != 1 || preview.Counts.Unchanged != 0 {
		t.Fatalf("preview counts: %+v", preview.Counts)
	}
	requirePusdatinSchoolAbsent(t, repo, newNPSN)

	applied, err := svc.ApplyPusdatinImport(ctx, strings.NewReader(csvData), opts, preview.ReviewedChecksum)
	if err != nil {
		t.Fatalf("ApplyPusdatinImport: %v", err)
	}
	if applied.Counts.Inserted != 1 || applied.Counts.Updated != 1 {
		t.Fatalf("apply counts: %+v", applied.Counts)
	}
	requirePusdatinSchool(t, repo, existingNPSN, "Updated Pusdatin "+existingNPSN, "SMA", cityID)
	requirePusdatinSchool(t, repo, newNPSN, "New Pusdatin "+newNPSN, "MI", cityID)

	repeatPreview, err := svc.DryRunPusdatinImport(ctx, strings.NewReader(csvData), opts)
	if err != nil {
		t.Fatalf("repeat DryRunPusdatinImport: %v", err)
	}
	if repeatPreview.Counts.Inserted != 0 || repeatPreview.Counts.Updated != 0 || repeatPreview.Counts.Unchanged != 2 {
		t.Fatalf("repeat preview counts: %+v", repeatPreview.Counts)
	}
	if _, err := svc.ApplyPusdatinImport(ctx, strings.NewReader(csvData), opts, preview.ReviewedChecksum); err == nil {
		t.Fatal("apply with stale first-run reviewed checksum should fail")
	}
}

func TestPusdatinImport_ApplyRequiresExternalNPSNIndex(t *testing.T) {
	svc, repo := newRealDBService(t)
	ctx := context.Background()
	suffix := uniqueSuffix()

	var cityID, provinceID, cityName, provinceName string
	if err := repo.Pool().QueryRow(ctx,
		`SELECT c.id, c.province_id, c.name, p.name FROM city c JOIN province p ON p.id = c.province_id ORDER BY c.id LIMIT 1`,
	).Scan(&cityID, &provinceID, &cityName, &provinceName); err != nil {
		t.Fatalf("load city: %v", err)
	}

	npsn := "7" + strings.ToUpper(suffix[:7])
	csvData := pusdatinApplyCSV(npsn, "6"+strings.ToUpper(suffix[:7]), cityName)
	sum := sha256.Sum256([]byte(csvData))
	opts := model.PusdatinTransformOptions{
		ExpectedSourceSHA256: hex.EncodeToString(sum[:]),
		Cities: []model.PusdatinCityReference{{
			ID: cityID, Name: cityName, ProvinceID: provinceID, ProvinceName: provinceName,
		}},
	}
	preview, err := svc.DryRunPusdatinImport(ctx, strings.NewReader(csvData), opts)
	if err != nil {
		t.Fatalf("DryRunPusdatinImport: %v", err)
	}

	if _, err := repo.Pool().Exec(ctx, `DROP INDEX uq_school_npsn_normalized`); err != nil {
		t.Fatalf("drop prerequisite index: %v", err)
	}
	t.Cleanup(func() {
		_, _ = repo.Pool().Exec(context.Background(), `CREATE UNIQUE INDEX IF NOT EXISTS uq_school_npsn_normalized ON school (UPPER(BTRIM(npsn))) WHERE npsn IS NOT NULL`)
	})

	if _, err := svc.ApplyPusdatinImport(ctx, strings.NewReader(csvData), opts, preview.ReviewedChecksum); err == nil {
		t.Fatal("apply should fail without the externally managed NPSN index")
	}
	requirePusdatinSchoolAbsent(t, repo, npsn)
}

func pusdatinApplyCSV(existingNPSN, newNPSN, cityName string) string {
	return strings.Join([]string{
		"NPSN,Nama,Bentuk,Jenis,Status,Jenjang,Kabupaten,Kecamatan,Kelurahan,Alamat,Jalur,Pembina",
		existingNPSN + ",Updated Pusdatin " + existingNPSN + ",SMA,NEGERI,NEGERI,SMA," + cityName + ",KEC. SATU,KEL SATU,Jalan Baru,FORMAL,KEMDIKBUD",
		newNPSN + ",New Pusdatin " + newNPSN + ",MI,SWASTA,SWASTA,MI," + cityName + ",KEC. DUA,KEL DUA,Jalan Baru,FORMAL,KEMENAG",
		"",
	}, "\n")
}

func requirePusdatinSchool(t *testing.T, repo *repository.Repository, npsn, name, category, cityID string) {
	t.Helper()
	var gotName, gotCategory, gotCityID string
	var schoolTypes []string
	if err := repo.Pool().QueryRow(context.Background(),
		`SELECT name, category, city_id, school_types FROM school WHERE UPPER(BTRIM(npsn)) = $1`,
		npsn,
	).Scan(&gotName, &gotCategory, &gotCityID, &schoolTypes); err != nil {
		t.Fatalf("load imported school %s: %v", npsn, err)
	}
	if gotName != name || gotCategory != category || gotCityID != cityID || len(schoolTypes) != 1 || schoolTypes[0] != category {
		t.Fatalf("school %s mismatch: name=%q category=%q city=%q types=%+v", npsn, gotName, gotCategory, gotCityID, schoolTypes)
	}
}

func requirePusdatinSchoolAbsent(t *testing.T, repo *repository.Repository, npsn string) {
	t.Helper()
	var count int
	if err := repo.Pool().QueryRow(context.Background(),
		`SELECT count(*) FROM school WHERE UPPER(BTRIM(npsn)) = $1`,
		npsn,
	).Scan(&count); err != nil {
		t.Fatalf("count school %s: %v", npsn, err)
	}
	if count != 0 {
		t.Fatalf("school %s should be absent, count=%d", npsn, count)
	}
}
