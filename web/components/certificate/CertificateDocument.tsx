"use client";

import { useLayoutEffect, useRef, useState, type CSSProperties } from "react";
import { normalizeCertificateLayout, previewContent } from "@/lib/certificate-studio";
import type { CertificateLayout, CertificateLayoutField } from "@/lib/types";

// CSS absolute units are fixed at 96px/in regardless of device DPI, so 1mm
// is always 96/25.4 CSS px - this lets a print-context caller (no measured
// container) render at true size without needing a DOM measurement.
const CSS_PX_PER_MM = 96 / 25.4;

const fonts: Record<string, string> = {
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

function safeColor(color?: string) {
  return /^#[0-9a-f]{6}$/i.test(color || "") ? color : "#000000";
}

function fieldHeightMm(field: CertificateLayoutField) {
  return field.kind === "image" ? (field.h_mm || 30) : (field.size_pt || 12) * 0.3528 * 1.15;
}

function CertificateTextValue({ field, content, pxPerMm, style }: {
  field: CertificateLayoutField;
  content: string;
  pxPerMm: number;
  style: CSSProperties;
}) {
  const ref = useRef<HTMLSpanElement>(null);
  const baseSize = (field.size_pt || 12) * 0.3528 * pxPerMm;
  const [fontSize, setFontSize] = useState(baseSize);

  useLayoutEffect(() => {
    const element = ref.current;
    if (!element) return;
    element.style.fontSize = `${baseSize}px`;
    const available = element.clientWidth;
    const needed = element.scrollWidth;
    const minimum = 6 * 0.3528 * pxPerMm;
    setFontSize(available > 0 && needed > available ? Math.max(minimum, baseSize * available / needed * 0.97) : baseSize);
  }, [baseSize, content, field.font, field.italic, field.weight, pxPerMm]);

  return <span ref={ref} data-testid={`certificate-field-value-${field.id}`} className="block size-full overflow-hidden" style={{ ...style, fontSize: `${fontSize}px` }}>{content}</span>;
}

export interface CertificateFieldInteraction {
  selected?: boolean;
  onPointerDown?: (e: React.PointerEvent) => void;
  onKeyDown?: (e: React.KeyboardEvent) => void;
  onClick?: () => void;
  onResizePointerDown?: (e: React.PointerEvent) => void;
}

export interface CertificateDocumentProps {
  layout: CertificateLayout;
  values: Record<string, string>;
  assetUrls?: Record<string, string>;
  backgroundUrl?: string | null;
  // px-per-mm used only for the live shrink-to-fit text sizing below; when
  // omitted the document renders at true print size (real mm -> 96dpi px).
  pxPerMm?: number;
  getFieldInteraction?: (field: CertificateLayoutField) => CertificateFieldInteraction | undefined;
}

export function CertificateDocument({
  layout,
  values,
  assetUrls = {},
  backgroundUrl,
  pxPerMm = CSS_PX_PER_MM,
  getFieldInteraction,
}: CertificateDocumentProps) {
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
              <CertificateTextValue field={field} content={previewContent(field.content || "", values)} pxPerMm={pxPerMm} style={textStyle} />
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
