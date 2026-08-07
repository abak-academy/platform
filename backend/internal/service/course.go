package service

import (
	"context"
	"errors"
	"math"
	"time"

	"github.com/google/uuid"

	"akademi-bimbel/internal/model"
	"akademi-bimbel/internal/repository"
)

// --- Course CRUD ---

func (s *Service) CreateCourse(ctx context.Context, title, level, subject, instructorName, role string) (model.Course, error) {
	if role != RoleAdminStore && role != RoleSuperAdmin {
		return model.Course{}, ErrForbidden
	}

	c := model.Course{
		Title:          title,
		Level:          level,
		Subject:        subject,
		InstructorName: instructorName,
	}
	return s.storeRepo.CreateCourse(ctx, c)
}

// canReadCourses allows the admin roles that need to browse the course catalogue
// read access, which is broader than canAuthorCourses' write guard above.
func canReadCourses(role string) bool {
	return role == RoleAdminExam || role == RoleAdminStore || role == RoleSuperAdmin
}

func (s *Service) ListCourses(ctx context.Context, limit int, cursor string, role string) ([]model.Course, string, error) {
	if !canReadCourses(role) {
		return nil, "", ErrForbidden
	}
	return s.storeRepo.ListCourses(ctx, limit, cursor)
}

func (s *Service) GetCourse(ctx context.Context, id string, role string) (model.Course, int, int, error) {
	if !canReadCourses(role) {
		return model.Course{}, 0, 0, ErrForbidden
	}
	cID, err := parseUUID(id)
	if err != nil {
		return model.Course{}, 0, 0, err
	}
	c, err := s.storeRepo.GetCourseByID(ctx, cID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return model.Course{}, 0, 0, ErrCourseNotFound
		}
		return model.Course{}, 0, 0, err
	}
	sections, err := s.storeRepo.ListSections(ctx, cID)
	if err != nil {
		return model.Course{}, 0, 0, err
	}
	total, err := s.storeRepo.CountLessonsByCourse(ctx, cID)
	if err != nil {
		return model.Course{}, 0, 0, err
	}
	return c, len(sections), total, nil
}

func (s *Service) DeleteCourse(ctx context.Context, id, role string) error {
	if role != RoleAdminStore && role != RoleSuperAdmin {
		return ErrForbidden
	}
	cID, err := parseUUID(id)
	if err != nil {
		return err
	}
	_, err = s.storeRepo.GetCourseByID(ctx, cID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return ErrCourseNotFound
		}
		return err
	}
	return s.storeRepo.DeleteCourse(ctx, cID)
}

func (s *Service) UpdateCourse(ctx context.Context, id, title, level, subject, instructorName, role string) (model.Course, error) {
	if role != RoleAdminStore && role != RoleSuperAdmin {
		return model.Course{}, ErrForbidden
	}

	courseID, err := parseUUID(id)
	if err != nil {
		return model.Course{}, err
	}

	// Fetch existing to preserve fields not sent by partial PATCH.
	existing, err := s.storeRepo.GetCourseByID(ctx, courseID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return model.Course{}, ErrCourseNotFound
		}
		return model.Course{}, err
	}

	c := model.Course{
		Title:          title,
		Level:          level,
		Subject:        subject,
		InstructorName: instructorName,
	}
	// Preserve existing values for any field not supplied (zero-value).
	if c.Title == "" {
		c.Title = existing.Title
	}
	if c.Level == "" {
		c.Level = existing.Level
	}
	if c.Subject == "" {
		c.Subject = existing.Subject
	}
	if c.InstructorName == "" {
		c.InstructorName = existing.InstructorName
	}

	return s.storeRepo.UpdateCourse(ctx, courseID, c)
}

// --- Section CRUD (re-keyed to course_id) ---

