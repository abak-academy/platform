package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"strings"
	"testing"

	"akademi-bimbel/internal/model"
)

func TestPusdatinImport_VerifyAndRollback(t *testing.T) {
	svc, repo := newRealDBService(t)
	ctx := context.Background()
	suffix := uniqueSuffix()

	var cityID, provinceID, cityName, provinceName string
	if err := repo.Pool().QueryRow(ctx,
		`SELECT c.id, c.province_id, c.name, p.name FROM city c JOIN province p ON p.id = c.province_id ORDER BY c.id LIMIT 1`,
	).Scan(&cityID, &provinceID, &cityName, &provinceName); err != nil {
		t.Fatalf("load city: %v", err)
	}

	existingNPSN := "5" + strings.ToUpper(suffix[:7])
	originalAddress := "Rollback Old Address"
	existing, err := svc.CreateSchool(ctx, "Rollback Old "+suffix, "rollback_old_"+suffix, &existingNPSN, []string{"SMP"}, &originalAddress)
	if err != nil {
		t.Fatalf("seed existing school: %v", err)
	}
	newNPSN := "4" + strings.ToUpper(suffix[:7])
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
	_, manifest, err := svc.ApplyPusdatinImportWithManifest(ctx, strings.NewReader(csvData), opts, preview.ReviewedChecksum)
	if err != nil {
		t.Fatalf("ApplyPusdatinImportWithManifest: %v", err)
	}
	if err := svc.VerifyPusdatinImport(ctx, manifest); err != nil {
		t.Fatalf("VerifyPusdatinImport: %v", err)
	}

	var newID string
	if err := repo.Pool().QueryRow(ctx, `SELECT id FROM school WHERE UPPER(BTRIM(npsn)) = $1`, newNPSN).Scan(&newID); err != nil {
		t.Fatalf("load inserted school: %v", err)
	}
	if _, err := repo.Pool().Exec(ctx, `INSERT INTO users (email, role, name, school_id) VALUES ($1, 'student', $2, $3)`, "rollback-ref-"+suffix+"@test.local", "Rollback Ref", newID); err != nil {
		t.Fatalf("insert new reference: %v", err)
	}
	if err := svc.RollbackPusdatinImport(ctx, manifest); err == nil {
		t.Fatal("rollback should refuse a newly referenced inserted school")
	}
	if _, err := repo.Pool().Exec(ctx, `DELETE FROM users WHERE email = $1`, "rollback-ref-"+suffix+"@test.local"); err != nil {
		t.Fatalf("delete new reference: %v", err)
	}

	if err := svc.RollbackPusdatinImport(ctx, manifest); err != nil {
		t.Fatalf("RollbackPusdatinImport: %v", err)
	}
	requirePusdatinSchoolAbsent(t, repo, newNPSN)
	var restoredName, restoredAddress string
	if err := repo.Pool().QueryRow(ctx, `SELECT name, alamat FROM school WHERE id = $1`, existing.ID).Scan(&restoredName, &restoredAddress); err != nil {
		t.Fatalf("load restored school: %v", err)
	}
	if restoredName != "Rollback Old "+suffix || restoredAddress != originalAddress {
		t.Fatalf("restored existing school mismatch: name=%q address=%q", restoredName, restoredAddress)
	}
}
