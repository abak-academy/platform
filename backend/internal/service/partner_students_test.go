package service

import (
	"context"
	"testing"

	"akademi-bimbel/internal/model"
	"akademi-bimbel/internal/repository"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

// seedStudent inserts a student row directly. registeredBy and schoolID may be empty.
func seedStudent(t *testing.T, repo *repository.Repository, name, schoolID, registeredBy string, temporary bool) string {
	t.Helper()
	ctx := context.Background()
	suffix := uniqueSuffix()
	var school, registrar any
	if schoolID != "" {
		school = schoolID
	}
	if registeredBy != "" {
		registrar = registeredBy
	}
	var id string
	require.NoError(t, repo.Pool().QueryRow(ctx,
		`INSERT INTO users (email, username, role, name, status, school_id, registered_by, password_temporary, jenjang)
		 VALUES ($1, $2, 'student', $3, 'active', $4, $5, $6, 'sma') RETURNING id`,
		"stu-"+suffix+"@test.local", "stu"+suffix, name, school, registrar, temporary,
	).Scan(&id))
	t.Cleanup(func() { _, _ = repo.Pool().Exec(ctx, `DELETE FROM users WHERE id = $1`, id) })
	return id
}

func namesOf(page *PartnerStudentsPage) []string {
	out := make([]string, 0, len(page.Data))
	for _, s := range page.Data {
		out = append(out, s.Name)
	}
	return out
}

func TestPartnerRegisterStudent_recordsTheMitraAndGivesATemporaryPassword(t *testing.T) {
	svc, repo := newRealDBService(t)
	ctx := context.Background()
	partner := seedProgramAccount(t, repo, "active", "active", "", partnerOnlyPrograms)
	schoolID := createTestSchool(t, svc)

	for name, school := range map[string]string{"with a school": schoolID, "without a school": ""} {
		t.Run(name, func(t *testing.T) {
			resp, err := svc.PartnerRegisterStudent(ctx, partner.String(), PartnerRegisterStudentInput{SchoolID: school, Name: "Siswa " + uniqueSuffix(), Jenjang: "sma"})
			require.NoError(t, err)
			require.NotEmpty(t, resp.TempPassword, "the temporary password comes back once")
			t.Cleanup(func() { _, _ = repo.Pool().Exec(ctx, `DELETE FROM users WHERE id = $1`, resp.ID) })

			var registeredBy *string
			var temporary bool
			var storedSchool *string
			require.NoError(t, repo.Pool().QueryRow(ctx,
				`SELECT registered_by::text, password_temporary, school_id::text FROM users WHERE id = $1`, resp.ID,
			).Scan(&registeredBy, &temporary, &storedSchool))
			require.NotNil(t, registeredBy)
			require.Equal(t, partner.String(), *registeredBy)
			require.True(t, temporary, "the student has to choose their own password")
			if school == "" {
				require.Nil(t, storedSchool)
			} else {
				require.Equal(t, school, *storedSchool)
			}
		})
	}

	t.Run("an unknown school is refused", func(t *testing.T) {
		_, err := svc.PartnerRegisterStudent(ctx, partner.String(), PartnerRegisterStudentInput{SchoolID: uuid.NewString(), Name: "Siswa", Jenjang: "sma"})
		require.Error(t, err)
	})
}

func TestListPartnerStudents_isTheAllowedSchoolsPlusWhatTheMitraRegistered(t *testing.T) {
	svc, repo := newRealDBService(t)
	ctx := context.Background()
	partner := seedProgramAccount(t, repo, "active", "active", "", partnerOnlyPrograms).String()
	other := seedProgramAccount(t, repo, "active", "active", "", partnerOnlyPrograms).String()
	schoolA := createTestSchool(t, svc)
	schoolB := createTestSchool(t, svc)

	tag := uniqueSuffix()
	inAllowed := seedStudent(t, repo, "Ani "+tag, schoolA, "", false)
	registeredElsewhere := seedStudent(t, repo, "Budi "+tag, schoolB, partner, true)
	registeredNoSchool := seedStudent(t, repo, "Cici "+tag, "", partner, true)
	otherMitraInAllowed := seedStudent(t, repo, "Dedi "+tag, schoolA, other, false)
	outside := seedStudent(t, repo, "Eko "+tag, schoolB, "", false)
	otherNoSchool := seedStudent(t, repo, "Fani "+tag, "", other, false)
	_ = []string{inAllowed, registeredElsewhere, registeredNoSchool, otherMitraInAllowed, outside, otherNoSchool}

	list := func(allowed []string, f PartnerStudentListFilter) *PartnerStudentsPage {
		f.Q = tag + f.Q
		page, err := svc.ListPartnerStudents(ctx, partner, allowed, f)
		require.NoError(t, err)
		return page
	}

	page := list([]string{schoolA}, PartnerStudentListFilter{})
	require.ElementsMatch(t, []string{"Ani " + tag, "Budi " + tag, "Cici " + tag, "Dedi " + tag}, namesOf(page),
		"the allowed school and the registered students, and nobody else's")
	for _, s := range page.Data {
		switch s.Name {
		case "Budi " + tag, "Cici " + tag:
			require.True(t, s.RegisteredByMe)
			require.False(t, s.Activated, "still on the temporary password")
		case "Ani " + tag, "Dedi " + tag:
			require.False(t, s.RegisteredByMe)
			require.True(t, s.Activated)
		}
	}

	t.Run("registered_by_me narrows to the Mitra's own", func(t *testing.T) {
		page := list([]string{schoolA}, PartnerStudentListFilter{RegisteredByMe: true})
		require.ElementsMatch(t, []string{"Budi " + tag, "Cici " + tag}, namesOf(page))
	})

	t.Run("a school filter narrows within the scope", func(t *testing.T) {
		page := list([]string{schoolA}, PartnerStudentListFilter{SchoolID: schoolA})
		require.ElementsMatch(t, []string{"Ani " + tag, "Dedi " + tag}, namesOf(page))
		page = list([]string{schoolA}, PartnerStudentListFilter{SchoolID: schoolB})
		require.ElementsMatch(t, []string{"Budi " + tag}, namesOf(page), "a school outside the allowed set still shows only what the Mitra registered there")
	})

	t.Run("without schools only the registered students remain", func(t *testing.T) {
		page := list(nil, PartnerStudentListFilter{})
		require.ElementsMatch(t, []string{"Budi " + tag, "Cici " + tag}, namesOf(page))
	})

	t.Run("a Mitra with nothing to show is told why", func(t *testing.T) {
		fresh := seedProgramAccount(t, repo, "active", "active", "", partnerOnlyPrograms).String()
		page, err := svc.ListPartnerStudents(ctx, fresh, nil, PartnerStudentListFilter{})
		require.NoError(t, err)
		require.Empty(t, page.Data)
		require.Equal(t, "no_schools", page.Hint)

		page, err = svc.ListPartnerStudents(ctx, fresh, []string{schoolA}, PartnerStudentListFilter{Q: "tidak-ada-" + tag})
		require.NoError(t, err)
		require.Empty(t, page.Hint, "an empty search is not a missing school assignment")
	})

	t.Run("paging walks the whole list once", func(t *testing.T) {
		var got []string
		cursor := ""
		for i := 0; i < 5; i++ {
			page, err := svc.ListPartnerStudents(ctx, partner, []string{schoolA}, PartnerStudentListFilter{Q: tag, Limit: 2, Cursor: cursor})
			require.NoError(t, err)
			got = append(got, namesOf(page)...)
			if page.NextCursor == "" {
				break
			}
			cursor = page.NextCursor
		}
		require.ElementsMatch(t, []string{"Ani " + tag, "Budi " + tag, "Cici " + tag, "Dedi " + tag}, got)
	})
}

func TestPartnerReissueStudentCredentials(t *testing.T) {
	svc, repo := newRealDBService(t)
	ctx := context.Background()
	partner := seedProgramAccount(t, repo, "active", "active", "", partnerOnlyPrograms).String()
	other := seedProgramAccount(t, repo, "active", "active", "", partnerOnlyPrograms).String()

	hashOf := func(id string) string {
		var hash string
		require.NoError(t, repo.Pool().QueryRow(ctx, `SELECT password_hash FROM users WHERE id = $1`, id).Scan(&hash))
		return hash
	}

	t.Run("a student who has not chosen a password gets a new temporary one", func(t *testing.T) {
		id := seedStudent(t, repo, "Gita "+uniqueSuffix(), "", partner, true)
		before := hashOf(id)
		resp, err := svc.PartnerReissueStudentCredentials(ctx, partner, id)
		require.NoError(t, err)
		require.NotEmpty(t, resp.Username)
		require.NotEmpty(t, resp.TempPassword)
		require.NotEqual(t, before, hashOf(id))
		require.True(t, comparePassword(hashOf(id), resp.TempPassword, ""))

		var audited int
		require.NoError(t, repo.Pool().QueryRow(ctx,
			`SELECT count(*) FROM audit_log WHERE target_id = $1 AND action = 'student.reissue_credentials'`, id).Scan(&audited))
		require.Equal(t, 1, audited)
	})

	t.Run("once the student has their own password the Mitra cannot touch the account", func(t *testing.T) {
		id := seedStudent(t, repo, "Hadi "+uniqueSuffix(), "", partner, false)
		before := hashOf(id)
		_, err := svc.PartnerReissueStudentCredentials(ctx, partner, id)
		require.ErrorIs(t, err, ErrStudentAlreadyActivated)
		require.Equal(t, before, hashOf(id), "the password must be left alone")
	})

	t.Run("another Mitra's student and an unregistered student are not found", func(t *testing.T) {
		theirs := seedStudent(t, repo, "Indah "+uniqueSuffix(), "", other, true)
		unregistered := seedStudent(t, repo, "Joko "+uniqueSuffix(), "", "", true)
		for _, id := range []string{theirs, unregistered, uuid.NewString()} {
			_, err := svc.PartnerReissueStudentCredentials(ctx, partner, id)
			require.ErrorIs(t, err, ErrStudentNotFound, id)
		}
	})

	t.Run("a student who sets a password at the same moment is not overtaken", func(t *testing.T) {
		id := seedStudent(t, repo, "Kiki "+uniqueSuffix(), "", partner, true)
		_, err := repo.Pool().Exec(ctx, `UPDATE users SET password_temporary = false WHERE id = $1`, id)
		require.NoError(t, err)
		before := hashOf(id)

		changed, err := repo.ReissueStudentCredentialsByRegistrar(ctx, id, partner, "new-hash")
		require.NoError(t, err)
		require.False(t, changed, "the check and the write are one statement")
		require.Equal(t, before, hashOf(id))
	})
}

func TestPartnerJobStatus_isOnlyForTheMitraThatCreatedTheJob(t *testing.T) {
	svc, repo := newRealDBService(t)
	ctx := context.Background()
	owner := seedProgramAccount(t, repo, "active", "active", "", partnerOnlyPrograms).String()
	other := seedProgramAccount(t, repo, "active", "active", "", partnerOnlyPrograms).String()

	key := "student-bulk/" + owner + "/x.csv"
	job := &model.Job{Type: "student_bulk", InputURL: &key, CreatedBy: owner}
	require.NoError(t, repo.CreateJob(ctx, job))
	t.Cleanup(func() { _, _ = repo.Pool().Exec(ctx, `DELETE FROM job WHERE id = $1`, job.ID) })

	got, err := svc.PartnerJobStatus(ctx, owner, job.ID)
	require.NoError(t, err)
	require.Equal(t, job.ID, got.ID)

	_, err = svc.PartnerJobStatus(ctx, other, job.ID)
	require.ErrorIs(t, err, ErrJobNotFound, "another Mitra must not read the report with temporary passwords")
}

func TestProcessStudentBulkRows_forAMitra(t *testing.T) {
	svc, repo := newRealDBService(t)
	ctx := context.Background()
	partner := seedProgramAccount(t, repo, "active", "active", "", partnerOnlyPrograms).String()
	schoolID := createTestSchool(t, svc)
	var schoolCode string
	require.NoError(t, repo.Pool().QueryRow(ctx, `SELECT code FROM school WHERE id = $1`, schoolID).Scan(&schoolCode))

	tag := uniqueSuffix()
	password := "rahasia123"
	rows := []StudentBulkRow{
		{Row: 1, Name: "Lina " + tag, Jenjang: "sma", SchoolCode: schoolCode},
		{Row: 2, Name: "Maman " + tag, Jenjang: "sma"},
		{Row: 3, Name: "Nina " + tag, Jenjang: "sma", Password: &password},
		{Row: 4, Name: "Oki " + tag, Jenjang: "sma", SchoolCode: "tidak-ada-" + tag},
	}
	results, ok, err := svc.ProcessStudentBulkRows(ctx, partner, nil, RoleAffiliator, rows, nil)
	require.NoError(t, err)
	require.Equal(t, 2, ok)
	t.Cleanup(func() {
		_, _ = repo.Pool().Exec(ctx, `DELETE FROM users WHERE registered_by = $1`, partner)
	})

	require.Equal(t, "success", results[0].Status, "a row with a school")
	require.Equal(t, "success", results[1].Status, "a row without any school")
	require.Equal(t, "failed", results[2].Status, "a Mitra cannot choose a password")
	require.Equal(t, "failed", results[3].Status, "a school that does not exist is still an error")
	for _, i := range []int{0, 1} {
		require.NotEmpty(t, results[i].TempPassword, "the report carries the temporary password")
	}

	var registered int
	require.NoError(t, repo.Pool().QueryRow(ctx,
		`SELECT count(*) FROM users WHERE registered_by = $1 AND password_temporary AND name LIKE $2`, partner, "% "+tag).Scan(&registered))
	require.Equal(t, 2, registered, "every created student is recorded as registered by the Mitra")
	var withSchool int
	require.NoError(t, repo.Pool().QueryRow(ctx,
		`SELECT count(*) FROM users WHERE registered_by = $1 AND school_id = $2`, partner, schoolID).Scan(&withSchool))
	require.Equal(t, 1, withSchool)
}

func TestParseStudentBulkCSVForWorker_aMitraCsvNeedsNoSchoolColumn(t *testing.T) {
	csv := []byte("name,jenjang\nLina,sma\nMaman,smp\n")

	rows, err := ParseStudentBulkCSVForWorker(csv, false)
	require.NoError(t, err)
	require.Len(t, rows, 2)

	_, err = ParseStudentBulkCSVForWorker(csv, true)
	require.Error(t, err, "the other roles still have to say which school each student belongs to")
}
