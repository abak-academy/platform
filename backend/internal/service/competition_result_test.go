package service

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"akademi-bimbel/config"
	"akademi-bimbel/internal/model"
	"akademi-bimbel/internal/repository"

	"github.com/google/uuid"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeObjectStore serves GET /private/<key> from memory so the real minio
// client path (GetObject + LimitReader) runs in tests.
type fakeObjectStore struct {
	mu   sync.Mutex
	objs map[string][]byte
}

func newCompetitionSvc(t *testing.T) (*Service, *repository.Repository, *fakeObjectStore) {
	t.Helper()
	_, repo := newRealDBService(t)
	fake := &fakeObjectStore{objs: map[string][]byte{}}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fake.mu.Lock()
		defer fake.mu.Unlock()
		body, ok := fake.objs[strings.TrimPrefix(r.URL.Path, "/private/")]
		if !ok {
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(`<?xml version="1.0"?><Error><Code>NoSuchKey</Code><Message>missing</Message></Error>`))
			return
		}
		w.Header().Set("Last-Modified", time.Now().UTC().Format(http.TimeFormat))
		w.Header().Set("ETag", `"fake"`)
		_, _ = w.Write(body)
	}))
	t.Cleanup(srv.Close)
	client, err := minio.New(strings.TrimPrefix(srv.URL, "http://"), &minio.Options{
		Creds: credentials.NewStaticV4("ak", "sk", ""), Region: "us-east-1",
	})
	require.NoError(t, err)
	svc := NewWithStore(repo, repo, nil, nil, &NoopOTPProvider{}, &NoopEmailProvider{}, nil, nil, client,
		&config.Config{ObjectStoragePrivateBucketName: "private", ObjectStorageRegion: "us-east-1"}, nil)
	return svc, repo, fake
}

func (f *fakeObjectStore) put(examID uuid.UUID, name, csv string) string {
	key := competitionKeyPrefix(examID) + name
	f.mu.Lock()
	f.objs[key] = []byte(csv)
	f.mu.Unlock()
	return key
}

type competitionFixture struct {
	examID   uuid.UUID
	examNo   int
	regs     map[int]uuid.UUID
	students map[int]uuid.UUID
	revoked  int
	actor    string
}

func seedCompetitionExam(t *testing.T, repo *repository.Repository, productID *uuid.UUID) (examID uuid.UUID, examNo int) {
	t.Helper()
	ctx := context.Background()
	require.NoError(t, repo.Pool().QueryRow(ctx,
		`INSERT INTO exam (title, status, scheduled_at) VALUES ($1, 'draft', $2) RETURNING id, exam_number`,
		"Competition Exam "+uniqueSuffix(), time.Date(2025, 6, 20, 9, 0, 0, 0, time.UTC)).Scan(&examID, &examNo))
	if productID != nil {
		_, err := repo.Pool().Exec(ctx, `INSERT INTO product_exam (product_id, exam_id) VALUES ($1, $2)`, *productID, examID)
		require.NoError(t, err)
	}
	return examID, examNo
}

func seedCompetitionProduct(t *testing.T, repo *repository.Repository) uuid.UUID {
	t.Helper()
	var id uuid.UUID
	require.NoError(t, repo.Pool().QueryRow(context.Background(),
		`INSERT INTO product (type, name, price, status) VALUES ('exam', $1, 0, 'published') RETURNING id`,
		"Edition "+uniqueSuffix()).Scan(&id))
	return id
}

