package service

import (
	"bytes"
	"context"
	"encoding/csv"
	"io"
	"strconv"
	"strings"
)

type SchoolBulkRow struct {
	Row         int
	Name        string
	Code        string
	NPSN        *string
	Alamat      *string
	Category    *string
	Provinsi    *string
	Kota        *string
	SchoolTypes []string
}

type SchoolBulkResultRow struct {
	Row         int
	Name        string
	Code        string
	NPSN        string
	SchoolTypes string
	Alamat      string
	Category    string
	Provinsi    string
	Kota        string
	Status      string
	Error       string
}

// parseSchoolTypes splits a school_types cell on both '|' and ',' per §D-4:
// pipe is the unquoted-friendly template encoding, comma is accepted because
// the admin UI displays school_types comma-joined and a copy-paste from there
// is the likely user error. Returns nil when nothing parses out: pgx encodes a
// non-nil empty slice as '{}', not NULL, which would defeat the COALESCE in the
// repository UPDATE and blank the stored jenjang list.
func parseSchoolTypes(cell string) []string {
	var out []string
	for _, part := range strings.Split(strings.ReplaceAll(cell, "|", ","), ",") {
		part = strings.TrimSpace(part)
		if part != "" {
			out = append(out, part)
		}
	}
	return out
}

// ParseSchoolBulkCSV reads a school-bulk upload. name and code are required;
// npsn/school_types/alamat/category/provinsi/kota are optional. Mirrors ParseStudentBulkCSV.
func ParseSchoolBulkCSV(data []byte) ([]SchoolBulkRow, error) {
	r := newBulkCSVReader(data)

	header, err := r.Read()
	if err != nil {
		if err == io.EOF {
			return nil, ErrMissingSchoolCSVHeader
		}
		return nil, ErrInvalidCSV
	}

	nameIdx, codeIdx, npsnIdx, schoolTypesIdx, alamatIdx := -1, -1, -1, -1, -1
	categoryIdx, provinsiIdx, kotaIdx := -1, -1, -1
	for i, h := range header {
		switch normalizeCSVHeader(h) {
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
		case "category":
			categoryIdx = i
		case "provinsi":
			provinsiIdx = i
		case "kota":
			kotaIdx = i
		}
	}
	if nameIdx == -1 || codeIdx == -1 {
		return nil, ErrMissingSchoolCSVHeader
	}

	line := 1
	var rows []SchoolBulkRow
	for {
		record, err := r.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, ErrInvalidCSV
		}
		line++
		if bulkRowIsBlank(record) {
			continue
		}
		if len(rows)+1 > maxBulkRows {
			return nil, ErrRowLimitExceeded
		}

		rows = append(rows, SchoolBulkRow{
			Row:         line,
			Name:        bulkCell(record, nameIdx),
			Code:        bulkCell(record, codeIdx),
			NPSN:        bulkOptionalCell(record, npsnIdx),
			Alamat:      bulkOptionalCell(record, alamatIdx),
			Category:    bulkOptionalCell(record, categoryIdx),
			Provinsi:    bulkOptionalCell(record, provinsiIdx),
			Kota:        bulkOptionalCell(record, kotaIdx),
			SchoolTypes: parseSchoolTypes(bulkCell(record, schoolTypesIdx)),
		})
	}

	return rows, nil
}

// ProcessSchoolBulkRows creates new codes and refreshes existing codes through the main school service methods.
func (s *Service) ProcessSchoolBulkRows(ctx context.Context, rows []SchoolBulkRow, onProgress func(pct int)) ([]SchoolBulkResultRow, int, error) {
	results := make([]SchoolBulkResultRow, len(rows))
	successCount := 0

	checkpoint := len(rows) / 10
	if checkpoint < 1 {
		checkpoint = 1
	}

	for i, r := range rows {
		result := SchoolBulkResultRow{
			Row:         r.Row,
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
		if r.Category != nil {
			result.Category = *r.Category
		}
		if r.Provinsi != nil {
			result.Provinsi = *r.Provinsi
		}
		if r.Kota != nil {
			result.Kota = *r.Kota
		}

		var provinceID, cityID *string
		var err error
		if (r.Provinsi == nil) != (r.Kota == nil) {
			err = ErrIncompleteSchoolLocation
		}
		if err == nil && r.Provinsi != nil {
			province, lookupErr := s.storeRepo.GetProvinceByName(ctx, *r.Provinsi)
			switch {
			case lookupErr != nil:
				err = lookupErr
			case province == nil:
				err = ErrInvalidProvinsi
			default:
				provinceID = &province.ID
				result.Provinsi = province.Name
			}
		}
		if err == nil && r.Kota != nil {
			city, lookupErr := s.storeRepo.GetCityByNameInProvince(ctx, *r.Kota, *provinceID)
			switch {
			case lookupErr != nil:
				err = lookupErr
			case city == nil:
				err = ErrInvalidKota
			default:
				cityID = &city.ID
				result.Kota = city.Name
			}
		}

		var saved *SchoolResponse
		if err == nil {
			existing, lookupErr := s.storeRepo.GetSchoolByCode(ctx, r.Code)
			if lookupErr != nil {
				err = lookupErr
			} else if existing == nil {
				saved, err = s.CreateSchool(ctx, r.Name, r.Code, r.NPSN, r.SchoolTypes, r.Alamat, r.Category, provinceID, cityID)
			} else {
				name := r.Name
				saved, err = s.UpdateSchool(ctx, existing.ID, &name, r.NPSN, r.Alamat, r.SchoolTypes, nil, r.Category, provinceID, cityID)
			}
		}
		if err == nil {
			result.Status = "success"
			if saved.Category != nil {
				result.Category = *saved.Category
			} else {
				result.Category = ""
			}
			if saved.NPSN != nil {
				result.NPSN = *saved.NPSN
			} else {
				result.NPSN = ""
			}
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
	_ = w.Write([]string{"row", "name", "code", "npsn", "school_types", "alamat", "category", "provinsi", "kota", "status", "error"})
	for _, r := range results {
		_ = w.Write(csvSafeRow(strconv.Itoa(r.Row), r.Name, r.Code, r.NPSN, r.SchoolTypes, r.Alamat, r.Category, r.Provinsi, r.Kota, r.Status, r.Error))
	}
	w.Flush()
	return buf.Bytes()
}
