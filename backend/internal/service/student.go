package service

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/url"
	"strings"
	"time"

	"akademi-bimbel/internal/model"
	"akademi-bimbel/internal/repository"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

type DashboardCourseSummary struct {
	ID           string  `json:"id"`
	Title        string  `json:"title"`
	Progress     float64 `json:"progress"`
	TotalLessons int     `json:"total_lessons"`
	DoneLessons  int     `json:"done_lessons"`
}

type DashboardPendingOrder struct {
	ID      string  `json:"id"`
	Product *string `json:"product,omitempty"`
	Amount  float64 `json:"amount"`
}

type DashboardStudySummary struct {
	VisitedLectures      int     `json:"visited_lectures"`
	TotalLectures        int     `json:"total_lectures"`
	EnrolledCoursesCount int     `json:"enrolled_courses_count"`
	CompletedCourses     int     `json:"completed_courses"`
	TotalMinutes         float64 `json:"total_minutes"`
}

// DashboardLeaderboardEntry is a placeholder — populated when ranking data exists.
type DashboardLeaderboardEntry struct{}

type DashboardRanking struct {
	Position    *int                        `json:"position"`
	Points      *float64                    `json:"points"`
	Leaderboard []DashboardLeaderboardEntry `json:"leaderboard"`
}

type DashboardView struct {
	EnrolledCourses []DashboardCourseSummary `json:"enrolled_courses"`
	PendingOrder    *DashboardPendingOrder   `json:"pending_order,omitempty"`
	StudySummary    DashboardStudySummary    `json:"study_summary"`
	Ranking         DashboardRanking         `json:"ranking"`
	ExamProgress    []interface{}            `json:"exam_progress"`
	PopularLessons  []interface{}            `json:"popular_lessons"`
}

type PresignedUploadURL struct {
	URL    string            `json:"url"`
	Method string            `json:"method"`
	Fields map[string]string `json:"fields"`
	Key    string            `json:"key"`
}

