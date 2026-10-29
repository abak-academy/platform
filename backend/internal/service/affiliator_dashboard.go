package service

import (
	"context"
	"strings"
	"time"

	"akademi-bimbel/internal/repository"

	"github.com/google/uuid"
)

// AffiliatorDashboard is everything the affiliator home page shows.
type AffiliatorDashboard struct {
	Schools     []repository.AffiliatorSchoolCount   `json:"schools"`
	Totals      repository.AffiliatorOrderTotals     `json:"totals"`
	Monthly     []repository.AffiliatorMonthPoint    `json:"monthly"`
	TopProducts []repository.AffiliatorSchoolProduct `json:"top_products"`
	// UnpurchasedStudents counts students of the allowed schools that none of
	// the affiliator's live orders covers yet.
	UnpurchasedStudents int `json:"unpurchased_students"`
}

// AffiliatorDashboard gathers the dashboard figures for a period (paid_at bounds).
func (s *Service) AffiliatorDashboard(ctx context.Context, affiliatorID string, from, to *time.Time) (*AffiliatorDashboard, error) {
	aID, err := uuid.Parse(affiliatorID)
	if err != nil {
		return nil, ErrInvalidUUID
	}
	ids, err := s.AffiliatorSchoolIDs(ctx, affiliatorID)
	if err != nil {
		return nil, err
	}
	d := &AffiliatorDashboard{}
	if d.Schools, err = s.storeRepo.AffiliatorSchoolCounts(ctx, ids); err != nil {
		return nil, err
	}
	if d.Totals, err = s.storeRepo.AffiliatorTotals(ctx, aID, from, to); err != nil {
		return nil, err
	}
	if d.Monthly, err = s.storeRepo.AffiliatorMonthly(ctx, aID, from, to); err != nil {
		return nil, err
	}
	if d.TopProducts, err = s.storeRepo.AffiliatorTopProducts(ctx, aID, from, to, 20); err != nil {
		return nil, err
	}
	counts, err := s.storeRepo.CountStudentsAdmin(ctx, "", repository.StudentFilter{
		SchoolIDs: ids, Status: "active", NotPurchasedBy: affiliatorID,
	})
	if err != nil {
		return nil, err
	}
	d.UnpurchasedStudents = counts.Total
	return d, nil
}

// ListAffiliatorOrders returns the affiliator's orders, newest first.
func (s *Service) ListAffiliatorOrders(ctx context.Context, affiliatorID, status string, limit int, cursor string) ([]repository.AffiliatorOrderRow, string, error) {
	aID, err := uuid.Parse(affiliatorID)
	if err != nil {
		return nil, "", ErrInvalidUUID
	}
	return s.storeRepo.ListAffiliatorOrders(ctx, aID, repository.AffiliatorOrderFilter{Status: status, Limit: limit, Cursor: cursor})
}

// ListAffiliatorPurchases returns which students each of the affiliator's orders covers.
func (s *Service) ListAffiliatorPurchases(ctx context.Context, affiliatorID string, q repository.AffiliatorPurchaseQuery) ([]repository.AffiliatorPurchaseRow, int, error) {
	aID, err := uuid.Parse(affiliatorID)
	if err != nil {
		return nil, 0, ErrInvalidUUID
	}
	q.Q = strings.TrimSpace(q.Q)
	if q.SchoolID != "" {
		sid, err := uuid.Parse(q.SchoolID)
		if err != nil {
			return nil, 0, ErrInvalidUUID
		}
		q.SchoolID = sid.String()
	}
	return s.storeRepo.ListAffiliatorPurchases(ctx, aID, q)
}

// AffiliatorDetail is the super-admin monitoring view of one affiliator.
type AffiliatorDetail struct {
	Affiliator AffiliatorResponse               `json:"affiliator"`
	Totals     repository.AffiliatorOrderTotals `json:"totals"`
	Recent     []repository.AffiliatorOrderRow  `json:"recent_orders"`
}

// GetAffiliatorDetail returns an affiliator's profile, lifetime totals and latest orders.
func (s *Service) GetAffiliatorDetail(ctx context.Context, affiliatorID string) (*AffiliatorDetail, error) {
	if _, err := uuid.Parse(affiliatorID); err != nil {
		return nil, ErrInvalidUUID
	}
	profile, err := s.GetAffiliatorProfile(ctx, affiliatorID)
	if err != nil {
		return nil, err
	}
	aID := uuid.MustParse(affiliatorID)
	totals, err := s.storeRepo.AffiliatorTotals(ctx, aID, nil, nil)
	if err != nil {
		return nil, err
	}
	recent, _, err := s.storeRepo.ListAffiliatorOrders(ctx, aID, repository.AffiliatorOrderFilter{Limit: 10})
	if err != nil {
		return nil, err
	}
	return &AffiliatorDetail{Affiliator: *profile, Totals: totals, Recent: recent}, nil
}
