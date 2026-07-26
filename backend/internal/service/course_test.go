package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	"akademi-bimbel/internal/model"
	"akademi-bimbel/internal/repository"
)

// fakeCourseRepo stubs course repository methods.
type fakeCourseRepo struct {
	sections  map[string]*model.Section
	lessons   map[string]*model.Lesson
	courses   map[string]*model.Course
	pcLinks   map[string][]uuid.UUID // productID -> courseIDs
	sessions  map[string]*model.CourseSession
	seqSec    int
	seqLes    int
	seqCourse int
	seqSess   int
}

func newFakeCourseRepo() *fakeCourseRepo {
	return &fakeCourseRepo{
		sections: make(map[string]*model.Section),
		lessons:  make(map[string]*model.Lesson),
		courses:  make(map[string]*model.Course),
		pcLinks:  make(map[string][]uuid.UUID),
		sessions: make(map[string]*model.CourseSession),
	}
}

// --- Course CRUD fakes ---

func (f *fakeCourseRepo) CreateCourse(_ context.Context, c model.Course) (model.Course, error) {
	f.seqCourse++
	c.ID = uuid.New()
	c.CreatedAt = time.Now()
	c.UpdatedAt = time.Now()
	f.courses[c.ID.String()] = &c
	return c, nil
}

func (f *fakeCourseRepo) ListCourses(_ context.Context) ([]model.Course, error) {
	var out []model.Course
	for _, c := range f.courses {
		out = append(out, *c)
	}
	return out, nil
}

func (f *fakeCourseRepo) GetCourseByID(_ context.Context, id uuid.UUID) (model.Course, error) {
	c, ok := f.courses[id.String()]
	if !ok {
		return model.Course{}, repository.ErrNotFound
	}
	return *c, nil
}

func (f *fakeCourseRepo) DeleteCourse(_ context.Context, id uuid.UUID) error {
	delete(f.courses, id.String())
	return nil
}

func (f *fakeCourseRepo) UpdateCourse(_ context.Context, id uuid.UUID, c model.Course) (model.Course, error) {
	existing, ok := f.courses[id.String()]
	if !ok {
		return model.Course{}, repository.ErrNotFound
	}
	existing.Title = c.Title
	existing.Level = c.Level
	existing.Subject = c.Subject
	existing.InstructorName = c.InstructorName
	existing.UpdatedAt = time.Now()
	return *existing, nil
}

// --- Product-Course link fakes ---

func (f *fakeCourseRepo) GetCoursesByProductID(_ context.Context, productID uuid.UUID) ([]model.Course, error) {
	courseIDs, ok := f.pcLinks[productID.String()]
	if !ok || len(courseIDs) == 0 {
		return nil, nil
	}
	var out []model.Course
	for _, cid := range courseIDs {
		if c, exists := f.courses[cid.String()]; exists {
			out = append(out, *c)
		}
	}
	return out, nil
}

// --- Course session fakes ---

func (f *fakeCourseRepo) GetActiveSession(_ context.Context, studentID, courseID uuid.UUID) (model.CourseSession, error) {
	for _, sess := range f.sessions {
		if sess.StudentID == studentID && sess.CourseID == courseID && sess.Status == "active" {
			return *sess, nil
		}
	}
	return model.CourseSession{}, repository.ErrNotFound
}

func (f *fakeCourseRepo) MarkLessonComplete(_ context.Context, sessionID, lessonID uuid.UUID, at time.Time) error {
	sess, ok := f.sessions[sessionID.String()]
	if !ok {
		return repository.ErrNotFound
	}
	// first timestamp wins
	if _, exists := sess.CompletedLessons[lessonID]; !exists {
		if sess.CompletedLessons == nil {
			sess.CompletedLessons = make(map[uuid.UUID]time.Time)
		}
		sess.CompletedLessons[lessonID] = at
	}
	return nil
}

func (f *fakeCourseRepo) CountLessonsByCourse(_ context.Context, courseID uuid.UUID) (int, error) {
	var count int
	for _, l := range f.lessons {
		sec, ok := f.sections[l.SectionID.String()]
		if !ok {
			continue
		}
		if sec.CourseID == courseID {
			count++
		}
	}
	return count, nil
}

func (f *fakeCourseRepo) ListActiveSessionsByStudent(_ context.Context, studentID uuid.UUID) ([]model.CourseSession, error) {
	var result []model.CourseSession
	for _, sess := range f.sessions {
		if sess.StudentID == studentID && sess.Status == "active" {
			result = append(result, *sess)
		}
	}
	return result, nil
}

func (f *fakeCourseRepo) seedCourseSession(studentID, courseID uuid.UUID) *model.CourseSession {
	f.seqSess++
	sess := model.CourseSession{
		ID:               uuid.New(),
		StudentID:        studentID,
		CourseID:         courseID,
		Status:           "active",
		Source:           "order",
		EnrolledAt:       time.Now(),
		CompletedLessons: make(map[uuid.UUID]time.Time),
	}
	f.sessions[sess.ID.String()] = &sess
	return &sess
}

// --- Section CRUD fakes (keyed by course_id) ---

func (f *fakeCourseRepo) ListSections(_ context.Context, courseID uuid.UUID) ([]model.Section, error) {
	var result []model.Section
	for _, sec := range f.sections {
		if sec.CourseID == courseID {
			result = append(result, *sec)
		}
	}
	return result, nil
}

func (f *fakeCourseRepo) CreateSection(_ context.Context, s model.Section) (model.Section, error) {
	f.seqSec++
	s.ID = uuid.New()
	s.CreatedAt = time.Now()
	f.sections[s.ID.String()] = &s
	return s, nil
}

