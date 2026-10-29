"use client";

import { useEffect, useRef, useState } from "react";
import { useQueryClient } from "@tanstack/react-query";
import { CheckCircle, Download, Loader2 } from "lucide-react";
import { toast } from "sonner";
import { BulkFormatGuide } from "@/components/admin/BulkFormatGuide";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import {
  adminAffiliatorsKeys,
  putFileToPresignedURL,
  useEnqueueAffiliatorBulkImport,
  usePresignAffiliatorBulkUpload,
} from "@/lib/hooks/admin-affiliators";
import { useJobStatus } from "@/lib/hooks/jobs";
import { useTranslation } from "@/lib/i18n";
import {
  AFFILIATOR_ACCOUNT_BULK_FIELDS,
  AFFILIATOR_ACCOUNT_GUIDE_PITFALL_KEYS,
  buildAffiliatorGuideText,
  buildAffiliatorTemplateCSV,
  downloadTextFile,
} from "@/lib/bulk-import-format";

// Bulk account creation: download the template, upload the CSV, follow the job,
// then download the report. The report is the only place generated passwords appear.
export function AffiliatorBulkPanel() {
  const { t } = useTranslation();
  const [file, setFile] = useState<File | null>(null);
  const [jobId, setJobId] = useState<string | null>(null);
  const qc = useQueryClient();
  const refreshedFor = useRef<string | null>(null);
  const presign = usePresignAffiliatorBulkUpload();
  const enqueue = useEnqueueAffiliatorBulkImport();
  const job = useJobStatus(jobId);
  const uploading = presign.isPending || enqueue.isPending;
  const data = job.data;
  const done = data?.status === "succeeded";
  const failed = data?.status === "failed";

  // The job creates accounts out of band, so the list must be refetched once it ends.
  useEffect(() => {
    if (jobId && (done || failed) && refreshedFor.current !== jobId) {
      refreshedFor.current = jobId;
      qc.invalidateQueries({ queryKey: adminAffiliatorsKeys.all });
      qc.invalidateQueries({ queryKey: ["admin", "accounts"] });
    }
  }, [jobId, done, failed, qc]);

  async function submit(e: React.FormEvent) {
    e.preventDefault();
    if (!file) {
      toast.error(t("aff_bulk_no_file"));
      return;
    }
    try {
      const signed = await presign.mutateAsync({ filename: file.name, contentType: file.type || "text/csv" });
      try {
        await putFileToPresignedURL(signed.url, file, file.type || "text/csv");
      } catch (err) {
        toast.error(err instanceof Error ? err.message : t("aff_bulk_put_failed"));
        return;
      }
      const queued = await enqueue.mutateAsync({ fileKey: signed.key });
      setJobId(queued.job_id);
    } catch (err) {
      toast.error(err instanceof Error ? err.message : t("aff_bulk_enqueue_failed"));
    }
  }

  return (
    <div className="space-y-6">
      <section>
        <h3 className="text-sm font-semibold text-ink-900">1. {t("aff_bulk_download_template")}</h3>
        <div className="mt-2 flex flex-wrap gap-2">
          <Button
            type="button"
            variant="outline"
            size="sm"
            className="rounded-full"
            onClick={() => downloadTextFile("bulk_affiliator_template.csv", buildAffiliatorTemplateCSV(), "text/csv;charset=utf-8")}
          >
            <Download className="mr-2 size-4" />
            {t("aff_bulk_download_template")}
          </Button>
          <Button
            type="button"
            variant="outline"
            size="sm"
            className="rounded-full"
            onClick={() => downloadTextFile("bulk_affiliator_guide.txt", buildAffiliatorGuideText(t), "text/plain;charset=utf-8")}
          >
            <Download className="mr-2 size-4" />
            {t("bulk_format_download_guide")}
          </Button>
        </div>
        <div className="mt-3">
          <BulkFormatGuide fields={AFFILIATOR_ACCOUNT_BULK_FIELDS} pitfallKeys={AFFILIATOR_ACCOUNT_GUIDE_PITFALL_KEYS} />
        </div>
      </section>

      <section>
        <h3 className="text-sm font-semibold text-ink-900">2. {t("aff_bulk_upload")}</h3>
        <form onSubmit={submit} className="mt-2 space-y-3">
          <div className="grid gap-2">
            <Label htmlFor="aff-bulk-file">{t("aff_bulk_choose_file")}</Label>
            <Input id="aff-bulk-file" type="file" accept=".csv,text/csv" disabled={uploading || Boolean(jobId)} onChange={(e) => setFile(e.target.files?.[0] ?? null)} />
          </div>
          <Button type="submit" className="rounded-full" disabled={uploading || !file || Boolean(jobId)}>
            {uploading ? <Loader2 className="mr-2 size-4 animate-spin" /> : null}
            {uploading ? t("aff_bulk_uploading") : t("aff_bulk_upload")}
          </Button>
        </form>
      </section>

      {data && (
        <section className="md-card-outlined space-y-3 p-4" aria-live="polite">
          {done && (
            <div className="flex items-center gap-2">
              <CheckCircle className="size-5 text-success" />
              <h4 className="text-sm font-semibold text-ink-900">{t("aff_bulk_success")}</h4>
            </div>
          )}
          {!done && !failed && (
            <div className="space-y-2">
              <p className="text-sm font-medium text-ink-900">{t("aff_bulk_progress").replace("{pct}", String(Math.round(data.progress ?? 0)))}</p>
              <div className="h-2 w-full overflow-hidden rounded-full bg-surface-2">
                <div className="h-full bg-primary transition-all" style={{ width: `${Math.max(0, Math.min(100, data.progress ?? 0))}%` }} />
              </div>
            </div>
          )}
          {failed && (
            <div className="space-y-1">
              <h4 className="text-sm font-semibold text-danger">{t("aff_bulk_failed")}</h4>
              {data.error && <p className="text-sm text-danger">{data.error}</p>}
            </div>
          )}
          {(done || failed) && data.result_url && (
            <div className="space-y-1">
              <a
                href={data.result_url}
                className="inline-flex items-center gap-2 text-sm font-medium text-primary underline-offset-4 hover:underline"
                download="bulk_affiliator_result.csv"
              >
                <Download className="size-4" />
                {t("aff_bulk_download_result")}
              </a>
              <p className="text-xs text-ink-600">{t("aff_bulk_result_hint")}</p>
            </div>
          )}
        </section>
      )}
    </div>
  );
}
