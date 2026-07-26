"use client";

import { useEffect, useRef, useState, type ChangeEvent } from "react";
import { toast } from "sonner";
import { Upload } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Skeleton } from "@/components/ui/skeleton";
import { useTranslation } from "@/lib/i18n";
import { usePresignUpload } from "@/lib/hooks/students";
import {
  fetchCertificatePreview,
  useCertificateDesign,
  useUpdateCertificateDesign,
} from "@/lib/hooks/admin-exams";
import { CertificateFieldEditor } from "@/components/admin/CertificateFieldEditor";
import type { CertificateLayout, CertificateLayoutField, ExamDetail } from "@/lib/types";

const TEMPLATE_OPTIONS = ["classic", "modern", "elegant", "custom"] as const;
type CertificateTemplateOption = (typeof TEMPLATE_OPTIONS)[number];

const TEMPLATE_CARD_BASE =
  "flex items-center justify-center gap-2 rounded-md border px-3 py-2 text-sm transition-colors";
const TEMPLATE_CARD_ON = "border-brand-400 bg-brand-50 text-brand-800";
const TEMPLATE_CARD_OFF = "border-line text-ink-700 hover:border-ink-300";

interface CertificateDesignTabProps {
  examId: string;
  exam: ExamDetail;
  onSaved?: () => void;
}