func (f *fakeCourseRepo) UpdateSection(_ context.Context, id uuid.UUID, title string) (model.Section, error) {
	sec, ok := f.sections[id.String()]
	if !ok {
		return model.Section{}, repository.ErrNotFound
	}
	sec.Title = title
	return *sec, nil
}

func (f *fakeCourseRepo) DeleteSection(_ context.Context, id uuid.UUID) error {
	delete(f.sections, id.String())
	return nil
}

func (f *fakeCourseRepo) ReorderSections(_ context.Context, courseID uuid.UUID, orderedIDs []uuid.UUID) error {
	for i, id := range orderedIDs {
		sec, ok := f.sections[id.String()]
		if !ok || sec.CourseID != courseID {
			return repository.ErrNotFound
		}
		sec.Position = i
	}
	return nil
}

// --- Lesson CRUD fakes ---

func (f *fakeCourseRepo) CreateLesson(_ context.Context, l model.Lesson) (model.Lesson, error) {
	f.seqLes++
	l.ID = uuid.New()
	l.CreatedAt = time.Now()
	f.lessons[l.ID.String()] = &l
	return l, nil
}

func (f *fakeCourseRepo) UpdateLesson(_ context.Context, id uuid.UUID, l model.Lesson) (model.Lesson, error) {
	lesson, ok := f.lessons[id.String()]
	if !ok {
		return model.Lesson{}, repository.ErrNotFound
	}
	lesson.Title = l.Title
	lesson.VideoURL = l.VideoURL
	lesson.DurationSeconds = l.DurationSeconds
	return *lesson, nil
}

func (f *fakeCourseRepo) DeleteLesson(_ context.Context, id uuid.UUID) error {
	delete(f.lessons, id.String())
	return nil
}

func (f *fakeCourseRepo) ReorderLessons(_ context.Context, sectionID uuid.UUID, orderedIDs []uuid.UUID) error {
	for i, id := range orderedIDs {
		lesson, ok := f.lessons[id.String()]
		if !ok || lesson.SectionID != sectionID {
			return repository.ErrNotFound
		}
		lesson.Position = i
	}
	return nil
}

func (f *fakeCourseRepo) ListLessonsBySection(_ context.Context, sectionID uuid.UUID) ([]model.Lesson, error) {
	var result []model.Lesson
	for _, lesson := range f.lessons {
		if lesson.SectionID == sectionID {
			result = append(result, *lesson)
		}
	}
	return result, nil
}

// shimCourseService implements course methods via fake repo.
type shimCourseService struct {
	fake *fakeCourseRepo
}

// --- Course CRUD shim ---

func (s *shimCourseService) CreateCourse(ctx context.Context, title, level, subject, instructorName, role string) (model.Course, error) {
	if role != RoleAdminStore && role != RoleSuperAdmin {
		return model.Course{}, ErrForbidden
	}
	c := model.Course{
		Title:          title,
		Level:          level,
		Subject:        subject,
		InstructorName: instructorName,
	}
	return s.fake.CreateCourse(ctx, c)
}

func (s *shimCourseService) ListCourses(ctx context.Context, role string) ([]model.Course, error) {
	return s.fake.ListCourses(ctx)
}

func (s *shimCourseService) UpdateCourse(ctx context.Context, id, title, level, subject, instructorName, role string) (model.Course, error) {
	if role != RoleAdminStore && role != RoleSuperAdmin {
		return model.Course{}, ErrForbidden
	}
	courseID, err := uuid.Parse(id)
	if err != nil {
		return model.Course{}, err
	}

	// Fetch existing to preserve fields not sent by partial PATCH.
	existing, err := s.fake.GetCourseByID(ctx, courseID)
	if err != nil {
		return model.Course{}, err
	}

	c := model.Course{
		Title:          title,
		Level:          level,
		Subject:        subject,
		InstructorName: instructorName,
	}
	// Preserve existing values for any field not supplied (zero-value).
	if c.Level == "" {
		c.Level = existing.Level
	}
	if c.Subject == "" {
		c.Subject = existing.Subject
	}
	if c.InstructorName == "" {
		c.InstructorName = existing.InstructorName
	}

	return s.fake.UpdateCourse(ctx, courseID, c)
}

// --- Section shim (keyed by course_id) ---

func (s *shimCourseService) ListSections(ctx context.Context, courseID string) ([]model.Section, error) {
	cID, err := uuid.Parse(courseID)
	if err != nil {
		return nil, err
	}
	return s.fake.ListSections(ctx, cID)
}

func (s *shimCourseService) CreateSection(ctx context.Context, courseID string, title string, role string) (model.Section, error) {
	if role != RoleAdminStore && role != RoleSuperAdmin {
		return model.Section{}, ErrForbidden
	}

	cID, err := uuid.Parse(courseID)
	if err != nil {
		return model.Section{}, err
	}

	sections, err := s.fake.ListSections(ctx, cID)
	if err != nil {
		return model.Section{}, err
	}

	position := len(sections)
	sec := model.Section{
		CourseID: cID,
		Title:    title,
		Position: position,
	}
	return s.fake.CreateSection(ctx, sec)
}

func (s *shimCourseService) UpdateSection(ctx context.Context, courseID, sectionID string, title string, role string) (model.Section, error) {
	if role != RoleAdminStore && role != RoleSuperAdmin {
		return model.Section{}, ErrForbidden
	}

	sID, err := uuid.Parse(sectionID)
	if err != nil {
		return model.Section{}, err
	}

	return s.fake.UpdateSection(ctx, sID, title)
}

func (s *shimCourseService) DeleteSection(ctx context.Context, courseID, sectionID string, role string) error {
	if role != RoleAdminStore && role != RoleSuperAdmin {
		return ErrForbidden
	}

	sID, err := uuid.Parse(sectionID)
	if err != nil {
		return err
	}

	return s.fake.DeleteSection(ctx, sID)
}

