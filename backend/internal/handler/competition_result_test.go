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

type externalEnv struct {
	e        *echo.Echo
	studentE *echo.Echo
	objs     map[string]string
	examID   uuid.UUID
	examNo   int
	student  uuid.UUID
	actorID  uuid.UUID
	students []uuid.UUID
}

func newExternalEnv(t *testing.T, role string) *externalEnv {
	t.Helper()
	ctx := context.Background()
	pool := newTemplatedPool(t)
	env := &externalEnv{objs: map[string]string{}}
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
		`INSERT INTO exam (title, status, scheduled_at) VALUES ('Handler External', 'draft', $1) RETURNING id, exam_number`,
		time.Date(2025, 6, 20, 9, 0, 0, 0, time.UTC)).Scan(&env.examID, &env.examNo))
	for n := 1; n <= 2; n++ {
		sid := user(fmt.Sprintf("Peserta %d", n))
		env.students = append(env.students, sid)
		_, err := pool.Exec(ctx,
			`INSERT INTO exam_registration (student_id, exam_id, token, status, participant_number) VALUES ($1, $2, $3, 'registered', $4)`,
			sid, env.examID, uuid.NewString(), n)
		require.NoError(t, err)
	}
	env.student = env.students[0]

	e := echo.New()
	e.HideBanner = true
	e.Use(claimsInjector(&infra.Claims{Sub: env.actorID.String(), Role: role}))
	rbac := handler.RBACMiddleware("exam-results-external:write")
	e.GET("/admin/exams/:id/external-rounds", h.AdminListExternalRounds, rbac)
	e.POST("/admin/exams/:id/external-rounds/presign", h.AdminPresignExternalRound, rbac)
	e.POST("/admin/exams/:id/external-rounds/preview", h.AdminPreviewExternalRound, rbac)
	e.POST("/admin/exams/:id/external-rounds/apply", h.AdminApplyExternalRound, rbac)
	e.POST("/admin/external-rounds/:round_id/publish", h.AdminPublishExternalRound, rbac)
	e.POST("/admin/external-rounds/:round_id/unpublish", h.AdminUnpublishExternalRound, rbac)
	e.GET("/admin/external-rounds/:round_id/results", h.AdminListExternalResults, rbac)
	e.DELETE("/admin/external-rounds/:round_id", h.AdminDeleteExternalRound, rbac)
	env.e = e

	se := echo.New()
	se.HideBanner = true
	se.Use(claimsInjector(&infra.Claims{Sub: env.student.String(), Role: "student"}))
	se.GET("/exam/registrations", h.StudentListRegistrations)
	env.studentE = se
	return env
}

func (env *externalEnv) do(method, path, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	env.e.ServeHTTP(rec, req)
	return rec
}

func (env *externalEnv) no(n int) string {
	return fmt.Sprintf("250620-%04d-%06d", env.examNo, n)
}

func (env *externalEnv) fileBody(key, label string) string {
	b, _ := json.Marshal(map[string]string{"key": key, "round_label": label})
	return string(b)
}