// participants 1..3 are live, 4 is revoked.
func seedCompetitionFixture(t *testing.T, repo *repository.Repository) competitionFixture {
	t.Helper()
	ctx := context.Background()
	f := competitionFixture{regs: map[int]uuid.UUID{}, students: map[int]uuid.UUID{}, revoked: 4, actor: seedStudentDirect(t, ctx, repo, "Admin Uji", "adm-"+uniqueSuffix()).String()}
	f.examID, f.examNo = seedCompetitionExam(t, repo, nil)
	for n := 1; n <= 4; n++ {
		status := "registered"
		if n == f.revoked {
			status = "revoked"
		}
		f.students[n] = seedStudentDirect(t, ctx, repo, fmt.Sprintf("Peserta %d", n), fmt.Sprintf("p%d-%s", n, uniqueSuffix()))
		var regID uuid.UUID
		require.NoError(t, repo.Pool().QueryRow(ctx,
			`INSERT INTO exam_registration (student_id, exam_id, token, status, participant_number) VALUES ($1, $2, $3, $4, $5) RETURNING id`,
			f.students[n], f.examID, "TOK"+uniqueSuffix(), status, n).Scan(&regID))
		f.regs[n] = regID
	}
	return f
}

func (f competitionFixture) no(prefix string, n int) string {
	return formatParticipantNo(prefix, time.Date(2025, 6, 20, 0, 0, 0, 0, time.UTC), f.examNo, n)
}

const compHeader = "participant_no,score,rank,award,certificate_url\n"

func TestCompetitionResult_MatchesWithoutPrefixWithPrefixAndOldPrefix(t *testing.T) {
	svc, repo, store := newCompetitionSvc(t)
	ctx := context.Background()
	f := seedCompetitionFixture(t, repo)
	oldPfx, newPfx := "OLDPX", "NEWPX"
	gid := seedGrouping(t, repo, &oldPfx)
	productID := seedCompetitionProduct(t, repo)
	_, err := repo.Pool().Exec(ctx, `INSERT INTO product_exam (product_id, exam_id) VALUES ($1, $2)`, productID, f.examID)
	require.NoError(t, err)
	_, err = repo.Pool().Exec(ctx, `UPDATE product SET grouping_id = $1 WHERE id = $2`, gid, productID)
	require.NoError(t, err)
	_, err = repo.Pool().Exec(ctx, `UPDATE product_grouping SET participant_prefix = $1 WHERE id = $2`, newPfx, gid)
	require.NoError(t, err)

	key := store.put(f.examID, "a.csv", compHeader+
		f.no("", 1)+",90,1,Medali emas,https://certs.example.test/1\n"+
		f.no(newPfx, 2)+",80.5,,,\n"+
		f.no(oldPfx, 3)+",70,3,,\n")

	preview, err := svc.PreviewCompetitionResult(ctx, f.examID, key)
	require.NoError(t, err)
	assert.Equal(t, 3, preview.Matched)
	assert.Empty(t, preview.Errors)
	assert.False(t, preview.ResultExists)

	_, result, err := svc.ApplyCompetitionResult(ctx, f.examID, f.actor, key)
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, 3, result.RowCount)

	rows, err := svc.ListCompetitionResultRows(ctx, f.examID)
	require.NoError(t, err)
	require.Len(t, rows, 3)
	assert.Equal(t, f.no(newPfx, 1), rows[0].ParticipantNo, "shown in the CURRENT prefix, not the uploaded one")
	assert.Equal(t, "Peserta 1", rows[0].StudentName)
	assert.Equal(t, 90.0, rows[0].Score)
}

func TestCompetitionResult_RejectsParticipantNumberOfAnotherExam(t *testing.T) {
	svc, repo, store := newCompetitionSvc(t)
	ctx := context.Background()
	f := seedCompetitionFixture(t, repo)
	otherExam, otherNo := seedCompetitionExam(t, repo, nil)
	_, err := repo.Pool().Exec(ctx,
		`INSERT INTO exam_registration (student_id, exam_id, token, status, participant_number) VALUES ($1, $2, $3, 'registered', 1)`,
		f.students[1], otherExam, "TOK"+uniqueSuffix())
	require.NoError(t, err)

	at := time.Date(2025, 6, 20, 0, 0, 0, 0, time.UTC)
	key := store.put(f.examID, "m.csv", compHeader+f.no("", 1)+",1,,,\n"+formatParticipantNo("", at, otherNo, 1)+",2,,,\n")
	preview, err := svc.PreviewCompetitionResult(ctx, f.examID, key)
	require.NoError(t, err)
	assert.Equal(t, 1, preview.Matched)
	assert.Equal(t, []model.CompetitionRowError{
		{Line: 3, ParticipantNo: formatParticipantNo("", at, otherNo, 1), Reason: ReasonOtherExamParticipantNo},
	}, preview.Errors)
}

