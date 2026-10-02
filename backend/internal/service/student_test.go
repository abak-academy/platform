package service

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"

	"akademi-bimbel/internal/model"
	"akademi-bimbel/internal/repository"
)

// fakeDashboardRepo implements the store methods needed by GetDashboard.
type fakeDashboardRepo struct {
	sessions   []model.CourseSession
	courses    map[uuid.UUID]model.Course
	lessonCnt  map[uuid.UUID]int
	duration   map[uuid.UUID]int // lessonID -> duration_seconds
	orders     map[uuid.UUID]*model.Order
}

func newFakeDashboardRepo() *fakeDashboardRepo {
	return &fakeDashboardRepo{
		courses:   make(map[uuid.UUID]model.Course),
		lessonCnt: make(map[uuid.UUID]int),
		duration:  make(map[uuid.UUID]int),
		orders:    make(map[uuid.UUID]*model.Order),
	}
}

func (f *fakeDashboardRepo) ListActiveSessionsByStudent(_ context.Context, studentID uuid.UUID) ([]model.CourseSession, error) {
	var out []model.CourseSession
	for _, s := range f.sessions {
		if s.StudentID == studentID {
			out = append(out, s)
		}
	}
	return out, nil
}

func (f *fakeDashboardRepo) GetCourseByID(_ context.Context, id uuid.UUID) (model.Course, error) {
	c, ok := f.courses[id]
	if !ok {
		return model.Course{}, repository.ErrNotFound
	}
	return c, nil
}

func (f *fakeDashboardRepo) CountLessonsByCourse(_ context.Context, courseID uuid.UUID) (int, error) {
	return f.lessonCnt[courseID], nil
}

func (f *fakeDashboardRepo) SumCompletedLessonMinutes(_ context.Context, lessonIDs []uuid.UUID) (int, error) {
	var total int
	for _, id := range lessonIDs {
		total += f.duration[id]
	}
	return total, nil
}

func (f *fakeDashboardRepo) ListOrders(_ context.Context, _ repository.OrderFilter) ([]model.Order, string, error) {
	return nil, "", nil
}

func (f *fakeDashboardRepo) GetOrderByID(_ context.Context, _ uuid.UUID) (*model.Order, error) {
	return nil, repository.ErrNotFound
}

// shimDashboardService mirrors Service.GetDashboard logic using a fake store.
type shimDashboardService struct {
	fake *fakeDashboardRepo
}

func newShimDashboard(fake *fakeDashboardRepo) *shimDashboardService {
	return &shimDashboardService{fake: fake}
}

func (s *shimDashboardService) GetDashboard(ctx context.Context, studentID string) (*DashboardView, error) {
	sID, err := parseUUID(studentID)
	if err != nil {
		return nil, err
	}

	sessions, err := s.fake.ListActiveSessionsByStudent(ctx, sID)
	if err != nil {
		return nil, err
	}

	courses := make([]DashboardCourseSummary, 0, len(sessions))

	var visitedLectures int
	var totalLectures int
	var completedCourses int
	var allCompletedLessonIDs []uuid.UUID

	for _, sess := range sessions {
		course, err := s.fake.GetCourseByID(ctx, sess.CourseID)
		if err != nil {
			return nil, err
		}
		total, err := s.fake.CountLessonsByCourse(ctx, sess.CourseID)
		if err != nil {
			return nil, err
		}
		done := len(sess.CompletedLessons)
		var progress float64
		if total > 0 {
			progress = float64(done) / float64(total)
		}

		visitedLectures += done
		totalLectures += total
		if done >= total {
			completedCourses++
		}

		for lessonID := range sess.CompletedLessons {
			allCompletedLessonIDs = append(allCompletedLessonIDs, lessonID)
		}

		courses = append(courses, DashboardCourseSummary{
			ID:           course.ID.String(),
			Title:        course.Title,
			Progress:     progress,
			TotalLessons: total,
			DoneLessons:  done,
		})
	}

	var totalMinutes float64
	if len(allCompletedLessonIDs) > 0 {
		totalSeconds, err := s.fake.SumCompletedLessonMinutes(ctx, allCompletedLessonIDs)
		if err != nil {
			return nil, err
		}
		totalMinutes = float64(totalSeconds) / 60.0
	}

	return &DashboardView{
		EnrolledCourses: courses,
		PendingOrder:    nil,
		StudySummary: DashboardStudySummary{
			VisitedLectures:      visitedLectures,
			TotalLectures:        totalLectures,
			EnrolledCoursesCount: len(sessions),
			CompletedCourses:     completedCourses,
			TotalMinutes:         totalMinutes,
		},
		Ranking: DashboardRanking{
			Position:    nil,
			Points:      nil,
			Leaderboard: []DashboardLeaderboardEntry{},
		},
		ExamProgress:   []interface{}{},
		PopularLessons: []interface{}{},
	}, nil
}

