package service

import (
	"bytes"
	"context"
	"encoding/csv"
	"fmt"
	"slices"
	"strconv"
	"time"

	"akademi-bimbel/internal/repository"
)

// AffiliateAdminQueueFilter is the query every admin affiliate list shares.
// Status "all" or empty means no status filter (payouts: empty is the undecided
// queue, kept for the bank-transfer export); From/To is a half-open date range.
type AffiliateAdminQueueFilter struct {
	Q      string
	Status string
	Type   string
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

// AdminListAffiliateCommissions backs GET /admin/affiliates/:id/commissions: the
// orders behind one affiliator's commissions, with the student's full name (an
// admin may see it; the affiliator never does). Q matches student or product,
// Type is the product category, From/To bound the payment date.
func (s *Service) AdminListAffiliateCommissions(ctx context.Context, affiliatorID string, f AffiliateAdminQueueFilter) ([]repository.AffiliateCommissionRow, int, error) {
	id, err := parseUUID(affiliatorID)
	if err != nil {
		return nil, 0, ErrInvalidUUID
	}
	if f.Status != "" && f.Status != "all" && !affiliateMeCommissionStatuses[f.Status] {
		return nil, 0, fmt.Errorf("%w: %s", ErrInvalidStatusFilter, f.Status)
	}
	if f.Type != "" && !slices.Contains(affiliateCommissionCategories(), f.Type) {
		return nil, 0, fmt.Errorf("%w: %s", ErrValidation, f.Type)
	}
	aff, err := s.storeRepo.GetAffiliatorByID(ctx, nil, id)
	if err != nil {
		return nil, 0, fmt.Errorf("find affiliator: %w", err)
	}
	if aff == nil {
		return nil, 0, ErrAffiliateNotFound
	}
	status := f.Status
	if status == "all" {
		status = ""
	}
	filter := repository.AffiliateCommissionFilter{
		Q: f.Q, Status: status, ProductType: f.Type, PaidFrom: f.From, PaidTo: f.To, Limit: f.Limit, Offset: f.Offset,
	}
	total, err := s.storeRepo.CountAffiliateCommissionsForAffiliator(ctx, id, filter)
	if err != nil {
		return nil, 0, fmt.Errorf("count affiliate commissions: %w", err)
	}
	rows, err := s.storeRepo.ListAffiliateCommissionsForAffiliator(ctx, id, filter)
	if err != nil {
		return nil, 0, fmt.Errorf("list affiliate commissions: %w", err)
	}
	return rows, total, nil
}

// AdminAffiliateCommissionsCSV exports the commissions matching the filter,
// capped at affiliateAdminExportLimit rows.
func (s *Service) AdminAffiliateCommissionsCSV(ctx context.Context, affiliatorID string, f AffiliateAdminQueueFilter) ([]byte, error) {
	f.Limit, f.Offset = affiliateAdminExportLimit, 0
	rows, _, err := s.AdminListAffiliateCommissions(ctx, affiliatorID, f)
	if err != nil {
		return nil, err
	}
	return buildAdminCSV(
		[]string{"paid_at", "order_id", "student_name", "product_name", "category", "base_amount", "commission_type", "commission_value", "commission", "status", "available_from"},
		len(rows),
		func(i int) []string {
			row := rows[i]
			return []string{
				formatOptionalTime(row.PaidAt),
				row.OrderID.String(),
				csvSafeField(row.StudentName),
				csvSafeField(row.ProductName),
				row.ProductType,
				strconv.FormatFloat(row.BaseAmount, 'f', 2, 64),
				row.CommissionType,
				strconv.FormatFloat(row.CommissionValue, 'f', 2, 64),
				strconv.FormatFloat(row.Amount, 'f', 2, 64),
				row.Status,
				formatOptionalTime(row.WaitUntil),
			}
		},
	)
}

// AdminListAffiliateReportOrders backs GET /admin/affiliate-reports/orders: the
// commissions of every affiliator with full student names, filtered like the
// per-affiliator list.
func (s *Service) AdminListAffiliateReportOrders(ctx context.Context, f AffiliateAdminQueueFilter) ([]repository.AffiliateReportOrderRow, int, error) {
	if f.Status != "" && f.Status != "all" && !affiliateMeCommissionStatuses[f.Status] {
		return nil, 0, fmt.Errorf("%w: %s", ErrInvalidStatusFilter, f.Status)
	}
	if f.Type != "" && !slices.Contains(affiliateCommissionCategories(), f.Type) {
		return nil, 0, fmt.Errorf("%w: %s", ErrValidation, f.Type)
	}
	status := f.Status
	if status == "all" {
		status = ""
	}
	rows, total, err := s.storeRepo.ListAffiliateReportOrders(ctx, repository.AffiliateReportOrderFilter{
		Q: f.Q, Status: status, ProductType: f.Type, PaidFrom: f.From, PaidTo: f.To, Limit: f.Limit, Offset: f.Offset,
	})
	if err != nil {
		return nil, 0, fmt.Errorf("list affiliate report orders: %w", err)
	}
	return rows, total, nil
}

// AdminAffiliateReportOrdersCSV exports the orders matching the filter, capped
// at affiliateAdminExportLimit rows.
func (s *Service) AdminAffiliateReportOrdersCSV(ctx context.Context, f AffiliateAdminQueueFilter) ([]byte, error) {
	f.Limit, f.Offset = affiliateAdminExportLimit, 0
	rows, _, err := s.AdminListAffiliateReportOrders(ctx, f)
	if err != nil {
		return nil, err
	}
	return buildAdminCSV(
		[]string{"paid_at", "affiliator_name", "code", "order_id", "student_name", "product_name", "category", "base_amount", "commission_type", "commission_value", "commission", "status", "available_from"},
		len(rows),
		func(i int) []string {
			row := rows[i]
			return []string{
				formatOptionalTime(row.PaidAt),
				csvSafeField(row.AffiliatorName),
				csvSafeField(row.AffiliatorCode),
				row.OrderID.String(),
				csvSafeField(row.StudentName),
				csvSafeField(row.ProductName),
				row.ProductType,
				strconv.FormatFloat(row.BaseAmount, 'f', 2, 64),
				row.CommissionType,
				strconv.FormatFloat(row.CommissionValue, 'f', 2, 64),
				strconv.FormatFloat(row.Amount, 'f', 2, 64),
				row.Status,
				formatOptionalTime(row.WaitUntil),
			}
		},
	)
}
