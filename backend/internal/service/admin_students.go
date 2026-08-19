package service

import (
	"context"
	"crypto/rand"
	"fmt"
	"math/big"
	"strings"
	"time"

	"akademi-bimbel/internal/model"
	"akademi-bimbel/internal/repository"
)

// normalizeGender maps the API's male/female wire values to the DB's
// users_gender_check constraint ('m'/'f' only, see migration 0002_identity).
// Without this, RegisterStudent's INSERT fails with a raw Postgres
// check-constraint error for every request that sets a gender at all.
func normalizeGender(gender *string) (*string, error) {
	if gender == nil || *gender == "" {
		return nil, nil
	}
	switch strings.ToLower(*gender) {
	case "male", "m":
		v := "m"
		return &v, nil
	case "female", "f":
		v := "f"
		return &v, nil
	default:
		return nil, ErrInvalidGender
	}
}

const tempPasswordLen = 10

var tempPasswordChars = []rune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789")

func genTempPassword() (string, error) {
	pass := make([]rune, tempPasswordLen)
	for i := range pass {
		n, err := rand.Int(rand.Reader, big.NewInt(int64(len(tempPasswordChars))))
		if err != nil {
			return "", err
		}
		pass[i] = tempPasswordChars[n.Int64()]
	}
	return string(pass), nil
}

// --- response types ---

type StudentRegistrationResponse struct {
	ID           string  `json:"id"`
	Name         string  `json:"name"`
	Username     string  `json:"username"`
	Jenjang      string  `json:"jenjang"`
	ProvinsiID   *string `json:"provinsi_id"`
	KotaID       *string `json:"kota_id"`
	KecamatanID  *string `json:"kecamatan_id"`
	KodePos      *string `json:"kode_pos"`
	Email        *string `json:"email"`
	TempPassword string  `json:"temp_password"`
	CreatedAt    string  `json:"created_at"`
}

type StudentResponse struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	// Nullable: some accounts genuinely have no username on file.
	Username *string `json:"username"`
	Email    *string `json:"email"`
	Status   string  `json:"status"`
	Grade    *int    `json:"grade"`
	// SchoolName is NULL for registrants with no school on file;
	// UnlistedSchoolName carries what a self-registering user typed when their
	// school wasn't listed. Both are surfaced so operations can follow up.
	SchoolName         *string `json:"school_name"`
	UnlistedSchoolName *string `json:"unlisted_school_name"`
	CreatedAt          string  `json:"created_at"`
}

type StudentCredentialsResponse struct {
	Username     string `json:"username"`
	TempPassword string `json:"temp_password"`
}

func toStudentResponse(row repository.StudentRow) StudentResponse {
	var grade *int
	if row.Grade != nil {
		grade = row.Grade
	}
	return StudentResponse{
		ID:                 row.ID,
		Name:               row.Name,
		Username:           row.Username,
		Email:              row.Email,
		Status:             row.Status,
		Grade:              grade,
		SchoolName:         row.SchoolName,
		UnlistedSchoolName: row.UnlistedSchoolName,
		CreatedAt:          row.CreatedAt.Format(time.RFC3339),
	}
}

// jenjangInSchoolTypes checks whether jenjang is present in the school's
// SchoolTypes slice. Exported for reuse by the profile-update path (Task 30).
func jenjangInSchoolTypes(jenjang string, types []string) bool {
	for _, t := range types {
		if t == jenjang {
			return true
		}
	}
	return false
}

// --- methods ---

