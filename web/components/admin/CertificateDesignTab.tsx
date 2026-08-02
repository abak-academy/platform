"use client";

import { useEffect, useRef, useState, type ChangeEvent } from "react";
import { CalendarDays, ChevronDown, ChevronUp, Eye, EyeOff, FileSignature, ImagePlus, Save, Trophy, Type, Upload, UserRound } from "lucide-react";
import { toast } from "sonner";
import { Button } from "@/components/ui/button";
import { Skeleton } from "@/components/ui/skeleton";
import { CertificateFieldEditor } from "./CertificateFieldEditor";
import { CertificateInspector } from "./CertificateInspector";
import { createImageLayer, createTextLayer, moveLayer, normalizeCertificateLayout } from "@/lib/certificate-studio";
import { serializeCertificateTemplate, useCertificateDesign, usePresignCertificateAsset, useUpdateCertificateDesign } from "@/lib/hooks/admin-exams";
import { useTranslation } from "@/lib/i18n";
import type { CertificateLayout, CertificateLayoutField, ExamDetail } from "@/lib/types";

interface Props {
  examId: string;
  exam: ExamDetail;
  onSaved?: () => void;
}

const DATA_ELEMENTS = [
  ["student_name", "certificate_field_student_name", "student_name", UserRound],
  ["exam_title", "certificate_field_exam_title", "exam_title", Type],
  ["date", "certificate_studio_completion_date", "completion_date", CalendarDays],
  ["certificate_number", "certificate_field_certificate_number", "certificate_number", FileSignature],
  ["score", "score", "score", Trophy],
  ["max_score", "certificate_studio_max_score", "max_score", Trophy],
  ["score_percent", "certificate_studio_score_percent", "score_percent", Trophy],
  ["rank", "certificate_studio_rank", "rank", Trophy],
  ["percentile", "certificate_studio_percentile", "percentile", Trophy],
  ["duration", "certificate_studio_duration", "duration", CalendarDays],
  ["total_questions", "certificate_studio_total_questions", "total_questions", Type],
] as const;

