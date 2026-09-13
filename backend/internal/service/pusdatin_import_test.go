package service

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"
	"testing"

	"akademi-bimbel/internal/model"
)

func TestTransformPusdatinSource_ParsesMapsAndReportsDuplicateConflict(t *testing.T) {
	csvData := strings.Join([]string{
		"NPSN,Nama,Bentuk,Jenis,Status,Jenjang,Kabupaten,Kecamatan,Kelurahan,Alamat,Jalur,Pembina",
		"12345678, SD Negeri 1 , sd ,NEGERI,NEGERI,SD,KAB. MALANG,KEC. PAGAK,SUMBERREJO,-,FORMAL,KEMDIKBUD",
		"12345678, SD Negeri 1 , sd ,NEGERI,NEGERI,SD,KAB. MALANG,KEC. PAGAK,SUMBERREJO,-,FORMAL,KEMDIKBUD",
		"ABCDEFGH,Madrasah Satu,MI,SWASTA,SWASTA,MI,KOTA ADM. JAKARTA PUSAT,KEC. GAMBIR,GAMBIR,Jalan Kenanga,FORMAL,KEMENAG",
		"87654321,Sekolah Konflik,SMP,NEGERI,NEGERI,SMP,KAB. MALANG,KEC. PAGAK,SUMBERREJO,Jalan Lama,FORMAL,KEMDIKBUD",
		"87654321,Sekolah Konflik,SMP,NEGERI,NEGERI,SMP,KAB. MALANG,KEC. PAGAK,SUMBERREJO,Jalan Baru,FORMAL,KEMDIKBUD",
		"",
	}, "\n")

	sum := sha256.Sum256([]byte(csvData))
	report, err := TransformPusdatinSource(strings.NewReader(csvData), model.PusdatinTransformOptions{
		ExpectedSourceSHA256: hex.EncodeToString(sum[:]),
		Cities: []model.PusdatinCityReference{
			{ID: "3507", Name: "KABUPATEN MALANG", ProvinceID: "35", ProvinceName: "JAWA TIMUR"},
			{ID: "3173", Name: "KOTA JAKARTA PUSAT", ProvinceID: "31", ProvinceName: "DKI JAKARTA"},
		},
		Aliases: []model.PusdatinGeographicAlias{
			{SourceLabel: "KOTA ADM. JAKARTA PUSAT", TargetCityID: "3173", TargetCityName: "KOTA JAKARTA PUSAT", TargetProvinceID: "31", TargetProvinceName: "DKI JAKARTA", ReferenceEvidence: "seed city 3173"},
		},
	})
	if err != nil {
		t.Fatalf("TransformPusdatinSource: %v", err)
	}

	if report.Stats.RawRecords != 5 {
		t.Fatalf("RawRecords: want 5, got %d", report.Stats.RawRecords)
	}
	if report.Stats.UniqueNormalizedNPSN != 3 {
		t.Fatalf("UniqueNormalizedNPSN: want 3, got %d", report.Stats.UniqueNormalizedNPSN)
	}
	if report.Stats.BlankOrDashAddresses != 2 {
		t.Fatalf("BlankOrDashAddresses: want 2, got %d", report.Stats.BlankOrDashAddresses)
	}
	if len(report.DuplicateGroups) != 2 {
		t.Fatalf("DuplicateGroups: want 2, got %d", len(report.DuplicateGroups))
	}
	if report.DuplicateGroups[0].Status != model.PusdatinDuplicateIdentical {
		t.Fatalf("first duplicate group should be identical: %+v", report.DuplicateGroups[0])
	}
	if report.DuplicateGroups[1].Status != model.PusdatinDuplicateConflict {
		t.Fatalf("second duplicate group should be conflicting: %+v", report.DuplicateGroups[1])
	}
	if len(report.Blockers) != 1 || report.Blockers[0].NPSN != "87654321" {
		t.Fatalf("want duplicate conflict blocker for 87654321, got %+v", report.Blockers)
	}
	if len(report.Transformed) != 2 {
		t.Fatalf("Transformed: want 2 transformed non-conflicting identities, got %d", len(report.Transformed))
	}

	first := report.Transformed[0]
	if first.NPSN != "12345678" || first.Name != "SD Negeri 1" || first.Category != "SD" {
		t.Fatalf("unexpected first transformed row: %+v", first)
	}
	if first.Alamat != nil {
		t.Fatalf("dash address should map to NULL, got %q", *first.Alamat)
	}
	if len(first.SchoolTypes) != 1 || first.SchoolTypes[0] != "SD" {
		t.Fatalf("SchoolTypes: want [SD], got %+v", first.SchoolTypes)
	}
	if first.CityID != "3507" || first.ProvinceID != "35" {
		t.Fatalf("city/province mapping drifted: %+v", first)
	}

	second := report.Transformed[1]
	if second.CityID != "3173" || second.ProvinceID != "31" {
		t.Fatalf("alias city/province mapping drifted: %+v", second)
	}
	if second.Alamat == nil || *second.Alamat != "Jalan Kenanga" {
		t.Fatalf("address should be trimmed and preserved, got %v", second.Alamat)
	}
	if report.TransformedChecksum == "" {
		t.Fatal("TransformedChecksum should be populated")
	}
}

