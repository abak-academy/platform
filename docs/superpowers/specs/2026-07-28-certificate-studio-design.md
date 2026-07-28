# Certificate Studio Design

**Date:** 2026-07-28  
**Status:** Approved design, pending implementation plan

## 1. Goal

Replace the current exam certificate configuration page with a desktop-first
certificate builder where an admin can:

- start from a built-in certificate preset or upload a custom background;
- add only the text, result data, and images needed by that exam;
- edit text and typography directly;
- position and resize layers on an accurate A4 landscape canvas;
- save one layout contract that the student certificate generator renders;
- work without a separate PDF preview inside the admin configuration page.

The page's single job is composing the certificate students will receive after
an exam. It is not a general-purpose design application.

## 2. Current Problems

The existing page has four connected failures:

1. Browser preview text uses unscaled CSS point sizes inside a responsive
   canvas. When the canvas shrinks, the text does not shrink with it and fields
   overlap.
2. Switching `classic`, `modern`, or `elegant` updates the selected template
   name but does not load that preset's background and layout into the editor.
3. The persisted layout stores position and style but not editable copy. The
   renderer supplies fixed strings, so admins cannot change the wording.
4. Images are modelled as a fixed signature slot instead of reusable layers.

The manual X/Y form and embedded PDF preview expose implementation details
without solving the admin's primary task.

## 3. Chosen Approach

Build a three-pane "Certificate Studio":

```text
┌ Certificate builder                         Unsaved • Save changes ┐
├───────────────────────────────────────────────────────────────────┤
│ Elements and layers │        Live certificate         │ Inspector │
│                     │                                 │           │
│ Templates           │    drag • select • resize       │ Content   │
│ + Text              │                                 │ Font      │
│ + Student name      │                                 │ Size      │
│ + Score             │                                 │ Style     │
│ + Date              │                                 │ Align     │
│ + Image             │                                 │ Color     │
│ + Signature         │                                 │ Actions   │
└───────────────────────────────────────────────────────────────────┘
```

This is preferred over:

- a fixed form beside a preview, which cannot produce the varied certificate
  compositions requested;
- a full Canva-style editor, which would introduce unnecessary free drawing,
  arbitrary shapes, collaboration, and document-management behavior.

## 4. Visual Direction

The subject is a digital proofing desk for exam administrators. The certificate
is the visual focus; the surrounding application behaves like quiet production
equipment.

### Palette

- Studio Ink `#17213B`: canvas worktop and high-contrast text
- Academy Indigo `#4355E7`: primary actions and active controls
- Canvas Ivory `#FAF9F5`: certificate and warm preview surfaces
- Proof Gold `#C99A3D`: selected layer outline and resize handles
- Panel White `#FFFFFF`: tools and inspector surfaces
- Divider `#D9DDEA`: low-emphasis structure

Existing application semantic colors remain the source for destructive,
disabled, success, and error states.

### Typography

- Public Sans: all builder controls, labels, and utility values
- Playfair Display: the restrained "Certificate builder" page identity
- Existing certificate families: Source Serif 4, Public Sans, Cinzel,
  Playfair Display, Cormorant Garamond, and Great Vibes

The editor must use the same bundled font files as the backend HTML renderer.

### Signature Element

The certificate sits on a dark navy proofing worktop. The selected layer uses a
thin gold outline and square gold resize handles. This is the single expressive
gesture; the rest of the interface stays light and restrained.

## 5. Page Structure

### 5.1 Command Bar

The sticky command bar contains:

- "Certificate builder"
- saved/unsaved state
- "Reset preset" when a built-in preset is active
- "Save changes"

Saving is disabled while an upload or save is pending. Successful and failed
operations use the existing application toast system.

### 5.2 Elements and Layers Panel

The left panel contains three compact sections.

**Templates**

- Classic, Modern, and Elegant thumbnail cards
- Upload background action
- The active custom background is labelled "Custom"

Selecting a built-in preset loads its background and complete default layer
layout. If the current layout is dirty, the admin must confirm before it is
replaced. Uploading or replacing a custom background preserves existing layers
and positions.

**Add element**

- Text
- Student name
- Exam title
- Completion date
- Certificate number
- Score
- Maximum score
- Score percentage
- Rank
- Percentile
- Duration
- Total questions
- Image/logo/icon
- Signature

New text and data layers start at `x=48.5 mm`, `y=100 mm`, `w=200 mm`, with a
14 pt Public Sans style. New images start at `x=133.5 mm`, `y=80 mm`,
`w=30 mm`, `h=30 mm`. The new layer is selected immediately.
Repeated static text and image layers are allowed. A dynamic data element may
be added once; after insertion its add action is disabled until that layer is
deleted.

**Layers**

- layer name or content summary
- selected state
- visible/hidden toggle
- move forward/backward
- delete

Array order is the paint order. Moving a layer changes its position in the
persisted array and therefore its z-order in both browser and PDF rendering.

