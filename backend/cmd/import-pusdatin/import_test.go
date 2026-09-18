package main

import (
	"strings"
	"testing"

	"akademi-bimbel/internal/model"
	"github.com/stretchr/testify/require"
)

const header = "NPSN,Nama,Bentuk,Jenis,Status,Jenjang,Kabupaten,Kecamatan,Kelurahan,Alamat,Jalur,Pembina\n"

func sourceCSV(t *testing.T, body string) source {
	t.Helper()
	s, err := readSource(strings.NewReader(header + body))
	require.NoError(t, err)
	return s
}

func ptr(s string) *string { return &s }

func TestPlanIdentityAndPreservation(t *testing.T) {
	s := sourceCSV(t, "00123456,Sumber,SMA,,,,KAB. BOGOR,,,Jalan Baru,,\nK1234567,Kursus,KURSUS,,,,KOTA BOGOR,,,Jalan Kursus,,\n")
	cities := []model.City{{ID: "1", ProvinceID: "p", Name: "KABUPATEN BOGOR"}, {ID: "2", ProvinceID: "p", Name: "KOTA BOGOR"}}
	old := model.School{ID: "old", Code: "legacy", Name: "Nama Lama", NPSN: ptr(" 00123456 "), Status: "deactivated", SchoolTypes: []string{"SD", "SMA"}, Alamat: ptr("Alamat Lama")}
	unrelated := model.School{ID: "other", Name: "Kursus", Code: "other", SchoolTypes: []string{}}
	p := makePlan(s, cities, []model.School{old, unrelated}, nil)
	require.Equal(t, "update", p[0].Action)
	require.Equal(t, old.ID, p[0].After.ID)
	require.Equal(t, old.Name, p[0].After.Name)
	require.Equal(t, old.Status, p[0].After.Status)
	require.Equal(t, old.SchoolTypes, p[0].After.SchoolTypes)
	require.Equal(t, old.Alamat, p[0].After.Alamat)
	require.Equal(t, old.NPSN, p[0].After.NPSN)
	require.Equal(t, "1", *p[0].After.KotaID)
	require.Equal(t, "insert", p[1].Action)
	require.Equal(t, "2", *p[1].After.KotaID)
	require.Equal(t, []string{"KURSUS"}, p[1].After.SchoolTypes)
	again := makePlan(s, cities, []model.School{*p[0].After, *p[1].After}, nil)
	require.Equal(t, "unchanged", again[0].Action)
	require.Equal(t, "unchanged", again[1].Action)
}

func TestPlanBlocksConflicts(t *testing.T) {
	row := "00123456,Sekolah,SMA,,,,KAB. BOGOR,,,Jalan,,\n"
	cities := []model.City{{ID: "1", ProvinceID: "p", Name: "KABUPATEN BOGOR"}}
	base := model.School{ID: "existing", Code: "old", Name: "Existing", NPSN: ptr("00123456"), SchoolTypes: []string{"SMA"}}
	for _, tc := range []struct {
		name, body string
		cities     []model.City
		old        []model.School
		reason     string
	}{
		{"source conflict", row + strings.Replace(row, "Jalan", "Other", 1), cities, nil, "conflicting source"},
		{"missing city", row, nil, nil, "city"},
		{"ambiguous city", row, append(append([]model.City{}, cities...), model.City{ID: "2", ProvinceID: "q", Name: "KABUPATEN BOGOR"}), nil, "city"},
		{"ownership", row, cities, []model.School{base, base}, "multiple NPSN"},
		{"code collision", row, cities, []model.School{{Code: " npsn-00123456 "}}, "code"},
		{"type", row, cities, []model.School{{NPSN: base.NPSN, SchoolTypes: []string{"SMK"}}}, "type"},
		{"location", row, cities, []model.School{{NPSN: base.NPSN, KotaID: ptr("elsewhere")}}, "location"},
		{"unknown type", strings.Replace(row, "SMA", "UNKNOWN", 1), cities, nil, "type"},
		{"invalid npsn", strings.Replace(row, "00123456", "123", 1), cities, nil, "NPSN"},
		{"invalid name", strings.Replace(row, "Sekolah", "---", 1), cities, nil, "name"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p := makePlan(sourceCSV(t, tc.body), tc.cities, tc.old, nil)
			require.Equal(t, "conflict", p[0].Action)
			require.Contains(t, p[0].Reason, tc.reason)
		})
	}
	p := makePlan(sourceCSV(t, row+row), cities, nil, nil)
	require.Len(t, p, 1)
	require.Equal(t, []int{2, 3}, p[0].Rows)
	require.Equal(t, "insert", p[0].Action)
}

func TestCityAliasesAreExplicitAndValidated(t *testing.T) {
	s := sourceCSV(t, "00123456,Sekolah,SMA,,,,KAB. OLD,,,Jalan,,\n")
	cities := []model.City{{ID: "1", ProvinceID: "p", Name: "KABUPATEN NEW"}}
	require.Equal(t, "insert", makePlan(s, cities, nil, map[string]string{"KABUPATEN OLD": "1"})[0].Action)
	require.Equal(t, "conflict", makePlan(s, cities, nil, map[string]string{"KABUPATEN OLD": "missing"})[0].Action)
	require.NotEqual(t, cityKey("KAB. BOGOR"), cityKey("KOTA BOGOR"))
}

func TestMalformedCSVRejected(t *testing.T) {
	for _, value := range []string{"", "NPSN,Nama\n123,Name\n", header, header + "123,Name\n", strings.Replace(header, "Nama", "NPSN", 1)} {
		_, err := readSource(strings.NewReader(value))
		require.Error(t, err)
	}
}
