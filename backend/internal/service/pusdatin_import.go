package service

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/csv"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"slices"
	"sort"
	"strings"

	"akademi-bimbel/internal/repository"
)

type PusdatinCityReference struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	ProvinsiID   string `json:"provinsi_id"`
	ProvinsiName string `json:"provinsi_name"`
}

type PusdatinGeographicAlias struct {
	SourceLabel        string `json:"source_label"`
	TargetKotaID       string `json:"target_kota_id"`
	TargetKotaName     string `json:"target_kota_name"`
	TargetProvinsiID   string `json:"target_provinsi_id"`
	TargetProvinsiName string `json:"target_provinsi_name"`
	ReferenceEvidence  string `json:"reference_evidence"`
}

type PusdatinDuplicateResolution struct {
	SourceSHA256 string `json:"source_sha256"`
	NPSN         string `json:"npsn"`
	RecordNumber int    `json:"record_number"`
	RowHash      string `json:"row_hash"`
}

type PusdatinTransformOptions struct {
	ExpectedSourceSHA256 string
	Cities               []PusdatinCityReference
	Aliases              []PusdatinGeographicAlias
	Resolutions          []PusdatinDuplicateResolution
}

type PusdatinTransformStats struct {
	RawRecords           int `json:"raw_records"`
	UniqueNormalizedNPSN int `json:"unique_normalized_npsn"`
	InvalidRows          int `json:"invalid_rows"`
	BlankOrDashAddresses int `json:"blank_or_dash_addresses"`
}

type PusdatinTransformedSchool struct {
	SourceRecordNumber int      `json:"source_record_number"`
	SourceRowHash      string   `json:"source_row_hash"`
	NPSN               string   `json:"npsn"`
	Name               string   `json:"name"`
	Category           string   `json:"category"`
	SchoolTypes        []string `json:"school_types"`
	Alamat             *string  `json:"alamat"`
	KotaID             string   `json:"kota_id"`
	KotaName           string   `json:"kota_name"`
	ProvinsiID         string   `json:"provinsi_id"`
	ProvinsiName       string   `json:"provinsi_name"`
}

type PusdatinDuplicateStatus string

const (
	PusdatinDuplicateIdentical PusdatinDuplicateStatus = "identical"
	PusdatinDuplicateConflict  PusdatinDuplicateStatus = "conflict"
)

type PusdatinDuplicateRow struct {
	RecordNumber int    `json:"record_number"`
	RowHash      string `json:"row_hash"`
}

type PusdatinDuplicateGroup struct {
	NPSN      string                  `json:"npsn"`
	Status    PusdatinDuplicateStatus `json:"status"`
	Identical bool                    `json:"identical"`
	Rows      []PusdatinDuplicateRow  `json:"rows"`
}

type PusdatinTransformBlocker struct {
	Code         string `json:"code"`
	NPSN         string `json:"npsn,omitempty"`
	RecordNumber int    `json:"record_number,omitempty"`
	Message      string `json:"message"`
}

type PusdatinTransformReport struct {
	SourceSHA256            string                      `json:"source_sha256"`
	Stats                   PusdatinTransformStats      `json:"stats"`
	RecordCount             int                         `json:"record_count"`
	UniqueNormalizedNPSN    int                         `json:"unique_normalized_npsn"`
	InvalidRows             int                         `json:"invalid_rows"`
	BlankOrDashAddressCount int                         `json:"blank_or_dash_address_count"`
	DuplicateGroups         []PusdatinDuplicateGroup    `json:"duplicate_groups"`
	Transformed             []PusdatinTransformedSchool `json:"transformed"`
	Schools                 []PusdatinTransformedSchool `json:"schools"`
	TransformedChecksum     string                      `json:"transformed_checksum"`
	Blockers                []PusdatinTransformBlocker  `json:"blockers"`
}

type PusdatinImportCounts struct {
	Inserted  int `json:"inserted"`
	Updated   int `json:"updated"`
	Unchanged int `json:"unchanged"`
}

type PusdatinImportReport struct {
	SourceSHA256        string                     `json:"source_sha256"`
	TransformedChecksum string                     `json:"transformed_checksum"`
	ReviewedChecksum    string                     `json:"reviewed_checksum"`
	Counts              PusdatinImportCounts       `json:"counts"`
	Blockers            []PusdatinTransformBlocker `json:"blockers"`
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
	city         PusdatinCityReference
}