func TestTransformPusdatinSource_ExplicitResolutionAndChecksum(t *testing.T) {
	csvData := strings.Join([]string{
		"NPSN,Nama,Bentuk,Jenis,Status,Jenjang,Kabupaten,Kecamatan,Kelurahan,Alamat,Jalur,Pembina",
		"87654321,Sekolah Konflik,SMP,NEGERI,NEGERI,SMP,KAB. MALANG,KEC. PAGAK,SUMBERREJO,Jalan Lama,FORMAL,KEMDIKBUD",
		"87654321,Sekolah Konflik,SMP,NEGERI,NEGERI,SMP,KAB. MALANG,KEC. PAGAK,SUMBERREJO,Jalan Baru,FORMAL,KEMDIKBUD",
		"",
	}, "\n")
	sum := sha256.Sum256([]byte(csvData))
	opts := model.PusdatinTransformOptions{
		ExpectedSourceSHA256: hex.EncodeToString(sum[:]),
		Cities: []model.PusdatinCityReference{
			{ID: "3507", Name: "KABUPATEN MALANG", ProvinceID: "35", ProvinceName: "JAWA TIMUR"},
		},
	}
	blocked, err := TransformPusdatinSource(strings.NewReader(csvData), opts)
	if err != nil {
		t.Fatalf("initial TransformPusdatinSource: %v", err)
	}
	if len(blocked.Blockers) != 1 || len(blocked.DuplicateGroups) != 1 {
		t.Fatalf("want one unresolved duplicate blocker, got blockers=%+v duplicates=%+v", blocked.Blockers, blocked.DuplicateGroups)
	}
	chosen := blocked.DuplicateGroups[0].Rows[1]

	opts.Resolutions = []model.PusdatinDuplicateResolution{
		{SourceSHA256: opts.ExpectedSourceSHA256, NPSN: "87654321", RecordNumber: chosen.RecordNumber, RowHash: chosen.RowHash},
	}
	resolved, err := TransformPusdatinSource(strings.NewReader(csvData), opts)
	if err != nil {
		t.Fatalf("resolved TransformPusdatinSource: %v", err)
	}
	if len(resolved.Blockers) != 0 {
		t.Fatalf("resolved transform should not block, got %+v", resolved.Blockers)
	}
	if len(resolved.Transformed) != 1 || resolved.Transformed[0].Alamat == nil || *resolved.Transformed[0].Alamat != "Jalan Baru" {
		t.Fatalf("resolution should select second row, got %+v", resolved.Transformed)
	}

	rerun, err := TransformPusdatinSource(strings.NewReader(csvData), opts)
	if err != nil {
		t.Fatalf("rerun TransformPusdatinSource: %v", err)
	}
	if rerun.TransformedChecksum != resolved.TransformedChecksum {
		t.Fatalf("checksum should be deterministic: %s != %s", rerun.TransformedChecksum, resolved.TransformedChecksum)
	}

	opts.Resolutions[0].RowHash = "stale"
	stale, err := TransformPusdatinSource(strings.NewReader(csvData), opts)
	if err != nil {
		t.Fatalf("stale TransformPusdatinSource: %v", err)
	}
	if len(stale.Blockers) != 1 || !strings.Contains(stale.Blockers[0].Message, "stale") {
		t.Fatalf("want stale resolution blocker, got %+v", stale.Blockers)
	}
}

func TestTransformPusdatinSource_RejectsMalformedSourceAndAliasDrift(t *testing.T) {
	t.Run("wrong checksum", func(t *testing.T) {
		_, err := TransformPusdatinSource(strings.NewReader("x"), model.PusdatinTransformOptions{ExpectedSourceSHA256: strings.Repeat("0", 64)})
		if err == nil {
			t.Fatal("wrong checksum should fail")
		}
	})

	t.Run("bad header", func(t *testing.T) {
		_, err := TransformPusdatinSource(strings.NewReader("NPSN,Nama\n12345678,Sekolah\n"), model.PusdatinTransformOptions{})
		if err == nil {
			t.Fatal("bad header should fail")
		}
	})

	t.Run("alias drift blocks mapping", func(t *testing.T) {
		csvData := strings.Join([]string{
			"NPSN,Nama,Bentuk,Jenis,Status,Jenjang,Kabupaten,Kecamatan,Kelurahan,Alamat,Jalur,Pembina",
			"ABCDEFGH,Madrasah Satu,MI,SWASTA,SWASTA,MI,KOTA ADM. JAKARTA PUSAT,KEC. GAMBIR,GAMBIR,Jalan Kenanga,FORMAL,KEMENAG",
			"",
		}, "\n")
		report, err := TransformPusdatinSource(strings.NewReader(csvData), model.PusdatinTransformOptions{
			Cities: []model.PusdatinCityReference{
				{ID: "3173", Name: "KOTA JAKARTA PUSAT", ProvinceID: "31", ProvinceName: "DKI JAKARTA"},
			},
			Aliases: []model.PusdatinGeographicAlias{
				{SourceLabel: "KOTA ADM. JAKARTA PUSAT", TargetCityID: "3173", TargetCityName: "KOTA JAKARTA RAYA", TargetProvinceID: "31", TargetProvinceName: "DKI JAKARTA", ReferenceEvidence: "bad fixture"},
			},
		})
		if err == nil || report != nil {
			t.Fatalf("want geography drift error, got report=%+v err=%v", report, err)
		}
	})
}