func TestCompetitionResult_ExamOnTwoProductsHasOneResultVisibleOnce(t *testing.T) {
	svc, repo, store := newCompetitionSvc(t)
	ctx := context.Background()
	f := seedCompetitionFixture(t, repo)
	for i := 0; i < 2; i++ {
		_, err := repo.Pool().Exec(ctx, `INSERT INTO product_exam (product_id, exam_id) VALUES ($1, $2)`, seedCompetitionProduct(t, repo), f.examID)
		require.NoError(t, err)
	}
	key := store.put(f.examID, "a.csv", compHeader+f.no("", 1)+",90,1,,\n")
	_, _, err := svc.ApplyCompetitionResult(ctx, f.examID, f.actor, key)
	require.NoError(t, err)
	_, err = svc.SetCompetitionResultPublished(ctx, f.examID, true, f.actor)
	require.NoError(t, err)

	var results int
	require.NoError(t, repo.Pool().QueryRow(ctx, `SELECT count(*) FROM competition_result WHERE exam_id = $1`, f.examID).Scan(&results))
	assert.Equal(t, 1, results)
	items, err := svc.GetExamRegistrations(ctx, f.students[1].String())
	require.NoError(t, err)
	require.Len(t, items, 1, "product attachments do not multiply registrations")
	require.NotNil(t, items[0].CompetitionResult)
	assert.Equal(t, 90.0, items[0].CompetitionResult.Score)
}

func TestCompetitionResult_PreviewReportsEveryErrorKindWithItsLine(t *testing.T) {
	svc, repo, store := newCompetitionSvc(t)
	ctx := context.Background()
	f := seedCompetitionFixture(t, repo)
	longAward := strings.Repeat("x", 61)
	otherExam := formatParticipantNo("", time.Now(), f.examNo+1000000, 1)

	key := store.put(f.examID, "bad.csv", compHeader+
		f.no("", 1)+",90,1,ok,https://ok.example.test/c\n"+ // line 2 valid
		f.no("", 99)+",50,,,\n"+ // 3 unknown
		f.no("", f.revoked)+",50,,,\n"+ // 4 revoked
		"garbage,50,,,\n"+ // 5 unreadable
		otherExam+",50,,,\n"+ // 6 other exam
		f.no("", 1)+",60,,,\n"+ // 7 duplicate of line 2
		f.no("", 2)+",abc,,,\n"+ // 8 bad score
		f.no("", 3)+",50,0,,\n"+ // 9 rank < 1
		f.no("", 3)+",50,1.5,,\n"+ // 10 duplicate (3 seen on line 9)
		"\n"+ // blank line skipped
		f.no("", 2)+",50,x,,\n"+ // 12 bad rank (2 seen on line 8)
		f.no("OTHR", 2)+",50,,,\n"+ // 13 duplicate through a different prefix
		f.no("", 2)+",50\n") // 14 bad row

	preview, err := svc.PreviewCompetitionResult(ctx, f.examID, key)
	require.NoError(t, err)
	got := map[int]string{}
	for _, e := range preview.Errors {
		got[e.Line] = e.Reason
	}
	assert.Equal(t, map[int]string{
		3: ReasonUnknownParticipant, 4: ReasonRevokedParticipant, 5: ReasonUnreadableParticipantNo, 6: ReasonOtherExamParticipantNo,
		7: ReasonDuplicateParticipant, 8: ReasonBadScore, 9: ReasonBadRank, 10: ReasonDuplicateParticipant,
		12: ReasonDuplicateParticipant, 13: ReasonDuplicateParticipant, 14: ReasonBadRow,
	}, got)
	assert.Equal(t, 1, preview.Matched)

	key2 := store.put(f.examID, "bad2.csv", compHeader+
		f.no("", 1)+",1,0,,\n"+
		f.no("", 2)+",1,,"+longAward+",\n"+
		f.no("", 3)+",1,,,http://insecure.example.test/c\n")
	preview, err = svc.PreviewCompetitionResult(ctx, f.examID, key2)
	require.NoError(t, err)
	assert.Equal(t, []model.CompetitionRowError{
		{Line: 2, ParticipantNo: f.no("", 1), Reason: ReasonBadRank},
		{Line: 3, ParticipantNo: f.no("", 2), Reason: ReasonAwardTooLong},
		{Line: 4, ParticipantNo: f.no("", 3), Reason: ReasonBadCertificateURL},
	}, preview.Errors)

	key3 := store.put(f.examID, "bad3.csv", compHeader+f.no("", 1)+",1,2147483648,,\n")
	preview, err = svc.PreviewCompetitionResult(ctx, f.examID, key3)
	require.NoError(t, err)
	assert.Equal(t, []model.CompetitionRowError{{Line: 2, ParticipantNo: f.no("", 1), Reason: ReasonBadRank}}, preview.Errors)
}

