package service

import (
	"bytes"
	"context"
	_ "embed"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"path"
	"strconv"
	"time"

	"akademi-bimbel/internal/model"
	"akademi-bimbel/internal/repository"

	"github.com/google/uuid"
	"github.com/minio/minio-go/v7"
)

// validCertificateTemplates is the closed set of accepted certificate layouts.
var validCertificateTemplates = map[string]bool{
	"classic": true,
	"modern":  true,
	"elegant": true,
	"custom":  true,
}

func validateCertificateTemplate(tmpl string) error {
	if !validCertificateTemplates[tmpl] {
		return fmt.Errorf("%w: unknown certificate template: %s", ErrValidation, tmpl)
	}
	return nil
}

//go:embed assets/cert_bg_classic.jpg
var certBgClassic []byte

//go:embed assets/cert_bg_modern.jpg
var certBgModern []byte

//go:embed assets/cert_bg_elegant.png
var certBgElegant []byte

// builtinCertificateBackground returns the embedded background PNG for a
// built-in background ref. An unrecognised ref falls back to classic, which
// covers the "custom template but no background key" case (FR-19) since the
// caller passes the default layout's own ref in that situation.
func builtinCertificateBackground(ref string) []byte {
	switch ref {
	case "modern":
		return certBgModern
	case "elegant":
		return certBgElegant
	default:
		return certBgClassic
	}
}

// certificateFieldValues assembles the fixed certificate copy plus the
// per-render student name, date, and certificate number shared by real
// generation and preview.
func certificateFieldValues(examTitle, studentName, dateStr, certNumber string) map[FieldID]string {
	return map[FieldID]string{
		"student_name":       studentName,
		"exam_title":         examTitle,
		"completion_date":    dateStr,
		"certificate_number": certNumber,
		"score":              "86",
		"max_score":          "100",
		"score_percent":      "86%",
		"rank":               "3",
		"percentile":         "Top 15%",
		"duration":           "90 minutes",
		"total_questions":    "50 questions",
	}
}

// resolveCertificateLayout returns the layout saved in exam.CertificateDesign
// when the admin has saved one (signalled by a non-empty Fields slice — a
// design blob that only carries a template has no fields yet), else the
// built-in default layout for the exam's template (FR-29) — an exam never
// opens to an empty canvas. A "custom" template with no saved layout seeds
// from classic, mirroring the background fallback (FR-19).
func resolveCertificateLayout(exam *model.Exam) (Layout, error) {
	design, err := parseCertificateDesign(exam.CertificateDesign)
	if err != nil {
		return Layout{}, err
	}
	if len(design.Fields) > 0 {
		return design.Layout, nil
	}
	tmpl := design.Template
	if tmpl == "custom" {
		tmpl = "classic"
	}
	return defaultLayout(tmpl), nil
}

// downloadCertificateBackground fetches an uploaded custom background from the
// private bucket by its object key — never a raw or presigned URL is stored
// (FR-18), so every render downloads fresh.
// resolveCertificateSignatureImages downloads the layout's uploaded signature
// image (if any) into the images map buildCertificateHTML consumes. Returns
// nil when no signature key is set.
func (s *Service) resolveCertificateImages(ctx context.Context, layout Layout) (map[FieldID][]byte, error) {
	layout = normalizeCertificateLayout(layout)
	images := make(map[FieldID][]byte)
	for _, field := range layout.Fields {
		if field.Kind != "image" || field.AssetKey == nil || *field.AssetKey == "" {
			continue
		}
		img, err := s.downloadCertificateBackground(ctx, *field.AssetKey)
		if err != nil {
			return nil, fmt.Errorf("download certificate image %s: %w", field.ID, err)
		}
		images[field.ID] = img
	}
	return images, nil
}

func (s *Service) downloadCertificateBackground(ctx context.Context, key string) ([]byte, error) {
	if s.storage == nil {
		return nil, ErrStorageNotConfigured
	}
	obj, err := s.storage.GetObject(ctx, s.cfg.ObjectStorageBucketName, key, minio.GetObjectOptions{})
	if err != nil {
		return nil, err
	}
	defer obj.Close()
	// minio-go defers the request until Stat/Read, so a missing object surfaces here.
	if _, err := obj.Stat(); err != nil {
		return nil, err
	}
	return io.ReadAll(obj)
}

