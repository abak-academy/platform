package repository

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"akademi-bimbel/internal/model"
)

type sectionSeed struct {
	title     string
	duration  int
	questions int
	sortOrder int
}

func seedStudentFlowExam(t *testing.T, pool *pgxpool.Pool, formatVersion int, mode, navigation, timer string, sections []sectionSeed) uuid.UUID {
	t.Helper()
	ctx := context.Background()
	var examID uuid.UUID
	if err := pool.QueryRow(ctx,
		`INSERT INTO exam (title, format_version, mode, navigation_policy, timer_mode, max_attempts)
		VALUES ('Flow Exam', $1, $2, $3, $4, 2) RETURNING id`,
		formatVersion, mode, navigation, timer,
	).Scan(&examID); err != nil {
		t.Fatalf("insert exam: %v", err)
	}
	for _, sec := range sections {
		var testID uuid.UUID
		if err := pool.QueryRow(ctx,
			`INSERT INTO test (title, subject, topic, duration_minutes) VALUES ($1, 'math', 'algebra', $2) RETURNING id`,
			sec.title, sec.duration,
		).Scan(&testID); err != nil {
			t.Fatalf("insert test: %v", err)
		}
		if _, err := pool.Exec(ctx,
			`INSERT INTO exam_test (exam_id, test_id, sort_order) VALUES ($1, $2, $3)`,
			examID, testID, sec.sortOrder,
		); err != nil {
			t.Fatalf("insert exam_test: %v", err)
		}
		for i := 1; i <= sec.questions; i++ {
			insertGradingEssayQuestion(t, pool, testID, "Q", 1, i)
		}
	}
	return examID
}

func TestExamStudentFlow_aggregatesAndSections(t *testing.T) {
	pool := newGradingTestPool(t)
	repo := New(pool)
	ctx := context.Background()

	tests := []struct {
		name         string
		sections     []sectionSeed
		wantSections int
		wantQuestion int
		wantDuration *int
		wantTitles   []string
	}{
		{"no tests", nil, 0, 0, nil, []string{}},
		{"several tests ordered by sort_order", []sectionSeed{
			{"Second", 30, 3, 2},
			{"First", 45, 2, 1},
			{"Third", 15, 1, 3},
		}, 3, 6, intPtr(90), []string{"First", "Second", "Third"}},
		{"test without questions", []sectionSeed{
			{"Empty", 20, 0, 1},
			{"Full", 10, 4, 2},
		}, 2, 4, intPtr(30), []string{"Empty", "Full"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			examID := seedStudentFlowExam(t, pool, 2, "standard", "sequential", "per_test", tt.sections)
			studentID := insertGradingUser(t, pool, "student", "Flow Student")
			regID := insertGradingRegistration(t, pool, studentID, examID)

			list, err := repo.GetExamRegistrationsByStudent(ctx, studentID)
			if err != nil {
				t.Fatalf("list: %v", err)
			}
			if len(list) != 1 {
				t.Fatalf("list len = %d, want 1", len(list))
			}
			item := list[0]
			assertAggregate(t, "list", item.SectionCount, item.QuestionCount, item.SectionsDurationMinutes, tt.wantSections, tt.wantQuestion, tt.wantDuration)
			if want := (model.ExamRuleColumns{FormatVersion: 2, Mode: "standard", NavigationPolicy: "sequential", TimerMode: "per_test"}); item.RawRules != want {
				t.Errorf("list RawRules = %+v, want %+v", item.RawRules, want)
			}

			detail, err := repo.GetExamRegistrationByID(ctx, regID, studentID)
			if err != nil {
				t.Fatalf("detail: %v", err)
			}
			assertAggregate(t, "detail", detail.Exam.SectionCount, detail.Exam.QuestionCount, detail.Exam.SectionsDurationMinutes, tt.wantSections, tt.wantQuestion, tt.wantDuration)
			if detail.Exam.RawRules.NavigationPolicy != "sequential" || detail.Exam.RawRules.TimerMode != "per_test" || detail.Exam.RawRules.FormatVersion != 2 {
				t.Errorf("detail RawRules = %+v", detail.Exam.RawRules)
			}
			if detail.Exam.MaxAttempts == nil || *detail.Exam.MaxAttempts != 2 {
				t.Errorf("detail MaxAttempts = %v, want 2", detail.Exam.MaxAttempts)
			}

			preview, err := repo.GetExamPreview(ctx, examID)
			if err != nil {
				t.Fatalf("preview: %v", err)
			}
			assertAggregate(t, "preview", preview.SectionCount, preview.QuestionCount, preview.SectionsDurationMinutes, tt.wantSections, tt.wantQuestion, tt.wantDuration)
			if preview.ID != examID || preview.Title != "Flow Exam" {
				t.Errorf("preview identity = %v %q", preview.ID, preview.Title)
			}

			sections, err := repo.GetExamSections(ctx, examID)
			if err != nil {
				t.Fatalf("sections: %v", err)
			}
			if sections == nil || len(sections) != len(tt.wantTitles) {
				t.Fatalf("sections = %+v, want titles %v", sections, tt.wantTitles)
			}
			for i, sec := range sections {
				if sec.Title != tt.wantTitles[i] {
					t.Errorf("sections[%d].Title = %q, want %q", i, sec.Title, tt.wantTitles[i])
				}
				if sec.QuestionPresentation != "one_per_page" {
					t.Errorf("sections[%d].QuestionPresentation = %q, want one_per_page", i, sec.QuestionPresentation)
				}
			}
		})
	}
}

