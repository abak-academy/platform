package service

import (
	"bytes"
	"context"
	"encoding/csv"
	"io"
	"strconv"
	"strings"
	"time"
)

const maxBulkRows = 1000

type StudentBulkRow struct {
	Name           string
	School         string
	Email          *string
	Jenjang        string
	DOB            *string
	Gender         *string
	Grade          *string
	AlamatDomisili *string
	TargetExam     *string
	Provinsi       *string
	Kota           *string
	Kecamatan      *string
	KodePos        *string
}

type StudentBulkResultRow struct {
	Name         string
	School       string
	Email        string
	Status       string
	Username     string
	TempPassword string
	Error        string
}

// ParseStudentBulkCSV reads a student-bulk upload. jenjang and school are
// required; nis is ignored if present; email/dob/gender/grade/
// alamat_domisili/target_exam/provinsi/kota/kecamatan/kode_pos are all
// optional — same field set as single registration (RegisterStudent), minus
// the region-name-vs-ID resolution which happens in ProcessStudentBulkRows.
func ParseStudentBulkCSV(data []byte) ([]StudentBulkRow, error) {
	r := csv.NewReader(bytes.NewReader(data))

	header, err := r.Read()
	if err != nil {
		if err == io.EOF {
			return nil, ErrMissingCSVHeader
		}
		return nil, ErrInvalidCSV
	}

	nameIdx, jenjangIdx, schoolIdx, emailIdx := -1, -1, -1, -1
	dobIdx, genderIdx, gradeIdx, alamatIdx, targetExamIdx := -1, -1, -1, -1, -1
	provinsiIdx, kotaIdx, kecamatanIdx, kodePosIdx := -1, -1, -1, -1
	for i, h := range header {
		switch strings.ToLower(strings.TrimSpace(h)) {
		case "name":
			nameIdx = i
		case "jenjang":
			jenjangIdx = i
		case "school":
			schoolIdx = i
		case "email":
			emailIdx = i
		case "dob":
			dobIdx = i
		case "gender":
			genderIdx = i
		case "grade":
			gradeIdx = i
		case "alamat_domisili":
			alamatIdx = i
		case "target_exam":
			targetExamIdx = i
		case "provinsi":
			provinsiIdx = i
		case "kota":
			kotaIdx = i
		case "kecamatan":
			kecamatanIdx = i
		case "kode_pos":
			kodePosIdx = i
			// "nis" is intentionally ignored
		}
	}
	if nameIdx == -1 || jenjangIdx == -1 || schoolIdx == -1 {
		return nil, ErrMissingCSVHeader
	}

	optionalStr := func(record []string, idx int) *string {
		if idx == -1 || record[idx] == "" {
			return nil
		}
		v := record[idx]
		return &v
	}

	var rows []StudentBulkRow
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

		rows = append(rows, StudentBulkRow{
			Name:           record[nameIdx],
			School:         record[schoolIdx],
			Jenjang:        record[jenjangIdx],
			Email:          optionalStr(record, emailIdx),
			DOB:            optionalStr(record, dobIdx),
			Gender:         optionalStr(record, genderIdx),
			Grade:          optionalStr(record, gradeIdx),
			AlamatDomisili: optionalStr(record, alamatIdx),
			TargetExam:     optionalStr(record, targetExamIdx),
			Provinsi:       optionalStr(record, provinsiIdx),
			Kota:           optionalStr(record, kotaIdx),
			Kecamatan:      optionalStr(record, kecamatanIdx),
			KodePos:        optionalStr(record, kodePosIdx),
		})
	}

	return rows, nil
}

