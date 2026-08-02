"use client";

import { useEffect, useRef, useState, type PointerEvent as ReactPointerEvent } from "react";
import { Minus, Plus, Scan } from "lucide-react";
import { CertificateDocument } from "@/components/certificate/CertificateDocument";
import { normalizeCertificateLayout, PREVIEW_VALUES } from "@/lib/certificate-studio";
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

// 1mm is always 96/25.4 CSS px regardless of device DPI - used to compute
// the auto-fit scale that shrinks the true-size mm document into the canvas.
const CSS_PX_PER_MM = 96 / 25.4;

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

  const documentWidthPx = normalizedLayout.page.width_mm * CSS_PX_PER_MM;
  const documentHeightPx = normalizedLayout.page.height_mm * CSS_PX_PER_MM;
  const fitScale = canvasWidth > 0 ? canvasWidth / documentWidthPx : 1;

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
          className="relative mx-auto w-full max-w-[1000px] select-none bg-[#FAF9F5] shadow-[0_24px_60px_rgba(0,0,0,.35)]"
          style={{ aspectRatio: `${normalizedLayout.page.width_mm}/${normalizedLayout.page.height_mm}` }}
          onPointerMove={move}
          onPointerUp={() => setInteraction(null)}
        >
          <div style={{ width: documentWidthPx, height: documentHeightPx, transform: `scale(${fitScale * zoom})`, transformOrigin: "top left" }}>
            <CertificateDocument
              layout={normalizedLayout}
              values={values}
              assetUrls={assetUrls}
              backgroundUrl={backgroundUrl}
              pxPerMm={pxPerMm}
              getFieldInteraction={(field) => ({
                selected: selectedId === field.id,
                onPointerDown: (e) => start(e, field, "move"),
                onKeyDown: (e) => keyboardMove(e, field),
                onClick: () => onSelect?.(field.id),
                onResizePointerDown: (e) => start(e, field, "resize"),
              })}
            />
          </div>
        </div>
      </div>
    </div>
  );
}