func (s *Service) GetDashboard(ctx context.Context, studentID string) (*DashboardView, error) {
	sID, err := parseUUID(studentID)
	if err != nil {
		return nil, err
	}

	sessions, err := s.storeRepo.ListActiveSessionsByStudent(ctx, sID)
	if err != nil {
		return nil, err
	}

	courses := make([]DashboardCourseSummary, 0, len(sessions))

	var visitedLectures int
	var totalLectures int
	var completedCourses int
	var allCompletedLessonIDs []uuid.UUID

	for _, sess := range sessions {
		course, err := s.storeRepo.GetCourseByID(ctx, sess.CourseID)
		if err != nil {
			return nil, err
		}
		total, err := s.storeRepo.CountLessonsByCourse(ctx, sess.CourseID)
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
		totalSeconds, err := s.storeRepo.SumCompletedLessonMinutes(ctx, allCompletedLessonIDs)
		if err != nil {
			return nil, err
		}
		totalMinutes = float64(totalSeconds) / 60.0
	}

	pending, err := s.getPendingOrder(ctx, sID)
	if err != nil {
		return nil, err
	}

	return &DashboardView{
		EnrolledCourses: courses,
		PendingOrder:    pending,
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

func (s *Service) getPendingOrder(ctx context.Context, studentID uuid.UUID) (*DashboardPendingOrder, error) {
	orders, _, err := s.storeRepo.ListOrders(ctx, repository.OrderFilter{
		StudentID: &studentID,
		Status:    "payment_pending",
		Limit:     1,
	})
	if err != nil {
		return nil, err
	}
	if len(orders) == 0 {
		return nil, nil
	}
	order, err := s.storeRepo.GetOrderByID(ctx, orders[0].ID)
	if err != nil {
		return nil, err
	}
	if order.ID.String() == "" {
		return nil, nil
	}
	var product *string
	if len(order.Items) > 0 {
		name := order.Items[0].Name
		product = &name
	}
	return &DashboardPendingOrder{
		ID:      order.ID.String(),
		Product: product,
		Amount:  order.Total,
	}, nil
}

func (s *Service) ListSchools(ctx context.Context) ([]*model.School, error) {
	return s.repo.ListSchools(ctx)
}

func (s *Service) UpdateProfile(ctx context.Context, userID string, name, email, username, phone, address, targetExam *string, grade *int, dob *time.Time, schoolID *string, unlistedSchoolName *string, jenjang *string, provinsiID, kotaID, kecamatanID, kodePos *string) (*model.User, error) {
	user, err := s.repo.GetUserByID(ctx, userID)
	if err != nil {
		return nil, err
	}
	if user == nil {
		return nil, ErrUserNotFound
	}

	var trimmedSchoolID *string
	if schoolID != nil {
		t := strings.TrimSpace(*schoolID)
		trimmedSchoolID = &t
		if t != "" {
			if _, err := uuid.Parse(t); err != nil {
				return nil, ErrInvalidUUID
			}
		}
	}

	if kodePos != nil {
		if err := ValidateKodePos(*kodePos); err != nil {
			return nil, err
		}
	}

	var normalizedEmail *string
	if email != nil {
		e := normalizeEmail(*email)
		if e != "" {
			normalizedEmail = &e
		}
	}

	if name != nil {
		n := stripFormatRunes(*name)
		name = &n
	}

	// Resolve one of three intents from the (schoolID, unlistedSchoolName)
	// pair: listed (non-empty school_id), unlisted (empty school_id and/or a
	// non-empty unlisted name), or absent (neither field mentioned — leave
	// the school pair untouched and keep the existing-school jenjang
	// fallback). applySchool tells the repository whether to touch the pair
	// at all, since COALESCE can't express "clear this column".
	var trimmedUnlistedName *string
	if unlistedSchoolName != nil {
		t := strings.TrimSpace(*unlistedSchoolName)
		trimmedUnlistedName = &t
	}

	listed := trimmedSchoolID != nil && *trimmedSchoolID != ""
	unlisted := (trimmedSchoolID != nil && *trimmedSchoolID == "") ||
		(trimmedUnlistedName != nil && *trimmedUnlistedName != "")

	var applySchool bool
	var repoSchoolID, repoUnlistedName *string
	var resolveSchoolID string
	switch {
	case listed:
		applySchool = true
		repoSchoolID = trimmedSchoolID
		resolveSchoolID = *trimmedSchoolID
	case unlisted:
		applySchool = true
		repoUnlistedName = trimmedUnlistedName
	default:
		if user.SchoolID != nil {
			resolveSchoolID = *user.SchoolID
		}
	}

	// Validate jenjang against school_types when a school is resolvable.
	if jenjang != nil && resolveSchoolID != "" {
		school, err := s.storeRepo.GetSchoolByID(ctx, resolveSchoolID)
		if err != nil {
			return nil, err
		}
		if school != nil && len(school.SchoolTypes) > 0 && !jenjangInSchoolTypes(*jenjang, school.SchoolTypes) {
			return nil, ErrInvalidJenjang
		}
	}

	// All-or-nothing address validation (FR-REG-02a).
	addrCount := 0
	if provinsiID != nil {
		addrCount++
	}
	if kotaID != nil {
		addrCount++
	}
	if kecamatanID != nil {
		addrCount++
	}
	if addrCount > 0 && addrCount < 3 {
		return nil, ErrIncompleteAddress
	}

	// If all three address fields are present, validate each.
	if addrCount == 3 {
		prov, err := s.storeRepo.GetProvinceByID(ctx, *provinsiID)
		if err != nil {
			return nil, err
		}
		if prov == nil {
			return nil, ErrInvalidProvinsi
		}

		city, err := s.storeRepo.GetCityByID(ctx, *kotaID)
		if err != nil {
			return nil, err
		}
		if city == nil || city.ProvinceID != *provinsiID {
			return nil, ErrInvalidKota
		}

		district, err := s.storeRepo.GetDistrictByID(ctx, *kecamatanID)
		if err != nil {
			return nil, err
		}
		if district == nil || district.CityID != *kotaID {
			return nil, ErrInvalidKecamatan
		}
	}

	if err := s.repo.UpdateUserProfile(ctx, userID, name, normalizedEmail, username, phone, address, targetExam, grade, dob, applySchool, repoSchoolID, repoUnlistedName, jenjang, provinsiID, kotaID, kecamatanID, kodePos); err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return nil, ErrEmailTaken
		}
		return nil, err
	}

	updated, err := s.repo.GetUserByID(ctx, userID)
	if err != nil {
		return nil, err
	}
	if updated == nil {
		return nil, ErrUserNotFound
	}
	return updated, nil
}