### 5.3 Canvas

The canvas:

- always represents 297 mm × 210 mm;
- uses one uniform scale derived from rendered canvas width;
- scales font size, line height, border, and image geometry with that scale;
- renders the selected preset or custom background;
- renders dynamic fields with realistic sample values;
- clamps drag and resize operations to page bounds;
- starts in "Fit" mode and provides minus/plus controls from 50% to 150% in
  10% increments;
- has no X/Y coordinate inputs;
- has no embedded PDF frame or "Generate PDF" action.

Dragging updates `x_mm` and `y_mm`. Resizing updates `w_mm` and, for image
layers, `h_mm`. Text height remains content-derived. A text field that exceeds
its width uses the same shrink-to-fit behavior as the backend renderer.

Keyboard users can select layers from the layer list and move the selected
layer in small increments with arrow keys. Focus indicators must remain
visible. Motion is limited to direct manipulation and respects reduced-motion
preferences.

### 5.4 Inspector

For text layers:

- editable content
- token insertion menu
- font family
- size
- regular/bold
- normal/italic
- left/center/right alignment
- text color
- visibility
- duplicate and delete

For image layers:

- image preview
- replace image
- contain fit
- visibility
- duplicate and delete

Position fields are intentionally absent. Geometry is changed on the canvas.

### 5.5 Responsive Behavior

The full three-pane layout is used at desktop widths. At narrower widths:

- the canvas remains first and horizontally safe;
- elements/layers and inspector become collapsible panels;
- the canvas never distort its A4 aspect ratio;
- all actions remain keyboard accessible.

The builder is optimized for desktop authoring, but must not overflow or make
save controls unreachable on a tablet-sized viewport.

## 6. Layer and Token Contract

### 6.1 Layer Shape

Extend the existing `LayoutField` JSON shape rather than introducing a second
certificate format:

```json
{
  "id": "student_name",
  "kind": "text",
  "name": "Student name",
  "content": "{{student_name}}",
  "x_mm": 48.5,
  "y_mm": 100,
  "w_mm": 200,
  "align": "center",
  "font": "cormorant_garamond",
  "weight": "bold",
  "italic": false,
  "size_pt": 34,
  "color": "#22315B",
  "visible": true
}
```

An uploaded image layer uses:

```json
{
  "id": "image_<stable-id>",
  "kind": "image",
  "name": "School logo",
  "asset_key": "certificates/assets/<object-key>",
  "x_mm": 138.5,
  "y_mm": 15,
  "w_mm": 28,
  "h_mm": 28,
  "visible": true
}
```

`asset_key` is a private-bucket object key, never a presigned URL. Presigned
display URLs are returned only in the certificate-design read model.

### 6.2 Tokens

Content is plain text with exact `{{token_name}}` placeholders. Initial tokens:

| Token | Preview value | Student render source |
|---|---|---|
| `{{student_name}}` | Nama Peserta Contoh | student profile name |
| `{{exam_title}}` | current exam title | exam title |
| `{{completion_date}}` | current Jakarta date | submitted date |
| `{{certificate_number}}` | ABK/2026/0000/000000 | allocated number |
| `{{score}}` | 86 | session score |
| `{{max_score}}` | 100 | sum of question maximum points |
| `{{score_percent}}` | 86% | score divided by maximum score |
| `{{rank}}` | 3 | count of higher scores plus one |
| `{{percentile}}` | Top 15% | rank divided by fully graded submissions |
| `{{duration}}` | 90 minutes | submitted time minus started time |
| `{{total_questions}}` | 50 questions | number of session questions |

Admins insert tokens from labelled controls and do not need to memorize the
syntax. Content may combine static copy and multiple tokens, for example:

```text
Awarded to {{student_name}} for completing {{exam_title}}
```

Unknown tokens remain visible in the admin canvas, make save fail with
`Unknown certificate token: {{token_name}}`, and never reach student
generation. Token values are substituted before HTML escaping.

Score and maximum score omit unnecessary trailing zeroes. Score percentage is
rounded to the nearest whole percent and returns `0%` when maximum score is
zero. Rank is `count(scores greater than this score) + 1`. Percentile is
`ceil(rank / fully graded submissions × 100)` and is formatted as `Top N%`.
Duration is elapsed time from session start to submission, rounded up to the
next whole minute.

### 6.3 Backward Compatibility

Saved layouts from the current schema must open and render without migration:

- missing `kind` is inferred from the existing field ID;
- missing `content` receives the current field's default static text or token;
- legacy `signature_key` maps to the existing signature layer when read;
- current built-in field IDs keep their semantic identity;
- saving from the new editor writes the new normalized shape and removes the
  legacy top-level `signature_key`.

No database migration is required because `exam.certificate_design` is already
a JSON document.

## 7. Presets and Backgrounds

The certificate-design GET response must provide all three built-in presets:

