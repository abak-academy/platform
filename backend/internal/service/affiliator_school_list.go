package service

import (
	"bytes"
	"context"
	"encoding/csv"
	"strings"

	"akademi-bimbel/internal/repository"
)

func (s *Service) validateAffiliatorSchoolQuery(q *repository.AffiliatorSchoolQuery) error {
	q.Q = strings.TrimSpace(q.Q)
	for _, id := range []*string{&q.AffiliatorID, &q.SchoolID} {
		*id = strings.ToLower(strings.TrimSpace(*id))
		if *id == "" {
			continue
		}
		if _, err := parseUUID(*id); err != nil {
			return ErrInvalidUUID
		}
	}
	return nil
}

// ListAffiliatorSchools returns which schools each affiliator has, one row per
// assignment, with the total matching the filters.
func (s *Service) ListAffiliatorSchools(ctx context.Context, q repository.AffiliatorSchoolQuery) ([]repository.AffiliatorSchoolRow, int, error) {
	if err := s.validateAffiliatorSchoolQuery(&q); err != nil {
		return nil, 0, err
	}
	return s.storeRepo.ListAffiliatorSchools(ctx, q)
}

// ExportAffiliatorSchools renders the same list, whole and unpaged (up to 10,000
// rows), as CSV with a BOM so Excel reads it as UTF-8; every cell is neutralised
// against spreadsheet formulas.
func (s *Service) ExportAffiliatorSchools(ctx context.Context, q repository.AffiliatorSchoolQuery) ([]byte, error) {
	if err := s.validateAffiliatorSchoolQuery(&q); err != nil {
		return nil, err
	}
	q.Limit, q.Offset = 10000, 0
	rows, _, err := s.storeRepo.ListAffiliatorSchools(ctx, q)
	if err != nil {
		return nil, err
	}

	var buf bytes.Buffer
	buf.Write([]byte{0xEF, 0xBB, 0xBF})
	w := csv.NewWriter(&buf)
	_ = w.Write([]string{"affiliator", "email", "status affiliator", "sekolah", "NPSN", "kode sekolah", "status sekolah", "siswa"})
	for _, r := range rows {
		email, npsn := "", ""
		if r.AffiliatorEmail != nil {
			email = *r.AffiliatorEmail
		}
		if r.SchoolNPSN != nil {
			npsn = *r.SchoolNPSN
		}
		_ = w.Write(csvSafeRow(r.AffiliatorName, email, r.AffiliatorStatus, r.SchoolName, npsn, r.SchoolCode, r.SchoolStatus, count(r.StudentCount)))
	}
	w.Flush()
	return buf.Bytes(), w.Error()
}
