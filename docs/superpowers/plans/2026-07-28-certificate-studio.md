# Certificate Studio Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace the exam certificate config page with a layered, token-aware, multi-image Certificate Studio whose saved JSON is rendered unchanged for students.

**Architecture:** Extend the existing certificate layout JSON in place, normalize legacy fields at the service boundary, and keep millimetres as the shared browser/renderer geometry. The frontend owns editing state and direct manipulation; the backend owns preset data, token validation/substitution, private image resolution, result visibility, and final HTML/PDF generation.

**Tech Stack:** Go 1.26.3, Echo, PostgreSQL JSONB, MinIO, Gotenberg HTML rendering, Next.js 15, React 19, TypeScript, Tailwind CSS 4, Vitest, Testing Library.

## Global Constraints

- No database migration: `exam.certificate_design` remains the persistence source.
- Persist object keys only; never persist presigned or raw URLs.
- A4 landscape is exactly `297 mm × 210 mm`, with top-left origin and Y increasing downward.
- Unknown tokens fail save with `Unknown certificate token: {{token_name}}`.
- Result-sensitive tokens must not expose a certificate before the existing full-result gate.
- Keep the existing PDF preview endpoint for compatibility, but remove it from the admin UI.
- Preserve existing certificate layouts through read-time normalization.
- Do not modify unrelated code or `docs/backlog/register.md`.
- Prefix every Go command with `export GOROOT=/opt/homebrew/Cellar/go/1.26.3/libexec`.

---

## File Structure

- `backend/internal/service/certificate_layout.go`: extended layer schema, defaults, normalization, validation, token metadata.
- `backend/internal/service/certificate_layout_test.go`: schema, legacy normalization, token and geometry tests.
- `backend/internal/service/certificate.go`: token values, asset download, result-sensitive generation, final render inputs.
- `backend/internal/service/certificate_test.go`: token derivation, gating, multiple image, regeneration tests.
- `backend/internal/service/certificate_html.go`: ordered text/image layer HTML rendering and italic styling.
- `backend/internal/service/certificate_html_test.go`: HTML paint order, style, escaping tests.
- `backend/internal/service/exam.go`: preset/background/image URL read model.
- `backend/internal/service/exam_test.go`: preset and private-key/read-URL tests.
- `backend/internal/handler/exam_certificate_handler_test.go`: GET/PUT wire contract regression tests.
- `backend/internal/service/exam_result.go`: pass question/result visibility context into certificate generation.
- `web/lib/types.ts`: new layer, preset, and asset URL types.
- `web/lib/i18n.ts`: Indonesian and English Certificate Studio copy.
- `web/lib/certificate-studio.ts`: pure normalization, layer creation, layer ordering, scaling, and placeholder helpers.
- `web/lib/certificate-studio.test.ts`: pure frontend model tests.
- `web/components/admin/CertificateFieldEditor.tsx`: accurate canvas, selection, drag, resize, keyboard movement, zoom.
- `web/components/admin/CertificateFieldEditor.test.tsx`: geometry and scale tests.
- `web/components/admin/CertificateInspector.tsx`: content/token/typography/image controls.
- `web/components/admin/CertificateDesignTab.tsx`: command bar, presets, element catalog, layers, upload, dirty state, save.
- `web/components/admin/CertificateDesignTab.test.tsx`: complete builder behavior and payload tests.

---

### Task 1: Extend and Normalize the Backend Layout Contract

**Files:**
- Modify: `backend/internal/service/certificate_layout.go`
- Modify: `backend/internal/service/certificate_layout_test.go`

**Interfaces:**
- Produces: `normalizeCertificateLayout(Layout) Layout`
- Produces: `defaultCertificateContent(fieldID string) string`
- Produces: `certificateTokens(content string) []string`
- Extends: `LayoutField` with `Kind`, `Name`, `Content`, `Italic`, and `AssetKey`
- Preserves: legacy `Layout.SignatureKey` as read-only compatibility input

- [ ] **Step 1: Write failing legacy-normalization and validation tests**

