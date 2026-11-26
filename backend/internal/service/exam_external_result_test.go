package service

import (
	"context"
	"errors"
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

func newExternalSvc(t *testing.T) (*Service, *repository.Repository, *fakeObjectStore) {
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
	key := externalKeyPrefix(examID) + name
	f.mu.Lock()
	f.objs[key] = []byte(csv)
	f.mu.Unlock()
	return key
}

type externalFixture struct {
	examID   uuid.UUID
	examNo   int
	regs     map[int]uuid.UUID
	students map[int]uuid.UUID
	revoked  int
	actor    string
}

// participants 1..3 are live, 4 is revoked.
func seedExternalExam(t *testing.T, repo *repository.Repository) externalFixture {
	t.Helper()
	ctx := context.Background()
	f := externalFixture{regs: map[int]uuid.UUID{}, students: map[int]uuid.UUID{}, revoked: 4, actor: seedStudentDirect(t, ctx, repo, "Admin Uji", "adm-"+uniqueSuffix()).String()}
	require.NoError(t, repo.Pool().QueryRow(ctx,
		`INSERT INTO exam (title, status, scheduled_at) VALUES ($1, 'draft', $2) RETURNING id, exam_number`,
		"External Exam "+uniqueSuffix(), time.Date(2025, 6, 20, 9, 0, 0, 0, time.UTC)).Scan(&f.examID, &f.examNo))
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

func (f externalFixture) no(prefix string, n int) string {
	return formatParticipantNo(prefix, time.Date(2025, 6, 20, 0, 0, 0, 0, time.UTC), f.examNo, n)
}

const extHeader = "participant_no,score,rank,award,certificate_url\n"

func TestExternalRound_MatchesWithoutPrefixWithPrefixAndOldPrefix(t *testing.T) {
	svc, repo, store := newExternalSvc(t)
	ctx := context.Background()
	f := seedExternalExam(t, repo)
	oldPfx, newPfx := "OLDPX", "NEWPX"
	gid := seedGrouping(t, repo, &oldPfx)
	sellExamIn(t, repo, f.examID, &gid)
	_, err := repo.Pool().Exec(ctx, `UPDATE product_grouping SET participant_prefix = $1 WHERE id = $2`, newPfx, gid)
	require.NoError(t, err)

	key := store.put(f.examID, "a.csv", extHeader+
		f.no("", 1)+",90,1,Medali emas,https://certs.example.test/1\n"+
		f.no(newPfx, 2)+",80.5,,,\n"+
		f.no(oldPfx, 3)+",70,3,,\n")

	preview, err := svc.PreviewExternalRound(ctx, f.examID, key, "Global Round")
	require.NoError(t, err)
	assert.Equal(t, 3, preview.Matched)
	assert.Empty(t, preview.Errors)
	assert.False(t, preview.RoundExists)

	_, round, err := svc.ApplyExternalRound(ctx, f.examID, f.actor, key, "Global Round")
	require.NoError(t, err)
	require.NotNil(t, round)
	assert.Equal(t, 3, round.RowCount)

	rows, err := svc.ListExternalResults(ctx, round.ID)
	require.NoError(t, err)
	require.Len(t, rows, 3)
	assert.Equal(t, f.no(newPfx, 1), rows[0].ParticipantNo, "shown in the CURRENT prefix, not the uploaded one")
	assert.Equal(t, "Peserta 1", rows[0].StudentName)
	assert.Equal(t, 90.0, rows[0].Score)
}

func TestExternalRound_PreviewReportsEveryErrorKindWithItsLine(t *testing.T) {
	svc, repo, store := newExternalSvc(t)
	ctx := context.Background()
	f := seedExternalExam(t, repo)
	longAward := strings.Repeat("x", 61)
	otherExam := formatParticipantNo("", time.Now(), f.examNo+1, 1)

	key := store.put(f.examID, "bad.csv", extHeader+
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
		f.no("", 2)+",50,,,\n"+ // 13 duplicate
		f.no("", 2)+",50\n") // 14 bad row

	preview, err := svc.PreviewExternalRound(ctx, f.examID, key, "R")
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

	// url, award and rank on rows that are otherwise unique
	key2 := store.put(f.examID, "bad2.csv", extHeader+
		f.no("", 1)+",1,0,,\n"+
		f.no("", 2)+",1,,"+longAward+",\n"+
		f.no("", 3)+",1,,,http://insecure.example.test/c\n")
	preview, err = svc.PreviewExternalRound(ctx, f.examID, key2, "R")
	require.NoError(t, err)
	assert.Equal(t, []model.ExternalRowError{
		{Line: 2, ParticipantNo: f.no("", 1), Reason: ReasonBadRank},
		{Line: 3, ParticipantNo: f.no("", 2), Reason: ReasonAwardTooLong},
		{Line: 4, ParticipantNo: f.no("", 3), Reason: ReasonBadCertificateURL},
	}, preview.Errors)

	key3 := store.put(f.examID, "bad3.csv", extHeader+f.no("", 1)+",1,2147483648,,\n")
	preview, err = svc.PreviewExternalRound(ctx, f.examID, key3, "R")
	require.NoError(t, err)
	assert.Equal(t, []model.ExternalRowError{{Line: 2, ParticipantNo: f.no("", 1), Reason: ReasonBadRank}}, preview.Errors)
}

func TestExternalRound_FileLevelErrors(t *testing.T) {
	svc, repo, store := newExternalSvc(t)
	f := seedExternalExam(t, repo)
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
			_, err := svc.PreviewExternalRound(context.Background(), f.examID, key, "R")
			assert.ErrorIs(t, err, ErrValidation)
		})
	}
	key := store.put(f.examID, "ok.csv", "Score , Participant_No\n5,"+f.no("", 1)+"\n")
	p, err := svc.PreviewExternalRound(context.Background(), f.examID, key, "R")
	require.NoError(t, err, "column order is free and header case/space insensitive")
	assert.Equal(t, 1, p.Matched)
}