func (s *shimCourseService) ReorderSections(ctx context.Context, courseID string, orderedIDs []string, role string) error {
	if role != RoleAdminStore && role != RoleSuperAdmin {
		return ErrForbidden
	}

	cID, err := uuid.Parse(courseID)
	if err != nil {
		return err
	}

	var ids []uuid.UUID
	for _, id := range orderedIDs {
		parsed, err := uuid.Parse(id)
		if err != nil {
			return err
		}
		ids = append(ids, parsed)
	}

	return s.fake.ReorderSections(ctx, cID, ids)
}

// --- Lesson shim (keyed by course_id) ---

func (s *shimCourseService) CreateLesson(ctx context.Context, courseID, sectionID string, title, videoURL string, duration int, role string) (model.Lesson, error) {
	if role != RoleAdminStore && role != RoleSuperAdmin {
		return model.Lesson{}, ErrForbidden
	}

	sID, err := uuid.Parse(sectionID)
	if err != nil {
		return model.Lesson{}, err
	}

	lessons, err := s.fake.ListLessonsBySection(ctx, sID)
	if err != nil {
		return model.Lesson{}, err
	}

	position := len(lessons)
	lesson := model.Lesson{
		SectionID:       sID,
		Title:           title,
		VideoURL:        videoURL,
		DurationSeconds: duration,
		Position:        position,
	}
	return s.fake.CreateLesson(ctx, lesson)
}

func (s *shimCourseService) UpdateLesson(ctx context.Context, courseID, sectionID, lessonID string, title, videoURL string, duration int, role string) (model.Lesson, error) {
	if role != RoleAdminStore && role != RoleSuperAdmin {
		return model.Lesson{}, ErrForbidden
	}

	lID, err := uuid.Parse(lessonID)
	if err != nil {
		return model.Lesson{}, err
	}

	lesson := model.Lesson{
		Title:           title,
		VideoURL:        videoURL,
		DurationSeconds: duration,
	}
	return s.fake.UpdateLesson(ctx, lID, lesson)
}

func (s *shimCourseService) DeleteLesson(ctx context.Context, courseID, sectionID, lessonID string, role string) error {
	if role != RoleAdminStore && role != RoleSuperAdmin {
		return ErrForbidden
	}

	lID, err := uuid.Parse(lessonID)
	if err != nil {
		return err
	}

	return s.fake.DeleteLesson(ctx, lID)
}

func (s *shimCourseService) ReorderLessons(ctx context.Context, courseID, sectionID string, orderedIDs []string, role string) error {
	if role != RoleAdminStore && role != RoleSuperAdmin {
		return ErrForbidden
	}

	sID, err := uuid.Parse(sectionID)
	if err != nil {
		return err
	}

	var ids []uuid.UUID
	for _, id := range orderedIDs {
		parsed, err := uuid.Parse(id)
		if err != nil {
			return err
		}
		ids = append(ids, parsed)
	}

	return s.fake.ReorderLessons(ctx, sID, ids)
}

// --- Student-facing course shim ---

func (s *shimCourseService) MarkLessonComplete(ctx context.Context, studentID, courseID, lessonID string) error {
	sID, err := uuid.Parse(studentID)
	if err != nil {
		return err
	}
	cID, err := uuid.Parse(courseID)
	if err != nil {
		return err
	}
	lID, err := uuid.Parse(lessonID)
	if err != nil {
		return err
	}

	session, err := s.fake.GetActiveSession(ctx, sID, cID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return ErrNoCourseAccess
		}
		return err
	}

	return s.fake.MarkLessonComplete(ctx, session.ID, lID, time.Now())
}

func (s *shimCourseService) CourseProgress(ctx context.Context, studentID, courseID string) (int, int, float64, error) {
	sID, err := uuid.Parse(studentID)
	if err != nil {
		return 0, 0, 0, err
	}
	cID, err := uuid.Parse(courseID)
	if err != nil {
		return 0, 0, 0, err
	}

	session, err := s.fake.GetActiveSession(ctx, sID, cID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return 0, 0, 0, ErrNoCourseAccess
		}
		return 0, 0, 0, err
	}

	completed := len(session.CompletedLessons)
	total, err := s.fake.CountLessonsByCourse(ctx, cID)
	if err != nil {
		return 0, 0, 0, err
	}
	return completed, total, courseProgressPct(completed, total), nil
}

func (s *shimCourseService) ListLibrary(ctx context.Context, studentID string) ([]model.CourseSession, error) {
	sID, err := uuid.Parse(studentID)
	if err != nil {
		return nil, err
	}
	return s.fake.ListActiveSessionsByStudent(ctx, sID)
}

// shimCourseServiceV2 extends shimCourseService with new methods.
// Uses embedded fake to avoid duplicating all methods.
type shimGetDeleteCourse struct {
	fake *fakeCourseRepo
}

func (s *shimGetDeleteCourse) GetCourse(ctx context.Context, id string) (model.Course, int, int, error) {
	cID, err := uuid.Parse(id)
	if err != nil {
		return model.Course{}, 0, 0, err
	}
	c, err := s.fake.GetCourseByID(ctx, cID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return model.Course{}, 0, 0, ErrCourseNotFound
		}
		return model.Course{}, 0, 0, err
	}
	sections, err := s.fake.ListSections(ctx, cID)
	if err != nil {
		return model.Course{}, 0, 0, err
	}
	total, err := s.fake.CountLessonsByCourse(ctx, cID)
	if err != nil {
		return model.Course{}, 0, 0, err
	}
	return c, len(sections), total, nil
}

func (s *shimGetDeleteCourse) DeleteCourse(ctx context.Context, id, role string) error {
	if role != RoleAdminStore && role != RoleSuperAdmin {
		return ErrForbidden
	}
	cID, err := uuid.Parse(id)
	if err != nil {
		return err
	}
	_, err = s.fake.GetCourseByID(ctx, cID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return ErrCourseNotFound
		}
		return err
	}
	return s.fake.DeleteCourse(ctx, cID)
}