```go
func TestNormalizeCertificateLayout_LegacyStudentNameGetsTokenContent(t *testing.T) {
	layout := Layout{Page: Page{WidthMm: 297, HeightMm: 210}, Fields: []LayoutField{{
		ID: "student_name", XMm: 48.5, YMm: 100, WMm: 200,
		Align: "center", Font: "source_serif_4", SizePt: 26, Visible: true,
	}}}
	got := normalizeCertificateLayout(layout)
	if got.Fields[0].Kind != "text" || got.Fields[0].Content != "{{student_name}}" {
		t.Fatalf("normalized field = %+v", got.Fields[0])
	}
}

func TestValidateLayout_RejectsUnknownToken(t *testing.T) {
	layout := defaultLayout("classic")
	layout.Fields[0].Content = "{{secret_score}}"
	err := ValidateLayout(normalizeCertificateLayout(layout))
	if err == nil || !strings.Contains(err.Error(), "Unknown certificate token: {{secret_score}}") {
		t.Fatalf("err = %v", err)
	}
}

func TestValidateLayout_AllowsGeneratedImageID(t *testing.T) {
	layout := Layout{Page: Page{WidthMm: 297, HeightMm: 210}, Fields: []LayoutField{{
		ID: "image_550e8400-e29b-41d4-a716-446655440000", Kind: "image",
		AssetKey: strPtr("certificates/assets/logo.png"),
		XMm: 10, YMm: 10, WMm: 30, HMm: 30, Visible: true,
	}}}
	if err := ValidateLayout(layout); err != nil {
		t.Fatal(err)
	}
}
```

- [ ] **Step 2: Run the targeted tests and confirm RED**

Run:

```bash
cd backend
export GOROOT=/opt/homebrew/Cellar/go/1.26.3/libexec && go test -count=1 -run 'TestNormalizeCertificateLayout|TestValidateLayout_RejectsUnknownToken|TestValidateLayout_AllowsGeneratedImageID' ./internal/service
```

Expected: compile or assertion failure because the new fields/helpers do not exist.

- [ ] **Step 3: Implement the minimum normalized schema**

```go
type LayoutField struct {
	ID       string  `json:"id"`
	Kind     string  `json:"kind,omitempty"`
	Name     string  `json:"name,omitempty"`
	Content  string  `json:"content,omitempty"`
	XMm      float64 `json:"x_mm"`
	YMm      float64 `json:"y_mm"`
	WMm      float64 `json:"w_mm"`
	HMm      float64 `json:"h_mm,omitempty"`
	Align    string  `json:"align"`
	Font     string  `json:"font,omitempty"`
	Weight   string  `json:"weight,omitempty"`
	Italic   bool    `json:"italic,omitempty"`
	SizePt   float64 `json:"size_pt,omitempty"`
	Color    string  `json:"color,omitempty"`
	Visible  bool    `json:"visible"`
	AssetKey *string `json:"asset_key,omitempty"`
}
```

Infer legacy image fields from `logo`/`signature`; infer every other existing
field as text. Default content uses the approved token/static copy table.
Generated IDs must match `^(text|image)_[0-9a-f-]{36}$`. Validate known fonts,
alignment, `#RRGGBB`, geometry, image dimensions/key, text width/size/content,
duplicate IDs, and the closed token catalog.

- [ ] **Step 4: Run all layout tests and confirm GREEN**

Run:

```bash
cd backend
export GOROOT=/opt/homebrew/Cellar/go/1.26.3/libexec && go test -count=1 ./internal/service -run 'Layout|CertificateToken'
```

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add backend/internal/service/certificate_layout.go backend/internal/service/certificate_layout_test.go
git commit -m "feat: extend certificate layer contract"
```

---

### Task 2: Render Ordered Text and Multiple Image Layers

**Files:**
- Modify: `backend/internal/service/certificate_html.go`
- Modify: `backend/internal/service/certificate_html_test.go`
- Modify: `backend/internal/service/certificate.go`
- Modify: `backend/internal/service/certificate_test.go`

**Interfaces:**
- Consumes: normalized `LayoutField.Kind`, `Content`, `Italic`, `AssetKey`
- Produces: `resolveCertificateImages(ctx, layout) (map[FieldID][]byte, error)`
- Produces: `interpolateCertificateContent(content string, values map[string]string) string`
- Preserves: `buildCertificateHTML(layout, values, background, images)`

- [ ] **Step 1: Write failing interpolation, image-order, and italic tests**

```go
func TestInterpolateCertificateContent_MixedStaticAndTokens(t *testing.T) {
	got := interpolateCertificateContent(
		"Awarded to {{student_name}} for {{exam_title}}",
		map[string]string{"student_name": "Budi", "exam_title": "Matematika"},
	)
	if got != "Awarded to Budi for Matematika" {
		t.Fatalf("got %q", got)
	}
}