func TestCompetitionResult_FileLevelErrors(t *testing.T) {
	svc, repo, store := newCompetitionSvc(t)
	f := seedCompetitionFixture(t, repo)
	row := f.no("", 1) + ",5\n"
	for name, csv := range map[string]string{
		"missing score":  "participant_no\n" + f.no("", 1) + "\n",
		"missing number": "score\n5\n",
		"extra column":   "participant_no,score,notes\n" + f.no("", 1) + ",5,x\n",
		"duplicate col":  "participant_no,score,score\n" + f.no("", 1) + ",5,5\n",
		"no rows":        "participant_no,score\n",
		"empty":          "",
		"broken quote":   "participant_no,score\n\"" + f.no("", 1) + ",5\n",
		"too large":      "participant_no,score\n" + strings.Repeat(row, 200000),
	} {
		t.Run(name, func(t *testing.T) {
			key := store.put(f.examID, strings.ReplaceAll(name, " ", "-")+".csv", csv)
			_, err := svc.PreviewCompetitionResult(context.Background(), f.examID, key)
			assert.ErrorIs(t, err, ErrValidation)
		})
	}
	key := store.put(f.examID, "ok.csv", "Score , Participant_No\n5,"+f.no("", 1)+"\n")
	p, err := svc.PreviewCompetitionResult(context.Background(), f.examID, key)
	require.NoError(t, err, "column order is free and header case/space insensitive")
	assert.Equal(t, 1, p.Matched)
}

func TestCompetitionResult_RejectsKeyOutsideTheExamPrefix(t *testing.T) {
	svc, repo, store := newCompetitionSvc(t)
	ctx := context.Background()
	f, other := seedCompetitionFixture(t, repo), seedCompetitionFixture(t, repo)
	foreign := store.put(other.examID, "x.csv", compHeader+other.no("", 1)+",5,,,\n")
	traversal := store.put(f.examID, "../../"+competitionKeyPrefix(other.examID)+"x.csv", compHeader+f.no("", 1)+",5,,,\n")
	for _, key := range []string{foreign, traversal, "exam-grant-bulk/" + f.examID.String() + "/x.csv"} {
		_, err := svc.PreviewCompetitionResult(ctx, f.examID, key)
		assert.ErrorIs(t, err, ErrUploadNotFound, key)
		_, _, err = svc.ApplyCompetitionResult(ctx, f.examID, f.actor, key)
		assert.ErrorIs(t, err, ErrUploadNotFound, key)
	}
	_, err := svc.PreviewCompetitionResult(ctx, f.examID, competitionKeyPrefix(f.examID)+"missing.csv")
	assert.ErrorIs(t, err, ErrUploadNotFound)
}