func (s *Service) ListSections(ctx context.Context, courseID string, role string) ([]model.Section, error) {
	if !canReadCourses(role) {
		return nil, ErrForbidden
	}
	cID, err := parseUUID(courseID)
	if err != nil {
		return nil, err
	}
	return s.storeRepo.ListSections(ctx, cID)
}

func (s *Service) CreateSection(ctx context.Context, courseID string, title string, role string) (model.Section, error) {
	if role != RoleAdminStore && role != RoleSuperAdmin {
		return model.Section{}, ErrForbidden
	}

	cID, err := parseUUID(courseID)
	if err != nil {
		return model.Section{}, err
	}

	sections, err := s.storeRepo.ListSections(ctx, cID)
	if err != nil {
		return model.Section{}, err
	}

	position := len(sections)
	sec := model.Section{
		CourseID: cID,
		Title:    title,
		Position: position,
	}
	return s.storeRepo.CreateSection(ctx, sec)
}

func (s *Service) UpdateSection(ctx context.Context, courseID, sectionID string, title string, role string) (model.Section, error) {
	if role != RoleAdminStore && role != RoleSuperAdmin {
		return model.Section{}, ErrForbidden
	}

	sID, err := parseUUID(sectionID)
	if err != nil {
		return model.Section{}, err
	}

	return s.storeRepo.UpdateSection(ctx, sID, title)
}

func (s *Service) DeleteSection(ctx context.Context, courseID, sectionID string, role string) error {
	if role != RoleAdminStore && role != RoleSuperAdmin {
		return ErrForbidden
	}

	sID, err := parseUUID(sectionID)
	if err != nil {
		return err
	}

	return s.storeRepo.DeleteSection(ctx, sID)
}

func (s *Service) ReorderSections(ctx context.Context, courseID string, orderedIDs []string, role string) error {
	if role != RoleAdminStore && role != RoleSuperAdmin {
		return ErrForbidden
	}

	cID, err := parseUUID(courseID)
	if err != nil {
		return err
	}

	var ids []uuid.UUID
	for _, id := range orderedIDs {
		parsed, err := parseUUID(id)
		if err != nil {
			return err
		}
		ids = append(ids, parsed)
	}

	return s.storeRepo.ReorderSections(ctx, cID, ids)
}

// --- Lesson CRUD (re-keyed to course_id, unchanged child of section) ---

func (s *Service) CreateLesson(ctx context.Context, courseID, sectionID string, title, videoURL string, duration int, role string) (model.Lesson, error) {
	if role != RoleAdminStore && role != RoleSuperAdmin {
		return model.Lesson{}, ErrForbidden
	}

	sID, err := parseUUID(sectionID)
	if err != nil {
		return model.Lesson{}, err
	}

	lessons, err := s.listLessonsBySection(ctx, sID)
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
	return s.storeRepo.CreateLesson(ctx, lesson)
}

func (s *Service) UpdateLesson(ctx context.Context, courseID, sectionID, lessonID string, title, videoURL string, duration int, role string) (model.Lesson, error) {
	if role != RoleAdminStore && role != RoleSuperAdmin {
		return model.Lesson{}, ErrForbidden
	}

	lID, err := parseUUID(lessonID)
	if err != nil {
		return model.Lesson{}, err
	}

	lesson := model.Lesson{
		Title:           title,
		VideoURL:        videoURL,
		DurationSeconds: duration,
	}
	return s.storeRepo.UpdateLesson(ctx, lID, lesson)
}

func (s *Service) DeleteLesson(ctx context.Context, courseID, sectionID, lessonID string, role string) error {
	if role != RoleAdminStore && role != RoleSuperAdmin {
		return ErrForbidden
	}

	lID, err := parseUUID(lessonID)
	if err != nil {
		return err
	}

	return s.storeRepo.DeleteLesson(ctx, lID)
}