func TestBuildCertificateHTML_PreservesLayerPaintOrder(t *testing.T) {
	layout := normalizedTestLayout([]LayoutField{
		{ID: "text_550e8400-e29b-41d4-a716-446655440000", Kind: "text", Content: "Front", Visible: true},
		{ID: "image_550e8400-e29b-41d4-a716-446655440001", Kind: "image", Visible: true},
	})
	html := mustBuildHTML(t, layout, map[string]string{}, map[FieldID][]byte{
		"image_550e8400-e29b-41d4-a716-446655440001": []byte("png"),
	})
	if strings.Index(html, "Front") > strings.Index(html, "data:image") {
		t.Fatal("HTML did not preserve layout field order")
	}
}
```

Add a style assertion that an italic text layer emits `font-style:italic`.

- [ ] **Step 2: Run targeted tests and confirm RED**

Run:

```bash
cd backend
export GOROOT=/opt/homebrew/Cellar/go/1.26.3/libexec && go test -count=1 -run 'TestInterpolateCertificateContent|TestBuildCertificateHTML_PreservesLayerPaintOrder|Italic' ./internal/service
```

Expected: FAIL because content interpolation and generic image resolution are absent.

- [ ] **Step 3: Implement ordered generic rendering**

Normalize the layout before rendering. Iterate `layout.Fields` once and append
each visible text or image view in array order. Text renders interpolated
`Content`; image bytes come from the field ID. Replace
`resolveCertificateSignatureImages` with `resolveCertificateImages`, downloading
each unique `AssetKey`. Map legacy `signature_key` to the normalized signature
layer before resolution.

- [ ] **Step 4: Run certificate HTML/service tests and confirm GREEN**

Run:

```bash
cd backend
export GOROOT=/opt/homebrew/Cellar/go/1.26.3/libexec && go test -count=1 -run 'Certificate|Layout' ./internal/service
```

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add backend/internal/service/certificate.go backend/internal/service/certificate_test.go backend/internal/service/certificate_html.go backend/internal/service/certificate_html_test.go
git commit -m "feat: render layered certificate content"
```

---

### Task 3: Add Presets and Private Asset URLs to the Design API

**Files:**
- Modify: `backend/internal/service/exam.go`
- Modify: `backend/internal/service/exam_test.go`
- Modify: `backend/internal/handler/exam_certificate_handler_test.go`

**Interfaces:**
- Produces: `CertificatePresetResponse{Template, Layout, BackgroundURL}`
- Extends: `CertificateDesignResponse` with `Presets []CertificatePresetResponse` and `AssetURLs map[string]string`
- Consumes: normalized layouts from Task 1

- [ ] **Step 1: Write failing service and handler contract tests**

```go
func TestGetCertificateDesign_ReturnsAllBuiltinPresets(t *testing.T) {
	got := mustGetCertificateDesign(t, seededExamID)
	if len(got.Presets) != 3 {
		t.Fatalf("presets = %d", len(got.Presets))
	}
	for _, preset := range got.Presets {
		if !strings.HasPrefix(preset.BackgroundURL, "data:image/png;base64,") {
			t.Fatalf("background = %q", preset.BackgroundURL)
		}
	}
}

func TestGetCertificateDesign_PresignsEachImageLayerWithoutExposingKeyAsURL(t *testing.T) {
	got := mustGetDesignWithImage(t, "image_550e8400-e29b-41d4-a716-446655440001", "certificates/assets/logo.png")
	if got.AssetURLs["image_550e8400-e29b-41d4-a716-446655440001"] == "certificates/assets/logo.png" {
		t.Fatal("raw object key exposed as URL")
	}
}
```

The handler JSON test asserts `presets` and `asset_urls` are present and that
the PUT response remains the normal exam update response.

- [ ] **Step 2: Run targeted tests and confirm RED**

Run:

```bash
cd backend
export GOROOT=/opt/homebrew/Cellar/go/1.26.3/libexec && go test -count=1 -run 'TestGetCertificateDesign_.*Preset|TestGetCertificateDesign_.*ImageLayer|AdminGetExamCertificateDesign' ./internal/service ./internal/handler
```

Expected: FAIL because the read model lacks presets and asset URLs.

- [ ] **Step 3: Implement the read model**

Build exactly three presets from `defaultLayout("classic"|"modern"|"elegant")`
and `builtinCertificateBackground`, encoded as PNG data URLs. Normalize saved
layout before returning it. Presign each non-empty image `AssetKey` into
`AssetURLs[field.ID]`. Preserve custom `background_url` behavior.