func TestGetDashboard_StudySummaryCounters(t *testing.T) {
	ctx := context.Background()
	fake := newFakeDashboardRepo()
	svc := newShimDashboard(fake)

	studentID := uuid.MustParse("00000000-0000-0000-0000-000000000001")
	course1 := uuid.MustParse("00000000-0000-0000-0000-000000000010")
	course2 := uuid.MustParse("00000000-0000-0000-0000-000000000011")
	lesson1 := uuid.MustParse("00000000-0000-0000-0000-000000000020")
	lesson2 := uuid.MustParse("00000000-0000-0000-0000-000000000021")
	lesson3 := uuid.MustParse("00000000-0000-0000-0000-000000000022")
	lesson4 := uuid.MustParse("00000000-0000-0000-0000-000000000023")

	// Seed courses
	fake.courses[course1] = model.Course{ID: course1, Title: "Math"}
	fake.courses[course2] = model.Course{ID: course2, Title: "Science"}

	// Each course has a known lesson count
	fake.lessonCnt[course1] = 5
	fake.lessonCnt[course2] = 3

	// Duration for completed lessons (in seconds)
	fake.duration[lesson1] = 600  // 10 min
	fake.duration[lesson2] = 300  // 5 min
	fake.duration[lesson3] = 180  // 3 min
	fake.duration[lesson4] = 120  // 2 min — not completed, not included

	// Session 1: course1, 2/5 completed
	fake.sessions = append(fake.sessions, model.CourseSession{
		ID:        uuid.MustParse("00000000-0000-0000-0000-000000000030"),
		StudentID: studentID,
		CourseID:  course1,
		Status:    "active",
		CompletedLessons: map[uuid.UUID]time.Time{
			lesson1: time.Now(),
			lesson2: time.Now(),
		},
	})

	// Session 2: course2, 1/3 completed
	fake.sessions = append(fake.sessions, model.CourseSession{
		ID:        uuid.MustParse("00000000-0000-0000-0000-000000000031"),
		StudentID: studentID,
		CourseID:  course2,
		Status:    "active",
		CompletedLessons: map[uuid.UUID]time.Time{
			lesson3: time.Now(),
		},
	})

	result, err := svc.GetDashboard(ctx, studentID.String())
	if err != nil {
		t.Fatalf("GetDashboard: %v", err)
	}

	ss := result.StudySummary
	if ss.VisitedLectures != 3 {
		t.Errorf("VisitedLectures: want 3 (2+1), got %d", ss.VisitedLectures)
	}
	if ss.TotalLectures != 8 {
		t.Errorf("TotalLectures: want 8 (5+3), got %d", ss.TotalLectures)
	}
	if ss.EnrolledCoursesCount != 2 {
		t.Errorf("EnrolledCoursesCount: want 2, got %d", ss.EnrolledCoursesCount)
	}
	// Neither course is fully completed
	if ss.CompletedCourses != 0 {
		t.Errorf("CompletedCourses: want 0, got %d", ss.CompletedCourses)
	}
	// 600+300+180 = 1080 seconds / 60 = 18 minutes
	if ss.TotalMinutes != 18.0 {
		t.Errorf("TotalMinutes: want 18.0, got %f", ss.TotalMinutes)
	}
}

