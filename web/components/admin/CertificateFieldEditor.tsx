"use client";

import { useEffect, useLayoutEffect, useRef, useState, type CSSProperties, type PointerEvent as ReactPointerEvent } from "react";
import { Minus, Plus, Scan } from "lucide-react";
import { normalizeCertificateLayout, previewContent, PREVIEW_VALUES } from "@/lib/certificate-studio";
import { useTranslation } from "@/lib/i18n";
import type { CertificateLayout, CertificateLayoutField } from "@/lib/types";

interface CertificateFieldEditorProps {
  layout: CertificateLayout;
  onChange: (fields: CertificateLayoutField[]) => void;
  backgroundUrl?: string | null;
  examTitle?: string;
  selectedId?: string | null;
  onSelect?: (id: string) => void;
  assetUrls?: Record<string, string>;
}

interface Interaction {
  id: string;
  mode: "move" | "resize";
  startX: number;
  startY: number;
  original: CertificateLayoutField;
}

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

export function CertificateFieldEditor({
  layout,
  onChange,
  backgroundUrl,
  examTitle,
  selectedId,
  onSelect,
  assetUrls = {},
}: CertificateFieldEditorProps) {
  const { t } = useTranslation();
  const canvasRef = useRef<HTMLDivElement>(null);
  const [canvasWidth, setCanvasWidth] = useState(0);
  const [zoom, setZoom] = useState(1);
  const [interaction, setInteraction] = useState<Interaction | null>(null);
  const normalizedLayout = normalizeCertificateLayout(layout);
  const values = { ...PREVIEW_VALUES, exam_title: examTitle || PREVIEW_VALUES.exam_title };
  const pxPerMm = canvasWidth > 0 ? canvasWidth / normalizedLayout.page.width_mm : 1;

  useEffect(() => {
    const element = canvasRef.current;
    if (!element) return;
    const measure = () => setCanvasWidth(element.getBoundingClientRect().width);
    measure();
    if (typeof ResizeObserver === "undefined") return;
    const observer = new ResizeObserver(measure);
    observer.observe(element);
    return () => observer.disconnect();
  }, []);

  function update(id: string, patch: Partial<CertificateLayoutField>) {
    onChange(normalizedLayout.fields.map((field) => field.id === id ? { ...field, ...patch } : field));
  }

  function start(e: ReactPointerEvent, field: CertificateLayoutField, mode: "move" | "resize") {
    e.stopPropagation();
    e.currentTarget.setPointerCapture?.(e.pointerId);
    onSelect?.(field.id);
    setInteraction({ id: field.id, mode, startX: e.clientX, startY: e.clientY, original: field });
  }

  function move(e: ReactPointerEvent) {
    if (!interaction || pxPerMm <= 0) return;
    const dx = (e.clientX - interaction.startX) / (pxPerMm * zoom);
    const dy = (e.clientY - interaction.startY) / (pxPerMm * zoom);
    const f = interaction.original;
    if (interaction.mode === "move") {
      const height = f.kind === "image" ? (f.h_mm || 0) : (f.size_pt || 12) * 0.3528 * 1.15;
      update(f.id, {
        x_mm: Math.max(0, Math.min(normalizedLayout.page.width_mm - f.w_mm, f.x_mm + dx)),
        y_mm: Math.max(0, Math.min(normalizedLayout.page.height_mm - height, f.y_mm + dy)),
      });
    } else {
      const availableWidth = Math.max(0, normalizedLayout.page.width_mm - f.x_mm);
      const availableHeight = Math.max(0, normalizedLayout.page.height_mm - f.y_mm);
      update(f.id, {
        w_mm: Math.max(Math.min(8, availableWidth), Math.min(availableWidth, f.w_mm + dx)),
        h_mm: Math.max(Math.min(8, availableHeight), Math.min(availableHeight, (f.h_mm || 30) + dy)),
      });
    }
  }

  function keyboardMove(e: React.KeyboardEvent, field: CertificateLayoutField) {
    const delta = e.shiftKey ? 5 : 1;
    const height = field.kind === "image" ? (field.h_mm || 30) : (field.size_pt || 12) * 0.3528 * 1.15;
    const patch: Partial<CertificateLayoutField> = {};
    if (e.key === "ArrowLeft") patch.x_mm = Math.max(0, field.x_mm - delta);
    else if (e.key === "ArrowRight") patch.x_mm = Math.min(normalizedLayout.page.width_mm - field.w_mm, field.x_mm + delta);
    else if (e.key === "ArrowUp") patch.y_mm = Math.max(0, field.y_mm - delta);
    else if (e.key === "ArrowDown") patch.y_mm = Math.min(normalizedLayout.page.height_mm - height, field.y_mm + delta);
    else return;
    e.preventDefault();
    update(field.id, patch);
  }

  return (
    <div className="space-y-3">
      <div className="flex items-center justify-between text-white/70">
        <span className="text-[11px] font-semibold uppercase tracking-[0.18em]">{t("certificate_studio_live_proof")}</span>
        <div className="flex items-center gap-1 rounded-full border border-white/10 bg-white/5 p-1">
          <button type="button" aria-label={t("certificate_studio_zoom_out")} className="rounded-full p-1 hover:bg-white/10" onClick={() => setZoom((z) => Math.max(.5, z - .1))}><Minus className="size-3.5" /></button>
          <button type="button" className="flex items-center gap-1 rounded-full px-2 py-1 text-[11px] hover:bg-white/10" onClick={() => setZoom(1)}><Scan className="size-3.5" /> {t("certificate_studio_fit")}</button>
          <button type="button" aria-label={t("certificate_studio_zoom_in")} className="rounded-full p-1 hover:bg-white/10" onClick={() => setZoom((z) => Math.min(1.5, z + .1))}><Plus className="size-3.5" /></button>
        </div>
      </div>
      <div className="overflow-auto rounded-xl bg-[#0E1629] p-3 sm:p-6">
        <div
          ref={canvasRef}
          data-testid="certificate-field-editor-canvas"
          className="relative mx-auto w-full max-w-[1000px] select-none overflow-hidden bg-[#FAF9F5] shadow-[0_24px_60px_rgba(0,0,0,.35)]"
          style={{ aspectRatio: `${normalizedLayout.page.width_mm}/${normalizedLayout.page.height_mm}`, transform: `scale(${zoom})`, transformOrigin: "top center", containerType: "inline-size" }}
          onPointerMove={move}
          onPointerUp={() => setInteraction(null)}
        >
          {backgroundUrl && <img src={backgroundUrl} alt="" data-testid="certificate-field-editor-background" className="pointer-events-none absolute inset-0 h-full w-full object-cover" />}
          {normalizedLayout.fields.filter((field) => field.visible).map((field) => {
            const selected = selectedId === field.id;
            const isImage = field.kind === "image";
            const style: CSSProperties = {
              position: "absolute",
              left: `${field.x_mm * pxPerMm}px`,
              top: `${field.y_mm * pxPerMm}px`,
              width: `${field.w_mm * pxPerMm}px`,
              height: `${(isImage ? (field.h_mm || 30) : (field.size_pt || 12) * 0.3528 * 1.15) * pxPerMm}px`,
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
                tabIndex={0}
                data-testid={`certificate-field-box-${field.id}`}
                className={`group touch-none overflow-visible whitespace-nowrap outline-none ${selected ? "ring-2 ring-[#C99A3D]" : "hover:ring-1 hover:ring-[#C99A3D]/70"} cursor-grab focus:ring-2 focus:ring-[#C99A3D]`}
                style={style}
                onPointerDown={(e) => start(e, field, "move")}
                onKeyDown={(e) => keyboardMove(e, field)}
                onClick={() => onSelect?.(field.id)}
              >
                {isImage ? (
                  assetUrls[field.id] ? <img src={assetUrls[field.id]} alt={field.name || ""} className="pointer-events-none size-full object-contain" /> : <span className="flex size-full items-center justify-center border border-dashed border-[#C99A3D] bg-white/70 text-[10px] text-[#17213B]">{field.name || t("certificate_studio_image")}</span>
                ) : (
                  <CertificateTextValue field={field} content={previewContent(field.content || "", values)} pxPerMm={pxPerMm} style={textStyle} />
                )}
                {selected && isImage && <button type="button" aria-label={`Resize ${field.name || "image"}`} className="absolute -bottom-1.5 -right-1.5 size-3 rounded-sm border border-white bg-[#C99A3D]" onPointerDown={(e) => start(e, field, "resize")} />}
              </div>
            );
          })}
        </div>
      </div>
    </div>
  );
}
