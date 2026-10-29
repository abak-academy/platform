package service

import (
	"bytes"
	"context"
	"encoding/csv"
	"fmt"
	"io"
	"strconv"
	"strings"
)

// AffiliatorBulkRow is one parsed line of an affiliator-bulk CSV. A line names
// one school, by NPSN or by code; an affiliator with several schools is several
// lines that share the same email.
type AffiliatorBulkRow struct {
	Row        int
	Name       string
	Email      string
	Password   string
	SchoolNPSN string
	SchoolCode string
}

// AffiliatorBulkResultRow is the per-row outcome written to the result CSV.
type AffiliatorBulkResultRow struct {
	Row          int
	Name         string
	Email        string
	School       string
	Status       string
	TempPassword string
	Error        string
}

// ParseAffiliatorBulkCSV reads an affiliator-bulk upload. name and email are
// required columns, password is optional (blank means generate one), and the
// school columns are school_npsn and/or school_code.
func ParseAffiliatorBulkCSV(data []byte) ([]AffiliatorBulkRow, error) {
	r := newBulkCSVReader(data)

	header, err := r.Read()
	if err != nil {
		if err == io.EOF {
			return nil, ErrMissingAffiliatorCSVHeader
		}
		return nil, ErrInvalidCSV
	}

	nameIdx, emailIdx, passwordIdx, npsnIdx, codeIdx := -1, -1, -1, -1, -1
	for i, h := range header {
		switch normalizeCSVHeader(h) {
		case "name":
			nameIdx = i
		case "email":
			emailIdx = i
		case "password":
			passwordIdx = i
		case "school_npsn":
			npsnIdx = i
		case "school_code":
			codeIdx = i
		}
	}
	if nameIdx == -1 || emailIdx == -1 || (npsnIdx == -1 && codeIdx == -1) {
		return nil, ErrMissingAffiliatorCSVHeader
	}

	line := 1
	var rows []AffiliatorBulkRow
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
		rows = append(rows, AffiliatorBulkRow{
			Row:        line,
			Name:       bulkCell(record, nameIdx),
			Email:      bulkCell(record, emailIdx),
			Password:   bulkCell(record, passwordIdx),
			SchoolNPSN: bulkCell(record, npsnIdx),
			SchoolCode: bulkCell(record, codeIdx),
		})
	}
	return rows, nil
}

// resolveBulkSchool turns one line's NPSN or code into a school. When both are
// filled the NPSN is used, as in the student import. An unknown or inactive
// school fails the line.
func (s *Service) resolveBulkSchool(ctx context.Context, r AffiliatorBulkRow) (id, name string, err error) {
	switch {
	case r.SchoolNPSN != "":
		raw := r.SchoolNPSN
		npsn, err := normalizeSchoolNPSN(&raw)
		if err != nil {
			return "", "", err
		}
		school, err := s.storeRepo.GetSchoolByNPSN(ctx, *npsn)
		if err != nil {
			return "", "", err
		}
		if school == nil {
			return "", "", fmt.Errorf("%w: %s", ErrSchoolNotFoundByNPSN, raw)
		}
		if school.Status != "active" {
			return "", "", fmt.Errorf("%w: %s", ErrSchoolDeactivated, school.Name)
		}
		return school.ID, school.Name, nil
	case r.SchoolCode != "":
		school, err := s.storeRepo.GetSchoolByCode(ctx, r.SchoolCode)
		if err != nil {
			return "", "", err
		}
		if school == nil {
			return "", "", fmt.Errorf("%w: %s", ErrSchoolNotFoundByCode, r.SchoolCode)
		}
		if school.Status != "active" {
			return "", "", fmt.Errorf("%w: %s", ErrSchoolDeactivated, school.Name)
		}
		return school.ID, school.Name, nil
	}
	return "", "", ErrAffiliatorSchoolsRequired
}