// ProcessStudentBulkRows applies RegisterStudent to each row, resolving
// province/city/district names to IDs before passing them to RegisterStudent.
// schoolBound, when non-nil, restricts every row's resolved school to match it.
// nil means no restriction (super_admin cross-school).
func (s *Service) ProcessStudentBulkRows(ctx context.Context, schoolBound *string, rows []StudentBulkRow, onProgress func(pct int)) ([]StudentBulkResultRow, int, error) {
	results := make([]StudentBulkResultRow, len(rows))
	successCount := 0

	checkpoint := len(rows) / 10
	if checkpoint < 1 {
		checkpoint = 1
	}

	for i, r := range rows {
		result := StudentBulkResultRow{Name: r.Name, School: r.School}
		if r.Email != nil {
			result.Email = *r.Email
		}

		// Resolve School name to school_id.
		school, err := s.storeRepo.GetSchoolByNameCI(ctx, r.School)
		if err != nil {
			result.Status = "failed"
			result.Error = err.Error()
			results[i] = result
			if onProgress != nil && (i+1)%checkpoint == 0 {
				onProgress((i + 1) * 100 / len(rows))
			}
			continue
		}
		if school == nil {
			result.Status = "failed"
			result.Error = ErrSchoolNotFoundByName.Error()
			results[i] = result
			if onProgress != nil && (i+1)%checkpoint == 0 {
				onProgress((i + 1) * 100 / len(rows))
			}
			continue
		}

		// Check school bound.
		if schoolBound != nil && school.ID != *schoolBound {
			result.Status = "failed"
			result.Error = ErrCrossSchoolBound.Error()
			results[i] = result
			if onProgress != nil && (i+1)%checkpoint == 0 {
				onProgress((i + 1) * 100 / len(rows))
			}
			continue
		}

		schoolID := school.ID
		// Update result School with resolved name (captures canonical casing).
		result.School = school.Name

		// Resolve address names to IDs (all-or-nothing).
		var provinsiID, kotaID, kecamatanID, kodePos *string
		addrCount := 0
		if r.Provinsi != nil {
			addrCount++
		}
		if r.Kota != nil {
			addrCount++
		}
		if r.Kecamatan != nil {
			addrCount++
		}
		if addrCount > 0 && addrCount < 3 {
			result.Status = "failed"
			result.Error = ErrIncompleteAddress.Error()
			results[i] = result
			if onProgress != nil && (i+1)%checkpoint == 0 {
				onProgress((i + 1) * 100 / len(rows))
			}
			continue
		}
		if addrCount == 3 {
			prov, err := s.storeRepo.GetProvinceByName(ctx, *r.Provinsi)
			if err != nil {
				result.Status = "failed"
				result.Error = err.Error()
				results[i] = result
				if onProgress != nil && (i+1)%checkpoint == 0 {
					onProgress((i + 1) * 100 / len(rows))
				}
				continue
			}
			if prov == nil {
				result.Status = "failed"
				result.Error = ErrInvalidProvinsi.Error()
				results[i] = result
				if onProgress != nil && (i+1)%checkpoint == 0 {
					onProgress((i + 1) * 100 / len(rows))
				}
				continue
			}
			provinsiID = &prov.ID

			city, err := s.storeRepo.GetCityByNameInProvince(ctx, *r.Kota, *provinsiID)
			if err != nil {
				result.Status = "failed"
				result.Error = err.Error()
				results[i] = result
				if onProgress != nil && (i+1)%checkpoint == 0 {
					onProgress((i + 1) * 100 / len(rows))
				}
				continue
			}
			if city == nil {
				result.Status = "failed"
				result.Error = ErrInvalidKota.Error()
				results[i] = result
				if onProgress != nil && (i+1)%checkpoint == 0 {
					onProgress((i + 1) * 100 / len(rows))
				}
				continue
			}
			kotaID = &city.ID

			district, err := s.storeRepo.GetDistrictByNameInCity(ctx, *r.Kecamatan, *kotaID)
			if err != nil {
				result.Status = "failed"
				result.Error = err.Error()
				results[i] = result
				if onProgress != nil && (i+1)%checkpoint == 0 {
					onProgress((i + 1) * 100 / len(rows))
				}
				continue
			}
			if district == nil {
				result.Status = "failed"
				result.Error = ErrInvalidKecamatan.Error()
				results[i] = result
				if onProgress != nil && (i+1)%checkpoint == 0 {
					onProgress((i + 1) * 100 / len(rows))
				}
				continue
			}
			kecamatanID = &district.ID
		}
		if r.KodePos != nil {
			kodePos = r.KodePos
		}

		var dob *time.Time
		if r.DOB != nil {
			parsed, err := time.Parse("2006-01-02", *r.DOB)
			if err != nil {
				result.Status = "failed"
				result.Error = ErrInvalidDOBFormat.Error()
				results[i] = result
				if onProgress != nil && (i+1)%checkpoint == 0 {
					onProgress((i + 1) * 100 / len(rows))
				}
				continue
			}
			dob = &parsed
		}

		var grade *int
		if r.Grade != nil {
			parsed, err := strconv.Atoi(*r.Grade)
			if err != nil {
				result.Status = "failed"
				result.Error = ErrInvalidGradeFormat.Error()
				results[i] = result
				if onProgress != nil && (i+1)%checkpoint == 0 {
					onProgress((i + 1) * 100 / len(rows))
				}
				continue
			}
			grade = &parsed
		}

		resp, err := s.RegisterStudent(ctx, schoolID, r.Name, r.Jenjang, r.Email, dob, r.Gender, grade, r.AlamatDomisili, r.TargetExam, provinsiID, kotaID, kecamatanID, kodePos)
		if err == nil {
			result.Status = "success"
			result.Username = resp.Username
			result.TempPassword = resp.TempPassword
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

// BuildStudentBulkResultCSV writes the per-row report as CSV bytes.
func BuildStudentBulkResultCSV(results []StudentBulkResultRow) []byte {
	var buf bytes.Buffer
	w := csv.NewWriter(&buf)
	_ = w.Write([]string{"name", "school", "email", "status", "username", "temp_password", "error"})
	for _, r := range results {
		_ = w.Write([]string{r.Name, r.School, r.Email, r.Status, r.Username, r.TempPassword, r.Error})
	}
	w.Flush()
	return buf.Bytes()
}
