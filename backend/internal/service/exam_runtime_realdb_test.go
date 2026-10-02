package service

import (
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"akademi-bimbel/internal/model"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestComposableRuntime_RealSessionLifecycle(t *testing.T) {
	svc, repo := newRealDBService(t)
	ctx := t.Context()
	for _, tc := range []struct {
		name, mode, navigation, timer string
		version                       int
		sequential                    bool
	}{
		{"free overall ignores ielts", "ielts", "free", "overall", 2, false},
		{"sequential overall ignores standard", "standard", "sequential", "overall", 2, true},
		{"sequential section ignores standard", "standard", "sequential", "per_test", 2, true},
		{"legacy standard ignores navigation", "standard", "sequential", "overall", 1, false},
		{"legacy ielts keeps section runtime", "ielts", "free", "overall", 1, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			duration := 60
			exam := &model.Exam{Title: "Runtime " + uniqueSuffix(), FormatVersion: tc.version, Mode: tc.mode, NavigationPolicy: tc.navigation, TimerMode: tc.timer, DurationMinutes: &duration, Randomize: true, ResultConfig: "score_only"}
			require.NoError(t, repo.CreateExam(ctx, exam))
			ids := []uuid.UUID{seedTestWithQuestion(t, repo), seedTestWithQuestion(t, repo)}
			for _, testID := range ids {
				for position := 2; position <= 4; position++ {
					var questionID uuid.UUID
					require.NoError(t, repo.Pool().QueryRow(ctx, `INSERT INTO question (format,body,point_correct,point_wrong) VALUES ('essay','Additional question',1,0) RETURNING id`).Scan(&questionID))
					_, err := repo.Pool().Exec(ctx, `INSERT INTO test_question(test_id,question_id,sort_order) VALUES($1,$2,$3)`, testID, questionID, position)
					require.NoError(t, err)
				}
			}
			require.NoError(t, svc.ReplaceExamTests(ctx, exam.ID, []model.ExamTest{{TestID: ids[0], QuestionPresentation: "all_on_page"}, {TestID: ids[1], QuestionPresentation: "one_per_page"}}, ""))
			_, err := repo.Pool().Exec(ctx, `UPDATE test SET shared_material_html='<p>shared</p>',instructions_html='<b>instructions</b>' WHERE id=$1`, ids[0])
			require.NoError(t, err)
			_, err = repo.Pool().Exec(ctx, `UPDATE question SET material_html='<p>own</p>',image_url='/own.png' WHERE id IN (SELECT question_id FROM test_question WHERE test_id=$1)`, ids[1])
			require.NoError(t, err)
			var student, reg uuid.UUID
			require.NoError(t, repo.Pool().QueryRow(ctx, `INSERT INTO users (name,username,jenjang,role,status,auth_provider) VALUES ('Runtime Student',$1,'sd','student','active','password') RETURNING id`, "rt_"+uniqueSuffix()).Scan(&student))
			require.NoError(t, repo.Pool().QueryRow(ctx, `INSERT INTO exam_registration (student_id,exam_id,token) VALUES ($1,$2,$3) RETURNING id`, student, exam.ID, uuid.NewString()).Scan(&reg))
			start, err := svc.StartSession(ctx, student.String(), reg.String(), "fp")
			require.NoError(t, err)
			require.Len(t, start.Tests, 2)
			require.Len(t, start.Tests[0].Questions, 4)
			require.Len(t, start.Tests[1].Questions, 4)
			if tc.sequential {
				require.Equal(t, &ids[0], start.ActiveTestID)
			} else {
				require.Nil(t, start.ActiveTestID)
			}
			if tc.version == 2 {
				raw, err := json.Marshal(start)
				require.NoError(t, err)
				var payload map[string]any
				require.NoError(t, json.Unmarshal(raw, &payload))
				require.Equal(t, float64(2), payload["format_version"])
				require.Equal(t, tc.navigation, payload["navigation_policy"])
				section := payload["tests"].([]any)[0].(map[string]any)
				require.Equal(t, "all_on_page", section["question_presentation"])
				require.Equal(t, "<p>shared</p>", section["shared_material_html"])
				require.Equal(t, "<b>instructions</b>", section["instructions_html"])
				question := payload["tests"].([]any)[1].(map[string]any)["questions"].([]any)[0].(map[string]any)
				require.Equal(t, "<p>own</p>", question["material_html"])
				require.Equal(t, "/own.png", question["image_url"])
				require.NotContains(t, question, "correct_answer")
				if tc.timer == "overall" {
					require.Greater(t, start.RemainingSeconds, int64(3500))
				} else {
					require.InDelta(t, 1800, start.RemainingSeconds, 5)
				}
			}
			first, second := start.Tests[0].Questions[0].ID, start.Tests[1].Questions[0].ID
			answer := "response"
			require.NoError(t, svc.SaveAnswers(ctx, student.String(), start.SessionID.String(), []AnswerInput{{QuestionID: first, Answer: &answer}}, nil))
			err = svc.SaveAnswers(ctx, student.String(), start.SessionID.String(), []AnswerInput{{QuestionID: second, Answer: &answer}}, nil)
			if tc.sequential {
				require.ErrorIs(t, err, ErrSectionLocked)
			} else {
				require.NoError(t, err)
			}
			resumed, err := svc.ReconnectSession(ctx, student.String(), start.SessionID.String())
			require.NoError(t, err)
			require.Equal(t, start.Tests[0].Questions, resumed.Tests[0].Questions)
			require.NotEmpty(t, resumed.Answers)
			if tc.sequential {
				advanced, err := svc.AdvanceSection(ctx, student.String(), start.SessionID.String(), ids[0].String())
				require.NoError(t, err)
				require.Equal(t, &ids[1], advanced.ActiveTestID)
				require.Equal(t, start.Tests[1].Questions, advanced.Tests[1].Questions)
				require.NoError(t, svc.SaveAnswers(ctx, student.String(), start.SessionID.String(), []AnswerInput{{QuestionID: second, Answer: &answer}}, nil))
				require.ErrorIs(t, svc.SaveAnswers(ctx, student.String(), start.SessionID.String(), []AnswerInput{{QuestionID: first, Answer: &answer}}, nil), ErrSectionLocked)
			} else {
				_, err := svc.AdvanceSection(ctx, student.String(), start.SessionID.String(), ids[0].String())
				require.ErrorIs(t, err, ErrValidation)
			}
			if tc.version == 2 && tc.sequential && tc.timer == "overall" {
				monitor, err := svc.GetSessionMonitor(ctx, exam.ID.String())
				require.NoError(t, err)
				require.Len(t, monitor.Rows, 1)
				require.Greater(t, monitor.Rows[0].ActiveSectionRemainingSeconds, int64(3500))
			}
			_, err = svc.SubmitSession(ctx, student.String(), start.SessionID.String())
			require.NoError(t, err)
			stored, err := repo.GetExamSessionByID(ctx, start.SessionID)
			require.NoError(t, err)
			require.Equal(t, "submitted", stored.Status)
		})
	}
}