func TestExternalRound_RejectsKeyOutsideTheExamPrefix(t *testing.T) {
	svc, repo, store := newExternalSvc(t)
	ctx := context.Background()
	f, other := seedExternalExam(t, repo), seedExternalExam(t, repo)
	foreign := store.put(other.examID, "x.csv", extHeader+other.no("", 1)+",5,,,\n")
	traversal := store.put(f.examID, "../../"+externalKeyPrefix(other.examID)+"x.csv", extHeader+f.no("", 1)+",5,,,\n")
	for _, key := range []string{foreign, traversal, "exam-grant-bulk/" + f.examID.String() + "/x.csv"} {
		_, err := svc.PreviewExternalRound(ctx, f.examID, key, "R")
		assert.ErrorIs(t, err, ErrUploadNotFound, key)
		_, _, err = svc.ApplyExternalRound(ctx, f.examID, f.actor, key, "R")
		assert.ErrorIs(t, err, ErrUploadNotFound, key)
	}
	_, err := svc.PreviewExternalRound(ctx, f.examID, externalKeyPrefix(f.examID)+"missing.csv", "R")
	assert.ErrorIs(t, err, ErrUploadNotFound)
}

func TestExternalRound_ApplyWithErrorsWritesNothing(t *testing.T) {
	svc, repo, store := newExternalSvc(t)
	ctx := context.Background()
	f := seedExternalExam(t, repo)
	key := store.put(f.examID, "e.csv", extHeader+f.no("", 1)+",5,,,\n"+f.no("", 99)+",5,,,\n")
	preview, round, err := svc.ApplyExternalRound(ctx, f.examID, f.actor, key, "R")
	require.NoError(t, err)
	assert.Nil(t, round)
	assert.Len(t, preview.Errors, 1)
	_, err = repo.GetExternalRoundByLabel(ctx, f.examID, "R")
	assert.ErrorIs(t, err, repository.ErrNotFound)
}

