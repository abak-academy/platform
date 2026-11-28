package handler_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"akademi-bimbel/config"
	"akademi-bimbel/internal/handler"
	"akademi-bimbel/internal/infra"
	"akademi-bimbel/internal/repository"
	"akademi-bimbel/internal/service"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type competitionEnv struct {
	e        *echo.Echo
	studentE *echo.Echo
	objs     map[string]string
	examID   uuid.UUID
	examNo   int
	student  uuid.UUID
	actorID  uuid.UUID
}

func newCompetitionEnv(t *testing.T, role string) *competitionEnv {
	t.Helper()
	ctx := context.Background()
	pool := newTemplatedPool(t)
	env := &competitionEnv{objs: map[string]string{}}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, ok := env.objs[strings.TrimPrefix(r.URL.Path, "/private/")]
		if !ok {
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(`<Error><Code>NoSuchKey</Code></Error>`))
			return
		}
		w.Header().Set("Last-Modified", time.Now().UTC().Format(http.TimeFormat))
		w.Header().Set("ETag", `"fake"`)
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(srv.Close)
	client, err := minio.New(strings.TrimPrefix(srv.URL, "http://"), &minio.Options{
		Creds: credentials.NewStaticV4("ak", "sk", ""), Region: "us-east-1",
	})
	require.NoError(t, err)
	store := repository.New(pool)
	svc := service.NewWithStore(store, store, nil, nil, &service.NoopOTPProvider{}, &service.NoopEmailProvider{}, nil, nil, client,
		&config.Config{ObjectStoragePrivateBucketName: "private", ObjectStorageRegion: "us-east-1"}, nil)
	h := handler.New(svc)

	user := func(name string) uuid.UUID {
		var id uuid.UUID
		require.NoError(t, pool.QueryRow(ctx,
			`INSERT INTO users (name, username, role, status, password_hash) VALUES ($1, $2, 'student', 'active', '') RETURNING id`,
			name, "u-"+uuid.NewString()[:8]).Scan(&id))
		return id
	}
	env.actorID = user("Admin Uji")
	require.NoError(t, pool.QueryRow(ctx,
		`INSERT INTO exam (title, status, scheduled_at) VALUES ('Handler Competition', 'draft', $1) RETURNING id, exam_number`,
		time.Date(2025, 6, 20, 9, 0, 0, 0, time.UTC)).Scan(&env.examID, &env.examNo))
	for n := 1; n <= 2; n++ {
		sid := user(fmt.Sprintf("Peserta %d", n))
		if n == 1 {
			env.student = sid
		}
		_, err := pool.Exec(ctx,
			`INSERT INTO exam_registration (student_id, exam_id, token, status, participant_number) VALUES ($1, $2, $3, 'registered', $4)`,
			sid, env.examID, uuid.NewString(), n)
		require.NoError(t, err)
	}

	e := echo.New()
	e.HideBanner = true
	e.Use(claimsInjector(&infra.Claims{Sub: env.actorID.String(), Role: role}))
	rbac := handler.RBACMiddleware("competition-results:write")
	e.GET("/admin/exams/:id/competition-result", h.AdminGetCompetitionResult, rbac)
	e.POST("/admin/exams/:id/competition-result/presign", h.AdminPresignCompetitionResult, rbac)
	e.POST("/admin/exams/:id/competition-result/preview", h.AdminPreviewCompetitionResult, rbac)
	e.POST("/admin/exams/:id/competition-result/apply", h.AdminApplyCompetitionResult, rbac)
	e.POST("/admin/exams/:id/competition-result/publish", h.AdminPublishCompetitionResult, rbac)
	e.POST("/admin/exams/:id/competition-result/unpublish", h.AdminUnpublishCompetitionResult, rbac)
	e.GET("/admin/exams/:id/competition-result/results", h.AdminListCompetitionResultRows, rbac)
	e.DELETE("/admin/exams/:id/competition-result", h.AdminDeleteCompetitionResult, rbac)
	env.e = e

	se := echo.New()
	se.HideBanner = true
	se.Use(claimsInjector(&infra.Claims{Sub: env.student.String(), Role: "student"}))
	se.GET("/exam/registrations", h.StudentListRegistrations)
	env.studentE = se
	return env
}

func (env *competitionEnv) do(method, path, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	env.e.ServeHTTP(rec, req)
	return rec
}

func (env *competitionEnv) base() string {
	return "/admin/exams/" + env.examID.String() + "/competition-result"
}

func (env *competitionEnv) no(n int) string {
	return fmt.Sprintf("250620-%04d-%06d", env.examNo, n)
}

func fileBody(key string) string {
	b, _ := json.Marshal(map[string]string{"key": key})
	return string(b)
}

