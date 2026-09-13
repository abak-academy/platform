package service

import (
	"bytes"
	"crypto/sha256"
	"encoding/csv"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strings"

	"akademi-bimbel/internal/model"
)

var pusdatinExpectedHeader = []string{
	"NPSN", "Nama", "Bentuk", "Jenis", "Status", "Jenjang",
	"Kabupaten", "Kecamatan", "Kelurahan", "Alamat", "Jalur", "Pembina",
}

type pusdatinSourceRow struct {
	recordNumber int
	rowHash      string
	npsn         string
	name         string
	category     string
	alamat       *string
	city         model.PusdatinCityReference
}

func TransformPusdatinSource(r io.Reader, opts model.PusdatinTransformOptions) (*model.PusdatinTransformReport, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return nil, err
	}
	sourceSHA := sha256Hex(data)
	if opts.ExpectedSourceSHA256 != "" && sourceSHA != opts.ExpectedSourceSHA256 {
		return nil, fmt.Errorf("%w: got %s", ErrPusdatinSourceChecksum, sourceSHA)
	}

	reader := csv.NewReader(bytes.NewReader(data))
	reader.FieldsPerRecord = len(pusdatinExpectedHeader)
	header, err := reader.Read()
	if err != nil {
		return nil, err
	}
	if !equalCSVHeader(header, pusdatinExpectedHeader) {
		return nil, ErrPusdatinHeaderMismatch
	}

	geo, err := newPusdatinGeographyResolver(opts.Cities, opts.Aliases)
	if err != nil {
		return nil, err
	}
	resolutions, err := normalizePusdatinResolutions(sourceSHA, opts.Resolutions)
	if err != nil {
		return nil, err
	}

	report := &model.PusdatinTransformReport{SourceSHA256: sourceSHA}
	byNPSN := map[string][]pusdatinSourceRow{}
	for recordNumber := 1; ; recordNumber++ {
		record, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		report.Stats.RawRecords++
		row, ok := parsePusdatinRow(recordNumber, record, geo, report)
		if !ok {
			report.Stats.InvalidRows++
			continue
		}
		byNPSN[row.npsn] = append(byNPSN[row.npsn], row)
	}
	report.Stats.UniqueNormalizedNPSN = len(byNPSN)
	report.RecordCount = report.Stats.RawRecords
	report.UniqueNormalizedNPSN = report.Stats.UniqueNormalizedNPSN
	report.InvalidRows = report.Stats.InvalidRows
	report.BlankOrDashAddressCount = report.Stats.BlankOrDashAddresses

	for npsn, rows := range byNPSN {
		chosen, include := choosePusdatinRow(npsn, rows, resolutions, report)
		if !include {
			continue
		}
		report.Transformed = append(report.Transformed, model.PusdatinTransformedSchool{
			SourceRecordNumber: chosen.recordNumber,
			SourceRowHash:      chosen.rowHash,
			NPSN:               chosen.npsn,
			Name:               chosen.name,
			Category:           chosen.category,
			SchoolTypes:        []string{chosen.category},
			Alamat:             chosen.alamat,
			CityID:             chosen.city.ID,
			CityName:           chosen.city.Name,
			ProvinceID:         chosen.city.ProvinceID,
			ProvinceName:       chosen.city.ProvinceName,
		})
	}
	report.Schools = report.Transformed

	for key := range resolutions {
		if _, ok := byNPSN[key.npsn]; !ok {
			return nil, ErrPusdatinResolutionMismatch
		}
	}
	sort.Slice(report.DuplicateGroups, func(i, j int) bool {
		return report.DuplicateGroups[i].NPSN < report.DuplicateGroups[j].NPSN
	})
	sort.Slice(report.Transformed, func(i, j int) bool {
		return report.Transformed[i].NPSN < report.Transformed[j].NPSN
	})
	checksum, err := checksumPusdatinTransformed(report.Transformed)
	if err != nil {
		return nil, err
	}
	report.TransformedChecksum = checksum
	return report, nil
}

func parsePusdatinRow(recordNumber int, record []string, geo pusdatinGeographyResolver, report *model.PusdatinTransformReport) (pusdatinSourceRow, bool) {
	npsn, err := normalizeSchoolNPSN(&record[0])
	if err != nil || npsn == nil {
		return pusdatinSourceRow{}, false
	}
	name := collapseSpace(record[1])
	if !validSchoolName(name) {
		return pusdatinSourceRow{}, false
	}
	category := strings.ToUpper(collapseSpace(record[2]))
	if category == "" || category == "-" {
		report.Blockers = append(report.Blockers, model.PusdatinTransformBlocker{Code: "invalid_category", RecordNumber: recordNumber, Message: fmt.Sprintf("row %d has empty Bentuk", recordNumber)})
		return pusdatinSourceRow{}, false
	}
	alamatRaw := strings.TrimSpace(record[9])
	var alamat *string
	if alamatRaw == "" || alamatRaw == "-" {
		report.Stats.BlankOrDashAddresses++
	} else {
		alamatValue := collapseSpace(alamatRaw)
		alamat = &alamatValue
	}
	city, ok := geo.resolve(record[6])
	if !ok {
		report.Blockers = append(report.Blockers, model.PusdatinTransformBlocker{Code: "geography_unresolved", RecordNumber: recordNumber, Message: fmt.Sprintf("row %d has unresolved Kabupaten %q", recordNumber, record[6])})
		return pusdatinSourceRow{}, false
	}
	return pusdatinSourceRow{
		recordNumber: recordNumber,
		rowHash:      pusdatinRowHash(record),
		npsn:         *npsn,
		name:         name,
		category:     category,
		alamat:       alamat,
		city:         city,
	}, true
}

