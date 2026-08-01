//go:build gotenberg_integration

// Replacement render gate (NFR-R4, FR-30) for certificate_integration_test.go:
// that file calls buildCertificateHTML directly, which Task 13 deleted the
// production call site for — certificates now render exclusively through the
// print route (renderCertificateThroughPrintRoute), so a gate that bypasses
// the print route no longer proves anything about what ships.
//
// This gate proves the real path end to end: it mints a print token into the
// SAME Redis and inserts an exam into the SAME Postgres the docker-compose
// `api` container reads (deploy/compose/local.yml), then asks a real
// Gotenberg to fetch the real running `web` container's /documents/certificate
// route — exactly what renderCertificateThroughPrintRoute does in production,
// down to the URL shape (certificatePrintRouteURL). No certificate HTML is
// built in this process.
//
// Unlike every other integration test in this package, it deliberately does
// NOT use an ephemeral testcontainers Postgres/Redis (see realdb_test.go,
// certificate_gate_db_test.go): those are invisible to the real `api`
// container, so a token or exam row written there would never be found when
// the print route's data endpoint (served by that container) tries to
// resolve them.
//
//	go test -tags gotenberg_integration -run 'TestCertificateRender_' -count=1 -v ./internal/service/
//
// Requires `deploy/compose/local.yml` up with a freshly built `web` image —
// the print routes this test exercises are not in a stale baked image (see
// deploy/pipeline/backend-render-gate.sh). Override RENDER_GATE_POSTGRES_DSN,
// RENDER_GATE_REDIS_ADDR, RENDER_GATE_WEB_INTERNAL_URL, or GOTENBERG_URL to
// point at a differently-addressed stack; defaults match local.yml's host
// port mappings and internal service DNS names.
package service

import (
	"bytes"
	"context"
	"net/http"
	"os"
	"os/exec"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"

	"akademi-bimbel/internal/model"
	"akademi-bimbel/internal/repository"
)

func renderGatePostgresDSN() string {
	if v := os.Getenv("RENDER_GATE_POSTGRES_DSN"); v != "" {
		return v
	}
	return "postgres://akademi:akademi@localhost:5432/akademi?sslmode=disable"
}

func renderGateRedisAddr() string {
	if v := os.Getenv("RENDER_GATE_REDIS_ADDR"); v != "" {
		return v
	}
	return "localhost:6379"
}

func renderGateGotenbergURL() string {
	if v := os.Getenv("GOTENBERG_URL"); v != "" {
		return v
	}
	return "http://localhost:3001"
}

// renderGateWebInternalURL is the address Gotenberg's headless Chromium
// resolves the print route from — inside the compose network, "web" is the
// service's DNS name (mirrors config.WebInternalURL in production; see
// config_test.go:337). It is NOT reachable from the host running this test,
// only from inside the Gotenberg container.
func renderGateWebInternalURL() string {
	if v := os.Getenv("RENDER_GATE_WEB_INTERNAL_URL"); v != "" {
		return v
	}
	return "http://web:3000"
}

// renderGateService connects to the real compose Postgres/Redis and fails
// loudly (not skips) when either is unreachable: this gate exists precisely
// so a broken render path cannot pass silently, and a stack that isn't up is
// exactly the same failure mode from the CI job's point of view — see
// deploy/pipeline/backend-render-gate.sh, which is what makes the stack's
// presence a hard precondition rather than an optional nicety.
func renderGateService(t *testing.T) *Service {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	pool, err := pgxpool.New(ctx, renderGatePostgresDSN())
	if err != nil {
		t.Fatalf("connect to compose postgres at %s: %v — is deploy/compose/local.yml up?", renderGatePostgresDSN(), err)
	}
	t.Cleanup(pool.Close)
	if err := pool.Ping(ctx); err != nil {
		t.Fatalf("compose postgres not reachable at %s: %v — is deploy/compose/local.yml up?", renderGatePostgresDSN(), err)
	}

	rdb := redis.NewClient(&redis.Options{Addr: renderGateRedisAddr()})
	t.Cleanup(func() { _ = rdb.Close() })
	if err := rdb.Ping(ctx).Err(); err != nil {
		t.Fatalf("compose redis not reachable at %s: %v — is deploy/compose/local.yml up?", renderGateRedisAddr(), err)
	}

	return &Service{storeRepo: repository.New(pool), rdb: rdb}
}

// renderGateCreateExam inserts a bare exam row directly into the compose
// Postgres so the real `api` container's GetCertificatePreviewData can
// resolve it by id — it needs nothing beyond a title (FR-29 preview never
// reads session/score data).
func renderGateCreateExam(t *testing.T, svc *Service, title string) model.Exam {
	t.Helper()
	exam, err := svc.CreateExam(context.Background(), model.Exam{Title: title})
	if err != nil {
		t.Fatalf("CreateExam: %v", err)
	}
	return exam
}