// resolveCertificateBackground returns the background image bytes for an
// exam's certificate: the embedded built-in asset for classic/modern/elegant,
// or the downloaded custom upload. A "custom" template with a NULL background
// key falls back to the classic built-in rather than failing (FR-19,
// Invariant 8) — there is no input state where generation fails for lack of a
// template.
func (s *Service) resolveCertificateBackground(ctx context.Context, exam *model.Exam) ([]byte, error) {
	tmpl := certificateTemplate(exam)
	if tmpl == "custom" {
		if key := certificateBackgroundKey(exam); key != nil {
			return s.downloadCertificateBackground(ctx, *key)
		}
		return builtinCertificateBackground("classic"), nil
	}
	return builtinCertificateBackground(tmpl), nil
}

// uploadCertificatePDF uploads a PDF certificate at certificates/<sessionID>.pdf
// and returns its object key. The bucket is private, so callers presign a GET to
// serve it — see resolveCertificateURL.
func (s *Service) uploadCertificatePDF(ctx context.Context, sessionID uuid.UUID, pdf []byte) (string, error) {
	if s.storage == nil {
		return "", ErrStorageNotConfigured
	}

	bucket := s.cfg.ObjectStorageBucketName
	key := fmt.Sprintf("certificates/%s.pdf", sessionID.String())
	if _, err := s.storage.PutObject(ctx, bucket, key, bytes.NewReader(pdf), int64(len(pdf)), minio.PutObjectOptions{
		ContentType: "application/pdf",
	}); err != nil {
		return "", err
	}

	return key, nil
}

func (s *Service) GeneratePresignedCertificateAssetUploadURL(ctx context.Context, examID uuid.UUID, filename, contentType string) (*PresignedUploadURL, error) {
	if s.storage == nil {
		return nil, ErrStorageNotConfigured
	}
	if _, err := s.storeRepo.GetExamByID(ctx, examID); err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, ErrExamNotFound
		}
		return nil, err
	}
	filename = path.Base(filename)
	if filename == "." || filename == "/" {
		return nil, fmt.Errorf("%w: invalid filename", ErrValidation)
	}
	key := fmt.Sprintf("certificates/%s/%s-%s", examID, uuid.New(), filename)
	presigned, err := s.presignStorage().PresignHeader(
		ctx,
		http.MethodPut,
		s.cfg.ObjectStorageBucketName,
		key,
		15*time.Minute,
		url.Values{},
		http.Header{"Content-Type": []string{contentType}},
	)
	if err != nil {
		return nil, err
	}
	return &PresignedUploadURL{URL: presigned.String(), Method: "PUT", Fields: map[string]string{}, Key: key}, nil
}

// latestGradedAt returns the latest non-nil GradedAt across all answers, or nil.
func latestGradedAt(answers []model.ExamSessionAnswer) *time.Time {
	var latest *time.Time
	for _, a := range answers {
		if a.GradedAt != nil {
			if latest == nil || a.GradedAt.After(*latest) {
				latest = a.GradedAt
			}
		}
	}
	return latest
}

func layoutUsesToken(layout Layout, wanted ...string) bool {
	set := make(map[string]bool, len(wanted))
	for _, token := range wanted {
		set[token] = true
	}
	for _, field := range normalizeCertificateLayout(layout).Fields {
		for _, token := range certificateTokens(field.Content) {
			if set[token] {
				return true
			}
		}
	}
	return false
}

func flattenCertificateQuestions(tests []model.TestDetail) []model.QuestionWithOptions {
	var questions []model.QuestionWithOptions
	for _, test := range tests {
		questions = append(questions, test.Questions...)
	}
	return questions
}

func formatCertificateNumber(value float64) string {
	return strconv.FormatFloat(value, 'f', -1, 64)
}

