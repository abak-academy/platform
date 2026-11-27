"use client";

import { useState } from "react";
import { toast } from "sonner";
import { Button } from "@/components/ui/button";
import { Label } from "@/components/ui/label";
import { Dialog, DialogContent, DialogDescription, DialogHeader, DialogTitle } from "@/components/ui/dialog";
import { groupingErrorParts } from "@/lib/hooks/admin-product-groupings";
import { putFileToPresignedURL } from "@/lib/hooks/admin-students-bulk";
import {
  useApplyCompetitionResult,
  useCompetitionResult,
  useCompetitionResultRows,
  useDeleteCompetitionResult,
  usePresignCompetitionResult,
  usePreviewCompetitionResult,
  useSetCompetitionResultPublished,
} from "@/lib/hooks/admin-competition-results";
import { DICT, useTranslation, type I18nKey } from "@/lib/i18n";
import type { CompetitionResultPreview } from "@/lib/types";

const dateTime = (iso: string) => new Date(iso).toLocaleString("id-ID", { dateStyle: "medium", timeStyle: "short" });

function ResultRows({ productId }: { productId: string }) {
  const { t } = useTranslation();
  const { data, isLoading } = useCompetitionResultRows(productId, true);
  if (isLoading) return <p className="text-sm text-muted-foreground">{t("cr_loading")}</p>;
  return (
    <table className="w-full text-left text-xs" aria-label={t("cr_results_label")}>
      <thead>
        <tr>
          <th>{t("cr_col_participant")}</th><th>{t("cr_col_name")}</th><th>{t("cr_col_score")}</th><th>{t("cr_col_rank")}</th><th>{t("cr_col_award")}</th>
        </tr>
      </thead>
      <tbody>
        {(data?.data ?? []).map((r) => (
          <tr key={r.registration_id}>
            <td>{r.participant_no}</td><td>{r.student_name}</td><td>{r.score}</td><td>{r.rank ?? "-"}</td><td>{r.award ?? "-"}</td>
          </tr>
        ))}
      </tbody>
    </table>
  );
}

