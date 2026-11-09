package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"akademi-bimbel/internal/repository"
)

// ErrStudentAlreadyActivated reports a credential reissue for a student who has
// already replaced the temporary password: the account is theirs now.
var ErrStudentAlreadyActivated = errors.New("student has already set their own password")

// PartnerRegisterStudentInput is one student a Mitra registers. The school is
// optional and may be any active school. There is no password: the student
// always starts with a temporary one.
type PartnerRegisterStudentInput struct {
	SchoolID       string
	Name           string
	Jenjang        string
	Email          *string
	DOB            *time.Time
	Gender         *string
	Grade          *int
	AlamatDomisili *string
	TargetExam     *string
	ProvinsiID     *string
	KotaID         *string
	KecamatanID    *string
	KodePos        *string
}

// PartnerStudentsPage is a page of a Mitra's student list. Hint explains an
// empty list that is empty for a reason the Mitra can act on.
type PartnerStudentsPage struct {
	Data       []repository.PartnerStudentRow `json:"data"`
	NextCursor string                         `json:"next_cursor"`
	Hint       string                         `json:"hint,omitempty"`
}

// PartnerStudentListFilter is the filter of GET /partners/me/students.
type PartnerStudentListFilter struct {
	SchoolID       string
	Status         string
	Q              string
	RegisteredByMe bool
	Cursor         string
	Limit          int
}

// PartnerRegisterStudent registers a student on behalf of a Mitra and records
// the Mitra as the registrar. The temporary password comes back once.
func (s *Service) PartnerRegisterStudent(ctx context.Context, partnerID string, in PartnerRegisterStudentInput) (*StudentRegistrationResponse, error) {
	if _, err := parseUUID(partnerID); err != nil {
		return nil, ErrInvalidUUID
	}
	return s.registerStudent(ctx, partnerID, in.SchoolID, nil, in.Name, in.Jenjang, in.Email, in.DOB, in.Gender, in.Grade,
		in.AlamatDomisili, in.TargetExam, in.ProvinsiID, in.KotaID, in.KecamatanID, in.KodePos, nil, false, &partnerID)
}

// ListPartnerStudents lists the students of the Mitra's allowed schools plus
// the ones it registered.
func (s *Service) ListPartnerStudents(ctx context.Context, partnerID string, allowedSchoolIDs []string, filter PartnerStudentListFilter) (*PartnerStudentsPage, error) {
	if _, err := parseUUID(partnerID); err != nil {
		return nil, ErrInvalidUUID
	}
	if filter.SchoolID != "" {
		id, err := parseUUID(filter.SchoolID)
		if err != nil {
			return nil, ErrInvalidUUID
		}
		filter.SchoolID = id.String()
	}
	filter.Q = strings.TrimSpace(filter.Q)

	rows, next, err := s.storeRepo.ListPartnerStudents(ctx, partnerID, allowedSchoolIDs, repository.PartnerStudentFilter{
		SchoolID:       filter.SchoolID,
		Status:         filter.Status,
		Q:              filter.Q,
		RegisteredByMe: filter.RegisteredByMe,
		Cursor:         filter.Cursor,
		Limit:          filter.Limit,
	})
	if err != nil {
		return nil, fmt.Errorf("list partner students: %w", err)
	}
	page := &PartnerStudentsPage{Data: rows, NextCursor: next}
	if len(rows) == 0 && len(allowedSchoolIDs) == 0 && filter.Cursor == "" && filter.Q == "" && filter.SchoolID == "" {
		page.Hint = "no_schools"
	}
	return page, nil
}

// PartnerReissueStudentCredentials gives a student the Mitra registered a new
// temporary password. It is refused once the student has chosen their own.
func (s *Service) PartnerReissueStudentCredentials(ctx context.Context, partnerID, studentID string) (*StudentCredentialsResponse, error) {
	if _, err := parseUUID(partnerID); err != nil {
		return nil, ErrInvalidUUID
	}
	id, err := parseUUID(studentID)
	if err != nil {
		return nil, ErrInvalidUUID
	}
	reg, err := s.storeRepo.GetStudentRegistration(ctx, id.String())
	if err != nil {
		return nil, fmt.Errorf("get student registration: %w", err)
	}
	if reg == nil || reg.RegisteredBy == nil || *reg.RegisteredBy != partnerID || reg.Username == nil {
		return nil, ErrStudentNotFound
	}
	if !reg.PasswordTemporary {
		return nil, ErrStudentAlreadyActivated
	}

	tempPass, err := genTempPassword()
	if err != nil {
		return nil, fmt.Errorf("generate temp password: %w", err)
	}
	hash, err := hashPassword(tempPass)
	if err != nil {
		return nil, err
	}
	changed, err := s.storeRepo.ReissueStudentCredentialsByRegistrar(ctx, id.String(), partnerID, string(hash))
	if err != nil {
		return nil, fmt.Errorf("reissue student credentials: %w", err)
	}
	if !changed {
		return nil, ErrStudentAlreadyActivated
	}
	s.revokeRefreshSessions(ctx, id.String())

	actor := partnerID
	if err := s.storeRepo.InsertAuditLogMeta(ctx, nil, &actor, "user", id.String(), "student.reissue_credentials", map[string]any{"by": "partner"}); err != nil {
		return nil, err
	}
	return &StudentCredentialsResponse{Username: *reg.Username, TempPassword: tempPass}, nil
}

// PartnerJobStatus returns a job the Mitra created, with the result link.
func (s *Service) PartnerJobStatus(ctx context.Context, partnerID, jobID string) (*JobResponse, error) {
	return s.GetJobStatus(ctx, jobID, partnerID)
}
