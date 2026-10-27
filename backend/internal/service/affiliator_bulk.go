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

// AffiliatorBulkRow is one parsed line of an affiliator-bulk CSV. Each school
// cell may hold several values separated by '|' (or ','), and a row needs at
// least one school across school_npsn and school_code.
type AffiliatorBulkRow struct {
	Row        int
	Name       string
	Email      string
	Password   string
	SchoolNPSN []string
	SchoolCode []string
}

// AffiliatorBulkResultRow is the per-row outcome written to the result CSV.
type AffiliatorBulkResultRow struct {
	Row          int
	Name         string
	Email        string
	Schools      string
	Status       string
	TempPassword string
	Error        string
}

func splitBulkList(cell string) []string {
	var out []string
	for _, part := range strings.Split(strings.ReplaceAll(cell, "|", ","), ",") {
		if part = strings.TrimSpace(part); part != "" {
			out = append(out, part)
		}
	}
	return out
}

// ParseAffiliatorBulkCSV reads an affiliator-bulk upload. name and email are
// required, password is optional (blank means generate one), and the school
// columns are school_npsn and/or school_code.
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
			SchoolNPSN: splitBulkList(bulkCell(record, npsnIdx)),
			SchoolCode: splitBulkList(bulkCell(record, codeIdx)),
		})
	}
	return rows, nil
}

// resolveBulkSchools turns a row's NPSNs and codes into school ids and names, in
// the order given and without repeats. Any unknown or inactive school fails the row.
func (s *Service) resolveBulkSchools(ctx context.Context, r AffiliatorBulkRow) (ids, names []string, err error) {
	seen := map[string]bool{}
	add := func(id, name string) {
		if !seen[id] {
			seen[id] = true
			ids = append(ids, id)
			names = append(names, name)
		}
	}
	for _, raw := range r.SchoolNPSN {
		raw := raw
		npsn, err := normalizeSchoolNPSN(&raw)
		if err != nil {
			return nil, nil, err
		}
		school, err := s.storeRepo.GetSchoolByNPSN(ctx, *npsn)
		if err != nil {
			return nil, nil, err
		}
		if school == nil {
			return nil, nil, fmt.Errorf("%w: %s", ErrSchoolNotFoundByNPSN, raw)
		}
		if school.Status != "active" {
			return nil, nil, fmt.Errorf("%w: %s", ErrSchoolDeactivated, school.Name)
		}
		add(school.ID, school.Name)
	}
	for _, code := range r.SchoolCode {
		school, err := s.storeRepo.GetSchoolByCode(ctx, code)
		if err != nil {
			return nil, nil, err
		}
		if school == nil {
			return nil, nil, fmt.Errorf("%w: %s", ErrSchoolNotFoundByCode, code)
		}
		if school.Status != "active" {
			return nil, nil, fmt.Errorf("%w: %s", ErrSchoolDeactivated, school.Name)
		}
		add(school.ID, school.Name)
	}
	if len(ids) == 0 {
		return nil, nil, ErrAffiliatorSchoolsRequired
	}
	return ids, names, nil
}

// ProcessAffiliatorBulkRows creates each affiliator through the same path as the
// single create, so a bad row never stops the others. A blank password is
// generated and reported in the result.
func (s *Service) ProcessAffiliatorBulkRows(ctx context.Context, actorID string, rows []AffiliatorBulkRow, onProgress func(pct int)) ([]AffiliatorBulkResultRow, int, error) {
	results := make([]AffiliatorBulkResultRow, len(rows))
	successCount := 0

	checkpoint := len(rows) / 10
	if checkpoint < 1 {
		checkpoint = 1
	}

	for i, r := range rows {
		result := AffiliatorBulkResultRow{Row: r.Row, Name: r.Name, Email: normalizeEmail(r.Email)}
		ids, names, err := s.resolveBulkSchools(ctx, r)
		if err == nil {
			result.Schools = strings.Join(names, "|")
			var acc *AffiliatorResponse
			acc, err = s.createAffiliator(ctx, actorID, r.Email, r.Name, r.Password, ids)
			if err == nil {
				result.TempPassword = acc.TempPassword
			}
		}
		if err != nil {
			result.Status = "failed"
			result.Error = err.Error()
		} else {
			result.Status = "success"
			successCount++
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

// BuildAffiliatorBulkResultCSV renders the per-row report. The temp_password
// column is the only place a generated password is ever shown.
func BuildAffiliatorBulkResultCSV(results []AffiliatorBulkResultRow) []byte {
	var buf bytes.Buffer
	w := csv.NewWriter(&buf)
	_ = w.Write([]string{"row", "name", "email", "schools", "status", "temp_password", "error"})
	for _, r := range results {
		_ = w.Write(csvSafeRow(strconv.Itoa(r.Row), r.Name, r.Email, r.Schools, r.Status, r.TempPassword, r.Error))
	}
	w.Flush()
	return buf.Bytes()
}