export function CompetitionResultDialog({
  open,
  onOpenChange,
  productId,
  productName,
}: {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  productId: string;
  productName: string;
}) {
  const { t } = useTranslation();
  const [file, setFile] = useState<File | null>(null);
  const [key, setKey] = useState("");
  const [preview, setPreview] = useState<CompetitionResultPreview | null>(null);
  const [fileError, setFileError] = useState("");
  const [confirmDelete, setConfirmDelete] = useState(false);
  const [viewing, setViewing] = useState(false);

  const result = useCompetitionResult(productId).data ?? null;
  const presign = usePresignCompetitionResult(productId);
  const previewMut = usePreviewCompetitionResult(productId);
  const apply = useApplyCompetitionResult(productId);
  const setPublished = useSetCompetitionResultPublished(productId);
  const del = useDeleteCompetitionResult(productId);

  const busy = presign.isPending || previewMut.isPending || apply.isPending;

  function resetPreview() {
    setPreview(null);
    setFileError("");
  }

  async function handlePreview() {
    if (!file) return;
    resetPreview();
    try {
      const p = await presign.mutateAsync(file.name);
      await putFileToPresignedURL(p.url, file, file.type || "text/csv");
      setKey(p.key);
      setPreview(await previewMut.mutateAsync(p.key));
    } catch (e) {
      setFileError(groupingErrorParts(e).message);
    }
  }

  async function handleApply() {
    try {
      await apply.mutateAsync(key);
      toast.success(t("cr_saved_toast"));
      setFile(null);
      resetPreview();
    } catch (e) {
      toast.error(groupingErrorParts(e).message);
    }
  }

  async function run(fn: () => Promise<unknown>, ok: string) {
    try {
      await fn();
      toast.success(ok);
    } catch (e) {
      toast.error(groupingErrorParts(e).message);
    }
  }

  const reasonCopy = (reason: string) => {
    const k = `cr_reason_${reason}`;
    return k in DICT.id ? t(k as I18nKey) : reason;
  };

  const canConfirm = preview !== null && preview.errors.length === 0 && preview.matched > 0 && !busy;

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="max-h-[90vh] overflow-y-auto sm:max-w-[740px]">
        <DialogHeader>
          <DialogTitle>{t("cr_title")} — {productName}</DialogTitle>
          <DialogDescription>{t("cr_desc")}</DialogDescription>
        </DialogHeader>

        <section className="space-y-2" aria-label={t("cr_saved_title")}>
          <h3 className="text-sm font-semibold">{t("cr_saved_title")}</h3>
          {!result && <p className="text-sm text-muted-foreground">{t("cr_none")}</p>}
          {result && (
            <div className="space-y-2 rounded-md border px-3 py-2">
              <div className="flex flex-wrap items-center justify-between gap-2">
                <div>
                  <span className="text-xs text-muted-foreground">{result.row_count} {t("cr_rows")} · {t("cr_updated")} {dateTime(result.updated_at)}</span>{" "}
                  <span className={`rounded-full px-2 py-0.5 text-xs font-semibold ${result.published_at ? "bg-teal-50 text-teal-700" : "bg-muted text-muted-foreground"}`}>
                    {result.published_at ? t("cr_live") : t("cr_draft")}
                  </span>
                </div>
                <div className="flex gap-1">
                  <Button type="button" size="sm" variant="ghost" onClick={() => setViewing(!viewing)}>
                    {viewing ? t("cr_close") : t("cr_view")}
                  </Button>
                  <Button
                    type="button"
                    size="sm"
                    variant="outline"
                    disabled={setPublished.isPending}
                    onClick={() => run(() => setPublished.mutateAsync(!result.published_at), result.published_at ? t("cr_unpublished_toast") : t("cr_published_toast"))}
                  >
                    {result.published_at ? t("cr_unpublish") : t("cr_publish")}
                  </Button>
                  {confirmDelete ? (
                    <>
                      <Button type="button" size="sm" variant="destructive" onClick={() => { setConfirmDelete(false); run(() => del.mutateAsync(), t("cr_deleted_toast")); }}>
                        {t("cr_delete_yes")}
                      </Button>
                      <Button type="button" size="sm" variant="ghost" onClick={() => setConfirmDelete(false)}>{t("cr_cancel")}</Button>
                    </>
                  ) : (
                    <Button type="button" size="sm" variant="ghost" onClick={() => setConfirmDelete(true)}>{t("cr_delete")}</Button>
                  )}
                </div>
              </div>
              {confirmDelete && <p role="alert" className="text-sm text-destructive">{t("cr_delete_confirm")}</p>}
              {viewing && <ResultRows productId={productId} />}
            </div>
          )}
        </section>

        <section className="space-y-3" aria-label={t("cr_upload_title")}>
          <h3 className="text-sm font-semibold">{t("cr_upload_title")}</h3>
          <div className="space-y-1">
            <Label htmlFor="cr-file">{t("cr_file")}</Label>
            <input id="cr-file" type="file" accept=".csv,text/csv" onChange={(e) => { setFile(e.target.files?.[0] ?? null); resetPreview(); }} />
          </div>
          <Button type="button" variant="outline" disabled={!file || busy} onClick={handlePreview}>{t("cr_preview")}</Button>

          {fileError && <p role="alert" className="text-sm text-destructive">{fileError}</p>}

          {preview && (
            <div className="space-y-2" aria-label={t("cr_preview")}>
              <p className="text-sm"><strong>{preview.matched}</strong> {t("cr_preview_matched")}, <strong>{preview.errors.length}</strong> {t("cr_preview_bad")}.</p>
              {preview.result_published && (
                <p role="alert" className="rounded-md bg-gold-50 px-3 py-2 text-sm text-gold-700">{t("cr_published_warn")}</p>
              )}
              {preview.errors.length > 0 && (
                <table className="w-full text-left text-xs" aria-label={t("cr_errors_label")}>
                  <thead><tr><th>{t("cr_col_line")}</th><th>{t("cr_col_participant")}</th><th>{t("cr_col_issue")}</th></tr></thead>
                  <tbody>
                    {preview.errors.map((e, i) => (
                      <tr key={`${e.line}-${i}`}><td>{e.line}</td><td>{e.participant_no}</td><td>{reasonCopy(e.reason)}</td></tr>
                    ))}
                  </tbody>
                </table>
              )}
              <Button type="button" disabled={!canConfirm} onClick={handleApply}>{t("cr_apply")}</Button>
            </div>
          )}
        </section>
      </DialogContent>
    </Dialog>
  );
}
