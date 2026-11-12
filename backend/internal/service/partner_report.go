package service

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"akademi-bimbel/internal/repository"
)

// PartnerReport is one page of the admin's Mitra purchase report with the totals
// of everything the filters match.
type PartnerReport struct {
	Data    []repository.PartnerReportRow   `json:"data"`
	Page    int                             `json:"page"`
	PerPage int                             `json:"per_page"`
	Total   int                             `json:"total"`
	Summary repository.PartnerReportSummary `json:"summary"`
}

func normalizePartnerReportFilter(f repository.PartnerReportFilter) (repository.PartnerReportFilter, error) {
	if f.PartnerID != "" {
		id, err := parseUUID(f.PartnerID)
		if err != nil {
			return f, ErrInvalidUUID
		}
		f.PartnerID = id.String()
	}
	f.Q = strings.TrimSpace(f.Q)
	return f, nil
}

// AdminPartnerReport lists Mitra purchases: who bought, what, for which
// students, with which voucher.
func (s *Service) AdminPartnerReport(ctx context.Context, f repository.PartnerReportFilter, page, perPage int) (*PartnerReport, error) {
	f, err := normalizePartnerReportFilter(f)
	if err != nil {
		return nil, err
	}
	rows, total, err := s.storeRepo.ListPartnerReport(ctx, f, page, perPage)
	if err != nil {
		return nil, fmt.Errorf("list partner report: %w", err)
	}
	summary, err := s.storeRepo.PartnerReportTotals(ctx, f)
	if err != nil {
		return nil, fmt.Errorf("partner report totals: %w", err)
	}
	return &PartnerReport{Data: rows, Page: page, PerPage: perPage, Total: total, Summary: summary}, nil
}

// AdminPartnerReportCSV exports the purchases matching the filter, capped at
// affiliateAdminExportLimit rows.
func (s *Service) AdminPartnerReportCSV(ctx context.Context, f repository.PartnerReportFilter) ([]byte, error) {
	f, err := normalizePartnerReportFilter(f)
	if err != nil {
		return nil, err
	}
	rows, _, err := s.storeRepo.ListPartnerReport(ctx, f, 1, affiliateAdminExportLimit)
	if err != nil {
		return nil, fmt.Errorf("list partner report: %w", err)
	}
	return buildAdminCSV(
		[]string{"created_at", "partner_name", "order_number", "order_id", "status", "products", "seats", "students", "voucher_code", "subtotal", "discount", "total"},
		len(rows),
		func(i int) []string {
			row := rows[i]
			return []string{
				row.CreatedAt.Format("2006-01-02 15:04:05"),
				csvSafeField(row.PartnerName),
				affiliateOrderNumber(row.OrderID),
				row.OrderID.String(),
				row.Status,
				csvSafeField(row.Products),
				strconv.Itoa(row.Seats),
				csvSafeField(row.Students),
				csvSafeField(row.PromoCode),
				strconv.FormatFloat(row.Subtotal, 'f', 2, 64),
				strconv.FormatFloat(row.Discount, 'f', 2, 64),
				strconv.FormatFloat(row.Total, 'f', 2, 64),
			}
		},
	)
}