func TransformPusdatinSource(r io.Reader, opts PusdatinTransformOptions) (*PusdatinTransformReport, error) {
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

	report := &PusdatinTransformReport{SourceSHA256: sourceSHA}
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
		report.Transformed = append(report.Transformed, PusdatinTransformedSchool{
			SourceRecordNumber: chosen.recordNumber,
			SourceRowHash:      chosen.rowHash,
			NPSN:               chosen.npsn,
			Name:               chosen.name,
			Category:           chosen.category,
			SchoolTypes:        []string{chosen.category},
			Alamat:             chosen.alamat,
			KotaID:             chosen.city.ID,
			KotaName:           chosen.city.Name,
			ProvinsiID:         chosen.city.ProvinsiID,
			ProvinsiName:       chosen.city.ProvinsiName,
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

func (s *Service) DryRunPusdatinImport(ctx context.Context, r io.Reader, opts PusdatinTransformOptions) (*PusdatinImportReport, error) {
	transform, err := TransformPusdatinSource(r, opts)
	if err != nil {
		return nil, err
	}
	return s.buildPusdatinImportReport(ctx, transform)
}

func (s *Service) ApplyPusdatinImport(ctx context.Context, r io.Reader, opts PusdatinTransformOptions, reviewedChecksum string) (*PusdatinImportReport, error) {
	report, _, err := s.ApplyPusdatinImportWithManifest(ctx, r, opts, reviewedChecksum)
	return report, err
}

func (s *Service) ApplyPusdatinImportWithManifest(ctx context.Context, r io.Reader, opts PusdatinTransformOptions, reviewedChecksum string) (*PusdatinImportReport, PusdatinImportManifest, error) {
	if err := s.storeRepo.VerifySchoolNPSNImportIndex(ctx); err != nil {
		return nil, PusdatinImportManifest{}, err
	}
	transform, err := TransformPusdatinSource(r, opts)
	if err != nil {
		return nil, PusdatinImportManifest{}, err
	}
	report, err := s.buildPusdatinImportReport(ctx, transform)
	if err != nil {
		return nil, PusdatinImportManifest{}, err
	}
	if len(report.Blockers) > 0 {
		return report, PusdatinImportManifest{}, ErrPusdatinImportBlocked
	}
	if reviewedChecksum == "" || reviewedChecksum != report.ReviewedChecksum {
		return nil, PusdatinImportManifest{}, ErrPusdatinReviewedPreviewMismatch
	}
	npsns := pusdatinTransformedNPSNs(transform.Transformed)
	before, err := s.storeRepo.LoadPusdatinSchoolImages(ctx, npsns)
	if err != nil {
		return nil, PusdatinImportManifest{}, err
	}
	if err := s.storeRepo.ApplyPusdatinSchools(ctx, pusdatinRepositoryInputs(transform.Transformed)); err != nil {
		return nil, PusdatinImportManifest{}, err
	}
	after, err := s.storeRepo.LoadPusdatinSchoolImages(ctx, npsns)
	if err != nil {
		return nil, PusdatinImportManifest{}, err
	}
	manifest := PusdatinImportManifest{ReviewedChecksum: reviewedChecksum}
	for _, row := range transform.Transformed {
		afterImage := pusdatinServiceImage(after[row.NPSN])
		manifestRow := PusdatinImportManifestRow{
			NPSN:     row.NPSN,
			Inserted: before[row.NPSN].ID == "",
			After:    afterImage,
		}
		if image, ok := before[row.NPSN]; ok {
			beforeImage := pusdatinServiceImage(image)
			manifestRow.Before = &beforeImage
		}
		manifest.Rows = append(manifest.Rows, manifestRow)
	}
	return report, manifest, nil
}

func (s *Service) VerifyPusdatinImport(ctx context.Context, manifest PusdatinImportManifest) error {
	npsns := make([]string, 0, len(manifest.Rows))
	for _, row := range manifest.Rows {
		npsns = append(npsns, row.NPSN)
	}
	current, err := s.storeRepo.LoadPusdatinSchoolImages(ctx, npsns)
	if err != nil {
		return err
	}
	for _, row := range manifest.Rows {
		image, ok := current[row.NPSN]
		if !ok || !pusdatinManifestImageEqual(pusdatinServiceImage(image), row.After) {
			return fmt.Errorf("pusdatin verify failed for %s", row.NPSN)
		}
	}
	return nil
}

func (s *Service) RollbackPusdatinImport(ctx context.Context, manifest PusdatinImportManifest) error {
	return s.storeRepo.RollbackPusdatinImport(ctx, pusdatinRepositoryManifest(manifest))
}

func pusdatinRepositoryInputs(rows []PusdatinTransformedSchool) []repository.PusdatinSchoolInput {
	out := make([]repository.PusdatinSchoolInput, 0, len(rows))
	for _, row := range rows {
		out = append(out, repository.PusdatinSchoolInput{
			NPSN:        row.NPSN,
			Name:        row.Name,
			Alamat:      row.Alamat,
			SchoolTypes: row.SchoolTypes,
			Category:    row.Category,
			ProvinsiID:  row.ProvinsiID,
			KotaID:      row.KotaID,
		})
	}
	return out
}

func pusdatinServiceImage(image repository.PusdatinSchoolImage) PusdatinSchoolImage {
	return PusdatinSchoolImage{
		ID:          image.ID,
		NPSN:        image.NPSN,
		Name:        image.Name,
		Alamat:      image.Alamat,
		SchoolTypes: image.SchoolTypes,
		Category:    image.Category,
		ProvinsiID:  image.ProvinsiID,
		KotaID:      image.KotaID,
	}
}

func pusdatinRepositoryImage(image PusdatinSchoolImage) repository.PusdatinSchoolImage {
	return repository.PusdatinSchoolImage{
		ID:          image.ID,
		NPSN:        image.NPSN,
		Name:        image.Name,
		Alamat:      image.Alamat,
		SchoolTypes: image.SchoolTypes,
		Category:    image.Category,
		ProvinsiID:  image.ProvinsiID,
		KotaID:      image.KotaID,
	}
}

func pusdatinRepositoryManifest(manifest PusdatinImportManifest) repository.PusdatinImportManifest {
	out := repository.PusdatinImportManifest{ReviewedChecksum: manifest.ReviewedChecksum}
	for _, row := range manifest.Rows {
		repoRow := repository.PusdatinImportManifestRow{
			NPSN:     row.NPSN,
			Inserted: row.Inserted,
			After:    pusdatinRepositoryImage(row.After),
		}
		if row.Before != nil {
			before := pusdatinRepositoryImage(*row.Before)
			repoRow.Before = &before
		}
		out.Rows = append(out.Rows, repoRow)
	}
	return out
}

func pusdatinTransformedNPSNs(rows []PusdatinTransformedSchool) []string {
	npsns := make([]string, 0, len(rows))
	for _, row := range rows {
		npsns = append(npsns, row.NPSN)
	}
	return npsns
}

func pusdatinManifestImageEqual(a, b PusdatinSchoolImage) bool {
	return a.ID == b.ID &&
		pusdatinStringPtrEqual(a.NPSN, b.NPSN) &&
		a.Name == b.Name &&
		pusdatinStringPtrEqual(a.Alamat, b.Alamat) &&
		slices.Equal(a.SchoolTypes, b.SchoolTypes) &&
		pusdatinStringPtrEqual(a.Category, b.Category) &&
		pusdatinStringPtrEqual(a.ProvinsiID, b.ProvinsiID) &&
		pusdatinStringPtrEqual(a.KotaID, b.KotaID)
}

func (s *Service) buildPusdatinImportReport(ctx context.Context, transform *PusdatinTransformReport) (*PusdatinImportReport, error) {
	report := &PusdatinImportReport{
		SourceSHA256:        transform.SourceSHA256,
		TransformedChecksum: transform.TransformedChecksum,
		Blockers:            append([]PusdatinTransformBlocker{}, transform.Blockers...),
	}
	npsns := make([]string, 0, len(transform.Transformed))
	for _, row := range transform.Transformed {
		npsns = append(npsns, row.NPSN)
	}
	targets, err := s.storeRepo.LoadPusdatinSchoolTargets(ctx, npsns)
	if err != nil {
		if errors.Is(err, repository.ErrAmbiguousSchoolIdentity) {
			report.Blockers = append(report.Blockers, PusdatinTransformBlocker{Code: "target_npsn_ambiguous", Message: "target has duplicate normalized NPSN"})
			return finalizePusdatinImportReport(report)
		}
		return nil, err
	}
	for _, row := range transform.Transformed {
		target, exists := targets[row.NPSN]
		if !exists {
			report.Counts.Inserted++
			continue
		}
		if pusdatinTargetMatches(row, target) {
			report.Counts.Unchanged++
		} else {
			report.Counts.Updated++
		}
	}
	return finalizePusdatinImportReport(report)
}

func finalizePusdatinImportReport(report *PusdatinImportReport) (*PusdatinImportReport, error) {
	payload := struct {
		SourceSHA256        string
		TransformedChecksum string
		Counts              PusdatinImportCounts
		Blockers            []PusdatinTransformBlocker
	}{
		SourceSHA256:        report.SourceSHA256,
		TransformedChecksum: report.TransformedChecksum,
		Counts:              report.Counts,
		Blockers:            report.Blockers,
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	report.ReviewedChecksum = sha256Hex(data)
	return report, nil
}

func pusdatinTargetMatches(row PusdatinTransformedSchool, target repository.PusdatinSchoolTarget) bool {
	return target.Name == row.Name &&
		pusdatinStringPtrEqual(target.Alamat, row.Alamat) &&
		slices.Equal(target.SchoolTypes, row.SchoolTypes) &&
		stringPtrValueEqual(target.Category, row.Category) &&
		stringPtrValueEqual(target.ProvinsiID, row.ProvinsiID) &&
		stringPtrValueEqual(target.KotaID, row.KotaID)
}

func pusdatinStringPtrEqual(a, b *string) bool {
	if a == nil || b == nil {
		return a == b
	}
	return *a == *b
}

func stringPtrValueEqual(ptr *string, value string) bool {
	return ptr != nil && *ptr == value
}

func parsePusdatinRow(recordNumber int, record []string, geo pusdatinGeographyResolver, report *PusdatinTransformReport) (pusdatinSourceRow, bool) {
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
		report.Blockers = append(report.Blockers, PusdatinTransformBlocker{Code: "invalid_category", RecordNumber: recordNumber, Message: fmt.Sprintf("row %d has empty Bentuk", recordNumber)})
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
		report.Blockers = append(report.Blockers, PusdatinTransformBlocker{Code: "geography_unresolved", RecordNumber: recordNumber, Message: fmt.Sprintf("row %d has unresolved Kabupaten %q", recordNumber, record[6])})
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

func choosePusdatinRow(npsn string, rows []pusdatinSourceRow, resolutions map[pusdatinResolutionKey]PusdatinDuplicateResolution, report *PusdatinTransformReport) (pusdatinSourceRow, bool) {
	if len(rows) == 1 {
		return rows[0], true
	}

	status := PusdatinDuplicateIdentical
	firstHash := rows[0].rowHash
	for _, row := range rows[1:] {
		if row.rowHash != firstHash {
			status = PusdatinDuplicateConflict
			break
		}
	}
	group := PusdatinDuplicateGroup{NPSN: npsn, Status: status, Identical: status == PusdatinDuplicateIdentical}
	for _, row := range rows {
		group.Rows = append(group.Rows, PusdatinDuplicateRow{
			RecordNumber: row.recordNumber,
			RowHash:      row.rowHash,
		})
	}
	report.DuplicateGroups = append(report.DuplicateGroups, group)
	if status == PusdatinDuplicateIdentical {
		return rows[0], true
	}

	for _, row := range rows {
		key := pusdatinResolutionKey{npsn: npsn, recordNumber: row.recordNumber}
		resolution, ok := resolutions[key]
		if !ok {
			continue
		}
		if resolution.RowHash != row.rowHash {
			report.Blockers = append(report.Blockers, PusdatinTransformBlocker{Code: "duplicate_resolution_invalid", NPSN: npsn, RecordNumber: row.recordNumber, Message: fmt.Sprintf("duplicate resolution for NPSN %s has stale row hash", npsn)})
			return pusdatinSourceRow{}, false
		}
		return row, true
	}
	report.Blockers = append(report.Blockers, PusdatinTransformBlocker{Code: "duplicate_conflict", NPSN: npsn, Message: fmt.Sprintf("conflicting duplicate NPSN %s requires explicit resolution", npsn)})
	return pusdatinSourceRow{}, false
}

type pusdatinResolutionKey struct {
	npsn         string
	recordNumber int
}

func normalizePusdatinResolutions(sourceSHA string, resolutions []PusdatinDuplicateResolution) (map[pusdatinResolutionKey]PusdatinDuplicateResolution, error) {
	out := map[pusdatinResolutionKey]PusdatinDuplicateResolution{}
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
	exact map[string]PusdatinCityReference
	alias map[string]PusdatinCityReference
}

func newPusdatinGeographyResolver(cities []PusdatinCityReference, aliases []PusdatinGeographicAlias) (pusdatinGeographyResolver, error) {
	byID := map[string]PusdatinCityReference{}
	resolver := pusdatinGeographyResolver{
		exact: map[string]PusdatinCityReference{},
		alias: map[string]PusdatinCityReference{},
	}
	for _, city := range cities {
		byID[city.ID] = city
		resolver.exact[normalizePusdatinRegionLabel(city.Name)] = city
	}
	for _, alias := range aliases {
		city, ok := byID[alias.TargetKotaID]
		if !ok || city.Name != alias.TargetKotaName || city.ProvinsiID != alias.TargetProvinsiID || city.ProvinsiName != alias.TargetProvinsiName {
			return pusdatinGeographyResolver{}, ErrPusdatinGeographyDrift
		}
		resolver.alias[normalizePusdatinRegionLabel(alias.SourceLabel)] = city
	}
	return resolver, nil
}

func (r pusdatinGeographyResolver) resolve(label string) (PusdatinCityReference, bool) {
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

func checksumPusdatinTransformed(rows []PusdatinTransformedSchool) (string, error) {
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