func certificateSessionValues(sess *model.ExamSession, questions []model.QuestionWithOptions, higher, total int) map[FieldID]string {
	score := 0.0
	if sess.Score != nil {
		score = *sess.Score
	}
	maxScore := 0.0
	for _, question := range questions {
		maxScore += questionMaxPoints(question)
	}
	values := map[FieldID]string{
		"score":           formatCertificateNumber(score),
		"max_score":       strconv.FormatFloat(maxScore, 'f', -1, 64),
		"score_percent":   "0%",
		"total_questions": strconv.Itoa(len(questions)) + " questions",
	}
	if maxScore > 0 {
		values["score_percent"] = strconv.Itoa(int(math.Round(score/maxScore*100))) + "%"
	}
	if sess.SubmittedAt != nil {
		minutes := int(math.Ceil(sess.SubmittedAt.Sub(sess.StartedAt).Minutes()))
		values["duration"] = strconv.Itoa(max(minutes, 0)) + " minutes"
	}
	if higher >= 0 && total >= 0 {
		rank := higher + 1
		values["rank"] = strconv.Itoa(rank)
		percentile := 100
		if total > 0 {
			percentile = int(math.Ceil(float64(rank) / float64(total) * 100))
		}
		values["percentile"] = "Top " + strconv.Itoa(percentile) + "%"
	}
	return values
}

func certificateLayoutAllowed(exam model.Exam, sess *model.ExamSession, layout Layout, questions []model.QuestionWithOptions, answers []model.ExamSessionAnswer) bool {
	if !layoutUsesToken(layout, "score", "max_score", "score_percent", "rank", "percentile") {
		return true
	}
	_, gated := resultGate(exam, sess.Status == "submitted", isFullyGraded(questions, answers))
	return !gated
}

// addCertificateSessionValues stamps score/rank-derived field values onto vals.
// questions/haveQuestions carry a gate-time GetSessionWithQuestions result
// (resolveCertificateURL fetches it once, up front, when the layout is
// score-bearing) so this never re-issues that query — it only fetches when the
// layout needs questions for a non-gated reason (e.g. total_questions alone)
// and the caller didn't already have them.
func (s *Service) addCertificateSessionValues(ctx context.Context, vals map[FieldID]string, sess *model.ExamSession, layout Layout, questions []model.QuestionWithOptions, haveQuestions bool) error {
	needsQuestions := layoutUsesToken(layout, "max_score", "score_percent", "total_questions", "rank", "percentile")
	var err error
	if needsQuestions && !haveQuestions {
		var tests []model.TestDetail
		tests, err = s.storeRepo.GetSessionWithQuestions(ctx, sess.ExamID)
		if err != nil {
			return err
		}
		questions = flattenCertificateQuestions(tests)
	}
	higher, total := -1, -1
	if layoutUsesToken(layout, "rank", "percentile") {
		score := 0.0
		if sess.Score != nil {
			score = *sess.Score
		}
		higher, err = s.storeRepo.CountHigherScores(ctx, sess.ExamID, score)
		if err != nil {
			return err
		}
		total, err = s.storeRepo.CountFullyGradedSessions(ctx, sess.ExamID)
		if err != nil {
			return err
		}
	}
	for field, value := range certificateSessionValues(sess, questions, higher, total) {
		vals[field] = value
	}
	return nil
}

// certificateValueBundle is the field-value map, certificate number, and
// layout stamped onto a single certificate render.
type certificateValueBundle struct {
	Layout Layout
	Values map[FieldID]string
	Number string
}

// buildCertificateValues resolves the value half of a certificate render —
// the fixed copy plus session-derived score/rank values plus an allocated
// certificate number — shared by resolveCertificateURL's regeneration path
// and the print-data endpoint (Task 7, FR-18) so the two code paths can never
// diverge. questions/haveQuestions carry a gate-time GetSessionWithQuestions
// result when the caller already fetched one (mirrors addCertificateSessionValues),
// so this never re-issues that query. Returns (nil, nil) when sess has not
// actually submitted (SubmittedAt nil) — callers that already checked
// sess.Status == "submitted" won't normally see this, but it guards the
// invariant explicitly rather than assuming it.
func (s *Service) buildCertificateValues(ctx context.Context, exam *model.Exam, sess *model.ExamSession, layout Layout, questions []model.QuestionWithOptions, haveQuestions bool, studentName string) (*certificateValueBundle, error) {
	if sess.SubmittedAt == nil {
		return nil, nil
	}
	loc, err := time.LoadLocation("Asia/Jakarta")
	if err != nil {
		return nil, err
	}
	dateStr := sess.SubmittedAt.In(loc).Format("2 January 2006")
	vals := certificateFieldValues(exam.Title, studentName, dateStr, "")
	if err := s.addCertificateSessionValues(ctx, vals, sess, layout, questions, haveQuestions); err != nil {
		return nil, err
	}

	number, err := s.storeRepo.AllocateCertificateNumber(ctx, sess.ID)
	if err != nil {
		return nil, fmt.Errorf("allocate certificate number: %w", err)
	}
	vals["certificate_number"] = number

	return &certificateValueBundle{Layout: layout, Values: vals, Number: number}, nil
}

