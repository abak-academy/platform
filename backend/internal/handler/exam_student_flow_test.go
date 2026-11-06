package handler_test

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/labstack/echo/v4"
	"github.com/redis/go-redis/v9"

	"akademi-bimbel/internal/handler"
	"akademi-bimbel/internal/infra"
	"akademi-bimbel/internal/repository"
	"akademi-bimbel/internal/server"
	"akademi-bimbel/internal/service"
)

type studentFlowEnv struct {
	pool   *pgxpool.Pool
	e      *echo.Echo
	signer *infra.JWTSigner
	mr     *miniredis.Miniredis
}

func newStudentFlowEnv(t *testing.T) *studentFlowEnv {
	t.Helper()
	pool := newTemplatedPool(t)
	store := repository.New(pool)
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("miniredis: %v", err)
	}
	t.Cleanup(mr.Close)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { rdb.Close() })
	signer := infra.NewJWTSigner("test-secret", 15*time.Minute)
	svc := service.NewWithStore(
		store, store, rdb, signer,
		&service.NoopOTPProvider{}, &service.NoopEmailProvider{},
		&service.NoopPaymentClient{}, &service.NoopLogisticsClient{},
		nil, nil, nil,
	)
	e := echo.New()
	e.HideBanner = true
	server.RegisterRoutesForTest(e, handler.New(svc), svc, signer)
	return &studentFlowEnv{pool: pool, e: e, signer: signer, mr: mr}
}

func (env *studentFlowEnv) token(t *testing.T, userID uuid.UUID, role string) string {
	t.Helper()
	tokenString, jti, err := env.signer.SignAccess(userID.String(), role, nil, []string{})
	if err != nil {
		t.Fatalf("SignAccess: %v", err)
	}
	env.mr.Set("session:access:"+jti, userID.String())
	return tokenString
}

func seedStudentFlowExam(t *testing.T, pool *pgxpool.Pool) uuid.UUID {
	t.Helper()
	ctx := context.Background()
	var examID uuid.UUID
	if err := pool.QueryRow(ctx,
		`INSERT INTO exam (title, mode, format_version, navigation_policy, timer_mode, duration_minutes, max_attempts)
		VALUES ($1, 'utbk', 1, 'free', 'overall', 90, 2) RETURNING id`,
		"Flow Exam "+uuid.NewString()[:8],
	).Scan(&examID); err != nil {
		t.Fatalf("insert exam: %v", err)
	}
	for i, title := range []string{"Penalaran", "Literasi"} {
		var testID uuid.UUID
		if err := pool.QueryRow(ctx,
			`INSERT INTO test (title, subject, topic, duration_minutes) VALUES ($1, 'math', 'algebra', 30) RETURNING id`, title,
		).Scan(&testID); err != nil {
			t.Fatalf("insert test: %v", err)
		}
		if _, err := pool.Exec(ctx, `INSERT INTO exam_test (exam_id, test_id, sort_order) VALUES ($1, $2, $3)`, examID, testID, i+1); err != nil {
			t.Fatalf("insert exam_test: %v", err)
		}
		var qID uuid.UUID
		if err := pool.QueryRow(ctx,
			`INSERT INTO question (format, body, point_correct, point_wrong, material_html) VALUES ('mcq', 'Secret body', 1, 0, '<p>secret</p>') RETURNING id`,
		).Scan(&qID); err != nil {
			t.Fatalf("insert question: %v", err)
		}
		if _, err := pool.Exec(ctx, `INSERT INTO test_question (test_id, question_id, sort_order) VALUES ($1, $2, 1)`, testID, qID); err != nil {
			t.Fatalf("insert test_question: %v", err)
		}
		if _, err := pool.Exec(ctx,
			`INSERT INTO question_option (question_id, key, text, is_correct, sort_order) VALUES ($1, 'a', 'Secret option', true, 1)`, qID,
		); err != nil {
			t.Fatalf("insert option: %v", err)
		}
	}
	return examID
}

func seedStudentFlowRegistration(t *testing.T, pool *pgxpool.Pool, studentID, examID uuid.UUID) uuid.UUID {
	t.Helper()
	var regID uuid.UUID
	if err := pool.QueryRow(context.Background(),
		`INSERT INTO exam_registration (student_id, exam_id, token) VALUES ($1, $2, $3) RETURNING id`,
		studentID, examID, uuid.NewString(),
	).Scan(&regID); err != nil {
		t.Fatalf("insert registration: %v", err)
	}
	return regID
}

