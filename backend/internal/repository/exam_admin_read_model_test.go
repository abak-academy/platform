package repository

import (
	"context"
	"encoding/json"
	"testing"

	"akademi-bimbel/internal/model"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
)

func seedAdminReadExam(t *testing.T, pool *pgxpool.Pool, title string) uuid.UUID {
	t.Helper()
	var id uuid.UUID
	require.NoError(t, pool.QueryRow(context.Background(),
		`INSERT INTO exam (title, status) VALUES ($1, 'draft') RETURNING id`, title).Scan(&id))
	return id
}

func attachAdminReadTest(t *testing.T, pool *pgxpool.Pool, examID uuid.UUID, sortOrder, duration int, materialByPosition map[int]string, questions int) uuid.UUID {
	t.Helper()
	ctx := context.Background()
	var testID uuid.UUID
	require.NoError(t, pool.QueryRow(ctx,
		`INSERT INTO test (title, subject, topic, duration_minutes) VALUES ($1, 'english', 'reading', $2) RETURNING id`,
		"AR "+uuid.NewString(), duration).Scan(&testID))
	_, err := pool.Exec(ctx,
		`INSERT INTO exam_test (exam_id, test_id, sort_order) VALUES ($1, $2, $3)`, examID, testID, sortOrder)
	require.NoError(t, err)
	for pos := 1; pos <= questions; pos++ {
		var material *string
		if m, ok := materialByPosition[pos]; ok {
			material = &m
		}
		var qID uuid.UUID
		require.NoError(t, pool.QueryRow(ctx,
			`INSERT INTO question (format, body, point_correct, point_wrong, material_html)
			 VALUES ('mcq', 'body', 1, 0, $1) RETURNING id`, material).Scan(&qID))
		_, err := pool.Exec(ctx,
			`INSERT INTO test_question (test_id, question_id, sort_order) VALUES ($1, $2, $3)`, testID, qID, pos*10)
		require.NoError(t, err)
	}
	return testID
}

func findListedExam(t *testing.T, items []model.ExamListItem, id uuid.UUID) model.ExamListItem {
	t.Helper()
	for _, it := range items {
		if it.ID == id {
			return it
		}
	}
	t.Fatalf("exam %s missing from list", id)
	return model.ExamListItem{}
}

func TestListExams_CompositionCounts(t *testing.T) {
	pool := newGradingTestPool(t)
	repo := New(pool)
	ctx := context.Background()
	prefix := "COMP " + uuid.NewString()

	empty := seedAdminReadExam(t, pool, prefix+" empty")
	two := seedAdminReadExam(t, pool, prefix+" two")
	attachAdminReadTest(t, pool, two, 0, 30, nil, 3)
	attachAdminReadTest(t, pool, two, 1, 45, nil, 2)

	items, _, err := repo.ListExams(ctx, ExamFilter{Q: prefix, Limit: 10})
	require.NoError(t, err)

	got := findListedExam(t, items, empty)
	require.Equal(t, 0, got.SectionCount)
	require.Equal(t, 0, got.QuestionCount)
	require.Nil(t, got.SectionsDurationMinutes)

	got = findListedExam(t, items, two)
	require.Equal(t, 2, got.SectionCount)
	require.Equal(t, 5, got.QuestionCount)
	require.NotNil(t, got.SectionsDurationMinutes)
	require.Equal(t, 75, *got.SectionsDurationMinutes)
}

func TestListExams_RegistrationCountIgnoresRevokedAndStaysSchoolScoped(t *testing.T) {
	pool := newGradingTestPool(t)
	repo := New(pool)
	ctx := context.Background()
	prefix := "REGC " + uuid.NewString()

	schoolA := insertSchool(t, pool, prefix+" A", "ra-"+uuid.NewString()[:8])
	schoolB := insertSchool(t, pool, prefix+" B", "rb-"+uuid.NewString()[:8])
	examID := seedAdminReadExam(t, pool, prefix)
	attachAdminReadTest(t, pool, examID, 0, 20, nil, 4)

	register := func(school, status string) {
		student := insertSchoolUser(t, pool, "student", "Reg Student", school)
		_, err := pool.Exec(ctx,
			`INSERT INTO exam_registration (student_id, exam_id, token, status) VALUES ($1, $2, $3, $4)`,
			student, examID, uuid.NewString(), status)
		require.NoError(t, err)
	}
	register(schoolA, "registered")
	register(schoolA, "revoked")
	register(schoolB, "registered")
	register(schoolB, "submitted")

	items, _, err := repo.ListExams(ctx, ExamFilter{Q: prefix, Limit: 10})
	require.NoError(t, err)
	all := findListedExam(t, items, examID)
	require.Equal(t, 3, all.RegistrationCount)
	require.Equal(t, 1, all.SectionCount)
	require.Equal(t, 4, all.QuestionCount)

	items, _, err = repo.ListExams(ctx, ExamFilter{Q: prefix, Limit: 10, SchoolFilter: &schoolA})
	require.NoError(t, err)
	scoped := findListedExam(t, items, examID)
	require.Equal(t, 1, scoped.RegistrationCount)
	require.Equal(t, 4, scoped.QuestionCount)
}