// shimStudentCourse supports GetCourseWithProgress via fake.
type shimStudentCourse struct {
	fake *fakeCourseRepo
}

func (s *shimStudentCourse) GetCourseWithProgress(ctx context.Context, studentID, courseID string) (CourseWithProgress, error) {
	sID, err := uuid.Parse(studentID)
	if err != nil {
		return CourseWithProgress{}, err
	}
	cID, err := uuid.Parse(courseID)
	if err != nil {
		return CourseWithProgress{}, err
	}

	session, err := s.fake.GetActiveSession(ctx, sID, cID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return CourseWithProgress{}, ErrNoCourseAccess
		}
		return CourseWithProgress{}, err
	}

	course, err := s.fake.GetCourseByID(ctx, cID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return CourseWithProgress{}, ErrCourseNotFound
		}
		return CourseWithProgress{}, err
	}

	sections, err := s.fake.ListSections(ctx, cID)
	if err != nil {
		return CourseWithProgress{}, err
	}

	var sectionsWithLessons []SectionWithLessons
	for _, sec := range sections {
		lessons, err := s.fake.ListLessonsBySection(ctx, sec.ID)
		if err != nil {
			return CourseWithProgress{}, err
		}
		var lessonsWithCompletion []LessonWithCompletion
		for _, l := range lessons {
			_, completed := session.CompletedLessons[l.ID]
			lessonsWithCompletion = append(lessonsWithCompletion, LessonWithCompletion{
				Lesson:    l,
				Completed: completed,
			})
		}
		sectionsWithLessons = append(sectionsWithLessons, SectionWithLessons{
			Section: sec,
			Lessons: lessonsWithCompletion,
		})
	}

	return CourseWithProgress{
		Course:   course,
		Sections: sectionsWithLessons,
	}, nil
}

// --- Tests ---

// Test: CreateCourse allows super_admin
func TestCreateCourse_SuperAdminCanCreate(t *testing.T) {
	ctx := context.Background()
	fake := newFakeCourseRepo()
	svc := &shimCourseService{fake: fake}

	course, err := svc.CreateCourse(ctx, "Math", "beginner", "math", "Mr. A", RoleSuperAdmin)
	if err != nil {
		t.Fatalf("super_admin CreateCourse: %v", err)
	}
	if course.Title != "Math" {
		t.Errorf("want title Math, got %s", course.Title)
	}
}

// Test: CreateCourse rejects non-store role
func TestCreateCourse_RejectsNonStoreRole(t *testing.T) {
	ctx := context.Background()
	fake := newFakeCourseRepo()
	svc := &shimCourseService{fake: fake}

	_, err := svc.CreateCourse(ctx, "Math", "beginner", "math", "Mr. A", RoleAdminExam)
	if !errors.Is(err, ErrForbidden) {
		t.Errorf("want ErrForbidden for non-store role, got %v", err)
	}

	// admin_store should succeed
	course, err := svc.CreateCourse(ctx, "Math", "beginner", "math", "Mr. A", RoleAdminStore)
	if err != nil {
		t.Fatalf("admin_store CreateCourse: %v", err)
	}
	if course.Title != "Math" {
		t.Errorf("want title Math, got %s", course.Title)
	}
}

// UpdateCourse preserve-fields regression: see
// integration/TestUpdateCourse_PreservesTitleAndFields_RealService (real service +
// Postgres; the shim-based test here was tautological and did not cover title).

// Test: UpdateCourse rejects non-store role
func TestUpdateCourse_RejectsNonStoreRole(t *testing.T) {
	ctx := context.Background()
	fake := newFakeCourseRepo()
	svc := &shimCourseService{fake: fake}

	course, err := svc.CreateCourse(ctx, "Math", "beginner", "math", "Mr. A", RoleAdminStore)
	if err != nil {
		t.Fatalf("CreateCourse: %v", err)
	}

	_, err = svc.UpdateCourse(ctx, course.ID.String(), "Updated", "advanced", "science", "Mr. B", RoleAdminExam)
	if !errors.Is(err, ErrForbidden) {
		t.Errorf("want ErrForbidden for non-store role, got %v", err)
	}

	updated, err := svc.UpdateCourse(ctx, course.ID.String(), "Updated", "advanced", "science", "Mr. B", RoleAdminStore)
	if err != nil {
		t.Fatalf("UpdateCourse: %v", err)
	}
	if updated.Title != "Updated" {
		t.Errorf("want title Updated, got %s", updated.Title)
	}
}

// Test: ListCourses returns all courses
func TestListCourses_ReturnsAll(t *testing.T) {
	ctx := context.Background()
	fake := newFakeCourseRepo()
	svc := &shimCourseService{fake: fake}

	svc.CreateCourse(ctx, "Math", "beginner", "math", "Mr. A", RoleAdminStore)
	svc.CreateCourse(ctx, "Science", "advanced", "science", "Mr. B", RoleAdminStore)

	courses, err := svc.ListCourses(ctx, RoleAdminStore)
	if err != nil {
		t.Fatalf("ListCourses: %v", err)
	}
	if len(courses) != 2 {
		t.Errorf("want 2 courses, got %d", len(courses))
	}
}

