package repository

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"akademi-bimbel/internal/model"
)

// --- Course CRUD ---

func (r *Repository) CreateCourse(ctx context.Context, c model.Course) (model.Course, error) {
	err := r.pool.QueryRow(ctx,
		`INSERT INTO course (title, level, subject, instructor_name)
		VALUES ($1, $2, $3, $4)
		RETURNING id, title, level, subject, instructor_name, created_at, updated_at`,
		c.Title, c.Level, c.Subject, c.InstructorName,
	).Scan(&c.ID, &c.Title, &c.Level, &c.Subject, &c.InstructorName, &c.CreatedAt, &c.UpdatedAt)
	return c, err
}

func (r *Repository) ListCourses(ctx context.Context, limit int, cursor string) ([]model.Course, string, error) {
	if limit <= 0 {
		limit = 20
	}

	query := `SELECT id, title, level, subject, instructor_name, created_at, updated_at
		FROM course WHERE 1=1`
	args := []interface{}{}
	argIdx := 1

	if cursor != "" {
		query += fmt.Sprintf(` AND id > $%d`, argIdx)
		args = append(args, cursor)
		argIdx++
	}

	query += fmt.Sprintf(` ORDER BY id LIMIT $%d`, argIdx)
	args = append(args, limit+1)

	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, "", err
	}
	defer rows.Close()

	courses := []model.Course{}
	for rows.Next() {
		var c model.Course
		err := rows.Scan(&c.ID, &c.Title, &c.Level, &c.Subject, &c.InstructorName, &c.CreatedAt, &c.UpdatedAt)
		if err != nil {
			return nil, "", err
		}
		courses = append(courses, c)
	}
	if err := rows.Err(); err != nil {
		return nil, "", err
	}

	var nextCursor string
	if len(courses) > limit {
		nextCursor = courses[limit].ID.String()
		courses = courses[:limit]
	}
	return courses, nextCursor, nil
}

func (r *Repository) GetCourseByID(ctx context.Context, id uuid.UUID) (model.Course, error) {
	c := model.Course{}
	err := r.pool.QueryRow(ctx,
		`SELECT id, title, level, subject, instructor_name, created_at, updated_at
		FROM course WHERE id = $1`,
		id,
	).Scan(&c.ID, &c.Title, &c.Level, &c.Subject, &c.InstructorName, &c.CreatedAt, &c.UpdatedAt)
	if err != nil {
		if isNotFound(err) {
			return model.Course{}, ErrNotFound
		}
		return model.Course{}, err
	}
	return c, nil
}

func (r *Repository) DeleteCourse(ctx context.Context, id uuid.UUID) error {
	_, err := r.pool.Exec(ctx, `DELETE FROM course WHERE id = $1`, id)
	return err
}

func (r *Repository) UpdateCourse(ctx context.Context, id uuid.UUID, c model.Course) (model.Course, error) {
	result := model.Course{}
	err := r.pool.QueryRow(ctx,
		`UPDATE course
		SET title = $1, level = $2, subject = $3, instructor_name = $4, updated_at = now()
		WHERE id = $5
		RETURNING id, title, level, subject, instructor_name, created_at, updated_at`,
		c.Title, c.Level, c.Subject, c.InstructorName, id,
	).Scan(&result.ID, &result.Title, &result.Level, &result.Subject, &result.InstructorName, &result.CreatedAt, &result.UpdatedAt)
	return result, err
}