- [ ] **Step 4: Run service and handler certificate tests**

Run:

```bash
cd backend
export GOROOT=/opt/homebrew/Cellar/go/1.26.3/libexec && go test -count=1 -run 'CertificateDesign|CertificateLayout' ./internal/service ./internal/handler
```

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add backend/internal/service/exam.go backend/internal/service/exam_test.go backend/internal/handler/exam_certificate_handler_test.go
git commit -m "feat: expose certificate presets and assets"
```

---

### Task 4: Derive Session Tokens and Enforce Result Visibility

**Files:**
- Modify: `backend/internal/service/certificate.go`
- Modify: `backend/internal/service/certificate_test.go`
- Modify: `backend/internal/service/exam_result.go`
- Modify: `backend/internal/service/exam_result_test.go`

**Interfaces:**
- Produces: `certificateRenderValues(exam, session, tests, studentName, certificateNumber, higherCount, fullyGradedCount) map[string]string`
- Produces: `layoutUsesResultSensitiveTokens(layout Layout) bool`
- Changes: `resolveCertificateURL` to receive `tests []model.TestDetail` and `resultVisible bool`

- [ ] **Step 1: Write failing deterministic token tests**

```go
func TestCertificateRenderValues_DerivesResultTokens(t *testing.T) {
	started := time.Date(2026, 7, 28, 10, 0, 0, 0, time.UTC)
	submitted := started.Add(89*time.Minute + time.Second)
	session := &model.ExamSession{StartedAt: started, SubmittedAt: &submitted, Score: floatPtr(86)}
	tests := testDetailsWithPointValues(40, 60)
	got := certificateRenderValues(&model.Exam{Title: "Math"}, session, tests, "Budi", "ABK/1", 2, 20)
	assert.Equal(t, "86", got["score"])
	assert.Equal(t, "100", got["max_score"])
	assert.Equal(t, "86%", got["score_percent"])
	assert.Equal(t, "3", got["rank"])
	assert.Equal(t, "Top 15%", got["percentile"])
	assert.Equal(t, "90 minutes", got["duration"])
	assert.Equal(t, "2 questions", got["total_questions"])
}
```

Add result-gate tests:

- a score-token layout returns no certificate while locked;
- a completion-only layout still returns its certificate while locked;
- the score-token layout generates after the full result gate.

- [ ] **Step 2: Run targeted tests and confirm RED**

Run:

```bash
cd backend
export GOROOT=/opt/homebrew/Cellar/go/1.26.3/libexec && go test -count=1 -run 'CertificateRenderValues|SensitiveCertificate|CompletionCertificate' ./internal/service
```

Expected: FAIL because the values and gate do not exist.

- [ ] **Step 3: Implement values and gate**

Flatten questions once, sum `PointCorrect`, count questions, format numeric
values without unnecessary zeroes, round score percentage to the nearest whole
number, compute rank and percentile per the spec, and ceil elapsed minutes.
Restructure `GetSessionResult` so it computes the existing gate before
certificate generation and passes `resultVisible`. Query ranking counts only
when the layout contains rank/percentile and the full result is visible.

- [ ] **Step 4: Run result and certificate tests**

Run:

```bash
cd backend
export GOROOT=/opt/homebrew/Cellar/go/1.26.3/libexec && go test -count=1 -run 'Certificate|GetSessionResult|ResultGate' ./internal/service
```

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add backend/internal/service/certificate.go backend/internal/service/certificate_test.go backend/internal/service/exam_result.go backend/internal/service/exam_result_test.go
git commit -m "feat: render dynamic certificate result data"
```

---

### Task 5: Build the Pure Frontend Studio Model

**Files:**
- Modify: `web/lib/types.ts`
- Create: `web/lib/certificate-studio.ts`
- Create: `web/lib/certificate-studio.test.ts`

**Interfaces:**
- Produces: `normalizeCertificateLayout(layout): CertificateLayout`
- Produces: `createTextLayer(content, name): CertificateLayoutField`
- Produces: `createImageLayer(assetKey, name): CertificateLayoutField`
- Produces: `moveLayer(fields, id, direction): CertificateLayoutField[]`
- Produces: `certificatePreviewValue(content, values): string`
- Extends: `CertificateDesign` with `presets` and `asset_urls`