func TestCompetitionResult_ApplyWithErrorsWritesNothing(t *testing.T) {
	svc, repo, store := newCompetitionSvc(t)
	ctx := context.Background()
	f := seedCompetitionFixture(t, repo)
	key := store.put(f.examID, "e.csv", compHeader+f.no("", 1)+",5,,,\n"+f.no("", 99)+",5,,,\n")
	preview, result, err := svc.ApplyCompetitionResult(ctx, f.examID, f.actor, key)
	require.NoError(t, err)
	assert.Nil(t, result)
	assert.Len(t, preview.Errors, 1)
	_, err = repo.GetCompetitionResult(ctx, f.examID)
	assert.ErrorIs(t, err, repository.ErrNotFound)
}

func dumpCompetition(t *testing.T, svc *Service, examID uuid.UUID) string {
	t.Helper()
	rows, err := svc.ListCompetitionResultRows(context.Background(), examID)
	require.NoError(t, err)
	var b strings.Builder
	for _, r := range rows {
		fmt.Fprintf(&b, "%s|%v|%v|%v|%v\n", r.RegistrationID, r.Score, valOf(r.Rank), valOf(r.Award), valOf(r.CertificateURL))
	}
	return b.String()
}

func valOf[T any](p *T) any {
	if p == nil {
		return nil
	}
	return *p
}

func TestCompetitionResult_ApplyIsIdempotentAndReplacesAllRows(t *testing.T) {
	svc, repo, store := newCompetitionSvc(t)
	ctx := context.Background()
	f := seedCompetitionFixture(t, repo)
	keyA := store.put(f.examID, "a.csv", compHeader+f.no("", 1)+",90,1,,\n"+f.no("", 2)+",80,2,,\n")

	_, a1, err := svc.ApplyCompetitionResult(ctx, f.examID, f.actor, keyA)
	require.NoError(t, err)
	first := dumpCompetition(t, svc, f.examID)

	_, a2, err := svc.ApplyCompetitionResult(ctx, f.examID, f.actor, keyA)
	require.NoError(t, err)
	assert.Equal(t, a1.ID, a2.ID)
	assert.Equal(t, first, dumpCompetition(t, svc, f.examID), "same file twice gives identical rows")

	_, err = svc.SetCompetitionResultPublished(ctx, f.examID, true, f.actor)
	require.NoError(t, err)
	keyB := store.put(f.examID, "b.csv", compHeader+f.no("", 3)+",55,9,,\n")
	preview, err := svc.PreviewCompetitionResult(ctx, f.examID, keyB)
	require.NoError(t, err)
	assert.True(t, preview.ResultExists)
	assert.True(t, preview.ResultPublished)
	_, a3, err := svc.ApplyCompetitionResult(ctx, f.examID, f.actor, keyB)
	require.NoError(t, err)
	assert.Equal(t, 1, a3.RowCount, "rows absent from the new file are dropped")
	assert.Equal(t, a1.ID, a3.ID)
	assert.NotNil(t, a3.PublishedAt, "replacing a published set keeps it published")
	assert.Equal(t, keyB, *a3.SourceKey)
	rows, err := svc.ListCompetitionResultRows(ctx, f.examID)
	require.NoError(t, err)
	require.Len(t, rows, 1)
	assert.Equal(t, f.regs[3], rows[0].RegistrationID)
}

func TestCompetitionResult_StudentSeesOnlyPublishedOwnRow(t *testing.T) {
	svc, repo, store := newCompetitionSvc(t)
	ctx := context.Background()
	f := seedCompetitionFixture(t, repo)
	key := store.put(f.examID, "a.csv", compHeader+f.no("", 1)+",90,1,Medali emas,https://certs.example.test/1\n"+f.no("", 2)+",80,2,,\n")
	_, _, err := svc.ApplyCompetitionResult(ctx, f.examID, f.actor, key)
	require.NoError(t, err)

	result := func(student int) *model.StudentCompetitionResult {
		items, err := svc.GetExamRegistrations(ctx, f.students[student].String())
		require.NoError(t, err)
		require.Len(t, items, 1)
		return items[0].CompetitionResult
	}
	assert.Nil(t, result(1), "an unpublished set is invisible")

	_, err = svc.SetCompetitionResultPublished(ctx, f.examID, true, f.actor)
	require.NoError(t, err)
	got := result(1)
	require.NotNil(t, got)
	assert.Equal(t, 90.0, got.Score)
	assert.Equal(t, "Medali emas", *got.Award)
	assert.Equal(t, "https://certs.example.test/1", *got.CertificateURL)
	assert.Equal(t, 1, *got.Rank)
	require.NotNil(t, result(2))
	assert.Equal(t, 80.0, result(2).Score, "each student sees their own row")
	assert.Nil(t, result(3), "student with no row")

	_, err = svc.SetCompetitionResultPublished(ctx, f.examID, false, f.actor)
	require.NoError(t, err)
	assert.Nil(t, result(1))
}