// presignStorage returns a MinIO client whose endpoint matches the host the
// browser uses (ObjectStoragePublicEndpoint). Presigned URLs bind the host into the
// signature, so they must be signed for the public host — not the internal
// docker hostname the API container connects through.
func (s *Service) presignStorage() *minio.Client {
	s.presignOnce.Do(func() {
		endpoint := s.cfg.ObjectStoragePublicEndpoint
		if endpoint == "" || endpoint == s.cfg.ObjectStorageEndpoint {
			s.presignClient = s.storage
			return
		}
		// Region must be set explicitly: this client's endpoint resolves to the
		// browser host, which the API container cannot reach, so presigning must
		// not trigger a bucket-region lookup. It is also folded into the V4
		// signature, so it must match the bucket's real region on GCS/S3.
		c, err := minio.New(endpoint, &minio.Options{
			Creds:  credentials.NewStaticV4(s.cfg.ObjectStorageAccessKey, s.cfg.ObjectStorageSecretKey, ""),
			Secure: s.cfg.ObjectStorageUseSSL,
			Region: s.cfg.ObjectStorageRegion,
		})
		if err != nil {
			s.presignClient = s.storage
			return
		}
		s.presignClient = c
	})
	return s.presignClient
}

// presignReadURL signs a time-limited GET for an object. Used where the client
// fetches directly from object storage (e.g. certificate PDFs). Avatars go
// through OpenAvatar instead, so their URLs stay stable and browser-cacheable.
func (s *Service) presignReadURL(ctx context.Context, bucket, key string, ttl time.Duration) (string, error) {
	u, err := s.presignStorage().PresignedGetObject(ctx, bucket, key, ttl, url.Values{})
	if err != nil {
		return "", err
	}
	return u.String(), nil
}

// presignInternalReadURL signs a time-limited GET against s.storage — the
// endpoint the API server itself talks to (ObjectStorageEndpoint) — rather
// than presignStorage()'s browser-facing ObjectStoragePublicEndpoint. Only
// for URLs embedded in server-rendered HTML that Gotenberg's headless
// Chromium fetches from inside the same docker network (the card print
// route's photo/logo <img> src): that consumer can resolve the internal
// "minio:9000" but, unlike a real browser on the host, not "localhost:9000"
// (see local dev's object_storage_public_endpoint). In prod/staging
// ObjectStoragePublicEndpoint is unset, so presignStorage() already returns
// s.storage and this is identical to presignReadURL.
func (s *Service) presignInternalReadURL(ctx context.Context, bucket, key string, ttl time.Duration) (string, error) {
	u, err := s.storage.PresignedGetObject(ctx, bucket, key, ttl, url.Values{})
	if err != nil {
		return "", err
	}
	return u.String(), nil
}

// OpenAvatar streams a stored upload for the read-proxy endpoint. Only the
// avatars/, product/ and question/ prefixes are served: certificates, payment
// proofs and private PII (including student-bulk imports) live in the same
// bucket but are reached exclusively through presigned URLs, so they can never
// be fetched through this unauthenticated proxy.
func (s *Service) OpenAvatar(ctx context.Context, key string) (io.ReadCloser, string, error) {
	if s.storage == nil {
		return nil, "", ErrStorageNotConfigured
	}
	prefix, _, ok := strings.Cut(key, "/")
	if !ok || !proxyReadPrefixAllowlist[prefix] || strings.Contains(key, "..") {
		return nil, "", ErrUploadNotFound
	}
	obj, err := s.storage.GetObject(ctx, s.cfg.ObjectStorageBucketName, key, minio.GetObjectOptions{})
	if err != nil {
		return nil, "", err
	}
	// minio-go defers the request until Stat/Read, so a missing object surfaces here.
	info, err := obj.Stat()
	if err != nil {
		obj.Close()
		return nil, "", err
	}
	return obj, info.ContentType, nil
}