- [ ] **Step 1: Write failing pure model tests**

```ts
it("normalizes a legacy student field into editable token content", () => {
  const layout = normalizeCertificateLayout(legacyLayout);
  expect(layout.fields[0]).toMatchObject({
    kind: "text",
    content: "{{student_name}}",
  });
});

it("creates independent image layers with stable generated ids", () => {
  const first = createImageLayer("certificates/assets/a.png", "Logo");
  const second = createImageLayer("certificates/assets/b.png", "Icon");
  expect(first.id).toMatch(/^image_/);
  expect(first.id).not.toBe(second.id);
});

it("moves a layer one paint step without changing other layers", () => {
  expect(moveLayer([a, b, c], "b", "forward")).toEqual([a, c, b]);
});
```

- [ ] **Step 2: Run the model test and confirm RED**

Run:

```bash
cd web
npx vitest run lib/certificate-studio.test.ts
```

Expected: FAIL because the module does not exist.

- [ ] **Step 3: Implement minimal pure helpers and types**

Use `crypto.randomUUID()` for `text_<uuid>` and `image_<uuid>`. Keep the
approved default geometry exact. Do not put React state or uploads in this
module.

- [ ] **Step 4: Run model tests and TypeScript**

Run:

```bash
cd web
npx vitest run lib/certificate-studio.test.ts
npx tsc --noEmit
```

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add web/lib/types.ts web/lib/certificate-studio.ts web/lib/certificate-studio.test.ts
git commit -m "feat: add certificate studio model"
```

---

### Task 6: Replace the Canvas with Correct Scaling, Drag, Resize, and Keyboard Movement

**Files:**
- Rewrite: `web/components/admin/CertificateFieldEditor.tsx`
- Rewrite: `web/components/admin/CertificateFieldEditor.test.tsx`
- Modify: `web/components/admin/CertificateFonts.module.css`

**Interfaces:**
- Consumes: normalized `CertificateLayout`, selected layer ID, background URL, asset URLs, placeholder values
- Produces: `onChange(fields)`, `onSelect(fieldID)`, and direct canvas interactions

- [ ] **Step 1: Write failing visual-geometry behavior tests**

Test a 594 px-wide canvas (`2 px/mm`) and assert:

```ts
expect(student.style.left).toBe("97px");      // 48.5 mm
expect(student.style.top).toBe("200px");      // 100 mm
expect(text.style.fontSize).toBe("18.3456px"); // 26 pt × 0.3528 mm/pt × 2 px/mm
```

Also test:

- drag updates `x_mm/y_mm`;
- image resize updates `w_mm/h_mm` and clamps to page;
- arrow keys move by 1 mm;
- hidden fields are not painted;
- array order is DOM paint order;
- selected field has the proof-gold treatment;
- text overflow uses the same shrink behavior contract.

- [ ] **Step 2: Run the canvas test and confirm RED**

Run:

```bash
cd web
npx vitest run components/admin/CertificateFieldEditor.test.tsx
```

Expected: FAIL against the existing unscaled editor.

- [ ] **Step 3: Implement the minimum accurate canvas**

Measure canvas width with `ResizeObserver`, calculate `pxPerMm`, and express all
geometry in pixels from that single scale. Convert points with
`sizePt × 0.3528 × pxPerMm`. Add selection, pointer drag, bottom-right image
resize, arrow-key movement, fit/zoom controls, and visible focus states.
Remove the X/Y input list.

- [ ] **Step 4: Run canvas/model tests**

Run:

```bash
cd web
npx vitest run components/admin/CertificateFieldEditor.test.tsx lib/certificate-studio.test.ts
```

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add web/components/admin/CertificateFieldEditor.tsx web/components/admin/CertificateFieldEditor.test.tsx web/components/admin/CertificateFonts.module.css
git commit -m "feat: rebuild certificate canvas interactions"
```

---

### Task 7: Build the Certificate Studio Shell and Inspector

**Files:**
- Create: `web/components/admin/CertificateInspector.tsx`
- Rewrite: `web/components/admin/CertificateDesignTab.tsx`
- Rewrite: `web/components/admin/CertificateDesignTab.test.tsx`
- Modify: `web/lib/i18n.ts`

**Interfaces:**
- Consumes: existing design hooks and upload presign hook
- Consumes: Task 5 model and Task 6 canvas
- Produces: the existing `CertificateDesignInput` PUT payload

- [ ] **Step 1: Write failing builder tests**