func TestCompetitionResult_AuditsApplyPublishUnpublishDelete(t *testing.T) {
	svc, repo, store := newCompetitionSvc(t)
	ctx := context.Background()
	f := seedCompetitionFixture(t, repo)
	key := store.put(f.examID, "a.csv", compHeader+f.no("", 1)+",90,,,\n"+f.no("", 2)+",80,,,\n")
	_, result, err := svc.ApplyCompetitionResult(ctx, f.examID, f.actor, key)
	require.NoError(t, err)
	_, err = svc.SetCompetitionResultPublished(ctx, f.examID, true, f.actor)
	require.NoError(t, err)
	_, err = svc.SetCompetitionResultPublished(ctx, f.examID, false, f.actor)
	require.NoError(t, err)
	require.NoError(t, svc.DeleteCompetitionResult(ctx, f.examID, f.actor))
	assert.ErrorIs(t, svc.DeleteCompetitionResult(ctx, f.examID, f.actor), ErrCompetitionResultNotFound)

	rows, err := repo.Pool().Query(ctx,
		`SELECT action, actor_id::text, (metadata->>'rows')::int, COALESCE(metadata->>'source_key','') FROM audit_log
		WHERE target_type = 'competition_result' AND target_id = $1 ORDER BY id`, result.ID.String())
	require.NoError(t, err)
	defer rows.Close()
	type entry struct {
		action, actor string
		n             int
		key           string
	}
	var got []entry
	for rows.Next() {
		var e entry
		require.NoError(t, rows.Scan(&e.action, &e.actor, &e.n, &e.key))
		got = append(got, e)
	}
	assert.Equal(t, []entry{
		{"apply", f.actor, 2, key}, {"publish", f.actor, 2, ""}, {"unpublish", f.actor, 2, ""}, {"delete", f.actor, 2, key},
	}, got)
}

func TestCompetitionResult_DoesNotTouchSessionsOrLeaderboard(t *testing.T) {
	svc, repo, store := newCompetitionSvc(t)
	ctx := context.Background()
	f := seedCompetitionFixture(t, repo)
	for n, score := range map[int]float64{1: 77, 2: 88} {
		_, err := repo.Pool().Exec(ctx,
			`INSERT INTO exam_session (registration_id, student_id, exam_id, started_at, submitted_at, status, score)
			VALUES ($1, $2, $3, now(), now(), 'submitted', $4)`, f.regs[n], f.students[n], f.examID, score)
		require.NoError(t, err)
	}
	snapshot := func() (string, []model.ExamLeaderboardEntry) {
		var sessions string
		require.NoError(t, repo.Pool().QueryRow(ctx,
			`SELECT COALESCE(string_agg(s::text, ';' ORDER BY s.id), '') FROM exam_session s WHERE s.exam_id = $1`, f.examID).Scan(&sessions))
		board, _, err := svc.AdminGetLeaderboard(ctx, f.examID, "", 50)
		require.NoError(t, err)
		return sessions, board
	}
	sessionsBefore, boardBefore := snapshot()
	require.Len(t, boardBefore, 2)

	key := store.put(f.examID, "a.csv", compHeader+f.no("", 1)+",1,2,,\n"+f.no("", 2)+",2,1,,\n"+f.no("", 3)+",3,3,,\n")
	_, _, err := svc.ApplyCompetitionResult(ctx, f.examID, f.actor, key)
	require.NoError(t, err)
	_, err = svc.SetCompetitionResultPublished(ctx, f.examID, true, f.actor)
	require.NoError(t, err)

	sessionsAfter, boardAfter := snapshot()
	assert.Equal(t, sessionsBefore, sessionsAfter)
	assert.Equal(t, boardBefore, boardAfter)
}