// Test: Section CRUD is keyed by course_id (not product_id)
func TestSection_KeyedByCourseID(t *testing.T) {
	ctx := context.Background()
	fake := newFakeCourseRepo()
	svc := &shimCourseService{fake: fake}

	// Create two courses
	course1, err := svc.CreateCourse(ctx, "Math", "beginner", "math", "Mr. A", RoleAdminStore)
	if err != nil {
		t.Fatalf("CreateCourse 1: %v", err)
	}
	course2, err := svc.CreateCourse(ctx, "Science", "advanced", "science", "Mr. B", RoleAdminStore)
	if err != nil {
		t.Fatalf("CreateCourse 2: %v", err)
	}

	// Create sections under course1
	sec1, err := svc.CreateSection(ctx, course1.ID.String(), "Intro", RoleAdminStore)
	if err != nil {
		t.Fatalf("CreateSection 1: %v", err)
	}
	sec2, err := svc.CreateSection(ctx, course1.ID.String(), "Basics", RoleAdminStore)
	if err != nil {
		t.Fatalf("CreateSection 2: %v", err)
	}

	// Create a section under course2
	_, err = svc.CreateSection(ctx, course2.ID.String(), "Overview", RoleAdminStore)
	if err != nil {
		t.Fatalf("CreateSection 3: %v", err)
	}

	// course1 should have 2 sections
	sections1, err := svc.ListSections(ctx, course1.ID.String())
	if err != nil {
		t.Fatalf("ListSections course1: %v", err)
	}
	if len(sections1) != 2 {
		t.Errorf("course1: want 2 sections, got %d", len(sections1))
	}

	// course2 should have 1 section
	sections2, err := svc.ListSections(ctx, course2.ID.String())
	if err != nil {
		t.Fatalf("ListSections course2: %v", err)
	}
	if len(sections2) != 1 {
		t.Errorf("course2: want 1 section, got %d", len(sections2))
	}

	// Update section under course1
	_, err = svc.UpdateSection(ctx, course1.ID.String(), sec1.ID.String(), "Updated Intro", RoleAdminStore)
	if err != nil {
		t.Fatalf("UpdateSection: %v", err)
	}

	// Verify update took effect
	updated, err := svc.fake.UpdateSection(ctx, sec1.ID, "Updated Intro")
	if err != nil {
		t.Fatalf("fake UpdateSection: %v", err)
	}
	if updated.Title != "Updated Intro" {
		t.Errorf("want title 'Updated Intro', got %s", updated.Title)
	}

	// Reorder sections: put sec2 before sec1
	err = svc.ReorderSections(ctx, course1.ID.String(), []string{sec2.ID.String(), sec1.ID.String()}, RoleAdminStore)
	if err != nil {
		t.Fatalf("ReorderSections: %v", err)
	}

	// Delete sec2
	err = svc.DeleteSection(ctx, course1.ID.String(), sec2.ID.String(), RoleAdminStore)
	if err != nil {
		t.Fatalf("DeleteSection: %v", err)
	}

	sections1, err = svc.ListSections(ctx, course1.ID.String())
	if err != nil {
		t.Fatalf("ListSections after delete: %v", err)
	}
	if len(sections1) != 1 {
		t.Errorf("after delete: want 1 section, got %d", len(sections1))
	}
}

// Test: Section CRUD rejects non-store role
func TestSection_RejectsNonStoreRole(t *testing.T) {
	ctx := context.Background()
	fake := newFakeCourseRepo()
	svc := &shimCourseService{fake: fake}

	course, err := svc.CreateCourse(ctx, "Math", "beginner", "math", "Mr. A", RoleAdminStore)
	if err != nil {
		t.Fatalf("CreateCourse: %v", err)
	}

	_, err = svc.CreateSection(ctx, course.ID.String(), "Sec 1", RoleStudent)
	if !errors.Is(err, ErrForbidden) {
		t.Errorf("want ErrForbidden for student CreateSection, got %v", err)
	}

	_, err = svc.UpdateSection(ctx, course.ID.String(), uuid.New().String(), "New Title", RoleStudent)
	if !errors.Is(err, ErrForbidden) {
		t.Errorf("want ErrForbidden for student UpdateSection, got %v", err)
	}

	err = svc.DeleteSection(ctx, course.ID.String(), uuid.New().String(), RoleStudent)
	if !errors.Is(err, ErrForbidden) {
		t.Errorf("want ErrForbidden for student DeleteSection, got %v", err)
	}

	err = svc.ReorderSections(ctx, course.ID.String(), []string{}, RoleStudent)
	if !errors.Is(err, ErrForbidden) {
		t.Errorf("want ErrForbidden for student ReorderSections, got %v", err)
	}
}

// Test: Section positions increment correctly
func TestCreateSection_AutoPosition(t *testing.T) {
	ctx := context.Background()
	fake := newFakeCourseRepo()
	svc := &shimCourseService{fake: fake}

	course, err := svc.CreateCourse(ctx, "Math", "beginner", "math", "Mr. A", RoleAdminStore)
	if err != nil {
		t.Fatalf("CreateCourse: %v", err)
	}

	courseID := course.ID.String()

	sec1, err := svc.CreateSection(ctx, courseID, "First", RoleAdminStore)
	if err != nil {
		t.Fatalf("CreateSection 1: %v", err)
	}
	if sec1.Position != 0 {
		t.Errorf("want position 0, got %d", sec1.Position)
	}

	sec2, err := svc.CreateSection(ctx, courseID, "Second", RoleAdminStore)
	if err != nil {
		t.Fatalf("CreateSection 2: %v", err)
	}
	if sec2.Position != 1 {
		t.Errorf("want position 1, got %d", sec2.Position)
	}
}

// Test: ReorderSections rejects mismatched course_id
func TestReorderSections_RejectsMismatchedCourseID(t *testing.T) {
	ctx := context.Background()
	fake := newFakeCourseRepo()
	svc := &shimCourseService{fake: fake}

	course, err := svc.CreateCourse(ctx, "Math", "beginner", "math", "Mr. A", RoleAdminStore)
	if err != nil {
		t.Fatalf("CreateCourse: %v", err)
	}

	sec1, err := svc.CreateSection(ctx, course.ID.String(), "Sec 1", RoleAdminStore)
	if err != nil {
		t.Fatalf("CreateSection: %v", err)
	}

	wrongCourseID := uuid.New().String()
	err = svc.ReorderSections(ctx, wrongCourseID, []string{sec1.ID.String()}, RoleAdminStore)
	if !errors.Is(err, repository.ErrNotFound) {
		t.Errorf("want ErrNotFound for mismatched course_id, got %v", err)
	}
}