Cover these user behaviors:

```ts
it("edits selected text content and typography in the saved payload", async () => {
  renderStudio();
  await user.click(screen.getByText("Student name"));
  await user.clear(screen.getByLabelText("Content"));
  await user.type(screen.getByLabelText("Content"), "Awarded to {{student_name}}");
  await user.click(screen.getByRole("button", { name: "Bold" }));
  await user.click(screen.getByRole("button", { name: "Save changes" }));
  expect(mockUpdate).toHaveBeenCalledWith(expect.objectContaining({
    layout: expect.objectContaining({
      fields: expect.arrayContaining([
        expect.objectContaining({ content: "Awarded to {{student_name}}", weight: "bold" }),
      ]),
    }),
  }));
});
```

Add tests for:

- preset selection replaces background/layout after confirmation;
- custom background upload preserves layers and sets template `custom`;
- two image uploads create two independent asset-key layers;
- image replace/delete/reorder works;
- token picker inserts at the text caret;
- dirty/saved state changes correctly;
- failed upload/save preserves working state;
- no "Generate PDF", iframe, or coordinate inputs exist;
- legacy layout opens editable;
- tablet layout keeps save control reachable.

- [ ] **Step 2: Run the builder test and confirm RED**

Run:

```bash
cd web
npx vitest run components/admin/CertificateDesignTab.test.tsx
```

Expected: FAIL because the current page has the old two-column form and PDF UI.

- [ ] **Step 3: Implement the studio**

Build the sticky command bar, left preset/element/layer panel, navy canvas
worktop, and right inspector. Use existing UI primitives and Lucide icons.
Maintain one normalized working layout, selected ID, asset URL map, dirty flag,
upload flags, and template/background state. Use `window.confirm` only when a
dirty preset change would replace work. Keep i18n parity for every new label.

- [ ] **Step 4: Run all Certificate Studio frontend tests**

Run:

```bash
cd web
npx vitest run lib/certificate-studio.test.ts components/admin/CertificateFieldEditor.test.tsx components/admin/CertificateDesignTab.test.tsx
npx tsc --noEmit
```

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add web/components/admin/CertificateInspector.tsx web/components/admin/CertificateDesignTab.tsx web/components/admin/CertificateDesignTab.test.tsx web/lib/i18n.ts
git commit -m "feat: replace certificate config with studio"
```

---

### Task 8: Full Verification and Visual Critique

**Files:**
- Modify only files directly required by failures found in this task.

**Interfaces:**
- Verifies all previous tasks as one system.

- [ ] **Step 1: Run focused backend and frontend suites**

```bash
cd backend
export GOROOT=/opt/homebrew/Cellar/go/1.26.3/libexec && go test -race -shuffle=on -count=1 ./internal/service ./internal/handler
cd ../web
npx vitest run lib/certificate-studio.test.ts components/admin/CertificateFieldEditor.test.tsx components/admin/CertificateDesignTab.test.tsx
```

- [ ] **Step 2: Run full static and build gates**

```bash
cd backend
export GOROOT=/opt/homebrew/Cellar/go/1.26.3/libexec && go test -race -shuffle=on -count=1 ./...
export GOROOT=/opt/homebrew/Cellar/go/1.26.3/libexec && go vet ./...
cd ../web
npm run test:run
npx tsc --noEmit
npm run build
```

- [ ] **Step 3: Inspect the live UI**

Open the admin certificate page with the local app, capture desktop and tablet
screenshots, and verify:

- no text overlap at Fit, 100%, or tablet width;
- template thumbnails load matching artwork/layout;
- the dark proofing worktop is the only dominant visual gesture;
- panels remain readable and the certificate stays the focus;
- keyboard focus is visible;
- save remains reachable;
- PDF preview UI is absent.

- [ ] **Step 4: Verify save/reload and generated output**

Create a layout with:

- edited title;
- `{{student_name}}`, `{{score_percent}}`, and `{{total_questions}}`;
- one logo and one signature;
- reordered layers.

Save, reload, and confirm JSON-equivalent layer order/content/style/geometry.
Generate the student's certificate through the result flow and compare it with
the canvas for content, order, style, image placement, and background.

- [ ] **Step 5: Review the final diff and commit only direct fixes**

```bash
git diff --check
git status --short
git diff --stat
```

Do not add `docs/backlog/register.md`. If verification required fixes, commit
only those files with:

```bash
git commit -m "fix: align certificate studio rendering"
```