func TestCompetitionResultHTTP_Flow(t *testing.T) {
	env := newCompetitionEnv(t, "admin_exam")
	base := env.base()

	rec := env.do(http.MethodGet, base, "")
	require.Equal(t, http.StatusOK, rec.Code)
	assert.JSONEq(t, `{"data":null}`, rec.Body.String())

	rec = env.do(http.MethodPost, base+"/presign?filename=r.csv", "")
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	var presigned service.PrivateUploadURL
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &presigned))
	assert.Regexp(t, `^competition-results/`+env.examID.String()+`/[0-9a-f-]{36}-r\.csv$`, presigned.Key)

	bad := presigned.Key + "-bad"
	env.objs[bad] = "participant_no,score\n" + env.no(1) + ",5\n" + env.no(9) + ",5\nnope,5\n"
	rec = env.do(http.MethodPost, base+"/preview", fileBody(bad))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.JSONEq(t, `{"matched":1,"result_exists":false,"result_published":false,"errors":[
		{"line":3,"participant_no":"`+env.no(9)+`","reason":"unknown_participant"},
		{"line":4,"participant_no":"nope","reason":"unreadable_participant_no"}]}`, rec.Body.String())

	rec = env.do(http.MethodPost, base+"/apply", fileBody(bad))
	assert.Equal(t, http.StatusUnprocessableEntity, rec.Code)
	assert.Contains(t, rec.Body.String(), `"unknown_participant"`)
	assert.JSONEq(t, `{"data":null}`, env.do(http.MethodGet, base, "").Body.String(), "a refused apply creates no result")

	good := presigned.Key + "-good"
	env.objs[good] = "participant_no,score,rank,award\n" + env.no(1) + ",90,1,Medali emas\n" + env.no(2) + ",80,2,\n"
	rec = env.do(http.MethodPost, base+"/apply", fileBody(good))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	var applied struct {
		Data struct {
			ID       string `json:"id"`
			ExamID   string `json:"exam_id"`
			RowCount int    `json:"row_count"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &applied))
	assert.Equal(t, 2, applied.Data.RowCount)
	assert.Equal(t, env.examID.String(), applied.Data.ExamID)

	rec = env.do(http.MethodGet, base+"/results", "")
	require.Equal(t, http.StatusOK, rec.Code)
	var results struct {
		Data []struct {
			StudentName   string  `json:"student_name"`
			ParticipantNo string  `json:"participant_no"`
			Score         float64 `json:"score"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &results))
	require.Len(t, results.Data, 2)
	assert.Equal(t, "Peserta 1", results.Data[0].StudentName)
	assert.Equal(t, env.no(1), results.Data[0].ParticipantNo)

	rec = env.do(http.MethodPost, base+"/publish", "")
	require.Equal(t, http.StatusOK, rec.Code)
	assert.NotContains(t, rec.Body.String(), `"published_at":null`)
	rec = env.do(http.MethodGet, base, "")
	assert.Contains(t, rec.Body.String(), `"row_count":2`)
	rec = env.do(http.MethodPost, base+"/unpublish", "")
	assert.Contains(t, rec.Body.String(), `"published_at":null`)

	assert.Equal(t, http.StatusNoContent, env.do(http.MethodDelete, base, "").Code)
	rec = env.do(http.MethodDelete, base, "")
	assert.Equal(t, http.StatusNotFound, rec.Code)
	assert.Contains(t, rec.Body.String(), "competition_result_not_found")
	assert.Equal(t, http.StatusNotFound, env.do(http.MethodPost, base+"/publish", "").Code)
}

func TestCompetitionResultHTTP_ForeignKeyAndRBAC(t *testing.T) {
	env := newCompetitionEnv(t, "admin_exam")
	base := env.base()
	foreign := "competition-results/" + uuid.NewString() + "/x.csv"
	env.objs[foreign] = "participant_no,score\n" + env.no(1) + ",5\n"
	assert.Equal(t, http.StatusNotFound, env.do(http.MethodPost, base+"/preview", fileBody(foreign)).Code)
	assert.Equal(t, http.StatusNotFound, env.do(http.MethodPost, base+"/apply", fileBody(foreign)).Code)
	assert.Equal(t, http.StatusBadRequest, env.do(http.MethodPost, base+"/preview", `{}`).Code)

	unknown := uuid.New()
	unknownKey := "competition-results/" + unknown.String() + "/x.csv"
	env.objs[unknownKey] = "participant_no,score\n" + env.no(1) + ",5\n"
	for _, op := range []string{"preview", "apply"} {
		rec := env.do(http.MethodPost, "/admin/exams/"+unknown.String()+"/competition-result/"+op, fileBody(unknownKey))
		assert.Equal(t, http.StatusNotFound, rec.Code, op)
		assert.Contains(t, rec.Body.String(), "exam_not_found", op)
	}

	denied := newCompetitionEnv(t, "admin_school")
	assert.Equal(t, http.StatusForbidden, denied.do(http.MethodGet, denied.base(), "").Code)
}

func TestStudentRegistrationsHTTP_CompetitionResultShape(t *testing.T) {
	env := newCompetitionEnv(t, "admin_exam")
	list := func() map[string]any {
		req := httptest.NewRequest(http.MethodGet, "/exam/registrations", nil)
		rec := httptest.NewRecorder()
		env.studentE.ServeHTTP(rec, req)
		require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
		var body struct {
			Data []map[string]any `json:"data"`
		}
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
		require.Len(t, body.Data, 1)
		return body.Data[0]
	}
	item := list()
	val, present := item["competition_result"]
	assert.True(t, present, "key is always present")
	assert.Nil(t, val, "null when there is no result")

	key := "competition-results/" + env.examID.String() + "/a.csv"
	env.objs[key] = "participant_no,score,rank,award,certificate_url\n" + env.no(1) + ",90.5,1,Medali emas,https://certs.example.test/1\n"
	rec := env.do(http.MethodPost, env.base()+"/apply", fileBody(key))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.Nil(t, list()["competition_result"], "unpublished is hidden")

	require.Equal(t, http.StatusOK, env.do(http.MethodPost, env.base()+"/publish", "").Code)
	got, ok := list()["competition_result"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, 90.5, got["score"])
	assert.Equal(t, 1.0, got["rank"])
	assert.Equal(t, "Medali emas", got["award"])
	assert.Equal(t, "https://certs.example.test/1", got["certificate_url"])
	assert.NotEmpty(t, got["published_at"])
}