func TestComposableRuntime_ExpiryClock(t *testing.T) {
	svc, repo := newRealDBService(t)
	ctx := t.Context()
	for _, tc := range []struct {
		name, navigation, timer string
		sectionDue, examDue     bool
		want                    string
	}{
		{"overall ignores elapsed section", "sequential", "overall", true, false, "in_progress"},
		{"overall expires sequential attempt", "sequential", "overall", false, true, "submitted"},
		{"overall expires free regardless of mode", "free", "overall", false, true, "submitted"},
		{"section clock expires last section", "sequential", "per_test", true, false, "submitted"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			examID, tid, _ := seedV2PublishExam(t, repo, "one_per_page", false)
			_, err := repo.Pool().Exec(ctx, `UPDATE exam SET mode='ielts',navigation_policy=$1,timer_mode=$2 WHERE id=$3`, tc.navigation, tc.timer, examID)
			require.NoError(t, err)
			student := seedExamSession(t, repo, examID)
			var sid uuid.UUID
			require.NoError(t, repo.Pool().QueryRow(ctx, `SELECT id FROM exam_session WHERE exam_id=$1`, examID).Scan(&sid))
			if tc.navigation == "sequential" {
				start := time.Now()
				if tc.sectionDue {
					start = start.Add(-31 * time.Minute)
				}
				_, err = repo.Pool().Exec(ctx, `INSERT INTO exam_session_section(session_id,test_id,sort_order,duration_minutes,status,started_at) VALUES($1,$2,0,30,'active',$3)`, sid, tid, start)
				require.NoError(t, err)
			}
			if tc.examDue {
				_, err = repo.Pool().Exec(ctx, `UPDATE exam_session SET started_at=now()-interval '61 minutes' WHERE id=$1`, sid)
				require.NoError(t, err)
			}
			results, err := svc.ExpireExamSessions(ctx, time.Now(), 100)
			require.NoError(t, err)
			for _, r := range results {
				require.NoError(t, r.Err)
			}
			state, err := svc.ReconnectSession(ctx, student.String(), sid.String())
			require.NoError(t, err)
			require.Equal(t, tc.want, state.Status)
		})
	}
}