func TestListExams_StatusFilterStillWorks(t *testing.T) {
	pool := newGradingTestPool(t)
	repo := New(pool)
	ctx := context.Background()
	prefix := "STAT " + uuid.NewString()

	draft := seedAdminReadExam(t, pool, prefix+" draft")
	var published uuid.UUID
	require.NoError(t, pool.QueryRow(ctx,
		`INSERT INTO exam (title, status) VALUES ($1, 'published') RETURNING id`, prefix+" pub").Scan(&published))

	items, _, err := repo.ListExams(ctx, ExamFilter{Q: prefix, Status: "published", Limit: 10})
	require.NoError(t, err)
	require.Len(t, items, 1)
	require.Equal(t, published, items[0].ID)
	require.NotEqual(t, draft, items[0].ID)
}

func TestGetExamDetail_HasSessions(t *testing.T) {
	pool := newGradingTestPool(t)
	repo := New(pool)
	ctx := context.Background()

	tests := []struct {
		name         string
		registration bool
		sessionState string
		want         bool
	}{
		{"no registration or session", false, "", false},
		{"registration without session", true, "", false},
		{"in progress session", true, "in_progress", true},
		{"submitted only session", true, "submitted", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			examID := seedAdminReadExam(t, pool, "HS "+uuid.NewString())
			if tt.registration {
				var studentID, regID uuid.UUID
				require.NoError(t, pool.QueryRow(ctx,
					`INSERT INTO users (name, username, jenjang, role, status, auth_provider)
					 VALUES ('HS Student', $1, 'sd', 'student', 'active', 'password') RETURNING id`,
					"hs_"+uuid.NewString()).Scan(&studentID))
				require.NoError(t, pool.QueryRow(ctx,
					`INSERT INTO exam_registration (student_id, exam_id, token) VALUES ($1, $2, $3) RETURNING id`,
					studentID, examID, uuid.NewString()).Scan(&regID))
				switch tt.sessionState {
				case "in_progress":
					_, err := pool.Exec(ctx,
						`INSERT INTO exam_session (registration_id, student_id, exam_id, started_at) VALUES ($1, $2, $3, now())`,
						regID, studentID, examID)
					require.NoError(t, err)
				case "submitted":
					_, err := pool.Exec(ctx,
						`INSERT INTO exam_session (registration_id, student_id, exam_id, started_at, submitted_at) VALUES ($1, $2, $3, now(), now())`,
						regID, studentID, examID)
					require.NoError(t, err)
				}
			}
			detail, err := repo.GetExamDetail(ctx, examID)
			require.NoError(t, err)
			require.Equal(t, tt.want, detail.HasSessions)
		})
	}
}

func TestGetExamDetail_MaterialQuestionPositions(t *testing.T) {
	pool := newGradingTestPool(t)
	repo := New(pool)
	ctx := context.Background()

	examID := seedAdminReadExam(t, pool, "MP "+uuid.NewString())
	secret := "<p>SECRET-MATERIAL-" + uuid.NewString() + "</p>"
	attachAdminReadTest(t, pool, examID, 0, 10, map[int]string{2: secret, 4: secret}, 4)
	attachAdminReadTest(t, pool, examID, 1, 10, nil, 2)

	detail, err := repo.GetExamDetail(ctx, examID)
	require.NoError(t, err)
	require.Len(t, detail.Tests, 2)
	require.Equal(t, []int{2, 4}, detail.Tests[0].MaterialQuestionPositions)
	require.Equal(t, []int{}, detail.Tests[1].MaterialQuestionPositions)

	raw, err := json.Marshal(detail)
	require.NoError(t, err)
	require.NotContains(t, string(raw), "SECRET-MATERIAL")
	require.Contains(t, string(raw), `"material_question_positions":[2,4]`)
	require.Contains(t, string(raw), `"has_sessions":false`)
}