func assertNoForbiddenKeys(t *testing.T, label string, body []byte) {
	t.Helper()
	forbidden := map[string]bool{"body": true, "options": true, "material_html": true, "correct_answer": true, "is_correct": true}
	var doc any
	if err := json.Unmarshal(body, &doc); err != nil {
		t.Fatalf("%s: decode: %v", label, err)
	}
	var walk func(v any)
	walk = func(v any) {
		switch x := v.(type) {
		case map[string]any:
			for k, child := range x {
				if forbidden[k] {
					t.Errorf("%s: JSON contains forbidden key %q", label, k)
				}
				walk(child)
			}
		case []any:
			for _, child := range x {
				walk(child)
			}
		}
	}
	walk(doc)
}

func TestStudentFlow_listAndDetailCarryEffectiveRules(t *testing.T) {
	env := newStudentFlowEnv(t)
	schoolID := seedSchool(t, env.pool)
	studentID := seedUserWithSchool(t, env.pool, "student", "Flow Student", schoolID)
	examID := seedStudentFlowExam(t, env.pool)
	regID := seedStudentFlowRegistration(t, env.pool, studentID, examID)
	token := env.token(t, studentID, service.RoleStudent)

	t.Run("list", func(t *testing.T) {
		rec := getRequest(t, env.e, "/api/v1/exam/registrations", token)
		if rec.Code != http.StatusOK {
			t.Fatalf("want 200, got %d body=%s", rec.Code, rec.Body.String())
		}
		var resp struct {
			Data []struct {
				NavigationPolicy        string `json:"navigation_policy"`
				TimerMode               string `json:"timer_mode"`
				SectionCount            int    `json:"section_count"`
				QuestionCount           int    `json:"question_count"`
				SectionsDurationMinutes *int   `json:"sections_duration_minutes"`
			} `json:"data"`
		}
		if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
			t.Fatalf("decode: %v", err)
		}
		if len(resp.Data) != 1 {
			t.Fatalf("data len = %d, want 1", len(resp.Data))
		}
		got := resp.Data[0]
		if got.NavigationPolicy != "sequential" || got.TimerMode != "per_test" {
			t.Errorf("effective rules = %s/%s, want sequential/per_test (utbk ignores raw columns)", got.NavigationPolicy, got.TimerMode)
		}
		if got.SectionCount != 2 || got.QuestionCount != 2 || got.SectionsDurationMinutes == nil || *got.SectionsDurationMinutes != 60 {
			t.Errorf("aggregates = %+v, want 2 sections, 2 questions, 60 minutes", got)
		}
		assertNoForbiddenKeys(t, "list", rec.Body.Bytes())
	})

	t.Run("detail", func(t *testing.T) {
		rec := getRequest(t, env.e, "/api/v1/exam/registrations/"+regID.String(), token)
		if rec.Code != http.StatusOK {
			t.Fatalf("want 200, got %d body=%s", rec.Code, rec.Body.String())
		}
		var resp struct {
			Exam struct {
				NavigationPolicy string `json:"navigation_policy"`
				TimerMode        string `json:"timer_mode"`
				MaxAttempts      *int   `json:"max_attempts"`
				SectionCount     int    `json:"section_count"`
				Sections         []struct {
					Title                string `json:"title"`
					DurationMinutes      int    `json:"duration_minutes"`
					QuestionCount        int    `json:"question_count"`
					QuestionPresentation string `json:"question_presentation"`
				} `json:"sections"`
			} `json:"exam"`
		}
		if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
			t.Fatalf("decode: %v", err)
		}
		if resp.Exam.NavigationPolicy != "sequential" || resp.Exam.TimerMode != "per_test" {
			t.Errorf("effective rules = %s/%s, want sequential/per_test", resp.Exam.NavigationPolicy, resp.Exam.TimerMode)
		}
		if resp.Exam.MaxAttempts == nil || *resp.Exam.MaxAttempts != 2 || resp.Exam.SectionCount != 2 {
			t.Errorf("exam = %+v, want max_attempts 2 and 2 sections", resp.Exam)
		}
		if len(resp.Exam.Sections) != 2 || resp.Exam.Sections[0].Title != "Penalaran" || resp.Exam.Sections[1].Title != "Literasi" {
			t.Fatalf("sections = %+v, want Penalaran then Literasi", resp.Exam.Sections)
		}
		if s := resp.Exam.Sections[0]; s.DurationMinutes != 30 || s.QuestionCount != 1 || s.QuestionPresentation != "one_per_page" {
			t.Errorf("section[0] = %+v", s)
		}
		assertNoForbiddenKeys(t, "detail", rec.Body.Bytes())
	})
}