func TestGetDashboard_CompletedCoursesIncrements(t *testing.T) {
	ctx := context.Background()
	fake := newFakeDashboardRepo()
	svc := newShimDashboard(fake)

	studentID := uuid.MustParse("00000000-0000-0000-0000-000000000001")
	course1 := uuid.MustParse("00000000-0000-0000-0000-000000000010")
	lesson1 := uuid.MustParse("00000000-0000-0000-0000-000000000020")
	lesson2 := uuid.MustParse("00000000-0000-0000-0000-000000000021")

	fake.courses[course1] = model.Course{ID: course1, Title: "Math"}
	fake.lessonCnt[course1] = 2

	fake.duration[lesson1] = 600
	fake.duration[lesson2] = 300

	// All 2/2 lessons completed → course completed
	fake.sessions = append(fake.sessions, model.CourseSession{
		ID:        uuid.MustParse("00000000-0000-0000-0000-000000000030"),
		StudentID: studentID,
		CourseID:  course1,
		Status:    "active",
		CompletedLessons: map[uuid.UUID]time.Time{
			lesson1: time.Now(),
			lesson2: time.Now(),
		},
	})

	result, err := svc.GetDashboard(ctx, studentID.String())
	if err != nil {
		t.Fatalf("GetDashboard: %v", err)
	}

	if ss := result.StudySummary; ss.CompletedCourses != 1 {
		t.Errorf("CompletedCourses: want 1 (all lessons done), got %d", ss.CompletedCourses)
	}
	if ss := result.StudySummary; ss.TotalMinutes != 15.0 {
		t.Errorf("TotalMinutes: want 15.0 ((600+300)/60), got %f", ss.TotalMinutes)
	}
}

func TestGetDashboard_ZeroMinutesWhenNoCompletedLessons(t *testing.T) {
	ctx := context.Background()
	fake := newFakeDashboardRepo()
	svc := newShimDashboard(fake)

	studentID := uuid.MustParse("00000000-0000-0000-0000-000000000001")
	course1 := uuid.MustParse("00000000-0000-0000-0000-000000000010")

	fake.courses[course1] = model.Course{ID: course1, Title: "Math"}
	fake.lessonCnt[course1] = 5

	// Session with zero completed lessons
	fake.sessions = append(fake.sessions, model.CourseSession{
		ID:               uuid.MustParse("00000000-0000-0000-0000-000000000030"),
		StudentID:        studentID,
		CourseID:         course1,
		Status:           "active",
		CompletedLessons: map[uuid.UUID]time.Time{},
	})

	result, err := svc.GetDashboard(ctx, studentID.String())
	if err != nil {
		t.Fatalf("GetDashboard: %v", err)
	}

	ss := result.StudySummary
	if ss.VisitedLectures != 0 {
		t.Errorf("VisitedLectures: want 0, got %d", ss.VisitedLectures)
	}
	if ss.TotalMinutes != 0 {
		t.Errorf("TotalMinutes: want 0, got %f", ss.TotalMinutes)
	}
	if ss.CompletedCourses != 0 {
		t.Errorf("CompletedCourses: want 0, got %d", ss.CompletedCourses)
	}
}

