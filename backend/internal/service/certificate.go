package service

import (
	"bytes"
	"context"
	_ "embed"
	"encoding/base64"
	"errors"
	"fmt"
	"html/template"
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
	if !certificateIsSensitive(exam, layout) {
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

// presignFunc abstracts which storage endpoint a caller presigns image URLs
// against: the browser-facing endpoint for the admin editor (presignReadURL)
// or the internal endpoint for a worker feeding Gotenberg's headless Chromium
// directly (presignInternalReadURL) — mirrors the same public/internal split
// presignCardAvatarURL/resolveCardLogoURL already use for the exam card.
type presignFunc func(ctx context.Context, bucket, key string, ttl time.Duration) (string, error)

// certificateImageAssetURLs presigns a time-limited GET for every layout
// image field backed by an uploaded asset (kind=="image" with a non-empty
// AssetKey), keyed by field id. Shared by the admin design editor's read
// model (GetCertificateDesign, browser-facing presign) and the worker's
// generation path (GenerateCertificatePDF, internal presign for Gotenberg).
func (s *Service) certificateImageAssetURLs(ctx context.Context, layout Layout, presign presignFunc) (map[string]string, error) {
	urls := make(map[string]string)
	for _, field := range normalizeCertificateLayout(layout).Fields {
		if field.Kind != "image" || field.AssetKey == nil || *field.AssetKey == "" {
			continue
		}
		signed, err := presign(ctx, s.cfg.ObjectStorageBucketName, *field.AssetKey, presignedDocumentURLTTL)
		if err != nil {
			return nil, fmt.Errorf("presign certificate image %s: %w", field.ID, err)
		}
		urls[field.ID] = signed
	}
	return urls, nil
}

// certificateBackgroundURL returns a loadable URL for an exam's certificate
// background: a presigned GET for a custom upload (the object actually lives
// in the bucket), or a data: URI embedding the compiled-in built-in asset
// directly (it has no object key to presign, and it is not sensitive — the
// same approach GetCertificateDesign's built-in presets already use). A
// "custom" template with no uploaded background falls back to classic,
// mirroring resolveCertificateBackground.
func (s *Service) certificateBackgroundURL(ctx context.Context, exam *model.Exam, presign presignFunc) (string, error) {
	tmpl := certificateTemplate(exam)
	if tmpl == "custom" {
		if key := certificateBackgroundKey(exam); key != nil {
			signed, err := presign(ctx, s.cfg.ObjectStorageBucketName, *key, presignedDocumentURLTTL)
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

// resolveCertificateURL determines a presigned certificate URL for a
// submitted, enabled session's already-cached PDF (issue #55, async redesign
// 2026-08-02). This is a READ-ONLY path: it never renders, uploads, or
// persists — that is GenerateCertificatePDF's job, run exclusively by the
// worker off the CertificateNeeded outbox event. A session with no cached
// certificate_key resolves to (nil, nil) — not an error — the same as a
// gate-denied session; the caller has no way to distinguish "not generated
// yet" from "not visible" and must not need to (NFR-S4: a caller must never
// infer generation state from this call).
//
// The layout is resolved first and certificateLayoutAllowed is checked on
// every access (NFR-S4) — a cached PDF must never stand in for an
// authorization decision. GetSessionWithQuestions is fetched only when the
// layout is score-bearing (NFR-P1), so a non-score layout issues zero extra
// queries.
func (s *Service) resolveCertificateURL(ctx context.Context, exam *model.Exam, sess *model.ExamSession, answers []model.ExamSessionAnswer) (*string, error) {
	if sess.Status != "submitted" {
		return nil, nil
	}
	if !exam.CertificateEnabled {
		return nil, nil
	}
	if sess.CertificateKey == nil {
		return nil, nil
	}

	layout, err := resolveCertificateLayout(exam)
	if err != nil {
		return nil, err
	}

	// certificateIsSensitive also scans the persisted template HTML, not just
	// the layout (Finding 4, 2026-08 review) — a template can carry a
	// {{score}} token the layout never declared, and this short-circuit is
	// what decides whether certificateLayoutAllowed even runs below.
	sensitive := certificateIsSensitive(*exam, layout)
	if sensitive {
		tests, err := s.storeRepo.GetSessionWithQuestions(ctx, sess.ExamID)
		if err != nil {
			return nil, err
		}
		questions := flattenCertificateQuestions(tests)
		if !certificateLayoutAllowed(*exam, sess, layout, questions, answers) {
			return nil, nil
		}
	}

	signed, err := s.presignReadURL(ctx, s.cfg.ObjectStorageBucketName, *sess.CertificateKey, presignedDocumentURLTTL)
	if err != nil {
		return nil, fmt.Errorf("presign certificate url: %w", err)
	}
	return &signed, nil
}

// certificateTokenSub matches every {{token}} spot substituteTemplateTokens
// replaces — the exact same grammar certificateTokenPattern (certificate_layout.go)
// validates layout Content strings against, plus the certificate_background_url/
// certificate_asset_<fieldID> image tokens GenerateCertificatePDF adds to the
// same values map.
var certificateTokenSub = certificateTokenPattern

// substituteTemplateTokens replaces every {{token}} in html with values[token],
// HTML-escaped. A token with no entry in values is blanked rather than left
// literal — the FE-authored template is untrusted input from the worker's point
// of view; nothing renders on the PDF that this function didn't put there from
// a verified DB value moments earlier (NFR-S2's forgery-boundary principle,
// applied to the async path: the worker, not the template, is the boundary).
func substituteTemplateTokens(html string, values map[FieldID]string) string {
	return certificateTokenSub.ReplaceAllStringFunc(html, func(m string) string {
		token := m[2 : len(m)-2]
		v, ok := values[token]
		if !ok {
			return ""
		}
		return template.HTMLEscapeString(v)
	})
}

// CertificateNeededPayload is the outbox payload for the "CertificateNeeded"
// event (async redesign 2026-08-02). SubmitSession, ForceSubmitSession, and
// GradeEssayAnswer each insert this event, in the same transaction as the
// state change that might make a certificate generatable — the worker
// decides readiness itself from DB state when it processes the event
// (GenerateCertificatePDF), so it is safe, and expected, for more than one of
// those call sites to enqueue this for the same session.
type CertificateNeededPayload struct {
	SessionID uuid.UUID `json:"session_id"`
}

// GenerateCertificatePDF is the ONLY place a certificate is ever generated —
// called exclusively by the worker's outbox handler for a "CertificateNeeded"
// event. It substitutes verified DB values into the exam's FE-authored
// template (exam.certificate_template_html) and renders through Gotenberg's
// RenderHTML; no certificate HTML is built from scratch in Go, and nothing
// here ever asks Gotenberg to fetch a URL.
//
// A nil error with nothing generated is the expected, ordinary outcome
// whenever the session isn't ready: not submitted, certificates disabled, no
// template saved yet, a score-bearing layout still awaiting grading, or an
// already-current cached PDF. Callers enqueue this event at more than one
// point in a session's lifecycle and cannot know in advance which one will
// find it ready — GenerateCertificatePDF is what actually decides, every
// time, from the current DB state.
func (s *Service) GenerateCertificatePDF(ctx context.Context, sessionID uuid.UUID) error {
	sess, err := s.storeRepo.GetExamSessionByID(ctx, sessionID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil
		}
		return err
	}
	if sess.Status != "submitted" {
		return nil
	}

	exam, err := s.storeRepo.GetExamForSession(ctx, sess.ExamID)
	if err != nil {
		return err
	}
	if !exam.CertificateEnabled {
		return nil
	}
	if exam.CertificateTemplateHTML == nil || *exam.CertificateTemplateHTML == "" {
		return nil
	}

	layout, err := resolveCertificateLayout(exam)
	if err != nil {
		return err
	}

	answers, err := s.storeRepo.GetSessionAnswers(ctx, sessionID)
	if err != nil {
		return err
	}

	sensitive := layoutUsesToken(layout, "score", "max_score", "score_percent", "rank", "percentile")
	var questions []model.QuestionWithOptions
	haveQuestions := false
	if sensitive {
		tests, err := s.storeRepo.GetSessionWithQuestions(ctx, sess.ExamID)
		if err != nil {
			return err
		}
		questions = flattenCertificateQuestions(tests)
		haveQuestions = true
		// The score isn't final yet — generating now would bake in an
		// incomplete grade. GradeEssayAnswer re-enqueues this event on every
		// grade, so this session gets another chance once grading finishes.
		if !isFullyGraded(questions, answers) {
			return nil
		}
	}

	gradedAt := latestGradedAt(answers)
	needsGeneration := sess.CertificateKey == nil || sess.CertificateGeneratedAt == nil ||
		(gradedAt != nil && gradedAt.After(*sess.CertificateGeneratedAt))
	if !needsGeneration {
		return nil
	}

	studentName := ""
	if user, mErr := s.Me(ctx, sess.StudentID.String()); mErr == nil && user != nil {
		studentName = user.Name
	}

	bundle, err := s.buildCertificateValues(ctx, exam, sess, layout, questions, haveQuestions, studentName)
	if err != nil {
		return err
	}
	if bundle == nil {
		return nil
	}

	bgURL, err := s.certificateBackgroundURL(ctx, exam, s.presignInternalReadURL)
	if err != nil {
		return fmt.Errorf("resolve certificate background url: %w", err)
	}
	bundle.Values["certificate_background_url"] = bgURL

	imageURLs, err := s.certificateImageAssetURLs(ctx, layout, s.presignInternalReadURL)
	if err != nil {
		return err
	}
	for fieldID, imgURL := range imageURLs {
		bundle.Values["certificate_asset_"+fieldID] = imgURL
	}

	html := substituteTemplateTokens(*exam.CertificateTemplateHTML, bundle.Values)

	pdf, err := s.renderer.RenderHTML(ctx, []byte(html))
	if err != nil {
		return fmt.Errorf("render certificate pdf: %w", err)
	}
	key, err := s.uploadCertificatePDF(ctx, sess.ID, pdf)
	if err != nil {
		return fmt.Errorf("upload certificate pdf: %w", err)
	}
	if err := s.storeRepo.UpdateSessionCertificate(ctx, sess.ID, key, time.Now()); err != nil {
		return fmt.Errorf("persist certificate key: %w", err)
	}
	return nil
}

// GetCertificatePreviewPDF renders an admin's in-progress certificate design
// straight to PDF for the live preview panel. The FE has already serialized
// the (possibly unsaved) layout to self-contained HTML with placeholder
// preview values baked in via the same in-browser preview path the editor
// canvas itself uses (web/app/api/admin/certificate-template +
// lib/certificate-studio's previewContent) — this is a thin pass-through to
// Gotenberg, no DB write, no object-store write, no field substitution. The
// exam lookup exists only to 404 an unknown id before spending a Gotenberg
// round trip.
func (s *Service) GetCertificatePreviewPDF(ctx context.Context, examID uuid.UUID, html string) ([]byte, error) {
	if _, err := s.storeRepo.GetExamByID(ctx, examID); err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, ErrExamNotFound
		}
		return nil, err
	}
	if html == "" {
		return nil, fmt.Errorf("%w: html is required", ErrValidation)
	}
	return s.renderer.RenderHTML(ctx, []byte(html))
}
