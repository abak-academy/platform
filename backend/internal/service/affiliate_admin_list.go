package service

import (
	"bytes"
	"context"
	"encoding/csv"
	"fmt"
	"slices"
	"time"

	"akademi-bimbel/internal/repository"
)

// AffiliateAdminQueueFilter is the query every admin affiliate list shares.
// Status "all" or empty means no status filter (payouts: empty is the undecided
// queue, kept for the bank-transfer export); From/To is a half-open date range.
type AffiliateAdminQueueFilter struct {
	Q      string
	Status string
	From   *time.Time
	To     *time.Time
	Limit  int
	Offset int
}

func (f AffiliateAdminQueueFilter) repoQuery() repository.AffiliateListQuery {
	status := f.Status
	if status == "all" {
		status = ""
	}
	return repository.AffiliateListQuery{Q: f.Q, Status: status, From: f.From, To: f.To}
}

const affiliateAdminExportLimit = 5000

func checkAdminFilterStatus(status string, allowed []string) error {
	if status == "" || status == "all" || slices.Contains(allowed, status) {
		return nil
	}
	return fmt.Errorf("%w: %s", ErrValidation, status)
}

// AdminListAffiliateFlags backs GET /admin/affiliate-flags.
func (s *Service) AdminListAffiliateFlags(ctx context.Context, f AffiliateAdminQueueFilter) ([]repository.AffiliateFlagRow, int, error) {
	if err := checkAdminFilterStatus(f.Status, affiliateFlagStatuses); err != nil {
		return nil, 0, err
	}
	rows, total, err := s.storeRepo.ListAffiliateFlagRows(ctx, f.repoQuery(), f.Limit, f.Offset)
	if err != nil {
		return nil, 0, fmt.Errorf("list affiliate flags: %w", err)
	}
	return rows, total, nil
}

// AdminAffiliateFlagsCSV exports the flags matching the filter, capped at
// affiliateAdminExportLimit rows.
func (s *Service) AdminAffiliateFlagsCSV(ctx context.Context, f AffiliateAdminQueueFilter) ([]byte, error) {
	f.Limit, f.Offset = affiliateAdminExportLimit, 0
	rows, _, err := s.AdminListAffiliateFlags(ctx, f)
	if err != nil {
		return nil, err
	}
	return buildAdminCSV(
		[]string{"affiliator_name", "code", "rule", "status", "detail", "note", "created_at", "reviewed_at"},
		len(rows),
		func(i int) []string {
			row := rows[i]
			return []string{
				csvSafeField(row.AffiliatorName),
				csvSafeField(row.AffiliatorCode),
				csvSafeField(row.Rule),
				row.Status,
				csvSafeField(string(row.Payload)),
				csvSafeField(derefString(row.Note)),
				row.CreatedAt.Format(time.RFC3339),
				formatOptionalTime(row.ReviewedAt),
			}
		},
	)
}

// AdminListAffiliateAppeals backs GET /admin/affiliate-appeals.
func (s *Service) AdminListAffiliateAppeals(ctx context.Context, f AffiliateAdminQueueFilter) ([]repository.AffiliateAppealRow, int, error) {
	if err := checkAdminFilterStatus(f.Status, affiliateAppealStatuses); err != nil {
		return nil, 0, err
	}
	rows, total, err := s.storeRepo.ListAffiliateAppealRows(ctx, f.repoQuery(), f.Limit, f.Offset)
	if err != nil {
		return nil, 0, fmt.Errorf("list affiliate appeals: %w", err)
	}
	return rows, total, nil
}

// AdminAffiliateAppealsCSV exports the appeals matching the filter, capped at
// affiliateAdminExportLimit rows.
func (s *Service) AdminAffiliateAppealsCSV(ctx context.Context, f AffiliateAdminQueueFilter) ([]byte, error) {
	f.Limit, f.Offset = affiliateAdminExportLimit, 0
	rows, _, err := s.AdminListAffiliateAppeals(ctx, f)
	if err != nil {
		return nil, err
	}
	return buildAdminCSV(
		[]string{"affiliator_name", "code", "category", "description", "sanction", "appeal_text", "appeal_status", "violation_date", "appeal_decided_at"},
		len(rows),
		func(i int) []string {
			row := rows[i]
			return []string{
				csvSafeField(row.AffiliatorName),
				csvSafeField(row.AffiliatorCode),
				row.Category,
				csvSafeField(row.Description),
				row.Sanction,
				csvSafeField(derefString(row.AppealText)),
				row.AppealStatus,
				row.CreatedAt.Format(time.RFC3339),
				formatOptionalTime(row.AppealDecidedAt),
			}
		},
	)
}

func buildAdminCSV(header []string, n int, record func(i int) []string) ([]byte, error) {
	var buf bytes.Buffer
	buf.WriteString("\xef\xbb\xbf")
	writer := csv.NewWriter(&buf)
	if err := writer.Write(header); err != nil {
		return nil, err
	}
	for i := 0; i < n; i++ {
		if err := writer.Write(record(i)); err != nil {
			return nil, err
		}
	}
	writer.Flush()
	return buf.Bytes(), writer.Error()
}

func derefString(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

func formatOptionalTime(t *time.Time) string {
	if t == nil {
		return ""
	}
	return t.Format(time.RFC3339)
}
