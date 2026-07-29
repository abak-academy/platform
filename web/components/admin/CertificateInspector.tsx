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
    return <div className="flex min-h-48 items-center justify-center rounded-xl border border-dashed border-[#D9DDEA] p-6 text-center text-sm text-[#767DA2]">{t("certificate_studio_select_element")}</div>;
  }
  if (field.kind === "image") {
    return (
      <div className="space-y-5">
        <div>
          <p className="text-xs font-semibold uppercase tracking-[0.16em] text-[#767DA2]">{t("certificate_studio_image")}</p>
          <h3 className="mt-1 font-semibold text-[#17213B]">{field.name || t("certificate_studio_image")}</h3>
        </div>
        <Button type="button" variant="outline" className="w-full" onClick={onReplaceImage}>{t("certificate_studio_replace_image")}</Button>
        <label className="flex items-center justify-between text-sm text-[#353A60]">
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
        <p className="text-xs font-semibold uppercase tracking-[0.16em] text-[#767DA2]">{t("certificate_studio_text")}</p>
        <h3 className="mt-1 font-semibold capitalize text-[#17213B]">{field.name || t("certificate_studio_text")}</h3>
      </div>
      <label className="grid gap-2 text-xs font-semibold text-[#565C84]">
        {t("certificate_studio_content")}
        <textarea aria-label={t("certificate_studio_content")} rows={4} value={field.content || ""} onChange={(e) => onChange({ content: e.target.value })} className="resize-none rounded-lg border border-[#D9DDEA] bg-white p-3 text-sm font-normal text-[#17213B] outline-none focus:border-[#4355E7] focus:ring-2 focus:ring-[#4355E7]/15" />
      </label>
      <div>
        <p className="mb-2 text-xs font-semibold text-[#565C84]">{t("certificate_studio_insert_data")}</p>
        <div className="flex flex-wrap gap-1.5">
          {CERTIFICATE_TOKENS.map(([token]) => <button key={token} type="button" className="rounded-full border border-[#D9DDEA] bg-white px-2.5 py-1 text-[11px] text-[#353A60] hover:border-[#4355E7] hover:text-[#4355E7]" onClick={() => onChange({ content: `${field.content || ""}{{${token}}}` })}>{t(TOKEN_LABELS[token])}</button>)}
        </div>
      </div>
      <label className="grid gap-2 text-xs font-semibold text-[#565C84]">
        {t("certificate_studio_font")}
        <select value={field.font || "public_sans"} onChange={(e) => onChange({ font: e.target.value })} className="h-10 rounded-lg border border-[#D9DDEA] bg-white px-3 text-sm font-normal text-[#17213B]">
          {FONTS.map(([value, label]) => <option key={value} value={value}>{label}</option>)}
        </select>
      </label>
      <label className="grid gap-2 text-xs font-semibold text-[#565C84]">
        {t("certificate_studio_size")}
        <input aria-label={`${t("certificate_studio_font")} ${t("certificate_studio_size")}`} type="range" min="6" max="72" value={field.size_pt || 14} onChange={(e) => onChange({ size_pt: Number(e.target.value) })} />
        <span className="text-right text-xs font-normal">{field.size_pt || 14} pt</span>
      </label>
      <div className="grid grid-cols-5 gap-1">
        <button type="button" aria-label={t("certificate_studio_bold")} className={`rounded-lg border p-2 ${field.weight === "bold" ? "border-[#4355E7] bg-[#EEF0FF] text-[#4355E7]" : "border-[#D9DDEA]"}`} onClick={() => onChange({ weight: field.weight === "bold" ? "regular" : "bold" })}><Bold className="mx-auto size-4" /></button>
        <button type="button" aria-label={t("certificate_studio_italic")} className={`rounded-lg border p-2 ${field.italic ? "border-[#4355E7] bg-[#EEF0FF] text-[#4355E7]" : "border-[#D9DDEA]"}`} onClick={() => onChange({ italic: !field.italic })}><Italic className="mx-auto size-4" /></button>
        {([["left", AlignLeft, "certificate_studio_align_left"], ["center", AlignCenter, "certificate_studio_align_center"], ["right", AlignRight, "certificate_studio_align_right"]] as const).map(([align, Icon, label]) => <button key={align} type="button" aria-label={t(label)} className={`rounded-lg border p-2 ${field.align === align ? "border-[#4355E7] bg-[#EEF0FF] text-[#4355E7]" : "border-[#D9DDEA]"}`} onClick={() => onChange({ align })}><Icon className="mx-auto size-4" /></button>)}
      </div>
      <label className="flex items-center justify-between text-sm text-[#353A60]">{t("certificate_studio_color")}<input aria-label={t("certificate_studio_color")} type="color" value={field.color || "#17213B"} onChange={(e) => onChange({ color: e.target.value })} /></label>
      <label className="flex items-center justify-between text-sm text-[#353A60]">{t("certificate_studio_visible")}<input type="checkbox" checked={field.visible} onChange={(e) => onChange({ visible: e.target.checked })} /></label>
      <Button type="button" variant="ghost" className="w-full text-destructive" onClick={onDelete}><Trash2 className="mr-2 size-4" />{t("certificate_studio_delete_layer")}</Button>
    </div>
  );
}
