"use client";

import { AlignCenter, AlignLeft, AlignRight, Bold, Italic, Trash2 } from "lucide-react";
import { Button } from "@/components/ui/button";
import { CERTIFICATE_TOKENS } from "@/lib/certificate-studio";
import { useTranslation } from "@/lib/i18n";
import type { CertificateLayoutField } from "@/lib/types";

interface Props {
  field: CertificateLayoutField | null;
  onChange: (patch: Partial<CertificateLayoutField>) => void;
  onDelete: () => void;
  onReplaceImage: () => void;
}

const FONTS = [
  ["public_sans", "Public Sans"],
  ["source_serif_4", "Source Serif"],
  ["cinzel", "Cinzel"],
  ["playfair_display", "Playfair Display"],
  ["cormorant_garamond", "Cormorant Garamond"],
  ["great_vibes", "Great Vibes"],
  ["poppins", "Poppins"],
  ["libre_baskerville", "Libre Baskerville"],
  ["allura", "Allura"],
  ["parisienne", "Parisienne"],
];

const TOKEN_LABELS = {
  student_name: "certificate_field_student_name",
  exam_title: "certificate_field_exam_title",
  completion_date: "certificate_studio_completion_date",
  certificate_number: "certificate_field_certificate_number",
  score: "score",
  max_score: "certificate_studio_max_score",
  score_percent: "certificate_studio_score_percent",
  rank: "certificate_studio_rank",
  percentile: "certificate_studio_percentile",
  duration: "certificate_studio_duration",
  total_questions: "certificate_studio_total_questions",
} as const;

export function CertificateInspector({ field, onChange, onDelete, onReplaceImage }: Props) {
  const { t } = useTranslation();
  if (!field) {
    return <div className="flex min-h-48 items-center justify-center rounded-[var(--radius-card)] border border-dashed border-line p-6 text-center text-sm text-ink-600">{t("certificate_studio_select_element")}</div>;
  }
  if (field.kind === "image") {
    return (
      <div className="space-y-5">
        <div>
          <p className="text-xs font-semibold uppercase tracking-[0.16em] text-ink-600">{t("certificate_studio_image")}</p>
          <h3 className="mt-1 font-semibold text-ink-900">{field.name || t("certificate_studio_image")}</h3>
        </div>
        <Button type="button" variant="outline" className="w-full" onClick={onReplaceImage}>{t("certificate_studio_replace_image")}</Button>
        <label className="flex items-center justify-between text-sm text-ink-700">
          {t("certificate_studio_visible")}
          <input type="checkbox" checked={field.visible} onChange={(e) => onChange({ visible: e.target.checked })} />
        </label>
        <Button type="button" variant="ghost" className="w-full text-destructive" onClick={onDelete}><Trash2 className="mr-2 size-4" />{t("certificate_studio_delete_layer")}</Button>
      </div>
    );
  }
  return (
    <div className="space-y-5">
      <div>
        <p className="text-xs font-semibold uppercase tracking-[0.16em] text-ink-600">{t("certificate_studio_text")}</p>
        <h3 className="mt-1 font-semibold capitalize text-ink-900">{field.name || t("certificate_studio_text")}</h3>
      </div>
      <label className="grid gap-2 text-xs font-semibold text-ink-600">
        {t("certificate_studio_content")}
        <textarea aria-label={t("certificate_studio_content")} rows={4} value={field.content || ""} onChange={(e) => onChange({ content: e.target.value })} className="resize-none rounded-lg border border-line bg-surface p-3 text-sm font-normal text-ink-900 outline-none focus:border-brand-600 focus:ring-2 focus:ring-brand-600/15" />
      </label>
      <div>
        <p className="mb-2 text-xs font-semibold text-ink-600">{t("certificate_studio_insert_data")}</p>
        <div className="flex flex-wrap gap-1.5">
          {CERTIFICATE_TOKENS.map(([token]) => <button key={token} type="button" className="rounded-full border border-line bg-surface px-2.5 py-1 text-[11px] text-ink-700 hover:border-brand-600 hover:text-brand-600" onClick={() => onChange({ content: `${field.content || ""}{{${token}}}` })}>{t(TOKEN_LABELS[token])}</button>)}
        </div>
      </div>
      <label className="grid gap-2 text-xs font-semibold text-ink-600">
        {t("certificate_studio_font")}
        <select value={field.font || "public_sans"} onChange={(e) => onChange({ font: e.target.value })} className="h-10 rounded-lg border border-line bg-surface px-3 text-sm font-normal text-ink-900">
          {FONTS.map(([value, label]) => <option key={value} value={value}>{label}</option>)}
        </select>
      </label>
      <label className="grid gap-2 text-xs font-semibold text-ink-600">
        {t("certificate_studio_size")}
        <input aria-label={`${t("certificate_studio_font")} ${t("certificate_studio_size")}`} type="range" min="6" max="72" value={field.size_pt || 14} onChange={(e) => onChange({ size_pt: Number(e.target.value) })} />
        <span className="text-right text-xs font-normal">{field.size_pt || 14} pt</span>
      </label>
      <div className="grid grid-cols-5 gap-1">
        <button type="button" aria-label={t("certificate_studio_bold")} className={`rounded-lg border p-2 ${field.weight === "bold" ? "border-brand-600 bg-brand-50 text-brand-600" : "border-line"}`} onClick={() => onChange({ weight: field.weight === "bold" ? "regular" : "bold" })}><Bold className="mx-auto size-4" /></button>
        <button type="button" aria-label={t("certificate_studio_italic")} className={`rounded-lg border p-2 ${field.italic ? "border-brand-600 bg-brand-50 text-brand-600" : "border-line"}`} onClick={() => onChange({ italic: !field.italic })}><Italic className="mx-auto size-4" /></button>
        {([["left", AlignLeft, "certificate_studio_align_left"], ["center", AlignCenter, "certificate_studio_align_center"], ["right", AlignRight, "certificate_studio_align_right"]] as const).map(([align, Icon, label]) => <button key={align} type="button" aria-label={t(label)} className={`rounded-lg border p-2 ${field.align === align ? "border-brand-600 bg-brand-50 text-brand-600" : "border-line"}`} onClick={() => onChange({ align })}><Icon className="mx-auto size-4" /></button>)}
      </div>
      <label className="flex items-center justify-between text-sm text-ink-700">{t("certificate_studio_color")}<input aria-label={t("certificate_studio_color")} type="color" value={field.color || "#17213B"} onChange={(e) => onChange({ color: e.target.value })} /></label>
      <label className="flex items-center justify-between text-sm text-ink-700">{t("certificate_studio_visible")}<input type="checkbox" checked={field.visible} onChange={(e) => onChange({ visible: e.target.checked })} /></label>
      <Button type="button" variant="ghost" className="w-full text-destructive" onClick={onDelete}><Trash2 className="mr-2 size-4" />{t("certificate_studio_delete_layer")}</Button>
    </div>
  );
}