// certificateImageAssetURLs presigns a time-limited GET for every layout
// image field backed by an uploaded asset (kind=="image" with a non-empty
// AssetKey), keyed by field id. Shared by the admin design editor's read
// model (GetCertificateDesign) and the certificate print-data endpoint (Task
// 7) so both surfaces resolve the same uploaded assets the same way.
func (s *Service) certificateImageAssetURLs(ctx context.Context, layout Layout) (map[string]string, error) {
	urls := make(map[string]string)
	for _, field := range normalizeCertificateLayout(layout).Fields {
		if field.Kind != "image" || field.AssetKey == nil || *field.AssetKey == "" {
			continue
		}
		signed, err := s.presignReadURL(ctx, s.cfg.ObjectStorageBucketName, *field.AssetKey, presignedDocumentURLTTL)
		if err != nil {
			return nil, fmt.Errorf("presign certificate image %s: %w", field.ID, err)
		}
		urls[field.ID] = signed
	}
	return urls, nil
}

// certificateBackgroundURL returns a browser-loadable URL for an exam's
// certificate background: a presigned GET for a custom upload (the object
// actually lives in the bucket), or a data: URI embedding the compiled-in
// built-in asset directly (it has no object key to presign, and it is not
// sensitive — the same approach GetCertificateDesign's built-in presets
// already use). A "custom" template with no uploaded background falls back
// to classic, mirroring resolveCertificateBackground.
func (s *Service) certificateBackgroundURL(ctx context.Context, exam *model.Exam) (string, error) {
	tmpl := certificateTemplate(exam)
	if tmpl == "custom" {
		if key := certificateBackgroundKey(exam); key != nil {
			signed, err := s.presignReadURL(ctx, s.cfg.ObjectStorageBucketName, *key, presignedDocumentURLTTL)
			if err != nil {
				return "", err
			}
			return signed, nil
		}
		tmpl = "classic"
	}
	bg := builtinCertificateBackground(tmpl)
	return "data:" + imageMimeOrFallback(bg) + ";base64," + base64.StdEncoding.EncodeToString(bg), nil
}

// ErrCertificateGateDenied signals the result gate denies a score-bearing
// layout for this session (FR-19): the print-data endpoint's independent
// re-check, separate from the mint-time check.
var ErrCertificateGateDenied = errors.New("certificate result gate denies this session")

// CertificatePrintData is the full set of server-authored values the
// certificate print route displays (FR-18): the resolved layout, every field
// value the document shows, the allocated certificate number, and
// browser-loadable URLs for the background and any image layers.
type CertificatePrintData struct {
	Layout            Layout             `json:"layout"`
	Values            map[FieldID]string `json:"values"`
	CertificateNumber string             `json:"certificate_number"`
	BackgroundURL     string             `json:"background_url"`
	ImageURLs         map[string]string  `json:"image_urls"`
}