// Test: Lesson CRUD is accessible through course_id parameter
func TestLesson_KeyedByCourseAndSection(t *testing.T) {
	ctx := context.Background()
	fake := newFakeCourseRepo()
	svc := &shimCourseService{fake: fake}

	course, err := svc.CreateCourse(ctx, "Math", "beginner", "math", "Mr. A", RoleAdminStore)
	if err != nil {
		t.Fatalf("CreateCourse: %v", err)
	}

	sec, err := svc.CreateSection(ctx, course.ID.String(), "Intro", RoleAdminStore)
	if err != nil {
		t.Fatalf("CreateSection: %v", err)
	}

	// Create lesson
	lesson, err := svc.CreateLesson(ctx, course.ID.String(), sec.ID.String(), "Welcome", "https://video/1", 300, RoleAdminStore)
	if err != nil {
		t.Fatalf("CreateLesson: %v", err)
	}
	if lesson.Position != 0 {
		t.Errorf("want position 0, got %d", lesson.Position)
	}

	// Update lesson
	updated, err := svc.UpdateLesson(ctx, course.ID.String(), sec.ID.String(), lesson.ID.String(), "Updated Welcome", "https://video/2", 400, RoleAdminStore)
	if err != nil {
		t.Fatalf("UpdateLesson: %v", err)
	}
	if updated.Title != "Updated Welcome" {
		t.Errorf("want title 'Updated Welcome', got %s", updated.Title)
	}

	// Delete lesson
	err = svc.DeleteLesson(ctx, course.ID.String(), sec.ID.String(), lesson.ID.String(), RoleAdminStore)
	if err != nil {
		t.Fatalf("DeleteLesson: %v", err)
	}

	lessons, err := svc.fake.ListLessonsBySection(ctx, sec.ID)
	if err != nil {
		t.Fatalf("ListLessonsBySection: %v", err)
	}
	if len(lessons) != 0 {
		t.Errorf("want 0 lessons after delete, got %d", len(lessons))
	}
}

// Test: Lesson CRUD rejects non-store role
func TestLesson_RejectsNonStoreRole(t *testing.T) {
	ctx := context.Background()
	fake := newFakeCourseRepo()
	svc := &shimCourseService{fake: fake}

	course, err := svc.CreateCourse(ctx, "Math", "beginner", "math", "Mr. A", RoleAdminStore)
	if err != nil {
		t.Fatalf("CreateCourse: %v", err)
	}
	sec, err := svc.CreateSection(ctx, course.ID.String(), "Intro", RoleAdminStore)
	if err != nil {
		t.Fatalf("CreateSection: %v", err)
	}

	_, err = svc.CreateLesson(ctx, course.ID.String(), sec.ID.String(), "L1", "", 0, RoleStudent)
	if !errors.Is(err, ErrForbidden) {
		t.Errorf("want ErrForbidden for student CreateLesson, got %v", err)
	}

	_, err = svc.UpdateLesson(ctx, course.ID.String(), sec.ID.String(), uuid.New().String(), "L1", "", 0, RoleStudent)
	if !errors.Is(err, ErrForbidden) {
		t.Errorf("want ErrForbidden for student UpdateLesson, got %v", err)
	}

	err = svc.DeleteLesson(ctx, course.ID.String(), sec.ID.String(), uuid.New().String(), RoleStudent)
	if !errors.Is(err, ErrForbidden) {
		t.Errorf("want ErrForbidden for student DeleteLesson, got %v", err)
	}

	err = svc.ReorderLessons(ctx, course.ID.String(), sec.ID.String(), []string{}, RoleStudent)
	if !errors.Is(err, ErrForbidden) {
		t.Errorf("want ErrForbidden for student ReorderLessons, got %v", err)
	}
}

// UpdateLesson position-preservation regression: see
// integration/TestUpdateLesson_PreservesPosition_RealRepo (real Postgres +
// Repository.UpdateLesson; the fake-repo test here was tautological).

// --- Student-facing course tests ---

func TestCourseProgressPct(t *testing.T) {
	tests := []struct {
		name      string
		completed int
		total     int
		want      float64
	}{
		{"zero total", 5, 0, 0},
		{"exact 3/4", 3, 4, 75},
		{"round down 1/3", 1, 3, 33.33},
		{"round up 2/3", 2, 3, 66.67},
		{"zero completed", 0, 10, 0},
		{"all completed", 10, 10, 100},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := courseProgressPct(tt.completed, tt.total)
			if got != tt.want {
				t.Errorf("courseProgressPct(%d, %d) = %v, want %v", tt.completed, tt.total, got, tt.want)
			}
		})
	}
}

func TestMarkLessonComplete_NoActiveSession(t *testing.T) {
	ctx := context.Background()
	fake := newFakeCourseRepo()
	svc := &shimCourseService{fake: fake}

	studentID := uuid.New()
	courseID := uuid.New()
	lessonID := uuid.New()

	err := svc.MarkLessonComplete(ctx, studentID.String(), courseID.String(), lessonID.String())
	if !errors.Is(err, ErrNoCourseAccess) {
		t.Errorf("want ErrNoCourseAccess, got %v", err)
	}
}

