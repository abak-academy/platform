"use client";

import { useRef, useState } from "react";
import { Download, Loader2, CheckCircle } from "lucide-react";
import { toast } from "sonner";
import { useTranslation } from "@/lib/i18n";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import {
  usePresignStudentBulkUpload,
  putFileToPresignedURL,
  useEnqueueStudentBulkImport,
} from "@/lib/hooks/admin-students-bulk";
import { useJobStatus } from "@/lib/hooks/jobs";
import { BulkFormatGuide } from "@/components/admin/BulkFormatGuide";
import {
  STUDENT_BULK_FIELDS,
  STUDENT_GUIDE_PITFALL_KEYS,
  SUPER_ADMIN_STUDENT_BULK_FIELDS,
  buildStudentGuideText,
  buildStudentTemplateCSV,
  downloadTextFile,
} from "@/lib/bulk-import-format";

function downloadTemplate(allowExplicitPassword: boolean): void {
  downloadTextFile("bulk_register_template.csv", buildStudentTemplateCSV(allowExplicitPassword), "text/csv;charset=utf-8");
}

interface BulkImportModalProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  allowExplicitPassword: boolean;
}

export function BulkImportModal({ open, onOpenChange, allowExplicitPassword }: BulkImportModalProps) {
  const { t } = useTranslation();

  const fileInputRef = useRef<HTMLInputElement | null>(null);
  const [file, setFile] = useState<File | null>(null);
  const [tempPassword, setTempPassword] = useState("");
  const [jobId, setJobId] = useState<string | null>(null);

  const presign = usePresignStudentBulkUpload();
  const enqueue = useEnqueueStudentBulkImport();
  const job = useJobStatus(jobId);

  const isUploading = presign.isPending || enqueue.isPending;

  function handleFileChange(e: React.ChangeEvent<HTMLInputElement>) {
    setFile(e.target.files?.[0] ?? null);
  }

  async function handleSubmit(e: React.FormEvent) {
    e.preventDefault();
    if (!file) {
      toast.error(t("bulk_register_no_file"));
      return;
    }
    try {
      const presignResp = await presign.mutateAsync({
        filename: file.name,
        contentType: file.type || "text/csv",
      });
      try {
        await putFileToPresignedURL(presignResp.url, file, file.type || "text/csv");
      } catch (err) {
        toast.error(err instanceof Error ? err.message : t("bulk_register_put_failed"));
        return;
      }
      const enqueueResp = await enqueue.mutateAsync({
        fileKey: presignResp.key,
        tempPassword: tempPassword || undefined,
      });
      setJobId(enqueueResp.job_id);
    } catch (err) {
      toast.error(err instanceof Error ? err.message : t("bulk_register_enqueue_failed"));
    }
  }

  function handleClose() {
    setFile(null);
    setTempPassword("");
    setJobId(null);
    onOpenChange(false);
  }

  const jobData = job.data;
  const isTerminalSuccess = jobData?.status === "succeeded";
  const isTerminalFailed = jobData?.status === "failed";
  const bulkFields = allowExplicitPassword ? SUPER_ADMIN_STUDENT_BULK_FIELDS : STUDENT_BULK_FIELDS;

  return (
    <Dialog
      open={open}
      onOpenChange={(o) => {
        if (!o) handleClose();
      }}
    >
        <DialogContent className="sm:max-w-2xl">
        <DialogHeader>
          <DialogTitle className="font-serif">{t("bulk_register_title")}</DialogTitle>
          <DialogDescription>{t("bulk_register_subtitle")}</DialogDescription>
        </DialogHeader>

        <div className="space-y-6">
          {/* Step 1: Download template */}
          <section>
            <h3 className="text-sm font-semibold text-ink-900">
              1. {t("bulk_register_download_template")}
            </h3>
            <div className="mt-2 flex flex-wrap gap-2">
              <Button
                type="button"
                variant="outline"
                size="sm"
                className="rounded-full"
                onClick={() => downloadTemplate(allowExplicitPassword)}
              >
                <Download className="mr-2 size-4" />
                {t("bulk_register_download_template")}
              </Button>
              <Button
                type="button"
                variant="outline"
                size="sm"
                className="rounded-full"
                onClick={() =>
                  downloadTextFile(
                    "bulk_register_guide.txt",
                    buildStudentGuideText(t, allowExplicitPassword),
                    "text/plain;charset=utf-8",
                  )
                }
              >
                <Download className="mr-2 size-4" />
                {t("bulk_format_download_guide")}
              </Button>
            </div>
            <div className="mt-3">
              <BulkFormatGuide fields={bulkFields} pitfallKeys={STUDENT_GUIDE_PITFALL_KEYS} />
            </div>
          </section>

          {/* Step 2: Upload */}
          <section>
            <h3 className="text-sm font-semibold text-ink-900">
              2. {t("bulk_register_upload")}
            </h3>
            <form onSubmit={handleSubmit} className="mt-2 space-y-3">
              <div className="grid gap-2">
                <Label htmlFor="bulk-register-file">{t("bulk_register_choose_file")}</Label>
                <Input
                  ref={fileInputRef}
                  id="bulk-register-file"
                  type="file"
                  accept=".csv,text/csv"
                  onChange={handleFileChange}
                  disabled={isUploading}
                />
                {file && <p className="text-sm text-muted-foreground">{file.name}</p>}
              </div>

              <div className="grid gap-2">
                <Label htmlFor="bulk-register-temp-password">
                  {t("bulk_register_temp_password_label")}
                </Label>
                <Input
                  id="bulk-register-temp-password"
                  type="text"
                  value={tempPassword}
                  onChange={(e) => setTempPassword(e.target.value)}
                  placeholder={t("bulk_register_temp_password_placeholder")}
                  autoComplete="off"
                  minLength={8}
                  disabled={isUploading}
                />
                <p className="text-[12px] leading-[1.5] text-muted-foreground">
                  {t("bulk_register_temp_password_hint")}
                </p>
              </div>

              <Button type="submit" className="rounded-full" disabled={isUploading || !file}>
                {isUploading ? <Loader2 className="mr-2 size-4 animate-spin" /> : null}
                {isUploading ? t("bulk_register_uploading") : t("bulk_register_upload")}
              </Button>
            </form>
          </section>

          {/* Step 3: Progress + result */}
          {jobData && (
            <section className="md-card-outlined space-y-3 p-4">
              {isTerminalSuccess && (
                <div className="flex items-center gap-2">
                  <CheckCircle className="size-5 text-success" />
                  <h4 className="text-sm font-semibold text-ink-900">
                    {t("bulk_register_success")}
                  </h4>
                </div>
              )}

              {!isTerminalSuccess && !isTerminalFailed && (
                <div className="space-y-2">
                  <p className="text-sm font-medium text-ink-900">
                    {t("bulk_register_progress").replace(
                      "{pct}",
                      String(Math.round(jobData.progress ?? 0)),
                    )}
                  </p>
                  <div className="h-2 w-full overflow-hidden rounded-full bg-surface-2">
                    <div
                      className="h-full bg-primary transition-all"
                      style={{ width: `${Math.max(0, Math.min(100, jobData.progress ?? 0))}%` }}
                    />
                  </div>
                </div>
              )}

              {isTerminalFailed && (
                <div className="space-y-2">
                  <h4 className="text-sm font-semibold text-danger">
                    {t("bulk_register_failed")}
                  </h4>
                  {jobData.error && <p className="text-sm text-danger">{jobData.error}</p>}
                </div>
              )}

              {/* An all-rows-failed job still uploads a per-row report — that is
                  precisely when the operator needs to read it. */}
              {(isTerminalSuccess || isTerminalFailed) && jobData.result_url && (
                <a
                  href={jobData.result_url}
                  className="inline-flex items-center gap-2 text-sm font-medium text-primary underline-offset-4 hover:underline"
                  download="bulk_register_result.csv"
                >
                  <Download className="size-4" />
                  {t("bulk_register_download_result")}
                </a>
              )}
            </section>
          )}
        </div>
      </DialogContent>
    </Dialog>
  );
}