// GetCertificatePrintData computes the print-data response for the
// certificate print route (FR-18, FR-21): the ONLY input is sessionID, which
// the handler has already verified came from a redeemed print token — no
// other credential is trusted here. The result gate is re-applied
// independently of whatever check ran at mint time (FR-19, NFR-S4), through
// the same certificateLayoutAllowed rule resolveCertificateURL uses, and the
// field values are computed through buildCertificateValues — the same
// code path a real render uses — so the print route can never show a value
// generation would not have produced.
func (s *Service) GetCertificatePrintData(ctx context.Context, sessionID string) (*CertificatePrintData, error) {
	sessID, err := uuid.Parse(sessionID)
	if err != nil {
		return nil, fmt.Errorf("%w: invalid session id", ErrValidation)
	}
	sess, err := s.storeRepo.GetExamSessionByID(ctx, sessID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, ErrSessionNotFound
		}
		return nil, err
	}
	exam, err := s.storeRepo.GetExamForSession(ctx, sess.ExamID)
	if err != nil {
		return nil, err
	}
	if sess.Status != "submitted" {
		return nil, ErrSessionNotFound
	}
	if !exam.CertificateEnabled {
		return nil, ErrCertificateDisabled
	}

	answers, err := s.storeRepo.GetSessionAnswers(ctx, sessID)
	if err != nil {
		return nil, err
	}
	studentName := ""
	if user, mErr := s.Me(ctx, sess.StudentID.String()); mErr == nil && user != nil {
		studentName = user.Name
	}

	layout, err := resolveCertificateLayout(exam)
	if err != nil {
		return nil, err
	}

	sensitive := layoutUsesToken(layout, "score", "max_score", "score_percent", "rank", "percentile")
	var questions []model.QuestionWithOptions
	haveQuestions := false
	if sensitive {
		tests, err := s.storeRepo.GetSessionWithQuestions(ctx, sess.ExamID)
		if err != nil {
			return nil, err
		}
		questions = flattenCertificateQuestions(tests)
		haveQuestions = true
		if !certificateLayoutAllowed(*exam, sess, layout, questions, answers) {
			return nil, ErrCertificateGateDenied
		}
	}

	bundle, err := s.buildCertificateValues(ctx, exam, sess, layout, questions, haveQuestions, studentName)
	if err != nil {
		return nil, err
	}
	if bundle == nil {
		return nil, ErrSessionNotFound
	}

	bgURL, err := s.certificateBackgroundURL(ctx, exam)
	if err != nil {
		return nil, fmt.Errorf("resolve certificate background url: %w", err)
	}
	imageURLs, err := s.certificateImageAssetURLs(ctx, layout)
	if err != nil {
		return nil, err
	}

	return &CertificatePrintData{
		Layout:            layout,
		Values:            bundle.Values,
		CertificateNumber: bundle.Number,
		BackgroundURL:     bgURL,
		ImageURLs:         imageURLs,
	}, nil
}

// resolveCertificateURL determines a presigned certificate URL for a session,
// regenerating the PDF when missing, stale by grading, or stale by design edit
// (exam.certificate_design_updated_at newer than sess.certificate_generated_at,
// FR-13/C3). The DB stores the object key; a fresh time-limited GET is signed
// on every read since the bucket is private. A non-submitted session resolves
// to (nil, nil) and generates nothing (FR-16); generation/upload/persist
// failures are returned. Regeneration reuses the session's original
// certificate number — AllocateCertificateNumber is idempotent (FR-10).
//
// The layout is resolved first and certificateLayoutAllowed is checked on
// every access, cached or not (issue #55, NFR-S4) — a rendered PDF must never
// stand in for an authorization decision. GetSessionWithQuestions is fetched
// only when the layout is score-bearing (NFR-P1), so a non-score layout on the
// cache path issues zero extra queries.
func (s *Service) resolveCertificateURL(ctx context.Context, exam *model.Exam, sess *model.ExamSession, answers []model.ExamSessionAnswer, studentName string) (*string, error) {
	if sess.Status != "submitted" {
		return nil, nil
	}
	if !exam.CertificateEnabled {
		return nil, nil
	}

	layout, err := resolveCertificateLayout(exam)
	if err != nil {
		return nil, err
	}

	sensitive := layoutUsesToken(layout, "score", "max_score", "score_percent", "rank", "percentile")
	var questions []model.QuestionWithOptions
	haveQuestions := false
	if sensitive {
		tests, err := s.storeRepo.GetSessionWithQuestions(ctx, sess.ExamID)
		if err != nil {
			return nil, err
		}
		questions = flattenCertificateQuestions(tests)
		haveQuestions = true
		if !certificateLayoutAllowed(*exam, sess, layout, questions, answers) {
			return nil, nil
		}
	}

	gradedAt := latestGradedAt(answers)
	designStale := exam.CertificateDesignUpdatedAt != nil && sess.CertificateGeneratedAt != nil &&
		exam.CertificateDesignUpdatedAt.After(*sess.CertificateGeneratedAt)
	needsRegeneration := sess.CertificateKey == nil || sess.CertificateGeneratedAt == nil ||
		(gradedAt != nil && gradedAt.After(*sess.CertificateGeneratedAt)) || designStale

	if needsRegeneration {
		bundle, err := s.buildCertificateValues(ctx, exam, sess, layout, questions, haveQuestions, studentName)
		if err != nil {
			return nil, err
		}
		if bundle == nil {
			return nil, nil
		}
		bg, err := s.resolveCertificateBackground(ctx, exam)
		if err != nil {
			return nil, fmt.Errorf("resolve certificate background: %w", err)
		}
		images, err := s.resolveCertificateImages(ctx, layout)
		if err != nil {
			return nil, err
		}

		number := bundle.Number

		html, err := buildCertificateHTML(layout, bundle.Values, bg, images)
		if err != nil {
			return nil, fmt.Errorf("build certificate html: %w", err)
		}
		pdf, err := s.renderer.RenderHTML(ctx, html)
		if err != nil {
			return nil, fmt.Errorf("generate certificate pdf: %w", err)
		}
		key, err := s.uploadCertificatePDF(ctx, sess.ID, pdf)
		if err != nil {
			return nil, fmt.Errorf("upload certificate pdf: %w", err)
		}
		now := time.Now()
		if err := s.storeRepo.UpdateSessionCertificate(ctx, sess.ID, key, now); err != nil {
			return nil, fmt.Errorf("persist certificate key: %w", err)
		}
		sess.CertificateKey = &key
		sess.CertificateNumber = &number
	}

	signed, err := s.presignReadURL(ctx, s.cfg.ObjectStorageBucketName, *sess.CertificateKey, presignedDocumentURLTTL)
	if err != nil {
		return nil, fmt.Errorf("presign certificate url: %w", err)
	}
	return &signed, nil
}