func TestExternalRoundsHTTP_Flow(t *testing.T) {
	env := newExternalEnv(t, "admin_exam")
	base := "/admin/exams/" + env.examID.String() + "/external-rounds"

	rec := env.do(http.MethodPost, base+"/presign?filename=r.csv", "")
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	var presigned service.PrivateUploadURL
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &presigned))
	assert.Regexp(t, `^exam-external-results/`+env.examID.String()+`/[0-9a-f-]{36}-r\.csv$`, presigned.Key)

	bad := presigned.Key + "-bad"
	env.objs[bad] = "participant_no,score\n" + env.no(1) + ",5\n" + env.no(9) + ",5\nnope,5\n"
	rec = env.do(http.MethodPost, base+"/preview", env.fileBody(bad, "Global"))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.JSONEq(t, `{"matched":1,"round_exists":false,"round_published":false,"errors":[
		{"line":3,"participant_no":"`+env.no(9)+`","reason":"unknown_participant"},
		{"line":4,"participant_no":"nope","reason":"unreadable_participant_no"}]}`, rec.Body.String())

	rec = env.do(http.MethodPost, base+"/apply", env.fileBody(bad, "Global"))
	assert.Equal(t, http.StatusUnprocessableEntity, rec.Code)
	assert.Contains(t, rec.Body.String(), `"unknown_participant"`)
	rec = env.do(http.MethodGet, base, "")
	assert.JSONEq(t, `{"data":[]}`, rec.Body.String(), "a refused apply creates no round")

	good := presigned.Key + "-good"
	env.objs[good] = "participant_no,score,rank,award\n" + env.no(1) + ",90,1,Medali emas\n" + env.no(2) + ",80,2,\n"
	rec = env.do(http.MethodPost, base+"/apply", env.fileBody(good, "Global"))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	var applied struct {
		Data struct {
			ID       string `json:"id"`
			RowCount int    `json:"row_count"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &applied))
	assert.Equal(t, 2, applied.Data.RowCount)
	roundPath := "/admin/external-rounds/" + applied.Data.ID

	rec = env.do(http.MethodGet, roundPath+"/results", "")
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

	rec = env.do(http.MethodPost, roundPath+"/publish", "")
	require.Equal(t, http.StatusOK, rec.Code)
	assert.NotContains(t, rec.Body.String(), `"published_at":null`)
	rec = env.do(http.MethodPost, roundPath+"/unpublish", "")
	assert.Contains(t, rec.Body.String(), `"published_at":null`)

	assert.Equal(t, http.StatusNoContent, env.do(http.MethodDelete, roundPath, "").Code)
	assert.Equal(t, http.StatusNotFound, env.do(http.MethodDelete, roundPath, "").Code)
	assert.Equal(t, http.StatusNotFound, env.do(http.MethodPost, roundPath+"/publish", "").Code)
}

func TestExternalRoundsHTTP_ForeignKeyAndRBAC(t *testing.T) {
	env := newExternalEnv(t, "admin_exam")
	base := "/admin/exams/" + env.examID.String() + "/external-rounds"
	foreign := "exam-external-results/" + uuid.NewString() + "/x.csv"
	env.objs[foreign] = "participant_no,score\n" + env.no(1) + ",5\n"
	assert.Equal(t, http.StatusNotFound, env.do(http.MethodPost, base+"/preview", env.fileBody(foreign, "R")).Code)
	assert.Equal(t, http.StatusNotFound, env.do(http.MethodPost, base+"/apply", env.fileBody(foreign, "R")).Code)
	assert.Equal(t, http.StatusBadRequest, env.do(http.MethodPost, base+"/preview", `{}`).Code)

	denied := newExternalEnv(t, "admin_school")
	assert.Equal(t, http.StatusForbidden, denied.do(http.MethodGet, "/admin/exams/"+denied.examID.String()+"/external-rounds", "").Code)
}

func TestStudentRegistrationsHTTP_ExternalResultsShape(t *testing.T) {
	env := newExternalEnv(t, "admin_exam")
	list := func() []map[string]any {
		req := httptest.NewRequest(http.MethodGet, "/exam/registrations?lang=en", nil)
		rec := httptest.NewRecorder()
		env.studentE.ServeHTTP(rec, req)
		require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
		var body struct {
			Data []map[string]any `json:"data"`
		}
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
		require.Len(t, body.Data, 1)
		return body.Data
	}
	assert.Equal(t, []any{}, list()[0]["external_results"], "[] never null")

	key := "exam-external-results/" + env.examID.String() + "/a.csv"
	env.objs[key] = "participant_no,score,rank,award,certificate_url\n" + env.no(1) + ",90.5,1,Medali emas,https://certs.example.test/1\n"
	rec := env.do(http.MethodPost, "/admin/exams/"+env.examID.String()+"/external-rounds/apply", env.fileBody(key, "Global Round"))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	var applied struct {
		Data struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &applied))
	assert.Equal(t, []any{}, list()[0]["external_results"], "unpublished is hidden")

	require.Equal(t, http.StatusOK, env.do(http.MethodPost, "/admin/external-rounds/"+applied.Data.ID+"/publish", "").Code)
	got := list()[0]["external_results"].([]any)
	require.Len(t, got, 1)
	item := got[0].(map[string]any)
	assert.Equal(t, applied.Data.ID, item["round_id"])
	assert.Equal(t, "Global Round", item["round_label"])
	assert.Equal(t, 90.5, item["score"])
	assert.Equal(t, 1.0, item["rank"])
	assert.Equal(t, "Medali emas", item["award"])
	assert.Equal(t, "https://certs.example.test/1", item["certificate_url"])
	assert.NotEmpty(t, item["published_at"])
}
