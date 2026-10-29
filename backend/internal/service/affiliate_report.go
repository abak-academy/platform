package service

import (
	"bytes"
	"context"
	"encoding/csv"
	"fmt"
	"strconv"
	"time"

	"akademi-bimbel/internal/repository"
)

// AffiliateReportNames are the report ids the list and export routes accept.
var AffiliateReportNames = []string{"affiliators", "schools", "products", "promos", "trend"}

// ErrUnknownAffiliateReport is returned for a report id outside AffiliateReportNames.
var ErrUnknownAffiliateReport = fmt.Errorf("%w: unknown affiliate report", ErrValidation)

func (s *Service) AffiliatorReport(ctx context.Context, q repository.AffiliateReportQuery) ([]repository.AffiliatorReportRow, int, error) {
	return s.storeRepo.AffiliatorReport(ctx, q)
}

func (s *Service) AffiliateSchoolReport(ctx context.Context, q repository.AffiliateReportQuery) ([]repository.SchoolReportRow, int, error) {
	return s.storeRepo.SchoolReport(ctx, q)
}

func (s *Service) AffiliateProductReport(ctx context.Context, q repository.AffiliateReportQuery) ([]repository.ProductReportRow, int, error) {
	return s.storeRepo.ProductReport(ctx, q)
}

func (s *Service) AffiliatePromoReport(ctx context.Context, q repository.AffiliateReportQuery) ([]repository.PromoReportRow, int, error) {
	return s.storeRepo.PromoReport(ctx, q)
}

func (s *Service) AffiliateTrendReport(ctx context.Context, q repository.AffiliateReportQuery) ([]repository.TrendReportRow, error) {
	return s.storeRepo.TrendReport(ctx, q)
}

func money(v float64) string { return strconv.FormatFloat(v, 'f', 2, 64) }
func count(v int) string     { return strconv.Itoa(v) }

func optTime(t *time.Time) string {
	if t == nil {
		return ""
	}
	return t.In(time.FixedZone("WIB", 7*3600)).Format("2006-01-02")
}

func optFloat(v *float64) string {
	if v == nil {
		return ""
	}
	return money(*v)
}

func optInt(v *int) string {
	if v == nil {
		return ""
	}
	return count(*v)
}

func yesNo(b bool) string {
	if b {
		return "ya"
	}
	return "tidak"
}

// ExportAffiliateReport renders one report as CSV for the whole period (no
// paging, up to 10,000 rows). The file opens in Excel as UTF-8 and every cell is
// neutralised against spreadsheet formulas.
func (s *Service) ExportAffiliateReport(ctx context.Context, name string, q repository.AffiliateReportQuery) ([]byte, error) {
	q.Limit, q.Offset = 10000, 0
	var header []string
	var lines [][]string

	switch name {
	case "affiliators":
		rows, _, err := s.storeRepo.AffiliatorReport(ctx, q)
		if err != nil {
			return nil, err
		}
		header = []string{"nama", "email", "status", "sekolah", "siswa", "order dibayar", "unit", "total dibayar", "diskon promo", "order menunggu bayar", "nilai menunggu bayar", "order kedaluwarsa", "order pada periode", "order terakhir"}
		for _, r := range rows {
			email := ""
			if r.Email != nil {
				email = *r.Email
			}
			lines = append(lines, []string{r.Name, email, r.Status, count(r.SchoolCount), count(r.StudentCount), count(r.PaidOrders), count(r.PaidUnits),
				money(r.PaidAmount), money(r.DiscountTotal), count(r.PendingOrders), money(r.PendingAmount), count(r.ExpiredOrders), count(r.OrdersInRange), optTime(r.LastOrderAt)})
		}
	case "schools":
		rows, _, err := s.storeRepo.SchoolReport(ctx, q)
		if err != nil {
			return nil, err
		}
		header = []string{"sekolah", "affiliator", "siswa", "siswa sudah dibelikan", "siswa belum dibelikan", "order dibayar", "unit", "total dibayar", "diskon promo"}
		for _, r := range rows {
			lines = append(lines, []string{r.SchoolName, count(r.Affiliators), count(r.Students), count(r.CoveredStudent), count(r.Unpurchased),
				count(r.PaidOrders), count(r.PaidUnits), money(r.PaidAmount), money(r.DiscountTotal)})
		}
	case "products":
		rows, _, err := s.storeRepo.ProductReport(ctx, q)
		if err != nil {
			return nil, err
		}
		header = []string{"sekolah", "produk", "tipe", "unit", "order"}
		for _, r := range rows {
			lines = append(lines, []string{r.SchoolName, r.ProductName, r.ProductType, count(r.Units), count(r.Orders)})
		}
	case "promos":
		rows, _, err := s.storeRepo.PromoReport(ctx, q)
		if err != nil {
			return nil, err
		}
		header = []string{"kode", "khusus affiliator", "diskon persen", "diskon nominal", "min unit", "min belanja", "mulai", "berakhir", "batas pakai", "terpakai", "aktif sekarang", "order pada periode", "total diskon pada periode"}
		for _, r := range rows {
			lines = append(lines, []string{r.Code, yesNo(r.AffiliatorOnly), optFloat(r.DiscountPct), optFloat(r.DiscountAmount), optInt(r.MinQty), optFloat(r.MinOrderAmount),
				optTime(r.StartsAt), optTime(r.ExpiresAt), optInt(r.MaxUses), count(r.UsedCount), yesNo(r.ActiveNow), count(r.Orders), money(r.DiscountTotal)})
		}
	case "trend":
		rows, err := s.storeRepo.TrendReport(ctx, q)
		if err != nil {
			return nil, err
		}
		header = []string{"bulan", "order", "unit", "total dibayar", "diskon promo", "affiliator aktif"}
		for _, r := range rows {
			lines = append(lines, []string{r.Month, count(r.Orders), count(r.Units), money(r.Amount), money(r.Discount), count(r.Affiliators)})
		}
	default:
		return nil, ErrUnknownAffiliateReport
	}

	var buf bytes.Buffer
	buf.Write([]byte{0xEF, 0xBB, 0xBF})
	w := csv.NewWriter(&buf)
	_ = w.Write(header)
	for _, l := range lines {
		_ = w.Write(csvSafeRow(l...))
	}
	w.Flush()
	return buf.Bytes(), w.Error()
}
