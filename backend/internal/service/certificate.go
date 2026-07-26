package service

import (
	"bytes"
	"context"
	_ "embed"
	"errors"
	"fmt"
	"io"
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

//go:embed assets/cert_bg_classic.png
var certBgClassicPNG []byte

//go:embed assets/cert_bg_modern.png
var certBgModernPNG []byte

//go:embed assets/cert_bg_elegant.png
var certBgElegantPNG []byte

// builtinCertificateBackground returns the embedded background PNG for a
// built-in background ref. An unrecognised ref falls back to classic, which
// covers the "custom template but no background key" case (FR-19) since the
// caller passes the default layout's own ref in that situation.
func builtinCertificateBackground(ref string) []byte {
	switch ref {
	case "modern":
		return certBgModernPNG
	case "elegant":
		return certBgElegantPNG
	default:
		return certBgClassicPNG
	}
}

// certificateFieldValues assembles the fixed certificate copy plus the
// per-render student name, date, and certificate number shared by real
// generation and preview.
func certificateFieldValues(examTitle, studentName, dateStr, certNumber string) map[FieldID]string {
	return map[FieldID]string{
		"title":              "CERTIFICATE OF COMPLETION",
		"subtitle":           "This certificate is proudly awarded to",
		"student_name":       studentName,
		"completion_text":    "for successfully completing",
		"exam_title":         examTitle,
		"date":               dateStr,
		"certificate_number": certNumber,
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
func (s *Service) resolveCertificateSignatureImages(ctx context.Context, layout Layout) (map[FieldID][]byte, error) {
	if layout.SignatureKey == nil || *layout.SignatureKey == "" {
		return nil, nil
	}
	img, err := s.downloadCertificateBackground(ctx, *layout.SignatureKey)
	if err != nil {
		return nil, fmt.Errorf("download certificate signature: %w", err)
	}
	return map[FieldID][]byte{"signature": img}, nil
}

func (s *Service) downloadCertificateBackground(ctx context.Context, key string) ([]byte, error) {
	if s.storage == nil {
		return nil, errors.New("storage not configured")
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
		return "", errors.New("storage not configured")
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

// resolveCertificateURL determines a presigned certificate URL for a session,
// regenerating the PDF when missing, stale by grading, or stale by design edit
// (exam.certificate_design_updated_at newer than sess.certificate_generated_at,
// FR-13/C3). The DB stores the object key; a fresh time-limited GET is signed
// on every read since the bucket is private. A non-submitted session resolves
// to (nil, nil) and generates nothing (FR-16); generation/upload/persist
// failures are returned. Regeneration reuses the session's original
// certificate number — AllocateCertificateNumber is idempotent (FR-10).
func (s *Service) resolveCertificateURL(ctx context.Context, exam *model.Exam, sess *model.ExamSession, answers []model.ExamSessionAnswer, studentName string) (*string, error) {
	if sess.Status != "submitted" {
		return nil, nil
	}

	gradedAt := latestGradedAt(answers)
	designStale := exam.CertificateDesignUpdatedAt != nil && sess.CertificateGeneratedAt != nil &&
		exam.CertificateDesignUpdatedAt.After(*sess.CertificateGeneratedAt)

	// Regenerate when certificate is missing, grading is newer, or the design changed.
	if sess.CertificateKey == nil || sess.CertificateGeneratedAt == nil ||
		(gradedAt != nil && gradedAt.After(*sess.CertificateGeneratedAt)) || designStale {

		number, err := s.storeRepo.AllocateCertificateNumber(ctx, sess.ID)
		if err != nil {
			return nil, fmt.Errorf("allocate certificate number: %w", err)
		}
		layout, err := resolveCertificateLayout(exam)
		if err != nil {
			return nil, err
		}
		bg, err := s.resolveCertificateBackground(ctx, exam)
		if err != nil {
			return nil, fmt.Errorf("resolve certificate background: %w", err)
		}
		images, err := s.resolveCertificateSignatureImages(ctx, layout)
		if err != nil {
			return nil, err
		}

		loc, err := time.LoadLocation("Asia/Jakarta")
		if err != nil {
			return nil, err
		}
		dateStr := sess.SubmittedAt.In(loc).Format("2 January 2006")
		vals := certificateFieldValues(exam.Title, studentName, dateStr, number)

		html, err := buildCertificateHTML(layout, vals, bg, images)
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

	signed, err := s.presignReadURL(ctx, s.cfg.ObjectStorageBucketName, *sess.CertificateKey, time.Hour)
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

	images, err := s.resolveCertificateSignatureImages(ctx, layout)
	if err != nil {
		return nil, err
	}
	html, err := buildCertificateHTML(layout, vals, bg, images)
	if err != nil {
		return nil, fmt.Errorf("build certificate html: %w", err)
	}
	return s.renderer.RenderHTML(ctx, html)
}