// GetCoursesByProductID returns all courses linked to a product via product_course.
// Returns an empty slice (not error) when no links exist.
func (r *Repository) GetCoursesByProductID(ctx context.Context, productID uuid.UUID) ([]model.Course, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT c.id, c.title, c.level, c.subject, c.instructor_name, c.created_at, c.updated_at
		FROM course c
		JOIN product_course pc ON pc.course_id = c.id
		WHERE pc.product_id = $1
		ORDER BY c.title`,
		productID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	courses := []model.Course{}
	for rows.Next() {
		var c model.Course
		err := rows.Scan(&c.ID, &c.Title, &c.Level, &c.Subject, &c.InstructorName, &c.CreatedAt, &c.UpdatedAt)
		if err != nil {
			return nil, err
		}
		courses = append(courses, c)
	}
	return courses, rows.Err()
}

// CountLessonsByCourse counts all lessons across all sections of a course.
func (r *Repository) CountLessonsByCourse(ctx context.Context, courseID uuid.UUID) (int, error) {
	var count int
	err := r.pool.QueryRow(ctx,
		`SELECT COUNT(*)
		FROM lesson l
		JOIN section s ON l.section_id = s.id
		WHERE s.course_id = $1`,
		courseID,
	).Scan(&count)
	return count, err
}

// SumCompletedLessonMinutes returns the total duration_seconds for the given lesson IDs.
func (r *Repository) SumCompletedLessonMinutes(ctx context.Context, lessonIDs []uuid.UUID) (int, error) {
	if len(lessonIDs) == 0 {
		return 0, nil
	}
	var total int
	err := r.pool.QueryRow(ctx,
		`SELECT COALESCE(SUM(duration_seconds), 0)
		FROM lesson
		WHERE id = ANY($1)`,
		lessonIDs,
	).Scan(&total)
	return total, err
}

// --- Section CRUD (re-keyed to course_id) ---

func (r *Repository) ListSections(ctx context.Context, courseID uuid.UUID) ([]model.Section, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT s.id, s.course_id, s.title, s.position, s.created_at,
			COALESCE(
				jsonb_agg(
					jsonb_build_object(
						'id', l.id,
						'section_id', l.section_id,
						'title', l.title,
						'video_url', l.video_url,
						'duration_seconds', l.duration_seconds,
						'position', l.position,
						'created_at', l.created_at
					) ORDER BY l.position
				) FILTER (WHERE l.id IS NOT NULL),
				'[]'::jsonb
			) AS lessons
		FROM section s
		LEFT JOIN lesson l ON l.section_id = s.id
		WHERE s.course_id = $1
		GROUP BY s.id, s.course_id, s.title, s.position, s.created_at
		ORDER BY s.position ASC`,
		courseID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	sections := []model.Section{}
	for rows.Next() {
		s := model.Section{}
		var lessonBytes []byte
		err := rows.Scan(&s.ID, &s.CourseID, &s.Title, &s.Position, &s.CreatedAt, &lessonBytes)
		if err != nil {
			return nil, err
		}
		if len(lessonBytes) > 0 {
			if err := json.Unmarshal(lessonBytes, &s.Lessons); err != nil {
				return nil, err
			}
		}
		if s.Lessons == nil {
			s.Lessons = []model.Lesson{}
		}
		sections = append(sections, s)
	}
	return sections, rows.Err()
}

func (r *Repository) CreateSection(ctx context.Context, s model.Section) (model.Section, error) {
	err := r.pool.QueryRow(ctx,
		`INSERT INTO section (course_id, title, position)
		VALUES ($1, $2, $3)
		RETURNING id, course_id, title, position, created_at`,
		s.CourseID, s.Title, s.Position,
	).Scan(&s.ID, &s.CourseID, &s.Title, &s.Position, &s.CreatedAt)
	return s, err
}

func (r *Repository) UpdateSection(ctx context.Context, id uuid.UUID, title string) (model.Section, error) {
	s := model.Section{}
	err := r.pool.QueryRow(ctx,
		`UPDATE section SET title = $1 WHERE id = $2
		RETURNING id, course_id, title, position, created_at`,
		title, id,
	).Scan(&s.ID, &s.CourseID, &s.Title, &s.Position, &s.CreatedAt)
	return s, err
}

func (r *Repository) DeleteSection(ctx context.Context, id uuid.UUID) error {
	_, err := r.pool.Exec(ctx,
		`DELETE FROM section WHERE id = $1`,
		id,
	)
	return err
}

func (r *Repository) ReorderSections(ctx context.Context, courseID uuid.UUID, orderedIDs []uuid.UUID) error {
	for i, id := range orderedIDs {
		_, err := r.pool.Exec(ctx,
			`UPDATE section SET position = $1 WHERE id = $2`,
			i, id,
		)
		if err != nil {
			return err
		}
	}
	return nil
}

// --- Lesson CRUD (unchanged, references section_id) ---

func (r *Repository) CreateLesson(ctx context.Context, l model.Lesson) (model.Lesson, error) {
	err := r.pool.QueryRow(ctx,
		`INSERT INTO lesson (section_id, title, video_url, duration_seconds, position)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id, section_id, title, video_url, duration_seconds, position, created_at`,
		l.SectionID, l.Title, l.VideoURL, l.DurationSeconds, l.Position,
	).Scan(&l.ID, &l.SectionID, &l.Title, &l.VideoURL, &l.DurationSeconds, &l.Position, &l.CreatedAt)
	return l, err
}

func (r *Repository) UpdateLesson(ctx context.Context, id uuid.UUID, l model.Lesson) (model.Lesson, error) {
	result := model.Lesson{}
	err := r.pool.QueryRow(ctx,
		`UPDATE lesson SET title = $1, video_url = $2, duration_seconds = $3 WHERE id = $4
		RETURNING id, section_id, title, video_url, duration_seconds, position, created_at`,
		l.Title, l.VideoURL, l.DurationSeconds, id,
	).Scan(&result.ID, &result.SectionID, &result.Title, &result.VideoURL, &result.DurationSeconds, &result.Position, &result.CreatedAt)
	return result, err
}

func (r *Repository) DeleteLesson(ctx context.Context, id uuid.UUID) error {
	_, err := r.pool.Exec(ctx,
		`DELETE FROM lesson WHERE id = $1`,
		id,
	)
	return err
}

func (r *Repository) ReorderLessons(ctx context.Context, sectionID uuid.UUID, orderedIDs []uuid.UUID) error {
	for i, id := range orderedIDs {
		_, err := r.pool.Exec(ctx,
			`UPDATE lesson SET position = $1 WHERE id = $2`,
			i, id,
		)
		if err != nil {
			return err
		}
	}
	return nil
}