func TestStudentFlow_detailOfAnotherStudentIs404(t *testing.T) {
	env := newStudentFlowEnv(t)
	schoolID := seedSchool(t, env.pool)
	owner := seedUserWithSchool(t, env.pool, "student", "Owner", schoolID)
	other := seedUserWithSchool(t, env.pool, "student", "Other", schoolID)
	regID := seedStudentFlowRegistration(t, env.pool, owner, seedStudentFlowExam(t, env.pool))

	rec := getRequest(t, env.e, "/api/v1/exam/registrations/"+regID.String(), env.token(t, other, service.RoleStudent))
	if rec.Code != http.StatusNotFound {
		t.Fatalf("want 404, got %d body=%s", rec.Code, rec.Body.String())
	}
}

func TestStudentFlow_previewRBAC(t *testing.T) {
	env := newStudentFlowEnv(t)
	schoolID := seedSchool(t, env.pool)
	examID := seedStudentFlowExam(t, env.pool)
	path := "/api/v1/admin/exams/" + examID.String() + "/preview"

	for _, tc := range []struct {
		role string
		want int
	}{
		{service.RoleSuperAdmin, http.StatusOK},
		{service.RoleAdminExam, http.StatusOK},
		{service.RoleAdminSchool, http.StatusForbidden},
		{service.RoleAdminStore, http.StatusForbidden},
		{service.RoleStudent, http.StatusForbidden},
	} {
		t.Run(tc.role, func(t *testing.T) {
			userID := seedUserWithSchool(t, env.pool, tc.role, "Preview "+tc.role, schoolID)
			rec := getRequest(t, env.e, path, env.token(t, userID, tc.role))
			if rec.Code != tc.want {
				t.Fatalf("want %d, got %d body=%s", tc.want, rec.Code, rec.Body.String())
			}
			if tc.want != http.StatusOK {
				return
			}
			var resp struct {
				ID               string `json:"id"`
				NavigationPolicy string `json:"navigation_policy"`
				TimerMode        string `json:"timer_mode"`
				Sections         []struct {
					Title string `json:"title"`
				} `json:"sections"`
			}
			if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
				t.Fatalf("decode: %v", err)
			}
			if resp.ID != examID.String() || resp.NavigationPolicy != "sequential" || resp.TimerMode != "per_test" || len(resp.Sections) != 2 {
				t.Errorf("preview = %+v, want exam %s, sequential/per_test, 2 sections", resp, examID)
			}
			assertNoForbiddenKeys(t, "preview", rec.Body.Bytes())
		})
	}

	t.Run("unknown exam is 404", func(t *testing.T) {
		userID := seedUserWithSchool(t, env.pool, service.RoleAdminExam, "Preview 404", schoolID)
		rec := getRequest(t, env.e, "/api/v1/admin/exams/"+uuid.NewString()+"/preview", env.token(t, userID, service.RoleAdminExam))
		if rec.Code != http.StatusNotFound {
			t.Fatalf("want 404, got %d body=%s", rec.Code, rec.Body.String())
		}
	})
}

func TestStudentFlow_readsHaveNoSideEffects(t *testing.T) {
	env := newStudentFlowEnv(t)
	schoolID := seedSchool(t, env.pool)
	studentID := seedUserWithSchool(t, env.pool, "student", "Side Effect Student", schoolID)
	adminID := seedUserWithSchool(t, env.pool, "admin_exam", "Side Effect Admin", schoolID)
	examID := seedStudentFlowExam(t, env.pool)
	regID := seedStudentFlowRegistration(t, env.pool, studentID, examID)

	count := func(table string) int {
		t.Helper()
		var n int
		if err := env.pool.QueryRow(context.Background(), `SELECT COUNT(*) FROM `+table).Scan(&n); err != nil {
			t.Fatalf("count %s: %v", table, err)
		}
		return n
	}
	sessionsBefore, registrationsBefore := count("exam_session"), count("exam_registration")

	studentToken := env.token(t, studentID, service.RoleStudent)
	adminToken := env.token(t, adminID, service.RoleAdminExam)
	for _, req := range []struct{ path, token string }{
		{"/api/v1/exam/registrations/" + regID.String(), studentToken},
		{"/api/v1/exam/registrations", studentToken},
		{"/api/v1/admin/exams/" + examID.String() + "/preview", adminToken},
	} {
		if rec := getRequest(t, env.e, req.path, req.token); rec.Code != http.StatusOK {
			t.Fatalf("GET %s: %d %s", req.path, rec.Code, rec.Body.String())
		}
	}

	if got := count("exam_session"); got != sessionsBefore {
		t.Errorf("exam_session rows = %d, want %d", got, sessionsBefore)
	}
	if got := count("exam_registration"); got != registrationsBefore {
		t.Errorf("exam_registration rows = %d, want %d", got, registrationsBefore)
	}
}