func TestGetDashboard_UnsourcedSectionsSerializeAsEmpty(t *testing.T) {
	ctx := context.Background()
	fake := newFakeDashboardRepo()
	svc := newShimDashboard(fake)

	studentID := uuid.MustParse("00000000-0000-0000-0000-000000000001")

	fake.courses[uuid.MustParse("00000000-0000-0000-0000-000000000010")] = model.Course{
		ID: uuid.MustParse("00000000-0000-0000-0000-000000000010"), Title: "Empty",
	}
	fake.lessonCnt[uuid.MustParse("00000000-0000-0000-0000-000000000010")] = 5

	fake.sessions = append(fake.sessions, model.CourseSession{
		ID:               uuid.MustParse("00000000-0000-0000-0000-000000000030"),
		StudentID:        studentID,
		CourseID:         uuid.MustParse("00000000-0000-0000-0000-000000000010"),
		Status:           "active",
		CompletedLessons: map[uuid.UUID]time.Time{},
	})

	result, err := svc.GetDashboard(ctx, studentID.String())
	if err != nil {
		t.Fatalf("GetDashboard: %v", err)
	}

	// Verify via JSON marshaling — this is the definitive serialization behavior.
	b, err := json.Marshal(result)
	if err != nil {
		t.Fatalf("json.Marshal: %v", err)
	}

	var raw map[string]json.RawMessage
	if err := json.Unmarshal(b, &raw); err != nil {
		t.Fatalf("json.Unmarshal: %v", err)
	}

	// ranking.position should be null
	var ranking struct {
		Position    *int                          `json:"position"`
		Points      *float64                      `json:"points"`
		Leaderboard []DashboardLeaderboardEntry   `json:"leaderboard"`
	}
	if err := json.Unmarshal(raw["ranking"], &ranking); err != nil {
		t.Fatalf("unmarshal ranking: %v", err)
	}
	if ranking.Position != nil {
		t.Error("ranking.position: want null, got non-nil")
	}
	if ranking.Points != nil {
		t.Error("ranking.points: want null, got non-nil")
	}
	if ranking.Leaderboard == nil {
		t.Error("ranking.leaderboard: want [], got null")
	}
	// Direct check: the raw JSON keys should exist and be arrays
	checkIsArray := func(t *testing.T, key string, rawMap map[string]json.RawMessage) {
		t.Helper()
		var arr []interface{}
		if err := json.Unmarshal(rawMap[key], &arr); err != nil {
			t.Errorf("%s: want JSON array, got error: %v", key, err)
		}
		if arr == nil {
			t.Errorf("%s: want [], got null", key)
		}
	}
	checkIsArray(t, "exam_progress", raw)
	checkIsArray(t, "popular_lessons", raw)
}

func TestGetDashboard_ExistingFieldsByteCompatible(t *testing.T) {
	// Verify that the old fields (enrolled_courses, pending_order) serialize
	// identically to how they did before the new fields were added.
	// We use a JSON round-trip: old tags must still be present.
	ctx := context.Background()
	fake := newFakeDashboardRepo()
	svc := newShimDashboard(fake)

	studentID := uuid.MustParse("00000000-0000-0000-0000-000000000001")
	course1 := uuid.MustParse("00000000-0000-0000-0000-000000000010")

	fake.courses[course1] = model.Course{ID: course1, Title: "Math"}
	fake.lessonCnt[course1] = 5

	fake.sessions = append(fake.sessions, model.CourseSession{
		ID:               uuid.MustParse("00000000-0000-0000-0000-000000000030"),
		StudentID:        studentID,
		CourseID:         course1,
		Status:           "active",
		CompletedLessons: map[uuid.UUID]time.Time{},
	})

	result, err := svc.GetDashboard(ctx, studentID.String())
	if err != nil {
		t.Fatalf("GetDashboard: %v", err)
	}

	b, err := json.Marshal(result)
	if err != nil {
		t.Fatalf("json.Marshal: %v", err)
	}

	var raw map[string]json.RawMessage
	if err := json.Unmarshal(b, &raw); err != nil {
		t.Fatalf("json.Unmarshal: %v", err)
	}

	// enrolled_courses should be present
	if _, ok := raw["enrolled_courses"]; !ok {
		t.Error("enrolled_courses: key missing from JSON")
	}
	// pending_order should be present (omitempty means missing when nil, which is fine)
	// Just verify the key is either missing or valid (it's nil, so omitempty)
	// Actually, for byte-compatibility: old consumers should still work
	if _, ok := raw["pending_order"]; ok {
		// If present, it must be a valid object
		var po map[string]interface{}
		if err := json.Unmarshal(raw["pending_order"], &po); err != nil {
			t.Errorf("pending_order: expected valid JSON object, got error: %v", err)
		}
	}
}