func (r *Repository) ListLessonsBySection(ctx context.Context, sectionID uuid.UUID) ([]model.Lesson, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT id, section_id, title, video_url, duration_seconds, position, created_at
		FROM lesson
		WHERE section_id = $1
		ORDER BY position ASC`,
		sectionID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var lessons []model.Lesson
	for rows.Next() {
		l := model.Lesson{}
		err := rows.Scan(&l.ID, &l.SectionID, &l.Title, &l.VideoURL, &l.DurationSeconds, &l.Position, &l.CreatedAt)
		if err != nil {
			return nil, err
		}
		lessons = append(lessons, l)
	}
	return lessons, rows.Err()
}

// --- Course Session ---

func (r *Repository) CreateCourseSession(ctx context.Context, tx pgx.Tx, s model.CourseSession) error {
	lessonsJSON, err := serializeCompletedLessons(s.CompletedLessons)
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx,
		`INSERT INTO course_session (student_id, course_id, order_id, status, source, enrolled_at, completed_lessons)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		ON CONFLICT DO NOTHING`,
		s.StudentID, s.CourseID, s.OrderID, s.Status, s.Source, s.EnrolledAt, lessonsJSON,
	)
	return err
}

func (r *Repository) RevokeEnrollmentsByOrder(ctx context.Context, tx pgx.Tx, orderID uuid.UUID) error {
	_, err := tx.Exec(ctx,
		`UPDATE course_session SET status = 'revoked', revoked_at = now()
		WHERE order_id = $1`,
		orderID,
	)
	return err
}

func (r *Repository) MarkLessonComplete(ctx context.Context, sessionID uuid.UUID, lessonID uuid.UUID, at time.Time) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE course_session
		SET completed_lessons = completed_lessons || jsonb_build_object($2, $3::text)
		WHERE id = $1 AND NOT (completed_lessons ? $2)`,
		sessionID, lessonID.String(), at.Format(time.RFC3339Nano),
	)
	return err
}

func (r *Repository) GetActiveSession(ctx context.Context, studentID uuid.UUID, courseID uuid.UUID) (model.CourseSession, error) {
	var s model.CourseSession
	var lessonBytes []byte
	err := r.pool.QueryRow(ctx,
		`SELECT id, student_id, course_id, order_id, status, source, enrolled_at, revoked_at, completed_lessons
		FROM course_session
		WHERE student_id = $1 AND course_id = $2 AND status = 'active'`,
		studentID, courseID,
	).Scan(&s.ID, &s.StudentID, &s.CourseID, &s.OrderID, &s.Status, &s.Source, &s.EnrolledAt, &s.RevokedAt, &lessonBytes)
	if err != nil {
		if isNotFound(err) {
			return model.CourseSession{}, ErrNotFound
		}
		return model.CourseSession{}, err
	}
	if err := deserializeCompletedLessons(lessonBytes, &s.CompletedLessons); err != nil {
		return model.CourseSession{}, err
	}
	return s, nil
}

func (r *Repository) ListActiveSessionsByStudent(ctx context.Context, studentID uuid.UUID) ([]model.CourseSession, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT id, student_id, course_id, order_id, status, source, enrolled_at, revoked_at, completed_lessons
		FROM course_session
		WHERE student_id = $1 AND status = 'active'`,
		studentID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	sessions := []model.CourseSession{}
	for rows.Next() {
		var s model.CourseSession
		var lessonBytes []byte
		if err := rows.Scan(&s.ID, &s.StudentID, &s.CourseID, &s.OrderID, &s.Status, &s.Source, &s.EnrolledAt, &s.RevokedAt, &lessonBytes); err != nil {
			return nil, err
		}
		if err := deserializeCompletedLessons(lessonBytes, &s.CompletedLessons); err != nil {
			return nil, err
		}
		sessions = append(sessions, s)
	}
	return sessions, rows.Err()
}

func serializeCompletedLessons(m map[uuid.UUID]time.Time) ([]byte, error) {
	if m == nil {
		return []byte("{}"), nil
	}
	return json.Marshal(m)
}

func deserializeCompletedLessons(data []byte, m *map[uuid.UUID]time.Time) error {
	if len(data) == 0 || string(data) == "null" {
		*m = make(map[uuid.UUID]time.Time)
		return nil
	}
	return json.Unmarshal(data, m)
}

// FilterAlreadyEnrolled returns the subset of studentIDs with an active
// enrollment in the course. A revoked enrollment is no access, so re-ordering
// the course must re-grant it.
func (r *Repository) FilterAlreadyEnrolled(ctx context.Context, courseID uuid.UUID, studentIDs []uuid.UUID) ([]uuid.UUID, error) {
	if len(studentIDs) == 0 {
		return []uuid.UUID{}, nil
	}
	rows, err := r.pool.Query(ctx,
		`SELECT student_id FROM course_session WHERE course_id = $1 AND student_id = ANY($2) AND status = 'active'`,
		courseID, studentIDs,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	ids := make([]uuid.UUID, 0)
	for rows.Next() {
		var id uuid.UUID
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}