func TestComposableRuntime_CreateActivatesFormatTwo(t *testing.T) {
	svc, repo := newRealDBService(t)
	for _, nav := range []string{"free", "sequential"} {
		t.Run(nav, func(t *testing.T) {
			exam, err := svc.CreateExam(t.Context(), model.Exam{Title: "New Runtime " + uniqueSuffix(), NavigationPolicy: nav})
			require.NoError(t, err)
			require.Equal(t, 2, exam.FormatVersion)
			stored, err := repo.GetExamByID(t.Context(), exam.ID)
			require.NoError(t, err)
			require.Equal(t, 2, stored.FormatVersion)
		})
	}
}

func TestComposableRuntime_MonitorExtension(t *testing.T) {
	svc, repo := newRealDBService(t)
	for _, timer := range []string{"overall", "per_test"} {
		for _, overdue := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s overdue=%v", timer, overdue), func(t *testing.T) {
				ctx := t.Context()
				eid, tid, _ := seedV2PublishExam(t, repo, "one_per_page", false)
				_, err := repo.Pool().Exec(ctx, `UPDATE exam SET navigation_policy='sequential',timer_mode=$1 WHERE id=$2`, timer, eid)
				require.NoError(t, err)
				student := seedExamSession(t, repo, eid)
				var sid uuid.UUID
				require.NoError(t, repo.Pool().QueryRow(ctx, `SELECT id FROM exam_session WHERE exam_id=$1`, eid).Scan(&sid))
				peer := seedExamSession(t, repo, eid)
				started := time.Now()
				if overdue {
					started = started.Add(-2 * time.Hour)
				}
				_, err = repo.Pool().Exec(ctx, `UPDATE exam_session SET started_at=$2 WHERE id=$1`, sid, started)
				require.NoError(t, err)
				_, err = repo.Pool().Exec(ctx, `INSERT INTO exam_session_section(session_id,test_id,sort_order,duration_minutes,status,started_at) VALUES($1,$2,0,30,'active',$3)`, sid, tid, started)
				require.NoError(t, err)
				before, err := svc.ReconnectSession(ctx, student.String(), sid.String())
				require.NoError(t, err)
				require.NoError(t, svc.ReopenSession(ctx, sid.String(), 15))
				after, err := svc.ReconnectSession(ctx, student.String(), sid.String())
				require.NoError(t, err)
				require.InDelta(t, before.RemainingSeconds+900, after.RemainingSeconds, 5)
				require.NoError(t, svc.ReopenSession(ctx, sid.String(), 10))
				twice, err := svc.ReconnectSession(ctx, student.String(), sid.String())
				require.NoError(t, err)
				require.InDelta(t, after.RemainingSeconds+600, twice.RemainingSeconds, 5)
				exam, err := repo.GetExamByID(ctx, eid)
				require.NoError(t, err)
				require.Equal(t, 60, *exam.DurationMinutes)
				require.Equal(t, timer, exam.TimerMode)
				var peerExtension *time.Time
				require.NoError(t, repo.Pool().QueryRow(ctx, `SELECT extended_until FROM exam_session WHERE exam_id=$1 AND student_id=$2`, eid, peer).Scan(&peerExtension))
				require.Nil(t, peerExtension)
			})
		}
	}
}

func TestComposableRuntime_MonitorExtensionRejectsNonpositiveMinutes(t *testing.T) {
	svc, repo := newRealDBService(t)
	eid, _, _ := seedV2PublishExam(t, repo, "one_per_page", false)
	seedExamSession(t, repo, eid)
	var sid uuid.UUID
	require.NoError(t, repo.Pool().QueryRow(t.Context(), `SELECT id FROM exam_session WHERE exam_id=$1`, eid).Scan(&sid))
	for _, minutes := range []int{0, -15} {
		t.Run(fmt.Sprint(minutes), func(t *testing.T) {
			require.ErrorIs(t, svc.ReopenSession(t.Context(), sid.String(), minutes), ErrValidation)
			var extension *time.Time
			require.NoError(t, repo.Pool().QueryRow(t.Context(), `SELECT extended_until FROM exam_session WHERE id=$1`, sid).Scan(&extension))
			require.Nil(t, extension)
		})
	}
}