// RegisterStudent creates a new student user under the given school.
// Returns the plaintext temp password exactly once in the response.
// jenjang is required; provinsiID/kotaID/kecamatanID are optional but must be
// all-or-nothing (FR-REG-02a). kodePos is independently optional.
func (s *Service) RegisterStudent(ctx context.Context, schoolID, name, jenjang string, email *string, dob *time.Time, gender *string, grade *int, alamatDomisili, targetExam *string, provinsiID, kotaID, kecamatanID, kodePos *string) (*StudentRegistrationResponse, error) {
	if name == "" || jenjang == "" {
		return nil, ErrMissingField
	}

	if kodePos != nil {
		if err := ValidateKodePos(*kodePos); err != nil {
			return nil, err
		}
	}

	// School is optional: not every registrant is a school pupil — university
	// students and members of the public sign up for IELTS and similar. When
	// one is given it is still validated; when it is omitted an operator
	// confirms the school after registration.
	if schoolID != "" {
		school, err := s.storeRepo.GetSchoolByID(ctx, schoolID)
		if err != nil {
			return nil, err
		}
		if school == nil {
			return nil, ErrSchoolNotFound
		}
		if school.Status == "deactivated" {
			return nil, ErrSchoolDeactivated
		}

		// Validate jenjang against school's SchoolTypes when types are configured.
		if len(school.SchoolTypes) > 0 && !jenjangInSchoolTypes(jenjang, school.SchoolTypes) {
			return nil, ErrInvalidJenjang
		}
	}

	gender, err := normalizeGender(gender)
	if err != nil {
		return nil, err
	}

	// All-or-nothing address validation (FR-REG-02a).
	addrCount := 0
	if provinsiID != nil {
		addrCount++
	}
	if kotaID != nil {
		addrCount++
	}
	if kecamatanID != nil {
		addrCount++
	}
	if addrCount > 0 && addrCount < 3 {
		return nil, ErrIncompleteAddress
	}

	// If all three address fields are present, validate each.
	if addrCount == 3 {
		prov, err := s.storeRepo.GetProvinceByID(ctx, *provinsiID)
		if err != nil {
			return nil, err
		}
		if prov == nil {
			return nil, ErrInvalidProvinsi
		}

		city, err := s.storeRepo.GetCityByID(ctx, *kotaID)
		if err != nil {
			return nil, err
		}
		if city == nil || city.ProvinceID != *provinsiID {
			return nil, ErrInvalidKota
		}

		district, err := s.storeRepo.GetDistrictByID(ctx, *kecamatanID)
		if err != nil {
			return nil, err
		}
		if district == nil || district.CityID != *kotaID {
			return nil, ErrInvalidKecamatan
		}
	}

	// Generate unique username (Task 6).
	username, err := s.generateUniqueUsername(ctx, name)
	if err != nil {
		return nil, err
	}

	tempPass, err := genTempPassword()
	if err != nil {
		return nil, fmt.Errorf("generate temp password: %w", err)
	}

	hash, err := hashPassword(tempPass)
	if err != nil {
		return nil, err
	}

	// Leave school_id NULL rather than writing an empty string into a uuid column.
	var schoolIDPtr *string
	if schoolID != "" {
		schoolIDPtr = &schoolID
	}

	user := &model.User{
		Username:       &username,
		Name:           name,
		Email:          email,
		PasswordHash:   string(hash),
		Role:           RoleStudent,
		SchoolID:       schoolIDPtr,
		Status:         "active",
		OTPEnabled:     false,
		Jenjang:        &jenjang,
		ProvinsiID:     provinsiID,
		KotaID:         kotaID,
		KecamatanID:    kecamatanID,
		KodePos:        kodePos,
		DOB:            dob,
		Gender:         gender,
		Grade:          grade,
		AlamatDomisili: alamatDomisili,
		TargetExam:     targetExam,
	}
	if err := s.storeRepo.CreateStudent(ctx, user); err != nil {
		return nil, err
	}

	return &StudentRegistrationResponse{
		ID:           user.ID,
		Name:         user.Name,
		Username:     username,
		Jenjang:      jenjang,
		ProvinsiID:   provinsiID,
		KotaID:       kotaID,
		KecamatanID:  kecamatanID,
		KodePos:      kodePos,
		Email:        email,
		TempPassword: tempPass,
		CreatedAt:    user.CreatedAt.Format(time.RFC3339),
	}, nil
}

// CrossSchoolStudentResponse is the response shape for cross-school student
// search (FR-SEARCH-01). Includes school_name so results are distinguishable.
type CrossSchoolStudentResponse struct {
	ID       string  `json:"id"`
	Name     string  `json:"name"`
	Username *string `json:"username"`
	Email    *string `json:"email"`
	Status   string  `json:"status"`
	Grade    *int    `json:"grade"`
	// SchoolID/SchoolName are nullable for students with no school on file;
	// UnlistedSchoolName carries the free-text name they typed instead.
	SchoolID           *string `json:"school_id"`
	SchoolName         *string `json:"school_name"`
	UnlistedSchoolName *string `json:"unlisted_school_name"`
	CreatedAt          string  `json:"created_at"`
}

