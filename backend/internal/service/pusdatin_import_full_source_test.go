//go:build pusdatin_full

package service

import (
	"encoding/json"
	"os"
	"regexp"
	"testing"

	"akademi-bimbel/internal/model"
)

const pusdatinFullSourcePath = "/Users/Panca/Documents/MyBook/Project/akademi-bimbel/docs/Data Induk Satuan Pendidikan  - DAFTAR Nasional 360 - ASC - 17 Agustus 2026.csv"

func TestTransformPusdatinSource_FullSourceAudit(t *testing.T) {
	cities := loadPusdatinAuditCities(t)
	aliases := loadPusdatinAuditAliases(t)

	f, err := os.Open(pusdatinFullSourcePath)
	if err != nil {
		t.Fatalf("open full source: %v", err)
	}
	defer f.Close()

	report, err := TransformPusdatinSource(f, model.PusdatinTransformOptions{
		ExpectedSourceSHA256: "ebaf0e05b047d87270f78b4a44f7c35ddb586bc888672280df303f15dcc22887",
		Cities:               cities,
		Aliases:              aliases,
	})
	if err != nil {
		t.Fatalf("TransformPusdatinSource full source: %v", err)
	}

	if report.Stats.RawRecords != 554885 {
		t.Fatalf("RawRecords: want 554885, got %d", report.Stats.RawRecords)
	}
	if report.Stats.UniqueNormalizedNPSN != 554882 {
		t.Fatalf("UniqueNormalizedNPSN: want 554882, got %d", report.Stats.UniqueNormalizedNPSN)
	}
	if report.Stats.InvalidRows != 0 {
		t.Fatalf("InvalidRows: want 0, got %d", report.Stats.InvalidRows)
	}
	if report.Stats.BlankOrDashAddresses != 424 {
		t.Fatalf("BlankOrDashAddresses: want 424, got %d", report.Stats.BlankOrDashAddresses)
	}
	if len(report.DuplicateGroups) != 3 {
		t.Fatalf("DuplicateGroups: want 3, got %d", len(report.DuplicateGroups))
	}
	wantDuplicateStatus := map[string]model.PusdatinDuplicateStatus{
		"69931346": model.PusdatinDuplicateConflict,
		"70005045": model.PusdatinDuplicateIdentical,
		"70005078": model.PusdatinDuplicateIdentical,
	}
	for _, group := range report.DuplicateGroups {
		if wantDuplicateStatus[group.NPSN] != group.Status {
			t.Fatalf("duplicate %s: want %s, got %s", group.NPSN, wantDuplicateStatus[group.NPSN], group.Status)
		}
		delete(wantDuplicateStatus, group.NPSN)
	}
	if len(wantDuplicateStatus) != 0 {
		t.Fatalf("missing duplicate groups: %+v", wantDuplicateStatus)
	}
	if len(report.Blockers) != 1 || report.Blockers[0].Code != "duplicate_conflict" || report.Blockers[0].NPSN != "69931346" {
		t.Fatalf("want only unresolved 69931346 duplicate blocker, got %+v", report.Blockers)
	}
}

func loadPusdatinAuditAliases(t *testing.T) []model.PusdatinGeographicAlias {
	t.Helper()
	data, err := os.ReadFile("../../../docs/pusdatin-geographic-aliases.json")
	if err != nil {
		t.Fatalf("read alias file: %v", err)
	}
	var aliases []model.PusdatinGeographicAlias
	if err := json.Unmarshal(data, &aliases); err != nil {
		t.Fatalf("decode alias file: %v", err)
	}
	return aliases
}

func loadPusdatinAuditCities(t *testing.T) []model.PusdatinCityReference {
	t.Helper()
	provinces := map[string]string{}
	cities := map[string]model.PusdatinCityReference{}
	loadProvinceCitySQL(t, "../../../backend/db/migrations/0029_seed_province_city_district.up.sql", provinces, cities)
	loadProvinceCitySQL(t, "../../../backend/db/migrations/0046_papua_2022_regions.up.sql", provinces, cities)
	out := make([]model.PusdatinCityReference, 0, len(cities))
	for _, city := range cities {
		out = append(out, city)
	}
	return out
}

func loadProvinceCitySQL(t *testing.T, path string, provinces map[string]string, cities map[string]model.PusdatinCityReference) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	provinceRE := regexp.MustCompile(`INSERT INTO province \(id, name\) VALUES \('([^']+)', '([^']+)'\);`)
	for _, match := range provinceRE.FindAllStringSubmatch(string(data), -1) {
		provinces[match[1]] = match[2]
	}
	cityRE := regexp.MustCompile(`INSERT INTO city \(id, province_id, name\) VALUES \('([^']+)', '([^']+)', '([^']+)'\);`)
	for _, match := range cityRE.FindAllStringSubmatch(string(data), -1) {
		cities[match[1]] = model.PusdatinCityReference{
			ID:           match[1],
			ProvinceID:   match[2],
			Name:         match[3],
			ProvinceName: provinces[match[2]],
		}
	}
}
