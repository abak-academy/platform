package service

import (
	"bytes"
	"context"
	"encoding/csv"
	"io"
	"strings"
)

type SchoolBulkRow struct {
	Name        string
	Code        string
	NPSN        *string
	Alamat      *string
	SchoolTypes []string
}

type SchoolBulkResultRow struct {
	Name        string
	Code        string
	NPSN        string
	SchoolTypes string
	Alamat      string
	Status      string
	Error       string
}

// parseSchoolTypes splits a school_types cell on both '|' and ',' per §D-4:
// pipe is the unquoted-friendly template encoding, comma is accepted because
// the admin UI displays school_types comma-joined and a copy-paste from there
// is the likely user error. Always returns a non-nil slice.
func parseSchoolTypes(cell string) []string {
	out := []string{}
	if cell == "" {
		return out
	}
	for _, part := range strings.Split(strings.ReplaceAll(cell, "|", ","), ",") {
		part = strings.TrimSpace(part)
		if part != "" {
			out = append(out, part)
		}
	}
	return out
}

// ParseSchoolBulkCSV reads a school-bulk upload. name and code are required;
// npsn/school_types/alamat are optional. Mirrors ParseStudentBulkCSV.
func ParseSchoolBulkCSV(data []byte) ([]SchoolBulkRow, error) {
	r := csv.NewReader(bytes.NewReader(data))

	header, err := r.Read()
	if err != nil {
		if err == io.EOF {
			return nil, ErrMissingSchoolCSVHeader
		}
		return nil, ErrInvalidCSV
	}

	nameIdx, codeIdx, npsnIdx, schoolTypesIdx, alamatIdx := -1, -1, -1, -1, -1
	for i, h := range header {
		switch strings.ToLower(strings.TrimSpace(h)) {
		case "name":
			nameIdx = i
		case "code":
			codeIdx = i
		case "npsn":
			npsnIdx = i
		case "school_types":
			schoolTypesIdx = i
		case "alamat":
			alamatIdx = i
		}
	}
	if nameIdx == -1 || codeIdx == -1 {
		return nil, ErrMissingSchoolCSVHeader
	}

	optionalStr := func(record []string, idx int) *string {
		if idx == -1 || record[idx] == "" {
			return nil
		}
		v := record[idx]
		return &v
	}

	var rows []SchoolBulkRow
	for {
		record, err := r.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, ErrInvalidCSV
		}
		if len(rows)+1 > maxBulkRows {
			return nil, ErrRowLimitExceeded
		}

		var schoolTypesCell string
		if schoolTypesIdx != -1 {
			schoolTypesCell = record[schoolTypesIdx]
		}

		rows = append(rows, SchoolBulkRow{
			Name:        record[nameIdx],
			Code:        record[codeIdx],
			NPSN:        optionalStr(record, npsnIdx),
			Alamat:      optionalStr(record, alamatIdx),
			SchoolTypes: parseSchoolTypes(schoolTypesCell),
		})
	}

	return rows, nil
}

// ProcessSchoolBulkRows creates each row through Service.CreateSchool so that
// code-uniqueness and school_types NOT NULL coercion are not reimplemented. A
// row-level failure (e.g. ErrSchoolCodeTaken) does not abort the batch.
// Mirrors ProcessStudentBulkRows; there is no schoolBound parameter — schools
// are global (invariant 5).
func (s *Service) ProcessSchoolBulkRows(ctx context.Context, rows []SchoolBulkRow, onProgress func(pct int)) ([]SchoolBulkResultRow, int, error) {
	results := make([]SchoolBulkResultRow, len(rows))
	successCount := 0

	checkpoint := len(rows) / 10
	if checkpoint < 1 {
		checkpoint = 1
	}

	for i, r := range rows {
		result := SchoolBulkResultRow{
			Name:        r.Name,
			Code:        r.Code,
			SchoolTypes: strings.Join(r.SchoolTypes, "|"),
		}
		if r.NPSN != nil {
			result.NPSN = *r.NPSN
		}
		if r.Alamat != nil {
			result.Alamat = *r.Alamat
		}

		_, err := s.CreateSchool(ctx, r.Name, r.Code, r.NPSN, r.SchoolTypes, r.Alamat)
		if err == nil {
			result.Status = "success"
			successCount++
		} else {
			result.Status = "failed"
			result.Error = err.Error()
		}

		results[i] = result

		if onProgress != nil && (i+1)%checkpoint == 0 {
			onProgress((i + 1) * 100 / len(rows))
		}
	}

	if onProgress != nil {
		onProgress(100)
	}

	return results, successCount, nil
}

// BuildSchoolBulkResultCSV writes the per-row report as CSV bytes.
func BuildSchoolBulkResultCSV(results []SchoolBulkResultRow) []byte {
	var buf bytes.Buffer
	w := csv.NewWriter(&buf)
	_ = w.Write([]string{"name", "code", "npsn", "school_types", "alamat", "status", "error"})
	for _, r := range results {
		_ = w.Write(csvSafeRow(r.Name, r.Code, r.NPSN, r.SchoolTypes, r.Alamat, r.Status, r.Error))
	}
	w.Flush()
	return buf.Bytes()
}
