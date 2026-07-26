"use client";

import { useRef, useState, type CSSProperties, type PointerEvent as ReactPointerEvent } from "react";
import { useTranslation } from "@/lib/i18n";
import type { DICT } from "@/lib/i18n";
import type { CertificateLayout, CertificateLayoutField } from "@/lib/types";
import "./CertificateFonts.module.css";

interface CertificateFieldEditorProps {
  layout: CertificateLayout;
  onChange: (fields: CertificateLayoutField[]) => void;
  // backgroundUrl is the certificate artwork (FR-26/D5): the drag surface
  // renders it so the admin positions fields against the actual design
  // instead of a featureless rectangle. Optional/null falls back to a plain
  // fill so the editor still works before a background has resolved.
  backgroundUrl?: string | null;
  // examTitle seeds the exam_title field's WYSIWYG placeholder (FR-16); falls
  // back to the translated field label when unavailable.
  examTitle?: string;
}

// KNOWN_FONT_FAMILIES / resolveFontFamily mirror the backend's ResolveFontFamily
// (pdffonts.go): an unknown/unset family falls back to the brand default so a
// stale/corrupt design still renders (FR-9).
const KNOWN_FONT_FAMILIES = new Set([
  "source_serif_4",
  "public_sans",
  "cinzel",
  "playfair_display",
  "cormorant_garamond",
  "great_vibes",
]);
const DEFAULT_FONT_FAMILY = "source_serif_4";

function resolveFontFamily(font: string | undefined): string {
  return font && KNOWN_FONT_FAMILIES.has(font) ? font : DEFAULT_FONT_FAMILY;
}

// safeCssColor mirrors the backend's safeCSSColor (certificate_html.go): a
// malformed #RRGGBB degrades to black rather than breaking the render (FR-9).
function safeCssColor(color: string | undefined): string {
  return color && /^#[0-9a-fA-F]{6}$/.test(color) ? color : "#000000";
}

// cssAlign mirrors the backend's cssAlign; an unrecognised value centers.
function cssAlign(align: string): "left" | "right" | "center" {
  return align === "left" || align === "right" ? align : "center";
}

// certificateFieldPlaceholders mirrors the backend's certificateFieldValues
// (certificate.go) and the preview endpoint's fixed sample values (exam.go),
// so the WYSIWYG editor's placeholder copy matches what a real preview
// renders. This is admin template-authoring preview data, never a real
// certificate's values.
function certificateFieldPlaceholders(examTitle: string): Record<string, string> {
  return {
    title: "CERTIFICATE OF COMPLETION",
    subtitle: "This certificate is proudly awarded to",
    student_name: "Nama Peserta Contoh",
    completion_text: "for successfully completing",
    exam_title: examTitle,
    date: new Date().toLocaleDateString("en-GB", { day: "numeric", month: "long", year: "numeric" }),
    // Mirrors the backend's previewCertificateNumber — the four-segment shape
    // AllocateCertificateNumber really produces, so the WYSIWYG box shows the
    // width the issued number will occupy.
    certificate_number: "ABK/2026/0000/000000",
  };
}

interface DragState {
  fieldId: string;
  offsetPx: { x: number; y: number };
  currentPx: { x: number; y: number };
}

const FIELD_LABEL_KEY: Record<string, keyof (typeof DICT)["id"]> = {
  title: "certificate_field_title",
  subtitle: "certificate_field_subtitle",
  student_name: "certificate_field_student_name",
  exam_title: "certificate_field_exam_title",
  completion_text: "certificate_field_completion_text",
  date: "certificate_field_date",
  certificate_number: "certificate_field_certificate_number",
  logo: "certificate_field_logo",
  signature: "certificate_field_signature",
};

// imageFieldIDs mirror the backend's imageFieldIDs (certificate_layout.go): they
// carry an explicit h_mm box height rather than a font-derived line height.
const imageFieldIDs = new Set(["logo", "signature"]);