func dumpRound(t *testing.T, repo *repository.Repository, roundID uuid.UUID) string {
	t.Helper()
	rows, err := repo.ListExternalResults(context.Background(), roundID)
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

func TestExternalRound_ApplyIsIdempotentAndReplacesOnlyItsRound(t *testing.T) {
	svc, repo, store := newExternalSvc(t)
	ctx := context.Background()
	f := seedExternalExam(t, repo)
	keyA := store.put(f.examID, "a.csv", extHeader+f.no("", 1)+",90,1,,\n"+f.no("", 2)+",80,2,,\n")
	keyB := store.put(f.examID, "b.csv", extHeader+f.no("", 3)+",70,,,\n")

	_, a1, err := svc.ApplyExternalRound(ctx, f.examID, f.actor, keyA, "Round A")
	require.NoError(t, err)
	_, b, err := svc.ApplyExternalRound(ctx, f.examID, f.actor, keyB, "Round B")
	require.NoError(t, err)
	first := dumpRound(t, repo, a1.ID)

	_, a2, err := svc.ApplyExternalRound(ctx, f.examID, f.actor, keyA, "Round A")
	require.NoError(t, err)
	assert.Equal(t, a1.ID, a2.ID)
	assert.Equal(t, first, dumpRound(t, repo, a2.ID), "same file twice gives identical rows")

	bBefore := dumpRound(t, repo, b.ID)
	_, err = svc.SetExternalRoundPublished(ctx, a1.ID, true, f.actor)
	require.NoError(t, err)
	keyA2 := store.put(f.examID, "a2.csv", extHeader+f.no("", 3)+",55,9,,\n")
	preview, err := svc.PreviewExternalRound(ctx, f.examID, keyA2, "Round A")
	require.NoError(t, err)
	assert.True(t, preview.RoundExists)
	assert.True(t, preview.RoundPublished)
	_, a3, err := svc.ApplyExternalRound(ctx, f.examID, f.actor, keyA2, "Round A")
	require.NoError(t, err)
	assert.Equal(t, 1, a3.RowCount, "old rows replaced")
	assert.NotNil(t, a3.PublishedAt, "replacing a published round keeps it published")
	assert.Equal(t, bBefore, dumpRound(t, repo, b.ID), "other rounds untouched")
	assert.Equal(t, keyA2, *a3.SourceKey)
}

func TestExternalRound_StudentSeesOnlyPublishedOwnRows(t *testing.T) {
	svc, repo, store := newExternalSvc(t)
	ctx := context.Background()
	f := seedExternalExam(t, repo)
	keyA := store.put(f.examID, "a.csv", extHeader+f.no("", 1)+",90,1,Medali emas,https://certs.example.test/1\n"+f.no("", 2)+",80,2,,\n")
	keyB := store.put(f.examID, "b.csv", extHeader+f.no("", 1)+",70,,,\n")
	_, a, err := svc.ApplyExternalRound(ctx, f.examID, f.actor, keyA, "Ronde A")
	require.NoError(t, err)
	_, b, err := svc.ApplyExternalRound(ctx, f.examID, f.actor, keyB, "Ronde B")
	require.NoError(t, err)
	_, err = repo.Pool().Exec(ctx, `UPDATE exam_external_round SET label_en = 'Round B' WHERE id = $1`, b.ID)
	require.NoError(t, err)

	ext := func(student int, lang string) []model.StudentExternalResult {
		items, err := svc.GetExamRegistrations(ctx, f.students[student].String(), lang)
		require.NoError(t, err)
		require.Len(t, items, 1)
		require.NotNil(t, items[0].ExternalResults, "never null")
		return items[0].ExternalResults
	}
	assert.Empty(t, ext(1, ""), "unpublished rounds are invisible")

	_, err = svc.SetExternalRoundPublished(ctx, b.ID, true, f.actor)
	require.NoError(t, err)
	time.Sleep(5 * time.Millisecond)
	_, err = svc.SetExternalRoundPublished(ctx, a.ID, true, f.actor)
	require.NoError(t, err)

	got := ext(1, "")
	require.Len(t, got, 2)
	assert.Equal(t, []string{"Ronde B", "Ronde A"}, []string{got[0].RoundLabel, got[1].RoundLabel}, "ordered by published_at")
	assert.Equal(t, 90.0, got[1].Score)
	assert.Equal(t, "Medali emas", *got[1].Award)
	assert.Equal(t, "https://certs.example.test/1", *got[1].CertificateURL)
	assert.Equal(t, "Round B", ext(1, "en")[0].RoundLabel)
	assert.Equal(t, "Ronde A", ext(1, "en")[1].RoundLabel, "falls back when label_en is unset")

	own := ext(2, "")
	require.Len(t, own, 1, "student 2 is only in round A")
	assert.Equal(t, 80.0, own[0].Score)
	assert.Empty(t, ext(3, ""), "student with no rows")

	_, err = svc.SetExternalRoundPublished(ctx, a.ID, false, f.actor)
	require.NoError(t, err)
	assert.Len(t, ext(1, ""), 1)
	assert.Empty(t, ext(2, ""))
}

func TestExternalRound_AuditsApplyPublishUnpublishDelete(t *testing.T) {
	svc, repo, store := newExternalSvc(t)
	ctx := context.Background()
	f := seedExternalExam(t, repo)
	key := store.put(f.examID, "a.csv", extHeader+f.no("", 1)+",90,,,\n"+f.no("", 2)+",80,,,\n")
	_, round, err := svc.ApplyExternalRound(ctx, f.examID, f.actor, key, "Audit Round")
	require.NoError(t, err)
	_, err = svc.SetExternalRoundPublished(ctx, round.ID, true, f.actor)
	require.NoError(t, err)
	_, err = svc.SetExternalRoundPublished(ctx, round.ID, false, f.actor)
	require.NoError(t, err)
	require.NoError(t, svc.DeleteExternalRound(ctx, round.ID, f.actor))
	assert.ErrorIs(t, svc.DeleteExternalRound(ctx, round.ID, f.actor), ErrExternalRoundNotFound)

	rows, err := repo.Pool().Query(ctx,
		`SELECT action, actor_id::text, (metadata->>'rows')::int, COALESCE(metadata->>'source_key','') FROM audit_log
		WHERE target_type = 'exam_external_round' AND target_id = $1 ORDER BY id`, round.ID.String())
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

func TestExternalRound_DoesNotTouchSessionsOrLeaderboard(t *testing.T) {
	svc, repo, store := newExternalSvc(t)
	ctx := context.Background()
	f := seedExternalExam(t, repo)
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

	key := store.put(f.examID, "a.csv", extHeader+f.no("", 1)+",1,2,,\n"+f.no("", 2)+",2,1,,\n"+f.no("", 3)+",3,3,,\n")
	_, round, err := svc.ApplyExternalRound(ctx, f.examID, f.actor, key, "Global Round")
	require.NoError(t, err)
	_, err = svc.SetExternalRoundPublished(ctx, round.ID, true, f.actor)
	require.NoError(t, err)

	sessionsAfter, boardAfter := snapshot()
	assert.Equal(t, sessionsBefore, sessionsAfter)
	assert.Equal(t, boardBefore, boardAfter)
}

func TestExternalRound_BadLabelAndMissingExam(t *testing.T) {
	svc, repo, store := newExternalSvc(t)
	ctx := context.Background()
	f := seedExternalExam(t, repo)
	key := store.put(f.examID, "a.csv", extHeader+f.no("", 1)+",1,,,\n")
	_, err := svc.PreviewExternalRound(ctx, f.examID, key, "  ")
	assert.ErrorIs(t, err, ErrValidation)
	_, err = svc.PreviewExternalRound(ctx, f.examID, key, strings.Repeat("x", 81))
	assert.ErrorIs(t, err, ErrValidation)
	ghost := uuid.New()
	_, err = svc.PreviewExternalRound(ctx, ghost, externalKeyPrefix(ghost)+"a.csv", "R")
	assert.True(t, errors.Is(err, ErrExamNotFound) || errors.Is(err, ErrUploadNotFound), "got %v", err)
}