func TestMarkLessonComplete_Success(t *testing.T) {
	ctx := context.Background()
	fake := newFakeCourseRepo()
	svc := &shimCourseService{fake: fake}

	studentID := uuid.New()
	courseID := uuid.New()
	lessonID := uuid.New()

	fake.seedCourseSession(studentID, courseID)

	err := svc.MarkLessonComplete(ctx, studentID.String(), courseID.String(), lessonID.String())
	if err != nil {
		t.Fatalf("MarkLessonComplete: %v", err)
	}

	for _, sess := range fake.sessions {
		if sess.StudentID == studentID && sess.CourseID == courseID {
			if _, ok := sess.CompletedLessons[lessonID]; !ok {
				t.Errorf("expected lesson %s in completed lessons", lessonID)
			}
		}
	}
}

func TestMarkLessonComplete_ReMarkIsNoOp(t *testing.T) {
	ctx := context.Background()
	fake := newFakeCourseRepo()

	studentID := uuid.New()
	courseID := uuid.New()
	lessonID := uuid.New()

	session := fake.seedCourseSession(studentID, courseID)

	t1 := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)

	// Mark via fake directly to control timestamp
	err := fake.MarkLessonComplete(ctx, session.ID, lessonID, t1)
	if err != nil {
		t.Fatalf("first MarkLessonComplete: %v", err)
	}

	if ts, ok := session.CompletedLessons[lessonID]; !ok {
		t.Errorf("expected lesson in completed lessons")
	} else if !ts.Equal(t1) {
		t.Errorf("want timestamp %v, got %v", t1, ts)
	}

	// Re-mark at a later time
	t2 := t1.Add(time.Hour)
	err = fake.MarkLessonComplete(ctx, session.ID, lessonID, t2)
	if err != nil {
		t.Fatalf("second MarkLessonComplete: %v", err)
	}

	if ts, ok := session.CompletedLessons[lessonID]; !ok {
		t.Errorf("expected lesson in completed lessons after re-mark")
	} else if !ts.Equal(t1) {
		t.Errorf("re-mark changed timestamp: want %v, got %v", t1, ts)
	}
}

func TestListLibrary_ReturnsActiveSessions(t *testing.T) {
	ctx := context.Background()
	fake := newFakeCourseRepo()
	svc := &shimCourseService{fake: fake}

	student1 := uuid.New()
	student2 := uuid.New()
	course1 := uuid.New()
	course2 := uuid.New()
	course3 := uuid.New()

	// student1: 2 active sessions
	fake.seedCourseSession(student1, course1)
	fake.seedCourseSession(student1, course2)

	// student1: 1 revoked session
	revokedSess := fake.seedCourseSession(student1, course3)
	revokedSess.Status = "revoked"

	// student2: 1 active session
	fake.seedCourseSession(student2, course1)

	sessions, err := svc.ListLibrary(ctx, student1.String())
	if err != nil {
		t.Fatalf("ListLibrary student1: %v", err)
	}
	if len(sessions) != 2 {
		t.Errorf("student1: want 2 sessions, got %d", len(sessions))
	}

	sessions, err = svc.ListLibrary(ctx, student2.String())
	if err != nil {
		t.Fatalf("ListLibrary student2: %v", err)
	}
	if len(sessions) != 1 {
		t.Errorf("student2: want 1 session, got %d", len(sessions))
	}
}

// FR3: GetCourse returns course detail with section_count and lesson_count.
func TestGetCourse_ReturnsSectionAndLessonCount(t *testing.T) {
	ctx := context.Background()
	fake := newFakeCourseRepo()
	svc := &shimCourseService{fake: fake}
	gdSvc := &shimGetDeleteCourse{fake: fake}

	course, err := svc.CreateCourse(ctx, "Math", "beginner", "math", "Mr. A", RoleAdminStore)
	if err != nil {
		t.Fatalf("CreateCourse: %v", err)
	}

	sec, err := svc.CreateSection(ctx, course.ID.String(), "Intro", RoleAdminStore)
	if err != nil {
		t.Fatalf("CreateSection: %v", err)
	}
	_, err = svc.CreateLesson(ctx, course.ID.String(), sec.ID.String(), "L1", "", 0, RoleAdminStore)
	if err != nil {
		t.Fatalf("CreateLesson 1: %v", err)
	}
	_, err = svc.CreateLesson(ctx, course.ID.String(), sec.ID.String(), "L2", "", 0, RoleAdminStore)
	if err != nil {
		t.Fatalf("CreateLesson 2: %v", err)
	}

	got, sectionCount, lessonCount, err := gdSvc.GetCourse(ctx, course.ID.String())
	if err != nil {
		t.Fatalf("GetCourse: %v", err)
	}
	if got.Title != "Math" {
		t.Errorf("want title Math, got %s", got.Title)
	}
	if sectionCount != 1 {
		t.Errorf("want 1 section, got %d", sectionCount)
	}
	if lessonCount != 2 {
		t.Errorf("want 2 lessons, got %d", lessonCount)
	}
}

// FR3: GetCourse returns ErrCourseNotFound for unknown id.
func TestGetCourse_NotFound(t *testing.T) {
	ctx := context.Background()
	fake := newFakeCourseRepo()
	gdSvc := &shimGetDeleteCourse{fake: fake}

	_, _, _, err := gdSvc.GetCourse(ctx, uuid.New().String())
	if !errors.Is(err, ErrCourseNotFound) {
		t.Errorf("want ErrCourseNotFound, got %v", err)
	}
}

