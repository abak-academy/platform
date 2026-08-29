package service

import (
	"context"
	"errors"
	"testing"

	"akademi-bimbel/internal/repository"

	"golang.org/x/crypto/bcrypt"
)

func TestJenjangInSchoolTypes(t *testing.T) {
	if !jenjangInSchoolTypes("sma", []string{"SMA", "SMK"}) {
		t.Error("want sma to match SMA")
	}
	if jenjangInSchoolTypes("sma", []string{"SMP"}) {
		t.Error("sma must not match SMP")
	}
}

func TestGenTempPassword(t *testing.T) {
	p1, err := genTempPassword()
	if err != nil {
		t.Fatalf("genTempPassword: %v", err)
	}
	if len(p1) != tempPasswordLen {
		t.Errorf("want length %d, got %d", tempPasswordLen, len(p1))
	}

	// Two calls should produce different passwords
	p2, err := genTempPassword()
	if err != nil {
		t.Fatalf("genTempPassword: %v", err)
	}
	if p1 == p2 {
		t.Error("two successive calls returned the same password")
	}
}

func TestGenTempPassword_Characters(t *testing.T) {
	p, err := genTempPassword()
	if err != nil {
		t.Fatalf("genTempPassword: %v", err)
	}
	for _, r := range p {
		found := false
		for _, c := range tempPasswordChars {
			if r == c {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("character %q not in allowed set", r)
		}
	}
}

func TestTempPasswordIsBcryptable(t *testing.T) {
	p, err := genTempPassword()
	if err != nil {
		t.Fatalf("genTempPassword: %v", err)
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(p), bcrypt.MinCost)
	if err != nil {
		t.Fatalf("bcrypt: %v", err)
	}
	if err := bcrypt.CompareHashAndPassword(hash, []byte(p)); err != nil {
		t.Error("bcrypt compare failed for generated temp password")
	}
}

func TestStudentSentinelErrors(t *testing.T) {
	if ErrSchoolDeactivated == nil {
		t.Error("ErrSchoolDeactivated is nil")
	}
	if ErrStudentNotFound == nil {
		t.Error("ErrStudentNotFound is nil")
	}
	if ErrInvalidJenjang == nil {
		t.Error("ErrInvalidJenjang is nil")
	}
	if ErrIncompleteAddress == nil {
		t.Error("ErrIncompleteAddress is nil")
	}
	if ErrInvalidProvinsi == nil {
		t.Error("ErrInvalidProvinsi is nil")
	}
	if ErrInvalidKota == nil {
		t.Error("ErrInvalidKota is nil")
	}
	if ErrInvalidKecamatan == nil {
		t.Error("ErrInvalidKecamatan is nil")
	}
}

// createTestSchool is a small helper shared by the RegisterStudent/ListStudents
// integration tests below — it creates a school via the real Service so tests
// stay end-to-end rather than reaching around the Service into raw SQL.
func createTestSchool(t *testing.T, svc *Service) string {
	t.Helper()
	code := "stu_" + uniqueSuffix()
	resp, err := svc.CreateSchool(context.Background(), "Student Test School "+code, code, nil, nil, nil)
	if err != nil {
		t.Fatalf("CreateSchool: %v", err)
	}
	return resp.ID
}

// seedSchoolWithJenjang creates a school and sets its school_types to the given
// slice, so jenjang validation can be tested.
func seedSchoolWithJenjang(t *testing.T, svc *Service, repo *repository.Repository, jenjangTypes []string) string {
	t.Helper()
	schoolID := createTestSchool(t, svc)
	if len(jenjangTypes) > 0 {
		_, err := repo.Pool().Exec(context.Background(),
			`UPDATE school SET school_types = $1 WHERE id = $2`,
			jenjangTypes, schoolID,
		)
		if err != nil {
			t.Fatalf("update school_types: %v", err)
		}
	}
	return schoolID
}

func TestRegisterStudent_Integration(t *testing.T) {
	svc, repo := newRealDBService(t)
	ctx := context.Background()

	t.Run("happy path: username format, temp password once, bcrypt hash persisted", func(t *testing.T) {
		schoolID := seedSchoolWithJenjang(t, svc, repo, []string{"sma", "smp"})
		jenjang := "sma"
		resp, err := svc.RegisterStudent(ctx, schoolID, "Budi Santoso", "sma", nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)
		if err != nil {
			t.Fatalf("RegisterStudent: %v", err)
		}
		if resp.TempPassword == "" {
			t.Error("want non-empty temp_password")
		}
		if resp.Jenjang != jenjang {
			t.Errorf("Jenjang: want %s, got %s", jenjang, resp.Jenjang)
		}
		if resp.ProvinsiID != nil {
			t.Errorf("ProvinsiID: want nil, got %v", *resp.ProvinsiID)
		}

		u, err := repo.GetUserByUsername(ctx, resp.Username)
		if err != nil {
			t.Fatalf("GetUserByUsername: %v", err)
		}
		if u == nil {
			t.Fatal("student user not persisted")
		}
		if u.Role != RoleStudent || u.Status != "active" || u.OTPEnabled {
			t.Errorf("unexpected defaults: role=%s status=%s otp=%v", u.Role, u.Status, u.OTPEnabled)
		}
		if u.Jenjang == nil || *u.Jenjang != jenjang {
			t.Errorf("persisted Jenjang: want %s, got %v", jenjang, u.Jenjang)
		}
		if u.ProvinsiID != nil {
			t.Errorf("persisted ProvinsiID: want nil, got %v", *u.ProvinsiID)
		}
		if u.PasswordHash == resp.TempPassword {
			t.Error("password hash must not equal the plaintext temp password")
		}
		if err := bcrypt.CompareHashAndPassword([]byte(u.PasswordHash), []byte(resp.TempPassword)); err != nil {
			t.Errorf("persisted hash does not match returned temp password: %v", err)
		}
	})

	t.Run("duplicate email returns ErrEmailTaken", func(t *testing.T) {
		schoolID := seedSchoolWithJenjang(t, svc, repo, []string{"sma"})
		email := "dup-bulk-" + schoolID[:8] + "@example.com"
		if _, err := svc.RegisterStudent(ctx, schoolID, "First Dup", "sma", &email, nil, nil, nil, nil, nil, nil, nil, nil, nil); err != nil {
			t.Fatalf("first RegisterStudent: %v", err)
		}
		_, err := svc.RegisterStudent(ctx, schoolID, "Second Dup", "sma", &email, nil, nil, nil, nil, nil, nil, nil, nil, nil)
		if !errors.Is(err, ErrEmailTaken) {
			t.Errorf("want ErrEmailTaken, got %v", err)
		}
	})

	t.Run("missing name returns ErrMissingField", func(t *testing.T) {
		schoolID := seedSchoolWithJenjang(t, svc, repo, []string{"sma"})
		_, err := svc.RegisterStudent(ctx, schoolID, "", "sma", nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)
		if !errors.Is(err, ErrMissingField) {
			t.Errorf("want ErrMissingField, got %v", err)
		}
	})

	t.Run("missing jenjang returns ErrMissingField", func(t *testing.T) {
		schoolID := seedSchoolWithJenjang(t, svc, repo, []string{"sma"})
		_, err := svc.RegisterStudent(ctx, schoolID, "Some Name", "", nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)
		if !errors.Is(err, ErrMissingField) {
			t.Errorf("want ErrMissingField, got %v", err)
		}
	})

	t.Run("nonexistent school returns ErrSchoolNotFound", func(t *testing.T) {
		_, err := svc.RegisterStudent(ctx, "00000000-0000-0000-0000-000000000000", "Some Name", "sma", nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)
		if !errors.Is(err, ErrSchoolNotFound) {
			t.Errorf("want ErrSchoolNotFound, got %v", err)
		}
	})

	t.Run("jenjang not in school_types returns ErrInvalidJenjang", func(t *testing.T) {
		schoolID := seedSchoolWithJenjang(t, svc, repo, []string{"smp", "sd"})
		_, err := svc.RegisterStudent(ctx, schoolID, "Budi", "sma", nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)
		if !errors.Is(err, ErrInvalidJenjang) {
			t.Errorf("want ErrInvalidJenjang, got %v", err)
		}
	})

	t.Run("jenjang matches school_types case-insensitively", func(t *testing.T) {
		schoolID := seedSchoolWithJenjang(t, svc, repo, []string{"SMA"})
		_, err := svc.RegisterStudent(ctx, schoolID, "Budi Case", "sma", nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)
		if err != nil {
			t.Fatalf("want SMA vs sma to match, got %v", err)
		}
	})

	t.Run("deactivated school blocks registration", func(t *testing.T) {
		schoolID := seedSchoolWithJenjang(t, svc, repo, []string{"sma"})
		if _, err := svc.ChangeSchoolStatus(ctx, schoolID, "deactivated"); err != nil {
			t.Fatalf("ChangeSchoolStatus: %v", err)
		}
		_, err := svc.RegisterStudent(ctx, schoolID, "Blocked Student", "sma", nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)
		if !errors.Is(err, ErrSchoolDeactivated) {
			t.Errorf("want ErrSchoolDeactivated, got %v", err)
		}
	})

	t.Run("incomplete address returns ErrIncompleteAddress", func(t *testing.T) {
		schoolID := seedSchoolWithJenjang(t, svc, repo, []string{"sma"})
		provinsiID := "prov-a"
		// Only provinsiID is set, kotaID and kecamatanID are nil -> incomplete.
		_, err := svc.RegisterStudent(ctx, schoolID, "Budi", "sma", nil, nil, nil, nil, nil, nil, &provinsiID, nil, nil, nil)
		if !errors.Is(err, ErrIncompleteAddress) {
			t.Errorf("want ErrIncompleteAddress, got %v", err)
		}
	})

	t.Run("registration succeeds without address fields", func(t *testing.T) {
		schoolID := seedSchoolWithJenjang(t, svc, repo, []string{"sma"})
		resp, err := svc.RegisterStudent(ctx, schoolID, "Ali", "sma", nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)
		if err != nil {
			t.Fatalf("RegisterStudent without address: %v", err)
		}
		if resp.Username == "" {
			t.Error("want non-empty username")
		}
		if resp.ProvinsiID != nil {
			t.Error("ProvinsiID should be nil when not provided")
		}
	})

	// The frontend's Gender select sends "male"/"female" (see
	// students_field_gender in web/app/(admin)/admin/school/students/page.tsx),
	// but users_gender_check (migration 0002_identity) only allows 'm'/'f' —
	// without normalizeGender, every registration with a gender set at all
	// fails with a raw Postgres check-constraint error.
	t.Run("gender 'male'/'female' from the frontend is normalized to 'm'/'f' before insert", func(t *testing.T) {
		schoolID := seedSchoolWithJenjang(t, svc, repo, []string{"sma"})
		male := "male"
		resp, err := svc.RegisterStudent(ctx, schoolID, "Budi Gender", "sma", nil, nil, &male, nil, nil, nil, nil, nil, nil, nil)
		if err != nil {
			t.Fatalf("RegisterStudent with gender=male: %v", err)
		}
		u, err := repo.GetUserByUsername(ctx, resp.Username)
		if err != nil || u == nil {
			t.Fatalf("GetUserByUsername: %v", err)
		}
		if u.Gender == nil || *u.Gender != "m" {
			t.Errorf("want persisted gender 'm', got %v", u.Gender)
		}

		female := "female"
		resp2, err := svc.RegisterStudent(ctx, schoolID, "Siti Gender", "sma", nil, nil, &female, nil, nil, nil, nil, nil, nil, nil)
		if err != nil {
			t.Fatalf("RegisterStudent with gender=female: %v", err)
		}
		u2, err := repo.GetUserByUsername(ctx, resp2.Username)
		if err != nil || u2 == nil {
			t.Fatalf("GetUserByUsername: %v", err)
		}
		if u2.Gender == nil || *u2.Gender != "f" {
			t.Errorf("want persisted gender 'f', got %v", u2.Gender)
		}
	})

	t.Run("unrecognized gender value returns ErrInvalidGender", func(t *testing.T) {
		schoolID := seedSchoolWithJenjang(t, svc, repo, []string{"sma"})
		bogus := "other"
		_, err := svc.RegisterStudent(ctx, schoolID, "Bogus Gender", "sma", nil, nil, &bogus, nil, nil, nil, nil, nil, nil, nil)
		if !errors.Is(err, ErrInvalidGender) {
			t.Errorf("want ErrInvalidGender, got %v", err)
		}
	})

	t.Run("blank and whitespace emails persist as NULL and do not collide", func(t *testing.T) {
		schoolID := seedSchoolWithJenjang(t, svc, repo, []string{"sma"})
		empty := ""
		ws := "   "
		first, err := svc.RegisterStudent(ctx, schoolID, "Blank Email One", "sma", &empty, nil, nil, nil, nil, nil, nil, nil, nil, nil)
		if err != nil {
			t.Fatalf("first blank email: %v", err)
		}
		if first.Email != nil {
			t.Errorf("first response email: want nil, got %v", *first.Email)
		}
		second, err := svc.RegisterStudent(ctx, schoolID, "Blank Email Two", "sma", &ws, nil, nil, nil, nil, nil, nil, nil, nil, nil)
		if err != nil {
			t.Fatalf("second whitespace email: %v", err)
		}
		if second.Email != nil {
			t.Errorf("second response email: want nil, got %v", *second.Email)
		}

		var email1, email2 *string
		if err := repo.Pool().QueryRow(ctx, `SELECT email FROM users WHERE id = $1`, first.ID).Scan(&email1); err != nil {
			t.Fatalf("read first email: %v", err)
		}
		if err := repo.Pool().QueryRow(ctx, `SELECT email FROM users WHERE id = $1`, second.ID).Scan(&email2); err != nil {
			t.Fatalf("read second email: %v", err)
		}
		if email1 != nil {
			t.Errorf("persisted first email: want NULL, got %q", *email1)
		}
		if email2 != nil {
			t.Errorf("persisted second email: want NULL, got %q", *email2)
		}
	})

	t.Run("duplicate real email returns ErrEmailTaken", func(t *testing.T) {
		schoolID := seedSchoolWithJenjang(t, svc, repo, []string{"sma"})
		email := "dup-" + uniqueSuffix() + "@example.com"
		if _, err := svc.RegisterStudent(ctx, schoolID, "Dup Email One", "sma", &email, nil, nil, nil, nil, nil, nil, nil, nil, nil); err != nil {
			t.Fatalf("first real email: %v", err)
		}
		_, err := svc.RegisterStudent(ctx, schoolID, "Dup Email Two", "sma", &email, nil, nil, nil, nil, nil, nil, nil, nil, nil)
		if !errors.Is(err, ErrEmailTaken) {
			t.Errorf("want ErrEmailTaken, got %v", err)
		}
	})
}

func TestListStudents_ChangeStatus_Reissue_Integration(t *testing.T) {
	svc, repo := newRealDBService(t)
	ctx := context.Background()

	schoolA := seedSchoolWithJenjang(t, svc, repo, []string{"sma"})
	schoolB := seedSchoolWithJenjang(t, svc, repo, []string{"smp"})

	reg, err := svc.RegisterStudent(ctx, schoolA, "Row Scoped Student", "sma", nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)
	if err != nil {
		t.Fatalf("RegisterStudent: %v", err)
	}
	studentID := reg.ID

	t.Run("list is scoped to school", func(t *testing.T) {
		rowsA, _, _, err := svc.ListStudents(ctx, schoolA, "", "", 20, "", nil, "")
		if err != nil {
			t.Fatalf("ListStudents (schoolA): %v", err)
		}
		foundInA := false
		for _, r := range rowsA {
			if r.ID == studentID {
				foundInA = true
			}
		}
		if !foundInA {
			t.Error("student should be listed under its own school")
		}

		rowsB, _, _, err := svc.ListStudents(ctx, schoolB, "", "", 20, "", nil, "")
		if err != nil {
			t.Fatalf("ListStudents (schoolB): %v", err)
		}
		for _, r := range rowsB {
			if r.ID == studentID {
				t.Error("student from schoolA must not be listed under schoolB")
			}
		}
	})

	t.Run("change status is row-scoped: wrong school returns ErrStudentNotFound", func(t *testing.T) {
		err := svc.ChangeStudentStatus(ctx, schoolB, studentID, "deactivated")
		if !errors.Is(err, ErrStudentNotFound) {
			t.Errorf("want ErrStudentNotFound for cross-school access, got %v", err)
		}
	})

	t.Run("change status succeeds for the owning school", func(t *testing.T) {
		if err := svc.ChangeStudentStatus(ctx, schoolA, studentID, "deactivated"); err != nil {
			t.Fatalf("ChangeStudentStatus: %v", err)
		}
		rows, _, _, err := svc.ListStudents(ctx, schoolA, "", "", 20, "", nil, "")
		if err != nil {
			t.Fatalf("ListStudents: %v", err)
		}
		var status string
		for _, r := range rows {
			if r.ID == studentID {
				status = r.Status
			}
		}
		if status != "deactivated" {
			t.Errorf("Status: want deactivated, got %s", status)
		}
		// restore to active for the reissue subtests below
		if err := svc.ChangeStudentStatus(ctx, schoolA, studentID, "active"); err != nil {
			t.Fatalf("ChangeStudentStatus (restore): %v", err)
		}
	})

	t.Run("credential reissue is row-scoped: wrong school returns ErrStudentNotFound", func(t *testing.T) {
		_, err := svc.ReissueStudentCredentials(ctx, schoolB, studentID)
		if !errors.Is(err, ErrStudentNotFound) {
			t.Errorf("want ErrStudentNotFound for cross-school access, got %v", err)
		}
	})

	t.Run("credential reissue overwrites hash and returns a new password", func(t *testing.T) {
		creds, err := svc.ReissueStudentCredentials(ctx, schoolA, studentID)
		if err != nil {
			t.Fatalf("ReissueStudentCredentials: %v", err)
		}
		if creds.TempPassword == "" {
			t.Fatal("want non-empty temp_password")
		}
		if creds.TempPassword == reg.TempPassword {
			t.Error("reissue should return a different password than the original registration")
		}

		u, err := repo.GetUserByUsername(ctx, creds.Username)
		if err != nil {
			t.Fatalf("GetUserByUsername: %v", err)
		}
		if err := bcrypt.CompareHashAndPassword([]byte(u.PasswordHash), []byte(creds.TempPassword)); err != nil {
			t.Errorf("persisted hash does not match reissued temp password: %v", err)
		}
		if bcrypt.CompareHashAndPassword([]byte(u.PasswordHash), []byte(reg.TempPassword)) == nil {
			t.Error("old temp password should no longer validate against the persisted hash")
		}
	})

	t.Run("empty schoolID is id-only (super_admin)", func(t *testing.T) {
		if err := svc.ChangeStudentStatus(ctx, "", studentID, "deactivated"); err != nil {
			t.Fatalf("ChangeStudentStatus empty school: %v", err)
		}
		if err := svc.ChangeStudentStatus(ctx, "", studentID, "active"); err != nil {
			t.Fatalf("ChangeStudentStatus restore: %v", err)
		}
		creds, err := svc.ReissueStudentCredentials(ctx, "", studentID)
		if err != nil {
			t.Fatalf("ReissueStudentCredentials empty school: %v", err)
		}
		if creds.TempPassword == "" {
			t.Fatal("want non-empty temp_password")
		}
	})

	t.Run("empty schoolID reissues a student with no school", func(t *testing.T) {
		noSchoolID := createTestStudentNoSchool(t, svc)
		creds, err := svc.ReissueStudentCredentials(ctx, "", noSchoolID)
		if err != nil {
			t.Fatalf("ReissueStudentCredentials no-school student: %v", err)
		}
		if creds.Username == "" || creds.TempPassword == "" {
			t.Fatalf("want username and temp_password, got %+v", creds)
		}
		if err := svc.ChangeStudentStatus(ctx, "", noSchoolID, "deactivated"); err != nil {
			t.Fatalf("ChangeStudentStatus no-school student: %v", err)
		}
	})
}

func TestUpdateProfile_JenjangAndAddressValidation(t *testing.T) {
	svc, repo := newRealDBService(t)
	ctx := context.Background()

	t.Run("valid jenjang with known school_id succeeds", func(t *testing.T) {
		schoolID := seedSchoolWithJenjang(t, svc, repo, []string{"sma", "smp"})
		userID := createTestStudentWithSchool(t, svc, schoolID, "sma")

		jenjang := "smp"
		updated, err := svc.UpdateProfile(ctx, userID, nil, nil, nil, nil, nil, nil, nil, nil /* dob */, nil, nil, &jenjang, nil, nil, nil, nil)
		if err != nil {
			t.Fatalf("UpdateProfile (valid jenjang): %v", err)
		}
		if updated.Jenjang == nil || *updated.Jenjang != jenjang {
			t.Errorf("Jenjang: want %s, got %v", jenjang, updated.Jenjang)
		}
	})

	t.Run("invalid jenjang with known school_id returns ErrInvalidJenjang", func(t *testing.T) {
		schoolID := seedSchoolWithJenjang(t, svc, repo, []string{"smp", "sd"})
		userID := createTestStudentWithSchool(t, svc, schoolID, "sd")

		jenjang := "sma" // sma is NOT in school_types {smp, sd}
		_, err := svc.UpdateProfile(ctx, userID, nil, nil, nil, nil, nil, nil, nil, nil /* dob */, nil, nil, &jenjang, nil, nil, nil, nil)
		if !errors.Is(err, ErrInvalidJenjang) {
			t.Errorf("want ErrInvalidJenjang, got %v", err)
		}
	})

	t.Run("no school_id known allows any jenjang", func(t *testing.T) {
		// Register a student without a school
		userID := createTestStudentNoSchool(t, svc)

		jenjang := "sma"
		updated, err := svc.UpdateProfile(ctx, userID, nil, nil, nil, nil, nil, nil, nil, nil /* dob */, nil, nil, &jenjang, nil, nil, nil, nil)
		if err != nil {
			t.Fatalf("UpdateProfile (no school): %v", err)
		}
		if updated.Jenjang == nil || *updated.Jenjang != jenjang {
			t.Errorf("Jenjang: want %s, got %v", jenjang, updated.Jenjang)
		}
	})

	t.Run("partial address returns ErrIncompleteAddress", func(t *testing.T) {
		schoolID := seedSchoolWithJenjang(t, svc, repo, []string{"sma"})
		userID := createTestStudentWithSchool(t, svc, schoolID, "sma")

		provinsiID := "11"
		// Only provinsiID set, no kotaID/kecamatanID -> incomplete
		_, err := svc.UpdateProfile(ctx, userID, nil, nil, nil, nil, nil, nil, nil, nil /* dob */, nil, nil, nil, &provinsiID, nil, nil, nil)
		if !errors.Is(err, ErrIncompleteAddress) {
			t.Errorf("want ErrIncompleteAddress, got %v", err)
		}
	})

	t.Run("valid address with all three fields succeeds", func(t *testing.T) {
		schoolID := seedSchoolWithJenjang(t, svc, repo, []string{"sma"})
		userID := createTestStudentWithSchool(t, svc, schoolID, "sma")

		provinsiID := "11"       // ACEH
		kotaID := "1171"         // KOTA BANDA ACEH (provinsi 11)
		kecamatanID := "1171010" // MEURAXA (kota 1171)
		kodePos := "12345"

		updated, err := svc.UpdateProfile(ctx, userID, nil, nil, nil, nil, nil, nil, nil, nil /* dob */, nil, nil, nil, &provinsiID, &kotaID, &kecamatanID, &kodePos)
		if err != nil {
			t.Fatalf("UpdateProfile (valid address): %v", err)
		}
		if updated.ProvinsiID == nil || *updated.ProvinsiID != provinsiID {
			t.Errorf("ProvinsiID: want %s, got %v", provinsiID, updated.ProvinsiID)
		}
		if updated.KotaID == nil || *updated.KotaID != kotaID {
			t.Errorf("KotaID: want %s, got %v", kotaID, updated.KotaID)
		}
		if updated.KecamatanID == nil || *updated.KecamatanID != kecamatanID {
			t.Errorf("KecamatanID: want %s, got %v", kecamatanID, updated.KecamatanID)
		}
		if updated.KodePos == nil || *updated.KodePos != kodePos {
			t.Errorf("KodePos: want %s, got %v", kodePos, updated.KodePos)
		}
	})

	t.Run("invalid provinsi returns ErrInvalidProvinsi", func(t *testing.T) {
		schoolID := seedSchoolWithJenjang(t, svc, repo, []string{"sma"})
		userID := createTestStudentWithSchool(t, svc, schoolID, "sma")

		provinsiID := "999" // does not exist
		kotaID := "1171"
		kecamatanID := "1171010"

		_, err := svc.UpdateProfile(ctx, userID, nil, nil, nil, nil, nil, nil, nil, nil /* dob */, nil, nil, nil, &provinsiID, &kotaID, &kecamatanID, nil)
		if !errors.Is(err, ErrInvalidProvinsi) {
			t.Errorf("want ErrInvalidProvinsi, got %v", err)
		}
	})

	t.Run("mismatched kota/provinsi returns ErrInvalidKota", func(t *testing.T) {
		schoolID := seedSchoolWithJenjang(t, svc, repo, []string{"sma"})
		userID := createTestStudentWithSchool(t, svc, schoolID, "sma")

		provinsiID := "11" // ACEH
		kotaID := "3273"   // KOTA BANDUNG (provinsi 32, not 11)
		kecamatanID := "1171010"

		_, err := svc.UpdateProfile(ctx, userID, nil, nil, nil, nil, nil, nil, nil, nil /* dob */, nil, nil, nil, &provinsiID, &kotaID, &kecamatanID, nil)
		if !errors.Is(err, ErrInvalidKota) {
			t.Errorf("want ErrInvalidKota, got %v", err)
		}
	})

	t.Run("omitting all address fields succeeds", func(t *testing.T) {
		schoolID := seedSchoolWithJenjang(t, svc, repo, []string{"sma"})
		userID := createTestStudentWithSchool(t, svc, schoolID, "sma")

		// All address fields nil (kodePos also nil)
		updated, err := svc.UpdateProfile(ctx, userID, nil, nil, nil, nil, nil, nil, nil, nil /* dob */, nil, nil, nil, nil, nil, nil, nil)
		if err != nil {
			t.Fatalf("UpdateProfile (no address fields): %v", err)
		}
		if updated.ProvinsiID != nil {
			t.Error("ProvinsiID should be nil when not provided")
		}
	})
}

// TestUpdateProfile_SchoolPairClearing covers FB-14 / FR-9..FR-13: the
// (school_id, unlisted_school_name) pair must be clearable, and switching
// between a listed and unlisted school must resolve jenjang against the
// right school.
func TestUpdateProfile_SchoolPairClearing(t *testing.T) {
	svc, repo := newRealDBService(t)
	ctx := context.Background()

	t.Run("empty school_id is not ErrInvalidUUID (FR-9)", func(t *testing.T) {
		userID := createTestStudentNoSchool(t, svc)
		empty := ""
		_, err := svc.UpdateProfile(ctx, userID,
			nil, nil, nil, nil, nil, nil, nil, // name, email, username, phone, address, targetExam, grade
			nil,      // dob
			&empty,   // schoolID
			nil, nil, // unlistedSchoolName, jenjang
			nil, nil, nil, nil, // provinsiID, kotaID, kecamatanID, kodePos
		)
		if errors.Is(err, ErrInvalidUUID) {
			t.Errorf("empty school_id: want no ErrInvalidUUID, got %v", err)
		} else if err != nil {
			t.Fatalf("UpdateProfile (empty school_id): %v", err)
		}
	})

	t.Run("non-uuid school_id still returns ErrInvalidUUID (FR-9)", func(t *testing.T) {
		userID := createTestStudentNoSchool(t, svc)
		bad := "abc"
		_, err := svc.UpdateProfile(ctx, userID,
			nil, nil, nil, nil, nil, nil, nil,
			nil, // dob
			&bad,
			nil, nil,
			nil, nil, nil, nil,
		)
		if !errors.Is(err, ErrInvalidUUID) {
			t.Errorf("want ErrInvalidUUID, got %v", err)
		}
	})

	t.Run("listed SMA school switching to unlisted school with jenjang SMK succeeds (FR-13)", func(t *testing.T) {
		schoolID := seedSchoolWithJenjang(t, svc, repo, []string{"sma"})
		userID := createTestStudentWithSchool(t, svc, schoolID, "sma")

		emptySchoolID := ""
		unlistedName := "My Unlisted School " + uniqueSuffix()
		newJenjang := "smk"
		updated, err := svc.UpdateProfile(ctx, userID,
			nil, nil, nil, nil, nil, nil, nil,
			nil, // dob
			&emptySchoolID,
			&unlistedName, &newJenjang,
			nil, nil, nil, nil,
		)
		if err != nil {
			t.Fatalf("UpdateProfile (listed -> unlisted, FR-13): %v", err)
		}
		if updated.SchoolID != nil {
			t.Errorf("SchoolID: want nil, got %v", *updated.SchoolID)
		}
		if updated.UnlistedSchoolName == nil || *updated.UnlistedSchoolName != unlistedName {
			t.Errorf("UnlistedSchoolName: want %q, got %v", unlistedName, updated.UnlistedSchoolName)
		}
		if updated.Jenjang == nil || *updated.Jenjang != newJenjang {
			t.Errorf("Jenjang: want %s, got %v", newJenjang, updated.Jenjang)
		}

		// DB-backed assertion — this is the one a service-level mock cannot
		// catch, because a mock repo doesn't reproduce COALESCE($8, school_id).
		var dbSchoolID, dbUnlistedName *string
		if err := repo.Pool().QueryRow(ctx,
			`SELECT school_id, unlisted_school_name FROM users WHERE id = $1`, userID,
		).Scan(&dbSchoolID, &dbUnlistedName); err != nil {
			t.Fatalf("query users row: %v", err)
		}
		if dbSchoolID != nil {
			t.Errorf("db school_id: want NULL, got %v", *dbSchoolID)
		}
		if dbUnlistedName == nil || *dbUnlistedName != unlistedName {
			t.Errorf("db unlisted_school_name: want %q, got %v", unlistedName, dbUnlistedName)
		}

		// Switch back to a listed school: unlisted_school_name must clear.
		newSchoolID := seedSchoolWithJenjang(t, svc, repo, []string{"smk"})
		updated2, err := svc.UpdateProfile(ctx, userID,
			nil, nil, nil, nil, nil, nil, nil,
			nil, // dob
			&newSchoolID,
			nil, nil,
			nil, nil, nil, nil,
		)
		if err != nil {
			t.Fatalf("UpdateProfile (unlisted -> listed): %v", err)
		}
		if updated2.SchoolID == nil || *updated2.SchoolID != newSchoolID {
			t.Errorf("SchoolID: want %s, got %v", newSchoolID, updated2.SchoolID)
		}
		if updated2.UnlistedSchoolName != nil {
			t.Errorf("UnlistedSchoolName: want nil, got %v", *updated2.UnlistedSchoolName)
		}

		var dbUnlistedName2 *string
		if err := repo.Pool().QueryRow(ctx,
			`SELECT unlisted_school_name FROM users WHERE id = $1`, userID,
		).Scan(&dbUnlistedName2); err != nil {
			t.Fatalf("query users row: %v", err)
		}
		if dbUnlistedName2 != nil {
			t.Errorf("db unlisted_school_name: want NULL, got %v", *dbUnlistedName2)
		}
	})

	t.Run("neither field mentioned leaves school pair untouched (FR-12)", func(t *testing.T) {
		schoolID := seedSchoolWithJenjang(t, svc, repo, []string{"sma"})
		userID := createTestStudentWithSchool(t, svc, schoolID, "sma")

		newName := "Renamed Student " + uniqueSuffix()
		updated, err := svc.UpdateProfile(ctx, userID,
			&newName, nil, nil, nil, nil, nil, nil,
			nil, // dob
			nil,
			nil, nil,
			nil, nil, nil, nil,
		)
		if err != nil {
			t.Fatalf("UpdateProfile (name only, FR-12): %v", err)
		}
		if updated.Name != newName {
			t.Errorf("Name: want %s, got %s", newName, updated.Name)
		}
		if updated.SchoolID == nil || *updated.SchoolID != schoolID {
			t.Errorf("SchoolID: want unchanged %s, got %v", schoolID, updated.SchoolID)
		}
		if updated.UnlistedSchoolName != nil {
			t.Errorf("UnlistedSchoolName: want unchanged nil, got %v", *updated.UnlistedSchoolName)
		}
	})
}

// createTestStudentWithSchool registers a student under a given school via the
// backend so they have a real school_id in the profile, ready for UpdateProfile.
func createTestStudentWithSchool(t *testing.T, svc *Service, schoolID, jenjang string) string {
	t.Helper()
	resp, err := svc.RegisterStudent(ctxBg, schoolID, "Test Student "+uniqueSuffix(), jenjang, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)
	if err != nil {
		t.Fatalf("RegisterStudent: %v", err)
	}
	return resp.ID
}

// createTestStudentNoSchool inserts a student directly into the DB with no
// school_id, no OTP flow (status=active), so the profile has no school.
func createTestStudentNoSchool(t *testing.T, svc *Service) string {
	t.Helper()
	name := "No School Student " + uniqueSuffix()
	username := "ns_" + uniqueSuffix()
	var userID string
	err := svc.storeRepo.Pool().QueryRow(context.Background(),
		`INSERT INTO users (name, username, jenjang, role, status, auth_provider)
		VALUES ($1, $2, 'sd', 'student', 'active', 'password')
		RETURNING id`,
		name, username,
	).Scan(&userID)
	if err != nil {
		t.Fatalf("insert user without school: %v", err)
	}
	return userID
}

var ctxBg = context.Background()