// requireStoredObject verifies that a key names an object that actually exists
// in the bucket. A shape check cannot do this: "payment_proof/<uuid>/x.jpg" is
// well-formed whether or not anything was ever uploaded to it.
//
// Storage (and cfg) are nil only in unit tests that never exercise uploads; in
// every deployed configuration both are set, so the check is live wherever a
// real caller could reach it.
func (s *Service) requireStoredObject(ctx context.Context, key string) error {
	if s.storage == nil || s.cfg == nil {
		return nil
	}
	if _, err := s.storage.StatObject(ctx, s.cfg.ObjectStorageBucketName, key, minio.StatObjectOptions{}); err != nil {
		return ErrUploadNotFound
	}
	return nil
}

// uploadPrefixAllowlist gates GeneratePresignedUploadURL's caller-supplied
// prefix — what may be WRITTEN. It is deliberately not the same set as
// proxyReadPrefixAllowlist: a prefix can be uploadable without being world
// readable, and payment_proof is exactly that case.
var uploadPrefixAllowlist = map[string]bool{
	"avatars":       true,
	"product":       true,
	"question":      true,
	"payment_proof": true,
	"refund_proof":  true,
}

// proxyReadPrefixAllowlist gates OpenAvatar — what the UNAUTHENTICATED
// GET /files/* proxy may serve. Adding a prefix here publishes every object
// under it to anyone holding the key, forever and unrevocably. Payment proofs
// are bank transfer receipts (account numbers, names, amounts), so they are
// signable for upload but read only through a short-lived presigned URL minted
// for an authorised admin — see PresignPaymentProofURL.
var proxyReadPrefixAllowlist = map[string]bool{
	"avatars":  true,
	"product":  true,
	"question": true,
}

func (s *Service) GeneratePresignedUploadURL(ctx context.Context, userID, prefix, filename, contentType string) (*PresignedUploadURL, error) {
	if s.storage == nil {
		return nil, ErrStorageNotConfigured
	}
	if userID == "" || filename == "" {
		return nil, errors.New("user_id and filename are required")
	}
	if !uploadPrefixAllowlist[prefix] {
		return nil, fmt.Errorf("invalid upload prefix: %q", prefix)
	}

	// The public-read bucket is created and its access policy set at
	// provisioning time, not here — GCS has no bucket-policy operation, so
	// doing it per-request would hard-fail on managed storage. App code only signs.
	bucket := s.cfg.ObjectStorageBucketName
	key := fmt.Sprintf("%s/%s/%s-%s", prefix, userID, uuid.New().String(), filename)
	presigned, err := s.presignStorage().PresignedPutObject(ctx, bucket, key, 15*time.Minute)
	if err != nil {
		return nil, err
	}

	return &PresignedUploadURL{
		URL:    presigned.String(),
		Method: "PUT",
		Fields: map[string]string{},
		Key:    key,
	}, nil
}

func (s *Service) UpdatePhoto(ctx context.Context, userID, photoURL string) (*model.User, error) {
	if photoURL == "" {
		return nil, errors.New("photo_url is required")
	}
	// Only accept references that resolve to an uploaded avatar object (the FE
	// sends the /files/<key> proxy URL). Rejecting arbitrary URLs at the source
	// blocks a student from persisting an SSRF probe as their photo_url.
	if avatarKeyFromStored(photoURL) == "" {
		return nil, fmt.Errorf("%w: photo_url must reference an uploaded avatar", ErrValidation)
	}
	user, err := s.repo.GetUserByID(ctx, userID)
	if err != nil {
		return nil, err
	}
	if user == nil {
		return nil, ErrUserNotFound
	}
	if err := s.repo.UpdateUserPhoto(ctx, userID, photoURL); err != nil {
		return nil, err
	}
	updated, err := s.repo.GetUserByID(ctx, userID)
	if err != nil {
		return nil, err
	}
	if updated == nil {
		return nil, ErrUserNotFound
	}
	return updated, nil
}