func choosePusdatinRow(npsn string, rows []pusdatinSourceRow, resolutions map[pusdatinResolutionKey]model.PusdatinDuplicateResolution, report *model.PusdatinTransformReport) (pusdatinSourceRow, bool) {
	if len(rows) == 1 {
		return rows[0], true
	}

	status := model.PusdatinDuplicateIdentical
	firstHash := rows[0].rowHash
	for _, row := range rows[1:] {
		if row.rowHash != firstHash {
			status = model.PusdatinDuplicateConflict
			break
		}
	}
	group := model.PusdatinDuplicateGroup{NPSN: npsn, Status: status, Identical: status == model.PusdatinDuplicateIdentical}
	for _, row := range rows {
		group.Rows = append(group.Rows, model.PusdatinDuplicateRow{
			RecordNumber: row.recordNumber,
			RowHash:      row.rowHash,
		})
	}
	report.DuplicateGroups = append(report.DuplicateGroups, group)
	if status == model.PusdatinDuplicateIdentical {
		return rows[0], true
	}

	for _, row := range rows {
		key := pusdatinResolutionKey{npsn: npsn, recordNumber: row.recordNumber}
		resolution, ok := resolutions[key]
		if !ok {
			continue
		}
		if resolution.RowHash != row.rowHash {
			report.Blockers = append(report.Blockers, model.PusdatinTransformBlocker{Code: "duplicate_resolution_invalid", NPSN: npsn, RecordNumber: row.recordNumber, Message: fmt.Sprintf("duplicate resolution for NPSN %s has stale row hash", npsn)})
			return pusdatinSourceRow{}, false
		}
		return row, true
	}
	report.Blockers = append(report.Blockers, model.PusdatinTransformBlocker{Code: "duplicate_conflict", NPSN: npsn, Message: fmt.Sprintf("conflicting duplicate NPSN %s requires explicit resolution", npsn)})
	return pusdatinSourceRow{}, false
}

type pusdatinResolutionKey struct {
	npsn         string
	recordNumber int
}

func normalizePusdatinResolutions(sourceSHA string, resolutions []model.PusdatinDuplicateResolution) (map[pusdatinResolutionKey]model.PusdatinDuplicateResolution, error) {
	out := map[pusdatinResolutionKey]model.PusdatinDuplicateResolution{}
	for _, resolution := range resolutions {
		if resolution.SourceSHA256 != sourceSHA {
			return nil, ErrPusdatinResolutionMismatch
		}
		npsn, err := normalizeSchoolNPSN(&resolution.NPSN)
		if err != nil || npsn == nil || resolution.RecordNumber == 0 || resolution.RowHash == "" {
			return nil, ErrPusdatinResolutionMismatch
		}
		resolution.NPSN = *npsn
		out[pusdatinResolutionKey{npsn: *npsn, recordNumber: resolution.RecordNumber}] = resolution
	}
	return out, nil
}

type pusdatinGeographyResolver struct {
	exact map[string]model.PusdatinCityReference
	alias map[string]model.PusdatinCityReference
}

func newPusdatinGeographyResolver(cities []model.PusdatinCityReference, aliases []model.PusdatinGeographicAlias) (pusdatinGeographyResolver, error) {
	byID := map[string]model.PusdatinCityReference{}
	resolver := pusdatinGeographyResolver{
		exact: map[string]model.PusdatinCityReference{},
		alias: map[string]model.PusdatinCityReference{},
	}
	for _, city := range cities {
		byID[city.ID] = city
		resolver.exact[normalizePusdatinRegionLabel(city.Name)] = city
	}
	for _, alias := range aliases {
		city, ok := byID[alias.TargetCityID]
		if !ok || city.Name != alias.TargetCityName || city.ProvinceID != alias.TargetProvinceID || city.ProvinceName != alias.TargetProvinceName {
			return pusdatinGeographyResolver{}, ErrPusdatinGeographyDrift
		}
		resolver.alias[normalizePusdatinRegionLabel(alias.SourceLabel)] = city
	}
	return resolver, nil
}

func (r pusdatinGeographyResolver) resolve(label string) (model.PusdatinCityReference, bool) {
	normalized := normalizePusdatinRegionLabel(label)
	if city, ok := r.exact[normalized]; ok {
		return city, true
	}
	city, ok := r.alias[normalized]
	return city, ok
}

func normalizePusdatinRegionLabel(label string) string {
	normalized := strings.ToUpper(collapseSpace(label))
	if strings.HasPrefix(normalized, "KAB. ") {
		normalized = "KABUPATEN " + strings.TrimSpace(strings.TrimPrefix(normalized, "KAB. "))
	}
	return normalized
}

func equalCSVHeader(got, want []string) bool {
	if len(got) != len(want) {
		return false
	}
	for i := range want {
		if got[i] != want[i] {
			return false
		}
	}
	return true
}

func collapseSpace(s string) string {
	return strings.Join(strings.Fields(strings.TrimSpace(s)), " ")
}

func pusdatinRowHash(record []string) string {
	data, _ := json.Marshal(record)
	return sha256Hex(data)
}

func checksumPusdatinTransformed(rows []model.PusdatinTransformedSchool) (string, error) {
	data, err := json.Marshal(rows)
	if err != nil {
		return "", err
	}
	return sha256Hex(data), nil
}

func sha256Hex(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}