// renderGateRenderCertificate mints a certificate preview print token bound
// to examID (redeemable exactly once, exactly like a real generation — see
// print_token.go) and asks the real Gotenberg to render the real web
// container's print route for it, mirroring
// Service.renderCertificatePreviewPDF/renderCertificateThroughPrintRoute
// (certificate.go) instead of calling buildCertificateHTML.
func renderGateRenderCertificate(t *testing.T, svc *Service, examID uuid.UUID, layout *Layout) []byte {
	t.Helper()
	token, err := svc.MintCertificatePreviewToken(context.Background(), examID.String(), "", layout)
	if err != nil {
		t.Fatalf("MintCertificatePreviewToken: %v", err)
	}

	printURL := certificatePrintRouteURL(renderGateWebInternalURL(), token, examID.String())
	renderer := newGotenbergPDFGenerator(renderGateGotenbergURL(), &http.Client{Timeout: 30 * time.Second})
	pdf, err := renderer.RenderURL(context.Background(), printURL)
	if err != nil {
		t.Fatalf("RenderURL(%s) against gotenberg %s: %v", printURL, renderGateGotenbergURL(), err)
	}

	if !bytes.HasPrefix(pdf, []byte("%PDF-")) {
		t.Fatalf("output is not a PDF (no %%PDF- header); first bytes: %q", pdf[:min(16, len(pdf))])
	}
	if !bytes.Contains(pdf, []byte("%%EOF")) {
		t.Fatalf("PDF missing %%%%EOF trailer — likely truncated (%d bytes)", len(pdf))
	}
	if len(pdf) < 1024 {
		t.Fatalf("PDF suspiciously small (%d bytes) — expected a real rendered certificate", len(pdf))
	}
	return pdf
}

// TestCertificateRender_PrintRoute is the print-route successor to the old
// TestCertificateRender_RealGotenberg: same background/ink assertions, but
// the PDF comes from a real Gotenberg fetching the real running web
// container's /documents/certificate route through the real print-token/
// print-data machinery, not from buildCertificateHTML called in-process.
func TestCertificateRender_PrintRoute(t *testing.T) {
	if _, err := exec.LookPath("pdftoppm"); err != nil {
		t.Fatal("pdftoppm not installed: the render gate cannot verify layout without it")
	}
	svc := renderGateService(t)
	exam := renderGateCreateExam(t, svc, "Render Gate Print Route "+uuid.NewString())

	layout := defaultLayout("classic")
	pdf := renderGateRenderCertificate(t, svc, exam.ID, &layout)
	img := renderToPNG(t, pdf)

	bounds := img.Bounds()
	if bounds.Dx() <= bounds.Dy() {
		t.Errorf("expected A4 landscape, got %dx%d px", bounds.Dx(), bounds.Dy())
	}

	// The classic background's navy band runs along the top edge; a blank or
	// background-less render leaves this area white.
	r, g, b := avgColorAt(img, certificatePageWidthMm, certificatePageHeightMm, 148, 8)
	if r > 200 && g > 200 && b > 200 {
		t.Errorf("top band is near-white (%.0f,%.0f,%.0f) — background did not render", r, g, b)
	}

	var nameField LayoutField
	for _, f := range layout.Fields {
		if f.ID == "student_name" {
			nameField = f
		}
	}
	if nameField.ID == "" {
		t.Fatal("classic layout has no student_name field")
	}
	nameInk := regionMinBrightness(img, certificatePageWidthMm, certificatePageHeightMm,
		nameField.XMm, nameField.YMm, nameField.XMm+nameField.WMm, nameField.YMm+nominalLineHeightMm(nameField.SizePt))
	if nameInk > 600 {
		t.Errorf("no glyph ink in the student_name box (darkest pixel %.0f/765) — the field did not render where the layout puts it", nameInk)
	}
}

// TestCertificateRender_PrintRouteInvalidTokenFailsRenderURL is the real-stack
// proof behind the NFR-R1 fix: before documents/certificate/page.tsx was
// changed to call notFound() on failure, a rejected token still made the
// print route answer 200 with an empty body, which a real Gotenberg happily
// rasterized into a valid, empty PDF — RenderURL returned no error, and
// resolveCertificateURL (certificate.go) uploaded and cached that blank PDF
// permanently. This hits the real running `web` and `gotenberg` containers
// with a token that was never minted, so RenderURL must now surface an error
// instead of PDF bytes; renderCertificateThroughPrintRoute's own unit test
// (TestRenderCertificateThroughPrintRoute_RenderURLFailure_PropagatesError)
// already proves that error propagates out and blocks the upload/persist —
// this test is what makes that propagation happen for real print-route
// failures, not just a fake renderer.
func TestCertificateRender_PrintRouteInvalidTokenFailsRenderURL(t *testing.T) {
	printURL := certificatePrintRouteURL(renderGateWebInternalURL(), "not-a-real-token", uuid.NewString())
	renderer := newGotenbergPDFGenerator(renderGateGotenbergURL(), &http.Client{Timeout: 30 * time.Second})

	pdf, err := renderer.RenderURL(context.Background(), printURL)
	if err == nil {
		t.Fatalf("RenderURL succeeded for an invalid print token (%d PDF bytes) — a rejected token must not resolve to a cacheable PDF (NFR-R1)", len(pdf))
	}
}