func TestExamStudentFlow_sectionMetadataMatchesTests(t *testing.T) {
	pool := newGradingTestPool(t)
	repo := New(pool)
	ctx := context.Background()

	examID := seedStudentFlowExam(t, pool, 1, "utbk", "free", "overall", []sectionSeed{{"Only", 25, 2, 1}})
	sections, err := repo.GetExamSections(ctx, examID)
	if err != nil {
		t.Fatalf("sections: %v", err)
	}
	if len(sections) != 1 || sections[0].DurationMinutes != 25 || sections[0].QuestionCount != 2 || sections[0].TestID == uuid.Nil {
		t.Fatalf("sections = %+v, want one section of 25 min / 2 questions", sections)
	}

	if _, err := repo.GetExamPreview(ctx, uuid.New()); err != ErrNotFound {
		t.Errorf("GetExamPreview(unknown) err = %v, want ErrNotFound", err)
	}
}

func TestExamStudentFlow_payloadHasNoQuestionContent(t *testing.T) {
	pool := newGradingTestPool(t)
	repo := New(pool)
	ctx := context.Background()

	examID := seedStudentFlowExam(t, pool, 1, "standard", "free", "overall", []sectionSeed{{"A", 10, 1, 1}})
	preview, err := repo.GetExamPreview(ctx, examID)
	if err != nil {
		t.Fatalf("preview: %v", err)
	}
	preview.Sections, err = repo.GetExamSections(ctx, examID)
	if err != nil {
		t.Fatalf("sections: %v", err)
	}
	raw, err := json.Marshal(preview)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	for _, key := range []string{`"body"`, `"options"`, `"material_html"`, `"correct_answer"`, `"is_correct"`} {
		if strings.Contains(string(raw), key) {
			t.Errorf("preview JSON contains forbidden key %s: %s", key, raw)
		}
	}
}

func assertAggregate(t *testing.T, label string, sections, questions int, duration *int, wantSections, wantQuestions int, wantDuration *int) {
	t.Helper()
	if sections != wantSections || questions != wantQuestions {
		t.Errorf("%s section/question count = %d/%d, want %d/%d", label, sections, questions, wantSections, wantQuestions)
	}
	switch {
	case duration == nil && wantDuration != nil, duration != nil && wantDuration == nil:
		t.Errorf("%s sections duration = %v, want %v", label, duration, wantDuration)
	case duration != nil && *duration != *wantDuration:
		t.Errorf("%s sections duration = %d, want %d", label, *duration, *wantDuration)
	}
}
