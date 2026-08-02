import type { CSSProperties } from "react";
import { normalizeCertificateLayout, previewContent } from "@/lib/certificate-studio";
import type { CertificateLayout, CertificateLayoutField } from "@/lib/types";

// Deliberately NOT "use client": this file is imported both by the
// interactive browser editor (via CertificateDocument.tsx, which supplies a
// hook-based renderTextValue for shrink-to-fit sizing) and by the
// certificate-template route handler that runs in a plain Node/Route Handler
// context (via CertificateDocumentStatic.tsx) to serialize the self-contained
// HTML the async redesign stores in exam.certificate_template_html. A "use
// client" file cannot be invoked directly from a server context (Next.js
// turns its exports into client references there) — see the split's
// rationale in CertificateDocumentStatic.tsx. Everything that actually
// determines layout fidelity (positions, background, image handling, box
// sizing) lives here exactly once; only the ~15 lines of DOM-measurement
// shrink-to-fit sizing (which cannot run without a browser regardless of
// this split) differ between the two callers.

// CSS absolute units are fixed at 96px/in regardless of device DPI, so 1mm
// is always 96/25.4 CSS px - this lets a print-context caller (no measured
// container) render at true size without needing a DOM measurement.
export const CSS_PX_PER_MM = 96 / 25.4;

export const fonts: Record<string, string> = {
  source_serif_4: "var(--font-certificate-source-serif)",
  public_sans: "var(--font-certificate-public-sans)",
  cinzel: "var(--font-certificate-cinzel)",
  playfair_display: "var(--font-certificate-playfair-display)",
  cormorant_garamond: "var(--font-certificate-cormorant-garamond)",
  great_vibes: "var(--font-certificate-great-vibes)",
  poppins: "var(--font-certificate-poppins)",
  libre_baskerville: "var(--font-certificate-libre-baskerville)",
  allura: "var(--font-certificate-allura)",
  parisienne: "var(--font-certificate-parisienne)",
};

export function safeColor(color?: string) {
  return /^#[0-9a-f]{6}$/i.test(color || "") ? color : "#000000";
}

export function fieldHeightMm(field: CertificateLayoutField) {
  return field.kind === "image" ? (field.h_mm || 30) : (field.size_pt || 12) * 0.3528 * 1.15;
}

export interface CertificateFieldInteraction {
  selected?: boolean;
  onPointerDown?: (e: React.PointerEvent) => void;
  onKeyDown?: (e: React.KeyboardEvent) => void;
  onClick?: () => void;
  onResizePointerDown?: (e: React.PointerEvent) => void;
}

export interface RenderTextValueArgs {
  field: CertificateLayoutField;
  content: string;
  pxPerMm: number;
  style: CSSProperties;
}

export interface CertificateDocumentCoreProps {
  layout: CertificateLayout;
  values: Record<string, string>;
  assetUrls?: Record<string, string>;
  backgroundUrl?: string | null;
  pxPerMm?: number;
  getFieldInteraction?: (field: CertificateLayoutField) => CertificateFieldInteraction | undefined;
  // Injected by the two callers: CertificateDocument.tsx supplies a
  // hook-based shrink-to-fit span, CertificateDocumentStatic.tsx supplies a
  // plain span at the field's base font size.
  renderTextValue: (args: RenderTextValueArgs) => React.ReactNode;
}

export function CertificateDocumentCore({
  layout,
  values,
  assetUrls = {},
  backgroundUrl,
  pxPerMm = CSS_PX_PER_MM,
  getFieldInteraction,
  renderTextValue,
}: CertificateDocumentCoreProps) {
  const normalizedLayout = normalizeCertificateLayout(layout);
  const interactive = Boolean(getFieldInteraction);

  return (
    <div
      data-testid="certificate-document-page"
      className="relative select-none overflow-hidden bg-[#FAF9F5]"
      style={{ width: `${normalizedLayout.page.width_mm}mm`, height: `${normalizedLayout.page.height_mm}mm` }}
    >
      <style>{`@page { size: ${normalizedLayout.page.width_mm}mm ${normalizedLayout.page.height_mm}mm; margin: 0; }`}</style>
      {backgroundUrl && <img src={backgroundUrl} alt="" data-testid="certificate-field-editor-background" className="pointer-events-none absolute inset-0 h-full w-full object-cover" />}
      {normalizedLayout.fields.filter((field) => field.visible).map((field) => {
        const interaction = getFieldInteraction?.(field);
        const isImage = field.kind === "image";
        const style: CSSProperties = {
          position: "absolute",
          left: `${field.x_mm}mm`,
          top: `${field.y_mm}mm`,
          width: `${field.w_mm}mm`,
          height: `${fieldHeightMm(field)}mm`,
        };
        const textStyle: CSSProperties = {
          color: safeColor(field.color),
          fontFamily: fonts[field.font || ""] || "var(--font-certificate-source-serif)",
          fontWeight: field.weight === "bold" ? 700 : 400,
          fontStyle: field.italic ? "italic" : "normal",
          textAlign: field.align === "left" || field.align === "right" ? field.align : "center",
        };
        return (
          <div
            key={field.id}
            tabIndex={interactive ? 0 : undefined}
            data-testid={`certificate-field-box-${field.id}`}
            className={interactive ? `group touch-none overflow-visible whitespace-nowrap outline-none ${interaction?.selected ? "ring-2 ring-[#C99A3D]" : "hover:ring-1 hover:ring-[#C99A3D]/70"} cursor-grab focus:ring-2 focus:ring-[#C99A3D]` : "overflow-visible whitespace-nowrap"}
            style={style}
            onPointerDown={interaction?.onPointerDown}
            onKeyDown={interaction?.onKeyDown}
            onClick={interaction?.onClick}
          >
            {isImage ? (
              assetUrls[field.id] ? <img src={assetUrls[field.id]} alt={field.name || ""} className="pointer-events-none size-full object-contain" /> : <span className="flex size-full items-center justify-center border border-dashed border-[#C99A3D] bg-white/70 text-[10px] text-[#17213B]">{field.name || "Image"}</span>
            ) : (
              renderTextValue({ field, content: previewContent(field.content || "", values), pxPerMm, style: textStyle })
            )}
            {interaction?.selected && isImage && interaction.onResizePointerDown && (
              <button type="button" aria-label={`Resize ${field.name || "image"}`} className="absolute -bottom-1.5 -right-1.5 size-3 rounded-sm border border-white bg-[#C99A3D]" onPointerDown={interaction.onResizePointerDown} />
            )}
          </div>
        );
      })}
    </div>
  );
}