func TestCompetitionResult_MissingExamAndNoResult(t *testing.T) {
	svc, repo, store := newCompetitionSvc(t)
	ctx := context.Background()
	f := seedCompetitionFixture(t, repo)
	ghost := uuid.New()
	key := store.put(ghost, "a.csv", compHeader+f.no("", 1)+",1,,,\n")
	_, err := svc.PreviewCompetitionResult(ctx, ghost, key)
	assert.ErrorIs(t, err, ErrExamNotFound)

	got, err := svc.GetCompetitionResult(ctx, f.examID)
	require.NoError(t, err)
	assert.Nil(t, got, "no result yet is nil, not an error")
	_, err = svc.SetCompetitionResultPublished(ctx, f.examID, true, f.actor)
	assert.ErrorIs(t, err, ErrCompetitionResultNotFound)
	_, err = svc.ListCompetitionResultRows(ctx, f.examID)
	assert.ErrorIs(t, err, ErrCompetitionResultNotFound)
}

func TestParseCompetitionResultCSV_ResolvesOnlyWithinThisExam(t *testing.T) {
	regA, revoked := uuid.New(), uuid.New()
	regs := map[int]repository.CompetitionRegistration{1: {ID: regA}, 2: {ID: revoked, Revoked: true}}
	csv := compHeader +
		"250620-1001-000001,1,,,\n" + // 2 ok, no prefix
		"OLDPX-250620-1001-000001,2,,,\n" + // 3 same registration, with prefix
		"250620-1002-000001,3,,,\n" + // 4 another exam
		"OLDPX-250620-1002-000001,4,,,\n" + // 5 another exam, with prefix
		"250620-1001-000009,5,,,\n" + // 6 nobody
		"250620-1001-000002,6,,,\n" // 7 revoked
	inputs, errs, err := parseCompetitionResultCSV([]byte(csv), 1001, regs)
	require.NoError(t, err)
	require.Len(t, inputs, 1)
	assert.Equal(t, regA, inputs[0].RegistrationID)
	got := map[int]string{}
	for _, e := range errs {
		got[e.Line] = e.Reason
	}
	assert.Equal(t, map[int]string{
		3: ReasonDuplicateParticipant, 4: ReasonOtherExamParticipantNo, 5: ReasonOtherExamParticipantNo,
		6: ReasonUnknownParticipant, 7: ReasonRevokedParticipant,
	}, got)
}

func TestParseCompetitionResultCSV_ShortRowIsBadRowNotPanic(t *testing.T) {
	inputs, errs, err := parseCompetitionResultCSV([]byte("score,participant_no\n5\n"), 1001, nil)
	require.NoError(t, err)
	assert.Empty(t, inputs)
	require.Len(t, errs, 1)
	assert.Equal(t, ReasonBadRow, errs[0].Reason)
}

func TestParseCompetitionResultCSV_ScoreMustFitNumeric82(t *testing.T) {
	reg := uuid.New()
	regs := map[int]repository.CompetitionRegistration{1: {ID: reg}, 2: {ID: uuid.New()}}
	_, errs, err := parseCompetitionResultCSV([]byte(compHeader+
		"250620-1001-000001,999999.999,,,\n"+
		"250620-1001-000002,999999.99,,,\n"), 1001, regs)
	require.NoError(t, err)
	require.Len(t, errs, 1)
	assert.Equal(t, ReasonBadScore, errs[0].Reason)
	assert.Equal(t, 2, errs[0].Line)
}

func TestCompetitionFileName_NeverContainsDotDotOrPath(t *testing.T) {
	assert.Equal(t, "hasil_final.csv", competitionFileName("hasil..final.csv"))
	assert.Equal(t, "x.csv", competitionFileName("../../etc/x.csv"))
	assert.Equal(t, "x.csv", competitionFileName(`a\b\x.csv`))
}