export function CertificateDesignTab({ examId, exam, onSaved }: Props) {
  const { t } = useTranslation();
  const { data, isLoading, isError } = useCertificateDesign(examId);
  const updateDesign = useUpdateCertificateDesign(examId);
  const presign = usePresignCertificateAsset(examId);
  const [template, setTemplate] = useState("classic");
  const [backgroundKey, setBackgroundKey] = useState<string | null>(null);
  const [backgroundUrl, setBackgroundUrl] = useState<string | null>(null);
  const [layout, setLayout] = useState<CertificateLayout | null>(null);
  const [assetUrls, setAssetUrls] = useState<Record<string, string>>({});
  const [selectedId, setSelectedId] = useState<string | null>(null);
  const [dirty, setDirty] = useState(false);
  const [uploading, setUploading] = useState(false);
  const [imageMode, setImageMode] = useState<"add" | "replace" | "signature">("add");
  const backgroundInput = useRef<HTMLInputElement>(null);
  const imageInput = useRef<HTMLInputElement>(null);

  useEffect(() => {
    if (!data) return;
    const normalized = normalizeCertificateLayout(data.layout);
    setTemplate(data.template || "classic");
    setBackgroundKey(data.background_key);
    setBackgroundUrl(data.background_url ?? data.presets?.find((preset) => preset.template === data.template)?.background_url ?? data.presets?.[0]?.background_url ?? null);
    setLayout(normalized);
    setAssetUrls(data.asset_urls || (data.signature_url ? { signature: data.signature_url } : {}));
    setSelectedId(normalized.fields.find((field) => field.visible)?.id || null);
    setDirty(false);
  }, [data]);

  if (isLoading) return <div className="space-y-3"><Skeleton className="h-16" /><Skeleton className="h-[620px]" /></div>;
  if (isError || !layout) return <div className="rounded-xl border border-destructive/20 bg-destructive/10 p-5 text-destructive">{t("certificate_studio_load_failed")}</div>;
  const workingLayout = layout;

  const selected = workingLayout.fields.find((field) => field.id === selectedId) || null;

  function setFields(fields: CertificateLayoutField[]) {
    setLayout((current) => current ? { ...current, fields } : current);
    setDirty(true);
  }

  function patchSelected(patch: Partial<CertificateLayoutField>) {
    if (!selectedId) return;
    setFields(workingLayout.fields.map((field) => field.id === selectedId ? { ...field, ...patch } : field));
  }

  function addText() {
    const field = createTextLayer(t("certificate_studio_custom_text"), t("certificate_studio_custom_text"), undefined, workingLayout.fields);
    setFields([...workingLayout.fields, field]);
    setSelectedId(field.id);
  }

  function addData(id: string, label: string, token: string) {
    if (workingLayout.fields.some((field) => field.id === id)) return;
    const field = createTextLayer(`{{${token}}}`, label, id, workingLayout.fields);
    setFields([...workingLayout.fields, field]);
    setSelectedId(field.id);
  }

  async function upload(file: File) {
    const signed = await presign.mutateAsync({ filename: file.name, content_type: file.type });
    const response = await fetch(signed.url, { method: "PUT", body: file, headers: { "Content-Type": file.type } });
    if (!response.ok) throw new Error(`Upload failed: ${response.status}`);
    return { key: signed.key, url: URL.createObjectURL(file) };
  }

  async function backgroundChanged(e: ChangeEvent<HTMLInputElement>) {
    const file = e.target.files?.[0];
    if (!file) return;
    setUploading(true);
    try {
      const uploaded = await upload(file);
      setBackgroundKey(uploaded.key);
      setBackgroundUrl(uploaded.url);
      setTemplate("custom");
      setDirty(true);
    } catch (error) {
      toast.error(error instanceof Error ? error.message : t("certificate_studio_upload_failed"));
    } finally {
      setUploading(false);
      e.target.value = "";
    }
  }

  async function imageChanged(e: ChangeEvent<HTMLInputElement>) {
    const file = e.target.files?.[0];
    if (!file) return;
    setUploading(true);
    try {
      const uploaded = await upload(file);
      if (imageMode === "replace" && selected?.kind === "image") {
        patchSelected({ asset_key: uploaded.key, name: file.name });
        setAssetUrls((urls) => ({ ...urls, [selected.id]: uploaded.url }));
      } else {
        const field = createImageLayer(uploaded.key, imageMode === "signature" ? t("certificate_studio_signature") : file.name, workingLayout.fields, imageMode === "signature" ? "signature" : undefined);
        setFields([...workingLayout.fields, field]);
        setAssetUrls((urls) => ({ ...urls, [field.id]: uploaded.url }));
        setSelectedId(field.id);
      }
    } catch (error) {
      toast.error(error instanceof Error ? error.message : t("certificate_studio_upload_failed"));
    } finally {
      setUploading(false);
      setImageMode("add");
      e.target.value = "";
    }
  }

  function choosePreset(value: string) {
    const preset = data?.presets?.find((item) => item.template === value);
    if (!preset || value === template) return;
    if (dirty && !window.confirm(t("certificate_studio_replace_preset_confirm"))) return;
    const next = normalizeCertificateLayout(preset.layout);
    setTemplate(value);
    setBackgroundKey(null);
    setBackgroundUrl(preset.background_url);
    setLayout(next);
    setAssetUrls({});
    setSelectedId(next.fields.find((field) => field.visible)?.id || null);
    setDirty(true);
  }

  async function save() {
    try {
      const templateHtml = await serializeCertificateTemplate(workingLayout);
      await updateDesign.mutateAsync({ template, background_key: backgroundKey, layout: workingLayout, template_html: templateHtml });
      setDirty(false);
      toast.success(t("certificate_studio_saved"));
      onSaved?.();
    } catch {
      toast.error(t("certificate_studio_save_failed"));
    }
  }

  function removeSelected() {
    if (!selectedId) return;
    setFields(workingLayout.fields.filter((field) => field.id !== selectedId));
    setAssetUrls((urls) => {
      const next = { ...urls };
      delete next[selectedId];
      return next;
    });
    setSelectedId(null);
  }

  return (
    <div className="overflow-hidden rounded-2xl border border-[#D9DDEA] bg-[#F7F8FC] shadow-sm">
      <div className="sticky top-0 z-20 flex flex-wrap items-center justify-between gap-3 border-b border-[#D9DDEA] bg-white/95 px-5 py-4 backdrop-blur">
        <div>
          <p className="font-[var(--font-certificate-playfair-display)] text-xl font-semibold text-[#17213B]">{t("certificate_studio_title")}</p>
          <p className="text-xs text-[#767DA2]">{dirty ? t("certificate_studio_unsaved") : t("certificate_studio_saved_state")}</p>
        </div>
        <Button type="button" className="rounded-full bg-[#4355E7] px-5" disabled={!dirty || uploading || updateDesign.isPending} onClick={save}><Save className="mr-2 size-4" />{updateDesign.isPending ? t("saving") : t("save_changes")}</Button>
      </div>

      <div className="grid min-h-[680px] xl:h-[calc(100vh-180px)] xl:max-h-[860px] xl:grid-cols-[250px_minmax(0,1fr)_290px]">
        <aside className="space-y-7 border-b border-[#D9DDEA] bg-white p-4 xl:overflow-y-auto xl:border-b-0 xl:border-r">
          <section>
            <p className="mb-3 text-[11px] font-semibold uppercase tracking-[0.16em] text-[#767DA2]">{t("certificate_studio_templates")}</p>
            <div className="grid grid-cols-3 gap-2 xl:grid-cols-1">
              {(data?.presets || []).map((preset) => <button key={preset.template} type="button" onClick={() => choosePreset(preset.template)} className={`overflow-hidden rounded-xl border text-left transition ${template === preset.template ? "border-[#4355E7] ring-2 ring-[#4355E7]/15" : "border-[#D9DDEA] hover:border-[#9DA3C2]"}`}><img src={preset.background_url} alt="" className="aspect-[297/90] w-full object-cover" /><span className="block px-3 py-2 text-xs font-medium capitalize text-[#353A60]">{preset.template}</span></button>)}
            </div>
            <button type="button" className="mt-2 flex w-full items-center justify-center gap-2 rounded-xl border border-dashed border-[#9DA3C2] px-3 py-2.5 text-xs font-medium text-[#565C84] hover:border-[#4355E7] hover:text-[#4355E7]" onClick={() => backgroundInput.current?.click()}><Upload className="size-4" />{t("certificate_studio_upload_background")}</button>
          </section>

          <section>
            <p className="mb-3 text-[11px] font-semibold uppercase tracking-[0.16em] text-[#767DA2]">{t("certificate_studio_add_element")}</p>
            <div className="grid grid-cols-2 gap-2">
              <button type="button" onClick={addText} className="rounded-xl border border-[#D9DDEA] p-3 text-left text-xs text-[#353A60] hover:border-[#4355E7]"><Type className="mb-2 size-4 text-[#4355E7]" />{t("certificate_studio_text")}</button>
              <button type="button" onClick={() => { setImageMode("add"); imageInput.current?.click(); }} className="rounded-xl border border-[#D9DDEA] p-3 text-left text-xs text-[#353A60] hover:border-[#4355E7]"><ImagePlus className="mb-2 size-4 text-[#C99A3D]" />{t("certificate_studio_image")}</button>
              <button type="button" disabled={workingLayout.fields.some((field) => field.id === "signature")} onClick={() => { setImageMode("signature"); imageInput.current?.click(); }} className="rounded-xl border border-[#D9DDEA] p-3 text-left text-xs text-[#353A60] hover:border-[#4355E7] disabled:cursor-not-allowed disabled:opacity-35"><FileSignature className="mb-2 size-4 text-[#C99A3D]" />{t("certificate_studio_signature")}</button>
              {DATA_ELEMENTS.map(([id, label, token, Icon]) => <button key={id} type="button" disabled={workingLayout.fields.some((field) => field.id === id)} onClick={() => addData(id, t(label), token)} className="rounded-xl border border-[#D9DDEA] p-3 text-left text-xs text-[#353A60] hover:border-[#4355E7] disabled:cursor-not-allowed disabled:opacity-35"><Icon className="mb-2 size-4 text-[#4355E7]" />{t(label)}</button>)}
            </div>
          </section>

          <section>
            <p className="mb-3 text-[11px] font-semibold uppercase tracking-[0.16em] text-[#767DA2]">{t("certificate_studio_layers")}</p>
            <div className="space-y-1">
              {[...workingLayout.fields].reverse().map((field) => <div key={field.id} className={`flex items-center gap-1 rounded-lg px-2 py-1.5 ${selectedId === field.id ? "bg-[#EEF0FF]" : "hover:bg-[#F7F8FC]"}`}><button type="button" className="min-w-0 flex-1 truncate text-left text-xs capitalize text-[#353A60]" onClick={() => setSelectedId(field.id)}>{field.name || field.id.replaceAll("_", " ")}</button><button type="button" aria-label={`${t("certificate_studio_toggle")} ${field.name}`} className="p-1" onClick={() => setFields(workingLayout.fields.map((item) => item.id === field.id ? { ...item, visible: !item.visible } : item))}>{field.visible ? <Eye className="size-3.5" /> : <EyeOff className="size-3.5" />}</button><button type="button" aria-label={`${t("certificate_studio_move_forward")} ${field.name}`} className="p-1" onClick={() => setFields(moveLayer(workingLayout.fields, field.id, "forward"))}><ChevronUp className="size-3.5" /></button><button type="button" aria-label={`${t("certificate_studio_move_backward")} ${field.name}`} className="p-1" onClick={() => setFields(moveLayer(workingLayout.fields, field.id, "backward"))}><ChevronDown className="size-3.5" /></button></div>)}
            </div>
          </section>
        </aside>

        <main className="min-w-0 bg-[#17213B] p-4 sm:p-6 xl:overflow-y-auto">
          <CertificateFieldEditor layout={workingLayout} onChange={setFields} backgroundUrl={backgroundUrl} examTitle={exam.title} selectedId={selectedId} onSelect={setSelectedId} assetUrls={assetUrls} />
        </main>

        <aside className="border-t border-[#D9DDEA] bg-white p-5 xl:overflow-y-auto xl:border-l xl:border-t-0">
          <CertificateInspector field={selected} onChange={patchSelected} onDelete={removeSelected} onReplaceImage={() => { setImageMode("replace"); imageInput.current?.click(); }} />
        </aside>
      </div>

      <input ref={backgroundInput} hidden type="file" accept="image/png,image/jpeg" onChange={backgroundChanged} data-testid="certificate-background-upload-input" />
      <input ref={imageInput} hidden type="file" accept="image/png,image/jpeg" onChange={imageChanged} data-testid="certificate-image-upload-input" />
    </div>
  );
}