func (s *Service) ReorderLessons(ctx context.Context, courseID, sectionID string, orderedIDs []string, role string) error {
	if role != RoleAdminStore && role != RoleSuperAdmin {
		return ErrForbidden
	}

	sID, err := parseUUID(sectionID)
	if err != nil {
		return err
	}

	var ids []uuid.UUID
	for _, id := range orderedIDs {
		parsed, err := parseUUID(id)
		if err != nil {
			return err
		}
		ids = append(ids, parsed)
	}

	return s.storeRepo.ReorderLessons(ctx, sID, ids)
}

func (s *Service) listLessonsBySection(ctx context.Context, sectionID uuid.UUID) ([]model.Lesson, error) {
	return s.storeRepo.ListLessonsBySection(ctx, sectionID)
}

// CourseWithProgress is the student-facing course detail: sections + lessons + per-lesson completion.
type CourseWithProgress struct {
	model.Course
	Sections []SectionWithLessons `json:"sections"`
}

type SectionWithLessons struct {
	model.Section
	Lessons []LessonWithCompletion `json:"lessons"`
}

type LessonWithCompletion struct {
	model.Lesson
	Completed bool `json:"completed"`
}

func (s *Service) GetCourseWithProgress(ctx context.Context, studentID, courseID string) (CourseWithProgress, error) {
	sID, err := parseUUID(studentID)
	if err != nil {
		return CourseWithProgress{}, err
	}
	cID, err := parseUUID(courseID)
	if err != nil {
		return CourseWithProgress{}, err
	}

	session, err := s.storeRepo.GetActiveSession(ctx, sID, cID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return CourseWithProgress{}, ErrNoCourseAccess
		}
		return CourseWithProgress{}, err
	}

	course, err := s.storeRepo.GetCourseByID(ctx, cID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return CourseWithProgress{}, ErrCourseNotFound
		}
		return CourseWithProgress{}, err
	}

	sections, err := s.storeRepo.ListSections(ctx, cID)
	if err != nil {
		return CourseWithProgress{}, err
	}

	var sectionsWithLessons []SectionWithLessons
	for _, sec := range sections {
		lessons, err := s.storeRepo.ListLessonsBySection(ctx, sec.ID)
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

// --- Student-facing course methods ---

func (s *Service) MarkLessonComplete(ctx context.Context, studentID, courseID, lessonID string) error {
	sID, err := parseUUID(studentID)
	if err != nil {
		return err
	}
	cID, err := parseUUID(courseID)
	if err != nil {
		return err
	}
	lID, err := parseUUID(lessonID)
	if err != nil {
		return err
	}

	session, err := s.storeRepo.GetActiveSession(ctx, sID, cID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return ErrNoCourseAccess
		}
		return err
	}

	return s.storeRepo.MarkLessonComplete(ctx, session.ID, lID, time.Now())
}

// CourseProgress returns (completed, total, pct) where pct is a float64 in [0,100].
func (s *Service) CourseProgress(ctx context.Context, studentID, courseID string) (int, int, float64, error) {
	sID, err := parseUUID(studentID)
	if err != nil {
		return 0, 0, 0, err
	}
	cID, err := parseUUID(courseID)
	if err != nil {
		return 0, 0, 0, err
	}

	session, err := s.storeRepo.GetActiveSession(ctx, sID, cID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return 0, 0, 0, ErrNoCourseAccess
		}
		return 0, 0, 0, err
	}

	completed := len(session.CompletedLessons)
	total, err := s.storeRepo.CountLessonsByCourse(ctx, cID)
	if err != nil {
		return 0, 0, 0, err
	}
	return completed, total, courseProgressPct(completed, total), nil
}

func courseProgressPct(completed, total int) float64 {
	if total == 0 {
		return 0
	}
	return math.Round(float64(completed)/float64(total)*100*100) / 100
}

func (s *Service) ListLibrary(ctx context.Context, studentID string) ([]model.CourseSession, error) {
	sID, err := parseUUID(studentID)
	if err != nil {
		return nil, err
	}
	return s.storeRepo.ListActiveSessionsByStudent(ctx, sID)
}