// previewStudentName and previewCertificateNumber are the placeholder values a
// preview stamps instead of real data. The number mirrors the four-segment shape
// AllocateCertificateNumber produces (ABK/YYYY/<exam:4>/<participant:6>) so the
// preview shows the width the real string will occupy.
const (
	previewStudentName       = "Nama Peserta Contoh"
	previewCertificateNumber = "ABK/2026/0000/000000"
)

// GetCertificatePreview renders a preview certificate through the same
// background/layout resolution as real generation, using a placeholder
// student name and placeholder certificate number — it never allocates a real
// number (FR-12), since preview
// has no session to allocate against. templateOverride may be empty to use
// the exam's stored template; when it names a different template, the saved
// custom background/layout (authored for the stored template) are not carried
// over, and the override's own built-in default applies instead.
func (s *Service) GetCertificatePreview(ctx context.Context, examID uuid.UUID, templateOverride string) ([]byte, error) {
	exam, err := s.storeRepo.GetExamByID(ctx, examID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, ErrExamNotFound
		}
		return nil, err
	}

	storedTmpl := certificateTemplate(exam)
	tmpl := templateOverride
	if tmpl == "" {
		tmpl = storedTmpl
	}
	if err := validateCertificateTemplate(tmpl); err != nil {
		return nil, err
	}

	previewExam := *exam
	if templateOverride != "" && templateOverride != storedTmpl {
		// A different template than the one saved: the saved background/layout
		// were authored for that template, so don't carry them over — seed a
		// bare design naming only the override, and let resolveCertificateLayout/
		// resolveCertificateBackground fall back to the override's own built-in.
		raw, err := marshalCertificateDesign(certificateDesign{Template: tmpl})
		if err != nil {
			return nil, err
		}
		previewExam.CertificateDesign = raw
	}

	layout, err := resolveCertificateLayout(&previewExam)
	if err != nil {
		return nil, err
	}
	bg, err := s.resolveCertificateBackground(ctx, &previewExam)
	if err != nil {
		return nil, fmt.Errorf("resolve certificate background: %w", err)
	}

	loc, err := time.LoadLocation("Asia/Jakarta")
	if err != nil {
		return nil, err
	}
	vals := certificateFieldValues(exam.Title, previewStudentName, time.Now().In(loc).Format("2 January 2006"), previewCertificateNumber)

	images, err := s.resolveCertificateImages(ctx, layout)
	if err != nil {
		return nil, err
	}
	html, err := buildCertificateHTML(layout, vals, bg, images)
	if err != nil {
		return nil, fmt.Errorf("build certificate html: %w", err)
	}
	return s.renderer.RenderHTML(ctx, html)
}
