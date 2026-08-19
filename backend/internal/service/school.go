package service

import (
	"context"
	"fmt"
	"time"

	"akademi-bimbel/internal/model"
	"akademi-bimbel/internal/repository"
)

// SchoolResponse is the school shape returned in admin responses.
type SchoolResponse struct {
	ID           string   `json:"id"`
	Name         string   `json:"name"`
	Code         string   `json:"code"`
	NPSN         *string  `json:"npsn"`
	SchoolTypes  []string `json:"school_types"`
	Alamat       *string  `json:"alamat"`
	Status       string   `json:"status"`
	StudentCount int      `json:"student_count"`
	CreatedAt    string   `json:"created_at"`
	UpdatedAt    string   `json:"updated_at"`
}

func toSchoolResponse(row repository.SchoolAdminRow) SchoolResponse {
	return SchoolResponse{
		ID:           row.ID,
		Name:         row.Name,
		Code:         row.Code,
		NPSN:         row.NPSN,
		SchoolTypes:  row.SchoolTypes,
		Alamat:       row.Alamat,
		Status:       row.Status,
		StudentCount: row.StudentCount,
		CreatedAt:    row.CreatedAt.Format(time.RFC3339),
		UpdatedAt:    row.UpdatedAt.Format(time.RFC3339),
	}
}

// AdminListSchoolsParams carries the optional filters accepted by
// AdminListSchools, mirroring the StudentFilter convention used for the
// students list.
type AdminListSchoolsParams struct {
	Limit  int
	Cursor string
	Q      string
	Status string
}

// AdminListSchools returns schools cursor-paginated (keyset on name+id, see
// ListSchoolsAdmin), plus a filter-aware count for the caller's stat cards.
func (s *Service) AdminListSchools(ctx context.Context, params AdminListSchoolsParams) ([]SchoolResponse, string, repository.SchoolAdminCounts, error) {
	if params.Status != "" && params.Status != "active" && params.Status != "deactivated" {
		return nil, "", repository.SchoolAdminCounts{}, fmt.Errorf("%w: %s", ErrInvalidStatusFilter, params.Status)
	}

	filter := repository.SchoolAdminFilter{
		Q:      params.Q,
		Status: params.Status,
		Cursor: params.Cursor,
		Limit:  params.Limit,
	}

	rows, nextCursor, err := s.storeRepo.ListSchoolsAdmin(ctx, filter)
	if err != nil {
		return nil, "", repository.SchoolAdminCounts{}, err
	}

	counts, err := s.storeRepo.CountSchoolsAdmin(ctx, filter)
	if err != nil {
		return nil, "", repository.SchoolAdminCounts{}, err
	}

	schools := make([]SchoolResponse, len(rows))
	for i, r := range rows {
		schools[i] = toSchoolResponse(r)
	}
	return schools, nextCursor, counts, nil
}

// SchoolOptions returns the full active school registry (id/name/code) for
// picker dropdowns. See ListSchoolOptions for why this is unpaginated.
func (s *Service) SchoolOptions(ctx context.Context) ([]repository.SchoolOption, error) {
	return s.storeRepo.ListSchoolOptions(ctx)
}