// TestCardRender_PrintRouteInvalidTokenFailsRenderURL is the exam-card
// counterpart of TestCertificateRender_PrintRouteInvalidTokenFailsRenderURL —
// the blocker requires the fix to hold for both documents. See that test's
// comment for the full failure mechanism this guards against.
//
// Not prefixed TestCertificateRender_: deploy/pipeline/backend-render-gate.sh
// matches both TestCertificateRender_ and TestCardRender_ explicitly so this
// can't silently sit unrun the way the old gate did.
func TestCardRender_PrintRouteInvalidTokenFailsRenderURL(t *testing.T) {
	printURL := cardPrintRouteURL(renderGateWebInternalURL(), "not-a-real-token", uuid.NewString())
	renderer := newGotenbergPDFGenerator(renderGateGotenbergURL(), &http.Client{Timeout: 30 * time.Second})

	pdf, err := renderer.RenderURL(context.Background(), printURL)
	if err == nil {
		t.Fatalf("RenderURL succeeded for an invalid print token (%d PDF bytes) — a rejected token must not resolve to a cacheable PDF (NFR-R1)", len(pdf))
	}
}

// TestCertificateRender_PrintRouteDraggedFieldLandsWhereDragged is the
// print-route successor to the old
// TestCertificateRender_DraggedFieldLandsWhereDragged (FR-30): a field
// dragged to the lower-left of the page discriminates a Y-axis inversion,
// which the default layout's centred student_name cannot.
//
// This is the run's mutation-proof assertion: shifting draggedXMm by 30mm
// (see task_14_result.json for the recorded failing run) moves the expected
// ink location off of where the field actually renders, and the test goes
// red — proof this gate can fail, not just pass.
func TestCertificateRender_PrintRouteDraggedFieldLandsWhereDragged(t *testing.T) {
	if _, err := exec.LookPath("pdftoppm"); err != nil {
		t.Fatal("pdftoppm not installed: the render gate cannot verify layout without it")
	}
	svc := renderGateService(t)
	exam := renderGateCreateExam(t, svc, "Render Gate Dragged Field "+uuid.NewString())

	const (
		draggedXMm = 20
		draggedYMm = 175
		draggedWMm = 90
		sizePt     = 20
		// probeWMm is deliberately narrower than draggedWMm: it brackets only the
		// first few glyphs of the left-aligned text, close enough to draggedXMm
		// that a real (not just Y-mirrored) X shift of the field is caught too —
		// a probe as wide as the whole 90mm box would still overlap a 30mm shift
		// and pass. See the mutation check recorded in task_14_result.json.
		probeWMm = 25
		// probeInkThreshold is tighter than the 600 the old byte-agnostic gate
		// used: measured on this exact background region, real glyph ink reads
		// ~0/765 and the classic background's own texture there reads ~377/765 —
		// 600 would let a moved-away field pass on background alone.
		probeInkThreshold = 150
	)
	layout := Layout{
		Page:       Page{WidthMm: certificatePageWidthMm, HeightMm: certificatePageHeightMm},
		Background: Background{Kind: "builtin", Ref: "classic"},
		Fields: []LayoutField{{
			ID: "student_name", XMm: draggedXMm, YMm: draggedYMm, WMm: draggedWMm,
			Align: "left", Font: "public_sans", Weight: "bold", SizePt: sizePt,
			Color: "#000000", Visible: true,
		}},
	}

	pdf := renderGateRenderCertificate(t, svc, exam.ID, &layout)
	img := renderToPNG(t, pdf)

	boxH := nominalLineHeightMm(sizePt)
	inkAtDrop := regionMinBrightness(img, certificatePageWidthMm, certificatePageHeightMm,
		draggedXMm, draggedYMm, draggedXMm+probeWMm, draggedYMm+boxH)
	if inkAtDrop > probeInkThreshold {
		t.Errorf("no ink where the field was dropped (%.0fmm,%.0fmm), darkest pixel %.0f/765", float64(draggedXMm), float64(draggedYMm), inkAtDrop)
	}

	mirroredY := certificatePageHeightMm - draggedYMm - boxH
	inkAtMirror := regionMinBrightness(img, certificatePageWidthMm, certificatePageHeightMm,
		draggedXMm, mirroredY, draggedXMm+probeWMm, mirroredY+boxH)
	if inkAtMirror < inkAtDrop {
		t.Errorf("more ink at the mirrored position (y=%.0fmm, %.0f) than where the field was dropped (y=%.0fmm, %.0f) — the Y axis is inverted",
			mirroredY, inkAtMirror, float64(draggedYMm), inkAtDrop)
	}
}