// Coordinate contract (FR-1): x_mm,y_mm is the field box's top-left corner in
// millimetres, origin top-left, Y down - identical to the renderer. The only
// conversion here is the uniform scale mm = px * (page_width_mm /
// preview_width_px); there is no Y-axis flip.
function pxToMm(px: number, widthPx: number, pageWidthMm: number): number {
  return px * (pageWidthMm / widthPx);
}

// nominalLineHeightMm mirrors the backend's certificate_layout.go
// nominalLineHeightMm (1pt = 0.3528mm, 1.15 leading): a text field has no
// h_mm of its own, so this is what both sides use as its effective box
// height when clamping/validating y_mm (FR-28). Keep in sync with the Go copy.
function nominalLineHeightMm(sizePt: number | undefined): number {
  return (sizePt ?? 0) * 0.3528 * 1.15;
}

// clampFieldPosition mirrors the backend's ValidateLayout bounds (Task 3) so
// a drop the editor accepts never comes back as a 422 on save: x_mm,y_mm is
// the box's top-left corner, so x clamps against the box's own width, and y
// clamps against the box's own height too — a logo's h_mm, or a text field's
// derived nominal line height — so the box's bottom edge never runs off the
// page, not just its top-left corner (FR-28).
export function clampFieldPosition(
  field: CertificateLayoutField,
  page: CertificateLayout["page"],
): CertificateLayoutField {
  const maxX = Math.max(0, page.width_mm - field.w_mm);
  const x_mm = Math.min(Math.max(field.x_mm, 0), maxX);
  const boxHeightMm = imageFieldIDs.has(field.id) ? (field.h_mm ?? 0) : nominalLineHeightMm(field.size_pt);
  const maxY = Math.max(0, page.height_mm - boxHeightMm);
  const y_mm = Math.min(Math.max(field.y_mm, 0), maxY);
  return { ...field, x_mm, y_mm };
}