- normalized layout
- background data URL generated from the backend's embedded PNG

The embedded backend artwork remains the source of truth. The web application
must not carry duplicate preset PNG files.

The response also provides fresh presigned URLs for every custom image layer.
The saved JSON contains only object keys.

Selecting a preset replaces the working background and layers. Uploading a
custom background:

- stores its object key;
- changes the working template to `custom`;
- changes only the background;
- preserves current layers and their order.

## 8. Rendering and Data Flow

```text
GET certificate-design
  -> saved JSON or built-in default
  -> normalize legacy layers
  -> preset layouts + background data URLs
  -> presigned image-layer URLs
  -> Certificate Studio

Admin edits
  -> browser state in millimetres
  -> PUT certificate-design
  -> backend validates layers, tokens, bounds, fonts, colors, and asset keys
  -> save JSON and bump certificate_design_updated_at

Student opens result
  -> load session, exam, questions, answers, and ranking inputs
  -> build token value map
  -> interpolate saved text content
  -> download saved image assets
  -> render the same ordered layers through certificate HTML/Gotenberg
  -> cache and presign the generated PDF as today
```

The existing PDF preview endpoint remains unchanged for compatibility and
backend diagnostics. The new admin page does not call or display it.

## 9. Result Visibility

Score, maximum score, score percentage, rank, and percentile are
result-sensitive tokens. If a saved layout uses any of them, the service must
not generate or return its certificate URL while the existing result gate is
`hidden`, `grading`, or `locked`. Generation begins when the session reaches
the existing full `result` state.

Layouts without result-sensitive tokens retain the current completion
certificate behavior. Duration and total questions are not treated as score
disclosure.

This rule prevents the certificate from becoming a side channel that reveals a
score before the exam's configured release state.

## 10. Validation and Error Handling

The backend remains the security boundary and rejects:

- non-positive page dimensions;
- unknown layer kinds;
- duplicate IDs;
- IDs that are neither known data-layer IDs, `text_<uuid>`, nor
  `image_<uuid>`;
- off-page geometry;
- image layers without positive width and height;
- text layers without positive width and font size;
- invalid color values;
- unsupported alignments or font families;
- unknown content tokens;
- malformed or empty custom asset keys.

The UI prevents invalid choices where practical and displays server validation
messages without discarding the working layout.

Upload failure:

- keeps the previous image/background;
- shows a specific failure toast;
- leaves the page dirty state unchanged.

Preset confirmation cancellation keeps the current working layout unchanged.

## 11. Explicit Non-goals

- free drawing, vector shapes, arbitrary rotations, or gradients
- multi-page certificates
- real-time collaboration or version history
- arbitrary user-supplied fonts
- a public certificate verification endpoint
- generated QR verification codes
- an admin-side PDF viewer

An uploaded static QR image is allowed as an ordinary image layer, but dynamic
verification QR codes require a separate public verification feature.

## 12. Testing Strategy

### Frontend

- canvas applies one uniform mm-to-pixel scale to position and typography;
- dragging persists clamped millimetre coordinates;
- resizing persists clamped width/height;
- editing content/style updates the live layer;
- inserting a token updates content;
- adding/removing/reordering layers updates save payload;
- template selection loads its preset after confirmation;
- custom background upload preserves layers;
- multiple image uploads preserve distinct object keys;
- PDF preview controls are absent;
- old layouts normalize into editable layers;
- save and upload failures preserve the working state.

### Backend

- legacy layouts normalize with inferred kinds and default content;
- known tokens interpolate with real and preview values;
- unknown tokens fail validation;
- score, percentage, rank, percentile, duration, and question-count values are
  derived deterministically;
- multiple image assets are downloaded and rendered in array order;
- image asset keys are stored while only presigned URLs are returned;
- style, bounds, kind, and ID validation rejects malformed layouts;
- built-in preset backgrounds in the editor response match embedded renderer
  assets;
- existing certificate staleness and regeneration behavior remains intact;
- result-sensitive layouts return no certificate before the full result gate,
  while completion-only layouts retain current availability.

### End-to-End Verification

- frontend targeted tests
- backend certificate/service/handler targeted tests
- full frontend test suite, typecheck, and production build
- full backend test suite with the required Go toolchain override
- browser inspection at desktop and tablet widths
- save, reload, and student-generation comparison using the same layout

## 13. Acceptance Criteria

The redesign is complete when:

1. no certificate layer overlaps because of preview scaling;
2. changing a template visibly changes both its artwork and default layout;
3. an admin can edit copy and text styling without editing coordinates;
4. drag and resize changes survive save and reload;
5. an admin can add, reorder, replace, and remove multiple images;
6. optional data elements render real session values on student certificates;
7. the configuration page contains no PDF preview or PDF-generation control;
8. a saved legacy certificate still opens and generates correctly;
9. the browser canvas and generated student PDF use the same persisted layer
   order, copy, tokens, style, and geometry.