func toCrossSchoolStudentResponse(row repository.CrossSchoolStudentRow) CrossSchoolStudentResponse {
	return CrossSchoolStudentResponse{
		ID:                 row.ID,
		Name:               row.Name,
		Username:           row.Username,
		Email:              row.Email,
		Status:             row.Status,
		Grade:              row.Grade,
		SchoolID:           row.SchoolID,
		SchoolName:         row.SchoolName,
		UnlistedSchoolName: row.UnlistedSchoolName,
		CreatedAt:          row.CreatedAt.Format(time.RFC3339),
	}
}

// SearchStudentsAcrossSchools searches students across all schools with optional
// filters. Thin pass-through to the repository with bounded default limit.
// This is the super_admin cross-school search (FR-SEARCH-01/03).
func (s *Service) SearchStudentsAcrossSchools(ctx context.Context, q string, schoolID *string, noSchool bool, grade *int, jenjang string, limit int, cursor string) ([]CrossSchoolStudentResponse, string, error) {
	rows, nextCursor, err := s.storeRepo.SearchStudentsAcrossSchools(ctx, repository.StudentFilter{
		Cursor:   cursor,
		Limit:    limit,
		Q:        q,
		SchoolID: schoolID,
		NoSchool: noSchool,
		Grade:    grade,
		Jenjang:  jenjang,
	})
	if err != nil {
		return nil, "", err
	}

	students := make([]CrossSchoolStudentResponse, len(rows))
	for i, r := range rows {
		students[i] = toCrossSchoolStudentResponse(r)
	}
	return students, nextCursor, nil
}

// ListStudents returns cursor-paginated students scoped to the given school.
// Optional grade and jenjang filters narrow the result set.
func (s *Service) ListStudents(ctx context.Context, schoolID string, statusFilter, q string, limit int, cursor string, grade *int, jenjang string) ([]StudentResponse, string, error) {
	rows, nextCursor, err := s.storeRepo.ListStudentsBySchool(ctx, schoolID, repository.StudentFilter{
		Status:  statusFilter,
		Cursor:  cursor,
		Limit:   limit,
		Q:       q,
		Grade:   grade,
		Jenjang: jenjang,
		// An empty schoolID reaches here only from the roster endpoint, where
		// super_admin omitted school_id — list every registrant, including
		// those with no school on file.
		AllSchools: schoolID == "",
	})
	if err != nil {
		return nil, "", err
	}

	students := make([]StudentResponse, len(rows))
	for i, r := range rows {
		students[i] = toStudentResponse(r)
	}
	return students, nextCursor, nil
}

// ChangeStudentStatus toggles a student's active/deactivated status.
// Row-scoping via schoolID + student ID when schoolID is set; empty schoolID
// is id-only (super_admin). Returns ErrStudentNotFound if the student does
// not exist or belongs to a different school.
func (s *Service) ChangeStudentStatus(ctx context.Context, schoolID, targetID, newStatus string) error {
	if newStatus != "active" && newStatus != "deactivated" {
		return fmt.Errorf("%w: %s", ErrInvalidStatusFilter, newStatus)
	}

	student, err := s.storeRepo.GetStudentByID(ctx, targetID, schoolID)
	if err != nil {
		return err
	}
	if student == nil {
		return ErrStudentNotFound
	}
	return s.storeRepo.UpdateStudentStatus(ctx, targetID, schoolID, newStatus)
}

// ReissueStudentCredentials generates a new temp password, overwrites the
// stored hash, and returns the plaintext password exactly once.
// Empty schoolID is id-only (super_admin).
func (s *Service) ReissueStudentCredentials(ctx context.Context, schoolID, targetID string) (*StudentCredentialsResponse, error) {
	student, err := s.storeRepo.GetStudentByID(ctx, targetID, schoolID)
	if err != nil {
		return nil, err
	}
	if student == nil {
		return nil, ErrStudentNotFound
	}

	tempPass, err := genTempPassword()
	if err != nil {
		return nil, fmt.Errorf("generate temp password: %w", err)
	}

	hash, err := hashPassword(tempPass)
	if err != nil {
		return nil, err
	}

	if err := s.storeRepo.ResetStudentPasswordHash(ctx, targetID, schoolID, string(hash)); err != nil {
		return nil, err
	}

	return &StudentCredentialsResponse{
		Username:     *student.Username,
		TempPassword: tempPass,
	}, nil
}