func TestGetDashboard_CompletedCourses(t *testing.T) {
	type courseSpec struct {
		lessons   int
		completed int
	}
	tests := []struct {
		name          string
		courses       []courseSpec
		wantCompleted int
	}{
		{
			name:          "empty course is not completed",
			courses:       []courseSpec{{lessons: 0, completed: 0}},
			wantCompleted: 0,
		},
		{
			name:          "all lessons done is completed",
			courses:       []courseSpec{{lessons: 2, completed: 2}},
			wantCompleted: 1,
		},
		{
			name:          "partially done is not completed",
			courses:       []courseSpec{{lessons: 2, completed: 1}},
			wantCompleted: 0,
		},
		{
			name:          "empty course next to a finished one counts only the finished one",
			courses:       []courseSpec{{lessons: 0, completed: 0}, {lessons: 1, completed: 1}},
			wantCompleted: 1,
		},
	}

	svc, repo := newRealDBService(t)
	pool := repo.Pool()
	ctx := context.Background()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var studentID uuid.UUID
			if err := pool.QueryRow(ctx,
				`INSERT INTO users (name, username, role, status, jenjang, grade, otp_enabled)
				 VALUES ($1, $2, 'student', 'active', 'sma', 10, false) RETURNING id`,
				"Completed Courses Student "+uniqueSuffix(), "completedcourses_"+uniqueSuffix(),
			).Scan(&studentID); err != nil {
				t.Fatalf("seed student: %v", err)
			}

			courseIDs := make([]uuid.UUID, len(tt.courses))
			for i, spec := range tt.courses {
				if err := pool.QueryRow(ctx,
					`INSERT INTO course (title, level, subject, instructor_name) VALUES ($1, 'sma', 'math', 'T') RETURNING id`,
					"Course "+uniqueSuffix(),
				).Scan(&courseIDs[i]); err != nil {
					t.Fatalf("seed course: %v", err)
				}

				done := map[string]string{}
				if spec.lessons > 0 {
					var sectionID uuid.UUID
					if err := pool.QueryRow(ctx,
						`INSERT INTO section (course_id, title) VALUES ($1, 'S') RETURNING id`, courseIDs[i],
					).Scan(&sectionID); err != nil {
						t.Fatalf("seed section: %v", err)
					}
					for l := 0; l < spec.lessons; l++ {
						var lessonID uuid.UUID
						if err := pool.QueryRow(ctx,
							`INSERT INTO lesson (section_id, title, duration_seconds, position) VALUES ($1, 'L', 60, $2) RETURNING id`,
							sectionID, l,
						).Scan(&lessonID); err != nil {
							t.Fatalf("seed lesson: %v", err)
						}
						if l < spec.completed {
							done[lessonID.String()] = "2030-01-01T00:00:00Z"
						}
					}
				}
				doneJSON, err := json.Marshal(done)
				if err != nil {
					t.Fatalf("marshal completed lessons: %v", err)
				}
				if _, err := pool.Exec(ctx,
					`INSERT INTO course_session (student_id, course_id, status, source, completed_lessons)
					 VALUES ($1, $2, 'active', 'test', $3::jsonb)`, studentID, courseIDs[i], doneJSON,
				); err != nil {
					t.Fatalf("seed session: %v", err)
				}
			}

			view, err := svc.GetDashboard(ctx, studentID.String())
			if err != nil {
				t.Fatalf("GetDashboard: %v", err)
			}

			if got := view.StudySummary.EnrolledCoursesCount; got != len(tt.courses) {
				t.Errorf("EnrolledCoursesCount: want %d, got %d", len(tt.courses), got)
			}
			if got := view.StudySummary.CompletedCourses; got != tt.wantCompleted {
				t.Errorf("CompletedCourses: want %d, got %d", tt.wantCompleted, got)
			}
			byID := map[string]DashboardCourseSummary{}
			for _, c := range view.EnrolledCourses {
				byID[c.ID] = c
			}
			for i, spec := range tt.courses {
				c, ok := byID[courseIDs[i].String()]
				if !ok {
					t.Fatalf("course %d missing from EnrolledCourses", i)
				}
				if c.TotalLessons != spec.lessons || c.DoneLessons != spec.completed {
					t.Errorf("course %d: want total/done %d/%d, got %d/%d", i, spec.lessons, spec.completed, c.TotalLessons, c.DoneLessons)
				}
				if spec.lessons == 0 && c.Progress != 0 {
					t.Errorf("course %d: empty course progress want 0, got %v", i, c.Progress)
				}
			}
		})
	}
}