// ProcessAffiliatorBulkRows creates one affiliator per distinct email, with a
// school for each of its lines. An account is all or nothing: if any of its lines
// is wrong, none of its lines is created and each says why, so fixing the file
// and uploading it again cannot leave an account with only some of its schools.
// Name and password come from the first line; later lines may leave them blank
// or repeat them. A blank password is generated and reported on the first line.
func (s *Service) ProcessAffiliatorBulkRows(ctx context.Context, actorID string, rows []AffiliatorBulkRow, onProgress func(pct int)) ([]AffiliatorBulkResultRow, int, error) {
	results := make([]AffiliatorBulkResultRow, len(rows))
	for i, r := range rows {
		results[i] = AffiliatorBulkResultRow{Row: r.Row, Name: r.Name, Email: normalizeEmail(r.Email)}
	}

	var order []string
	groups := map[string][]int{}
	for i, r := range rows {
		key := normalizeEmail(r.Email)
		if key == "" {
			key = fmt.Sprintf("\x00row-%d", i)
		}
		if _, seen := groups[key]; !seen {
			order = append(order, key)
		}
		groups[key] = append(groups[key], i)
	}

	successCount, done := 0, 0
	checkpoint := len(order) / 10
	if checkpoint < 1 {
		checkpoint = 1
	}
	for n, key := range order {
		idxs := groups[key]
		successCount += s.createBulkAffiliator(ctx, actorID, rows, results, idxs)
		done += len(idxs)
		if onProgress != nil && (n+1)%checkpoint == 0 {
			onProgress(done * 100 / len(rows))
		}
	}
	if onProgress != nil {
		onProgress(100)
	}
	return results, successCount, nil
}

// createBulkAffiliator handles all lines of one account and returns how many
// lines ended up created.
func (s *Service) createBulkAffiliator(ctx context.Context, actorID string, rows []AffiliatorBulkRow, results []AffiliatorBulkResultRow, idxs []int) int {
	first := rows[idxs[0]]
	failed := -1 // index of the first line that is wrong
	var ids []string
	seen := map[string]bool{}

	for _, i := range idxs {
		r := rows[i]
		var err error
		if i != idxs[0] {
			switch {
			case r.Name != "" && !strings.EqualFold(r.Name, first.Name):
				err = fmt.Errorf("name differs from row %d for the same email", first.Row)
			case r.Password != "" && r.Password != first.Password:
				err = fmt.Errorf("password differs from row %d for the same email", first.Row)
			}
		}
		if err == nil {
			var id, name string
			if id, name, err = s.resolveBulkSchool(ctx, r); err == nil {
				results[i].School = name
				if !seen[id] {
					seen[id] = true
					ids = append(ids, id)
				}
			}
		}
		if err != nil {
			results[i].Status = "failed"
			results[i].Error = err.Error()
			if failed < 0 {
				failed = i
			}
		}
	}

	if failed >= 0 {
		for _, i := range idxs {
			if results[i].Status != "failed" {
				results[i].Status = "failed"
				results[i].Error = fmt.Sprintf("not created: row %d of the same account is wrong", rows[failed].Row)
			}
		}
		return 0
	}

	acc, err := s.createAffiliator(ctx, actorID, first.Email, first.Name, first.Password, ids)
	if err != nil {
		for _, i := range idxs {
			results[i].Status = "failed"
			results[i].Error = err.Error()
		}
		return 0
	}
	for n, i := range idxs {
		results[i].Status = "success"
		if n == 0 {
			results[i].TempPassword = acc.TempPassword
		}
	}
	return len(idxs)
}

// BuildAffiliatorBulkResultCSV renders the per-row report. The temp_password
// column is the only place a generated password is ever shown.
func BuildAffiliatorBulkResultCSV(results []AffiliatorBulkResultRow) []byte {
	var buf bytes.Buffer
	w := csv.NewWriter(&buf)
	_ = w.Write([]string{"row", "name", "email", "school", "status", "temp_password", "error"})
	for _, r := range results {
		_ = w.Write(csvSafeRow(strconv.Itoa(r.Row), r.Name, r.Email, r.School, r.Status, r.TempPassword, r.Error))
	}
	w.Flush()
	return buf.Bytes()
}