// FR5: DeleteCourse removes the course; non-store role is rejected.
func TestDeleteCourse_RBACAndDelete(t *testing.T) {
	ctx := context.Background()
	fake := newFakeCourseRepo()
	svc := &shimCourseService{fake: fake}
	gdSvc := &shimGetDeleteCourse{fake: fake}

	course, err := svc.CreateCourse(ctx, "Math", "beginner", "math", "Mr. A", RoleAdminStore)
	if err != nil {
		t.Fatalf("CreateCourse: %v", err)
	}

	// Non-store role is rejected
	if err := gdSvc.DeleteCourse(ctx, course.ID.String(), RoleAdminExam); !errors.Is(err, ErrForbidden) {
		t.Errorf("want ErrForbidden for admin_exam, got %v", err)
	}

	// Admin store can delete
	if err := gdSvc.DeleteCourse(ctx, course.ID.String(), RoleAdminStore); err != nil {
		t.Fatalf("DeleteCourse: %v", err)
	}

	// Course is gone
	_, _, _, err = gdSvc.GetCourse(ctx, course.ID.String())
	if !errors.Is(err, ErrCourseNotFound) {
		t.Errorf("want ErrCourseNotFound after delete, got %v", err)
	}
}

// FR20: GetCourseWithProgress returns sections, lessons, per-lesson completion.
func TestGetCourseWithProgress_ReturnsCompletionState(t *testing.T) {
	ctx := context.Background()
	fake := newFakeCourseRepo()
	svc := &shimCourseService{fake: fake}
	studentSvc := &shimStudentCourse{fake: fake}

	course, err := svc.CreateCourse(ctx, "Math", "beginner", "math", "Mr. A", RoleAdminStore)
	if err != nil {
		t.Fatalf("CreateCourse: %v", err)
	}

	sec, err := svc.CreateSection(ctx, course.ID.String(), "Intro", RoleAdminStore)
	if err != nil {
		t.Fatalf("CreateSection: %v", err)
	}

	l1, err := svc.CreateLesson(ctx, course.ID.String(), sec.ID.String(), "L1", "", 0, RoleAdminStore)
	if err != nil {
		t.Fatalf("CreateLesson 1: %v", err)
	}
	_, err = svc.CreateLesson(ctx, course.ID.String(), sec.ID.String(), "L2", "", 0, RoleAdminStore)
	if err != nil {
		t.Fatalf("CreateLesson 2: %v", err)
	}

	studentID := uuid.New()
	sess := fake.seedCourseSession(studentID, course.ID)

	// Mark l1 complete
	if err := fake.MarkLessonComplete(ctx, sess.ID, l1.ID, time.Now()); err != nil {
		t.Fatalf("MarkLessonComplete: %v", err)
	}

	result, err := studentSvc.GetCourseWithProgress(ctx, studentID.String(), course.ID.String())
	if err != nil {
		t.Fatalf("GetCourseWithProgress: %v", err)
	}

	if result.Title != "Math" {
		t.Errorf("want title Math, got %s", result.Title)
	}
	if len(result.Sections) != 1 {
		t.Fatalf("want 1 section, got %d", len(result.Sections))
	}
	if len(result.Sections[0].Lessons) != 2 {
		t.Fatalf("want 2 lessons, got %d", len(result.Sections[0].Lessons))
	}

	// Check completion flags
	completedCount := 0
	for _, l := range result.Sections[0].Lessons {
		if l.Completed {
			completedCount++
		}
	}
	if completedCount != 1 {
		t.Errorf("want 1 completed lesson, got %d", completedCount)
	}
}

// FR20: GetCourseWithProgress returns ErrNoCourseAccess when no active session.
func TestGetCourseWithProgress_NoSession(t *testing.T) {
	ctx := context.Background()
	fake := newFakeCourseRepo()
	svc := &shimCourseService{fake: fake}
	studentSvc := &shimStudentCourse{fake: fake}

	course, err := svc.CreateCourse(ctx, "Math", "beginner", "math", "Mr. A", RoleAdminStore)
	if err != nil {
		t.Fatalf("CreateCourse: %v", err)
	}

	studentID := uuid.New()
	_, err = studentSvc.GetCourseWithProgress(ctx, studentID.String(), course.ID.String())
	if !errors.Is(err, ErrNoCourseAccess) {
		t.Errorf("want ErrNoCourseAccess, got %v", err)
	}
}

// FR21: MarkLessonComplete no longer takes a role parameter.
func TestMarkLessonComplete_NoRoleParam(t *testing.T) {
	ctx := context.Background()
	fake := newFakeCourseRepo()
	svc := &shimCourseService{fake: fake}

	studentID := uuid.New()
	courseID := uuid.New()
	lessonID := uuid.New()
	fake.seedCourseSession(studentID, courseID)

	// Should compile and run without role parameter
	if err := svc.MarkLessonComplete(ctx, studentID.String(), courseID.String(), lessonID.String()); err != nil {
		t.Fatalf("MarkLessonComplete: %v", err)
	}
}

// FR22: CourseProgress returns (completed, total, pct float64).
func TestCourseProgress_ReturnsCompletedTotalPct(t *testing.T) {
	ctx := context.Background()
	fake := newFakeCourseRepo()
	svc := &shimCourseService{fake: fake}

	studentID := uuid.New()
	courseID := uuid.New()
	sess := fake.seedCourseSession(studentID, courseID)

	// Seed a section + 4 lessons under courseID
	secID := uuid.New()
	fake.sections[secID.String()] = &model.Section{ID: secID, CourseID: courseID}
	for i := 0; i < 4; i++ {
		lID := uuid.New()
		fake.lessons[lID.String()] = &model.Lesson{ID: lID, SectionID: secID}
		if i < 3 {
			sess.CompletedLessons[lID] = time.Now()
		}
	}

	completed, total, pct, err := svc.CourseProgress(ctx, studentID.String(), courseID.String())
	if err != nil {
		t.Fatalf("CourseProgress: %v", err)
	}
	if completed != 3 {
		t.Errorf("want completed=3, got %d", completed)
	}
	if total != 4 {
		t.Errorf("want total=4, got %d", total)
	}
	// 3/4 = 75.00
	if pct != 75.0 {
		t.Errorf("want pct=75.0, got %v", pct)
	}
}