export function CertificateDesignTab({ examId, exam, onSaved }: CertificateDesignTabProps) {
  const { t } = useTranslation();
  const { data, isLoading, isError } = useCertificateDesign(examId);
  const updateDesign = useUpdateCertificateDesign(examId);
  const presign = usePresignUpload();

  const [initialized, setInitialized] = useState(false);
  const [template, setTemplate] = useState<CertificateTemplateOption>("classic");
  const [backgroundKey, setBackgroundKey] = useState<string | null>(null);
  const [backgroundUrl, setBackgroundUrl] = useState<string | null>(null);
  const [layout, setLayout] = useState<CertificateLayout | null>(null);
  const [uploading, setUploading] = useState(false);
  const [signatureUrl, setSignatureUrl] = useState<string | null>(null);
  const [signatureUploading, setSignatureUploading] = useState(false);
  const [previewUrl, setPreviewUrl] = useState<string | null>(null);
  const [previewLoading, setPreviewLoading] = useState(false);

  const fileInputRef = useRef<HTMLInputElement | null>(null);
  const signatureInputRef = useRef<HTMLInputElement | null>(null);
  const previewUrlRef = useRef<string | null>(null);
  const backgroundObjectUrlRef = useRef<string | null>(null);
  const signatureObjectUrlRef = useRef<string | null>(null);

  useEffect(() => {
    if (!data || initialized) return;
    setTemplate((data.template as CertificateTemplateOption) ?? "classic");
    setBackgroundUrl(data.background_url ?? null);
    setBackgroundKey(data.background_key ?? null);
    setSignatureUrl(data.signature_url ?? null);
    setLayout(data.layout);
    setInitialized(true);
  }, [data, initialized]);

  const signatureField = layout?.fields.find((f) => f.id === "signature") ?? null;
  const signatureVisible = signatureField?.visible ?? false;

  function setSignatureVisible(visible: boolean) {
    setLayout((prev) => {
      if (!prev) return prev;
      const hasField = prev.fields.some((f) => f.id === "signature");
      const fields = hasField
        ? prev.fields.map((f) => (f.id === "signature" ? { ...f, visible } : f))
        : [
            ...prev.fields,
            { id: "signature", x_mm: 205, y_mm: 150, w_mm: 62, h_mm: 22, align: "center", visible },
          ];
      return { ...prev, fields };
    });
  }

  async function handleSignatureSelected(e: ChangeEvent<HTMLInputElement>) {
    const file = e.target.files?.[0];
    if (!file) return;
    setSignatureUploading(true);
    try {
      const presigned = await presign.mutateAsync({ filename: file.name, content_type: file.type });
      const uploadRes = await fetch(presigned.url, {
        method: "PUT",
        body: file,
        headers: { "Content-Type": file.type },
      });
      if (!uploadRes.ok) throw new Error(`Upload failed: ${uploadRes.status}`);
      if (signatureObjectUrlRef.current) URL.revokeObjectURL(signatureObjectUrlRef.current);
      const localUrl = URL.createObjectURL(file);
      signatureObjectUrlRef.current = localUrl;
      setSignatureUrl(localUrl);
      setLayout((prev) => (prev ? { ...prev, signature_key: presigned.key } : prev));
      setSignatureVisible(true);
    } catch (err) {
      toast.error(err instanceof Error ? err.message : t("error_generic"));
    } finally {
      setSignatureUploading(false);
      if (signatureInputRef.current) signatureInputRef.current.value = "";
    }
  }

  function handleFieldsChange(fields: CertificateLayoutField[]) {
    setLayout((prev) => (prev ? { ...prev, fields } : prev));
  }

  async function loadPreview(tmpl: string, layoutOverride?: CertificateLayout) {
    setPreviewLoading(true);
    try {
      const blob = await fetchCertificatePreview(examId, tmpl, layoutOverride);
      const url = URL.createObjectURL(blob);
      if (previewUrlRef.current) URL.revokeObjectURL(previewUrlRef.current);
      previewUrlRef.current = url;
      setPreviewUrl(url);
    } catch {
      toast.error(t("error_generic"));
    } finally {
      setPreviewLoading(false);
    }
  }

  // FR-17: the PDF preview is only rendered on explicit "Generate PDF" click,
  // not on every layout edit — the WYSIWYG editor above already gives live
  // feedback client-side with no round-trip.
  function handleGeneratePdf() {
    loadPreview(template, layout ?? undefined);
  }

  useEffect(() => {
    return () => {
      if (previewUrlRef.current) URL.revokeObjectURL(previewUrlRef.current);
      if (backgroundObjectUrlRef.current) URL.revokeObjectURL(backgroundObjectUrlRef.current);
      if (signatureObjectUrlRef.current) URL.revokeObjectURL(signatureObjectUrlRef.current);
    };
  }, []);

  async function handleFileSelected(e: ChangeEvent<HTMLInputElement>) {
    const file = e.target.files?.[0];
    if (!file) return;
    setUploading(true);
    try {
      const presigned = await presign.mutateAsync({
        filename: file.name,
        content_type: file.type,
      });
      const uploadRes = await fetch(presigned.url, {
        method: "PUT",
        body: file,
        headers: { "Content-Type": file.type },
      });
      if (!uploadRes.ok) {
        throw new Error(`Upload failed: ${uploadRes.status}`);
      }
      if (backgroundObjectUrlRef.current) URL.revokeObjectURL(backgroundObjectUrlRef.current);
      const localUrl = URL.createObjectURL(file);
      backgroundObjectUrlRef.current = localUrl;
      setBackgroundKey(presigned.key);
      setBackgroundUrl(localUrl);
    } catch (err) {
      toast.error(err instanceof Error ? err.message : t("error_generic"));
    } finally {
      setUploading(false);
      if (fileInputRef.current) fileInputRef.current.value = "";
    }
  }

  async function handleSave() {
    if (!data || !layout) return;
    try {
      await updateDesign.mutateAsync({
        template,
        background_key: backgroundKey,
        layout,
      });
      toast.success(t("changes_saved"));
      onSaved?.();
    } catch {
      toast.error(t("error_generic"));
    }
  }

  const saveDisabled = !initialized || updateDesign.isPending || uploading;

  return (
    <div className="md-card-outlined space-y-6 p-6">
      <div className="flex items-center justify-between">
        <h2 className="text-title-large font-semibold">
          {t("admin_exam_detail_tab_certificate")}
        </h2>
        <Button
          type="button"
          className="rounded-full"
          onClick={handleSave}
          disabled={saveDisabled}
        >
          {updateDesign.isPending ? t("saving") : t("save")}
        </Button>
      </div>

      {isLoading && (
        <div className="space-y-2">
          {Array.from({ length: 3 }).map((_, i) => (
            <Skeleton key={i} className="h-10 w-full" />
          ))}
        </div>
      )}

      {isError && (
        <div className="rounded-lg border border-destructive/20 bg-destructive/10 p-4 text-destructive">
          {t("error_generic")}
        </div>
      )}

      {initialized && (
        <div className="grid gap-6 lg:grid-cols-2">
          <div className="space-y-4">
            <div className="grid gap-2">
              <span className="text-xs font-semibold uppercase tracking-wide text-ink-500">
                {t("certificate_design_template_label")}
              </span>
              <div className="grid grid-cols-2 gap-2 sm:grid-cols-4">
                {TEMPLATE_OPTIONS.map((value) => (
                  <label
                    key={value}
                    className={`${TEMPLATE_CARD_BASE} ${
                      template === value ? TEMPLATE_CARD_ON : TEMPLATE_CARD_OFF
                    }`}
                  >
                    <input
                      type="radio"
                      name="certificate_design_template"
                      value={value}
                      checked={template === value}
                      onChange={() => setTemplate(value)}
                      disabled={updateDesign.isPending}
                    />
                    <span>{t(`certificate_template_${value}` as const)}</span>
                  </label>
                ))}
              </div>
            </div>

            <div className="grid gap-2">
              <span className="text-xs font-semibold uppercase tracking-wide text-ink-500">
                {t("certificate_design_background_label")}
              </span>
              {backgroundUrl ? (
                // eslint-disable-next-line @next/next/no-img-element
                <img
                  src={backgroundUrl}
                  alt={t("certificate_design_background_label")}
                  className="h-40 w-full rounded-md border border-line object-cover"
                />
              ) : (
                <div className="flex h-40 items-center justify-center rounded-md border border-dashed text-sm text-muted-foreground">
                  {t("certificate_design_no_background")}
                </div>
              )}
              <Button
                type="button"
                variant="outline"
                size="sm"
                className="w-fit rounded-full"
                onClick={() => fileInputRef.current?.click()}
                disabled={uploading || updateDesign.isPending}
              >
                <Upload className="mr-1 size-4" />
                {uploading ? t("saving") : t("certificate_design_upload_button")}
              </Button>
              <input
                ref={fileInputRef}
                type="file"
                accept="image/png,image/jpeg,image/gif"
                hidden
                data-testid="certificate-background-upload-input"
                onChange={handleFileSelected}
              />
            </div>

            <div className="grid gap-2">
              <span className="text-xs font-semibold uppercase tracking-wide text-ink-500">
                {t("certificate_design_signature_label")}
              </span>
              {signatureUrl ? (
                // eslint-disable-next-line @next/next/no-img-element
                <img
                  src={signatureUrl}
                  alt={t("certificate_design_signature_label")}
                  className="h-24 w-full rounded-md border border-line bg-white object-contain p-2"
                />
              ) : (
                <div className="flex h-24 items-center justify-center rounded-md border border-dashed text-sm text-muted-foreground">
                  {t("certificate_design_no_signature")}
                </div>
              )}
              <div className="flex flex-wrap items-center gap-3">
                <Button
                  type="button"
                  variant="outline"
                  size="sm"
                  className="w-fit rounded-full"
                  onClick={() => signatureInputRef.current?.click()}
                  disabled={signatureUploading || updateDesign.isPending}
                >
                  <Upload className="mr-1 size-4" />
                  {signatureUploading ? t("saving") : t("certificate_design_upload_signature_button")}
                </Button>
                <label className="flex items-center gap-2 text-sm text-ink-600">
                  <input
                    type="checkbox"
                    data-testid="certificate-signature-visible-toggle"
                    checked={signatureVisible}
                    onChange={(e) => setSignatureVisible(e.target.checked)}
                  />
                  {t("certificate_design_show_signature")}
                </label>
              </div>
              <input
                ref={signatureInputRef}
                type="file"
                accept="image/png,image/jpeg,image/gif"
                hidden
                data-testid="certificate-signature-upload-input"
                onChange={handleSignatureSelected}
              />
            </div>
          </div>

          <div className="grid gap-4">
            <div className="grid gap-2">
              <span className="text-xs font-semibold uppercase tracking-wide text-ink-500">
                {t("certificate_design_preview_label")}
              </span>
              {layout && (
                <CertificateFieldEditor
                  layout={layout}
                  onChange={handleFieldsChange}
                  backgroundUrl={backgroundUrl}
                  examTitle={exam.title}
                />
              )}
            </div>

            <div className="grid gap-2">
              <div className="flex items-center justify-between">
                <span className="text-xs font-semibold uppercase tracking-wide text-ink-500">
                  {t("certificate_design_pdf_fidelity_label")}
                </span>
                <Button
                  type="button"
                  variant="outline"
                  size="sm"
                  className="rounded-full"
                  data-testid="certificate-generate-pdf-button"
                  onClick={handleGeneratePdf}
                  disabled={previewLoading || !layout}
                >
                  {previewLoading ? t("certificate_design_generating_pdf") : t("certificate_design_generate_pdf_button")}
                </Button>
              </div>
              {previewLoading && !previewUrl ? (
                <Skeleton className="h-64 w-full" />
              ) : previewUrl ? (
                <iframe
                  title={t("certificate_design_pdf_fidelity_label")}
                  src={previewUrl}
                  className="h-64 w-full rounded-md border border-line"
                />
              ) : (
                <div className="flex h-64 items-center justify-center rounded-md border border-dashed text-sm text-muted-foreground">
                  —
                </div>
              )}
            </div>
          </div>
        </div>
      )}
    </div>
  );
}