export function CertificateFieldEditor({ layout, onChange, backgroundUrl, examTitle }: CertificateFieldEditorProps) {
  const { t } = useTranslation();
  const containerRef = useRef<HTMLDivElement | null>(null);
  const [drag, setDrag] = useState<DragState | null>(null);
  const { page, fields } = layout;
  const visibleFields = fields.filter((f) => f.visible);
  const placeholders = certificateFieldPlaceholders(examTitle || t("certificate_field_exam_title"));

  function commitField(fieldId: string, patch: Partial<CertificateLayoutField>) {
    const field = fields.find((f) => f.id === fieldId);
    if (!field) return;
    const updated = clampFieldPosition({ ...field, ...patch }, page);
    onChange(fields.map((f) => (f.id === fieldId ? updated : f)));
  }

  function handlePointerDown(e: ReactPointerEvent<HTMLDivElement>, field: CertificateLayoutField) {
    const rect = containerRef.current?.getBoundingClientRect();
    if (!rect) return;
    const boxLeftPx = (field.x_mm / page.width_mm) * rect.width;
    const boxTopPx = (field.y_mm / page.height_mm) * rect.height;
    const target = e.currentTarget;
    if (typeof target.setPointerCapture === "function") {
      try {
        target.setPointerCapture(e.pointerId);
      } catch {
        // Not every environment (e.g. jsdom) implements pointer capture; the
        // drag still works from move/up events bubbling normally.
      }
    }
    setDrag({
      fieldId: field.id,
      offsetPx: { x: e.clientX - rect.left - boxLeftPx, y: e.clientY - rect.top - boxTopPx },
      currentPx: { x: boxLeftPx, y: boxTopPx },
    });
  }

  function handlePointerMove(e: ReactPointerEvent<HTMLDivElement>) {
    if (!drag) return;
    const rect = containerRef.current?.getBoundingClientRect();
    if (!rect) return;
    setDrag({
      ...drag,
      currentPx: {
        x: e.clientX - rect.left - drag.offsetPx.x,
        y: e.clientY - rect.top - drag.offsetPx.y,
      },
    });
  }

  function handlePointerUp() {
    if (!drag) return;
    const rect = containerRef.current?.getBoundingClientRect();
    if (rect) {
      const x_mm = pxToMm(drag.currentPx.x, rect.width, page.width_mm);
      const y_mm = pxToMm(drag.currentPx.y, rect.width, page.width_mm);
      commitField(drag.fieldId, { x_mm, y_mm });
    }
    setDrag(null);
  }

  return (
    <div className="space-y-3">
      <div>
        <span className="text-xs font-semibold uppercase tracking-wide text-ink-500">
          {t("certificate_field_editor_label")}
        </span>
        <p className="text-xs text-muted-foreground">{t("certificate_field_editor_hint")}</p>
      </div>

      <div
        ref={containerRef}
        data-testid="certificate-field-editor-canvas"
        className={`relative w-full select-none overflow-hidden rounded-md border border-line ${backgroundUrl ? "" : "bg-ink-50"}`}
        style={{ aspectRatio: `${page.width_mm} / ${page.height_mm}` }}
        onPointerMove={handlePointerMove}
        onPointerUp={handlePointerUp}
      >
        {backgroundUrl && (
          // eslint-disable-next-line @next/next/no-img-element
          <img
            src={backgroundUrl}
            alt=""
            data-testid="certificate-field-editor-background"
            className="pointer-events-none absolute inset-0 h-full w-full object-cover"
          />
        )}
        {visibleFields.map((field) => {
          const isDragging = drag?.fieldId === field.id;
          const widthPct = (field.w_mm / page.width_mm) * 100;
          const style: CSSProperties = isDragging
            ? { position: "absolute", left: drag!.currentPx.x, top: drag!.currentPx.y, width: `${widthPct}%` }
            : {
                position: "absolute",
                left: `${(field.x_mm / page.width_mm) * 100}%`,
                top: `${(field.y_mm / page.height_mm) * 100}%`,
                width: `${widthPct}%`,
              };
          const isImage = imageFieldIDs.has(field.id);
          return (
            <div
              key={field.id}
              data-testid={`certificate-field-box-${field.id}`}
              className="cursor-grab touch-none rounded border border-dashed border-brand-400/70 active:cursor-grabbing"
              style={style}
              onPointerDown={(e) => handlePointerDown(e, field)}
            >
              {isImage ? (
                <span className="px-1 py-0.5 text-[10px] leading-tight text-brand-800">
                  {t(FIELD_LABEL_KEY[field.id] ?? "certificate_field_title")}
                </span>
              ) : (
                <span
                  data-testid={`certificate-field-value-${field.id}`}
                  className="block w-full overflow-hidden whitespace-nowrap"
                  style={{
                    fontFamily: resolveFontFamily(field.font),
                    fontWeight: field.weight === "bold" ? 700 : 400,
                    fontSize: `${field.size_pt ?? 12}pt`,
                    color: safeCssColor(field.color),
                    textAlign: cssAlign(field.align),
                  }}
                >
                  {placeholders[field.id] ?? ""}
                </span>
              )}
            </div>
          );
        })}
      </div>

      <div className="space-y-1">
        {visibleFields.map((field) => (
          <div key={field.id} className="flex items-center gap-2 text-xs">
            <span className="w-28 truncate text-ink-600">
              {t(FIELD_LABEL_KEY[field.id] ?? "certificate_field_title")}
            </span>
            <label className="flex items-center gap-1">
              x(mm)
              <input
                type="number"
                aria-label={`x_mm ${field.id}`}
                value={Math.round(field.x_mm * 10) / 10}
                onChange={(e) => commitField(field.id, { x_mm: Number(e.target.value) })}
                className="w-16 rounded border border-line px-1 py-0.5"
              />
            </label>
            <label className="flex items-center gap-1">
              y(mm)
              <input
                type="number"
                aria-label={`y_mm ${field.id}`}
                value={Math.round(field.y_mm * 10) / 10}
                onChange={(e) => commitField(field.id, { y_mm: Number(e.target.value) })}
                className="w-16 rounded border border-line px-1 py-0.5"
              />
            </label>
          </div>
        ))}
      </div>
    </div>
  );
}