// CreateSchool creates a new school with status='active' and student_count=0.
func (s *Service) CreateSchool(ctx context.Context, name, code string, npsn *string, schoolTypes []string, alamat *string) (*SchoolResponse, error) {
	if name == "" || code == "" {
		return nil, ErrMissingField
	}

	exists, err := s.storeRepo.SchoolCodeExists(ctx, code, nil)
	if err != nil {
		return nil, err
	}
	if exists {
		return nil, ErrSchoolCodeTaken
	}

	// school_types is TEXT[] NOT NULL with no default applied to an explicit
	// NULL — coerce a nil (omitted in the request) to an empty slice so the
	// INSERT doesn't violate the NOT NULL constraint.
	if schoolTypes == nil {
		schoolTypes = []string{}
	}

	school := &model.School{
		Name:        name,
		Code:        code,
		NPSN:        npsn,
		SchoolTypes: schoolTypes,
		Alamat:      alamat,
	}
	if err := s.storeRepo.CreateSchool(ctx, school); err != nil {
		return nil, err
	}

	return &SchoolResponse{
		ID:           school.ID,
		Name:         school.Name,
		Code:         school.Code,
		NPSN:         school.NPSN,
		SchoolTypes:  school.SchoolTypes,
		Alamat:       school.Alamat,
		Status:       "active",
		StudentCount: 0,
		CreatedAt:    school.CreatedAt.Format(time.RFC3339),
		UpdatedAt:    school.UpdatedAt.Format(time.RFC3339),
	}, nil
}

// UpdateSchool patches school fields. Nil pointers leave the corresponding
// column unchanged.
func (s *Service) UpdateSchool(ctx context.Context, id string, name, npsn, alamat *string, schoolTypes []string, code *string) (*SchoolResponse, error) {
	school, err := s.storeRepo.GetSchoolByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if school == nil {
		return nil, ErrSchoolNotFound
	}

	if code != nil && *code != school.Code {
		exists, err := s.storeRepo.SchoolCodeExists(ctx, *code, &id)
		if err != nil {
			return nil, err
		}
		if exists {
			return nil, ErrSchoolCodeTaken
		}
	}

	if err := s.storeRepo.UpdateSchool(ctx, id, name, npsn, alamat, schoolTypes, code); err != nil {
		return nil, err
	}

	updated, err := s.storeRepo.GetSchoolByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if updated == nil {
		return nil, ErrSchoolNotFound
	}

	count, err := s.storeRepo.CountStudentsBySchool(ctx, id)
	if err != nil {
		return nil, err
	}

	return &SchoolResponse{
		ID:           updated.ID,
		Name:         updated.Name,
		Code:         updated.Code,
		NPSN:         updated.NPSN,
		SchoolTypes:  updated.SchoolTypes,
		Alamat:       updated.Alamat,
		Status:       updated.Status,
		StudentCount: count,
		CreatedAt:    updated.CreatedAt.Format(time.RFC3339),
		UpdatedAt:    updated.UpdatedAt.Format(time.RFC3339),
	}, nil
}

// SchoolExists checks whether a school with the given ID exists.
func (s *Service) SchoolExists(ctx context.Context, id string) (bool, error) {
	school, err := s.storeRepo.GetSchoolByID(ctx, id)
	if err != nil {
		return false, err
	}
	return school != nil, nil
}

// ChangeSchoolStatus sets a school's status to active or deactivated.
func (s *Service) ChangeSchoolStatus(ctx context.Context, id, status string) (*SchoolResponse, error) {
	if status != "active" && status != "deactivated" {
		return nil, fmt.Errorf("%w: %s", ErrInvalidStatusFilter, status)
	}

	school, err := s.storeRepo.GetSchoolByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if school == nil {
		return nil, ErrSchoolNotFound
	}

	if err := s.storeRepo.UpdateSchoolStatus(ctx, id, status); err != nil {
		return nil, err
	}

	updated, err := s.storeRepo.GetSchoolByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if updated == nil {
		return nil, ErrSchoolNotFound
	}

	count, err := s.storeRepo.CountStudentsBySchool(ctx, id)
	if err != nil {
		return nil, err
	}

	return &SchoolResponse{
		ID:           updated.ID,
		Name:         updated.Name,
		Code:         updated.Code,
		NPSN:         updated.NPSN,
		SchoolTypes:  updated.SchoolTypes,
		Alamat:       updated.Alamat,
		Status:       updated.Status,
		StudentCount: count,
		CreatedAt:    updated.CreatedAt.Format(time.RFC3339),
		UpdatedAt:    updated.UpdatedAt.Format(time.RFC3339),
	}, nil
}
