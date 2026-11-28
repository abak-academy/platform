"use client";

import { useState } from "react";
import { toast } from "sonner";
import { Button } from "@/components/ui/button";
import { Label } from "@/components/ui/label";
import { ApiError } from "@/lib/api";
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

const FILE_ERRORS = [
  "invalid_csv",
  "unknown_column",
  "duplicate_column",
  "missing_column",
  "no_rows",
  "file_too_large",
];

function errorCopy(e: unknown, t: (k: I18nKey) => string): string {
  if (e instanceof ApiError) {
    if (e.code === "upload_not_found") return t("cr_err_upload_not_found");
    if (e.code === "competition_result_not_found")
      return t("cr_err_result_not_found");
    if (e.code === "validation_failed") {
      const token = FILE_ERRORS.find((k) => e.message.includes(k));
      if (token) return t(`cr_err_${token}` as I18nKey);
    }
  }
  return t("cr_err_generic");
}

function ResultRows({ examId }: { examId: string }) {
  const { t } = useTranslation();
  const { data, isLoading, isError } = useCompetitionResultRows(examId, true);
  if (isLoading)
    return <p className="text-sm text-muted-foreground">{t("cr_loading")}</p>;
  if (isError)
    return (
      <p role="alert" className="text-sm text-destructive">
        {t("cr_err_rows")}
      </p>
    );
  return (
    <table
      className="w-full text-left text-xs"
      aria-label={t("cr_results_label")}
    >
      <thead>
        <tr>
          <th>{t("cr_col_participant")}</th>
          <th>{t("cr_col_name")}</th>
          <th>{t("cr_col_score")}</th>
          <th>{t("cr_col_rank")}</th>
          <th>{t("cr_col_award")}</th>
        </tr>
      </thead>
      <tbody>
        {(data?.data ?? []).map((r) => (
          <tr key={r.registration_id}>
            <td>{r.participant_no}</td>
            <td>{r.student_name}</td>
            <td>{r.score}</td>
            <td>{r.rank ?? "-"}</td>
            <td>{r.award ?? "-"}</td>
          </tr>
        ))}
      </tbody>
    </table>
  );
}

export function CompetitionResultPanel({
  examId,
  canWrite,
}: {
  examId: string;
  canWrite: boolean;
}) {
  const { t, lang } = useTranslation();
  const dateTime = (iso: string) =>
    new Date(iso).toLocaleString(lang === "id" ? "id-ID" : "en-US", {
      dateStyle: "medium",
      timeStyle: "short",
    });
  const [file, setFile] = useState<File | null>(null);
  const [key, setKey] = useState("");
  const [preview, setPreview] = useState<CompetitionResultPreview | null>(null);
  const [fileError, setFileError] = useState("");
  const [confirmDelete, setConfirmDelete] = useState(false);
  const [viewing, setViewing] = useState(false);

  const query = useCompetitionResult(examId);
  const result = query.data ?? null;
  const presign = usePresignCompetitionResult(examId);
  const previewMut = usePreviewCompetitionResult(examId);
  const apply = useApplyCompetitionResult(examId);
  const setPublished = useSetCompetitionResultPublished(examId);
  const del = useDeleteCompetitionResult(examId);

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
      setFileError(errorCopy(e, t));
    }
  }

  async function handleApply() {
    try {
      await apply.mutateAsync(key);
      toast.success(t("cr_saved_toast"));
      setFile(null);
      resetPreview();
    } catch (e) {
      if (
        e instanceof ApiError &&
        e.status === 422 &&
        preview &&
        Array.isArray(e.body?.errors)
      ) {
        setPreview({
          ...preview,
          matched: Number(e.body.matched ?? 0),
          errors: e.body.errors as CompetitionResultPreview["errors"],
        });
      } else {
        toast.error(errorCopy(e, t));
      }
    }
  }

  async function run(fn: () => Promise<unknown>, ok: string) {
    try {
      await fn();
      toast.success(ok);
    } catch (e) {
      toast.error(errorCopy(e, t));
    }
  }

  const reasonCopy = (reason: string) => {
    const k = `cr_reason_${reason}`;
    return k in DICT.id ? t(k as I18nKey) : reason;
  };

  const canConfirm =
    preview !== null &&
    preview.errors.length === 0 &&
    preview.matched > 0 &&
    !busy;

  // The GET is permission-gated; a role without access sees no panel rather than an error.
  if (query.error instanceof ApiError && query.error.status === 403)
    return null;

  return (
    <section
      className="mt-6 space-y-4 rounded-md border p-4"
      aria-label={t("cr_title")}
    >
      <header className="space-y-1">
        <h2 className="text-base font-semibold">{t("cr_title")}</h2>
        <p className="text-sm text-muted-foreground">{t("cr_desc")}</p>
      </header>

      <section className="space-y-2" aria-label={t("cr_saved_title")}>
        <h3 className="text-sm font-semibold">{t("cr_saved_title")}</h3>
        {!result && (
          <p className="text-sm text-muted-foreground">{t("cr_none")}</p>
        )}
        {result && (
          <div className="space-y-2 rounded-md border px-3 py-2">
            <div className="flex flex-wrap items-center justify-between gap-2">
              <div>
                <span className="text-xs text-muted-foreground">
                  {result.row_count} {t("cr_rows")} · {t("cr_updated")}{" "}
                  {dateTime(result.updated_at)}
                </span>{" "}
                <span
                  className={`rounded-full px-2 py-0.5 text-xs font-semibold ${result.published_at ? "bg-teal-50 text-teal-700" : "bg-muted text-muted-foreground"}`}
                >
                  {result.published_at ? t("cr_live") : t("cr_draft")}
                </span>
              </div>
              <div className="flex gap-1">
                <Button
                  type="button"
                  size="sm"
                  variant="ghost"
                  onClick={() => setViewing(!viewing)}
                >
                  {viewing ? t("cr_close") : t("cr_view")}
                </Button>
                {canWrite && (
                  <>
                    <Button
                      type="button"
                      size="sm"
                      variant="outline"
                      disabled={setPublished.isPending}
                      onClick={() =>
                        run(
                          () => setPublished.mutateAsync(!result.published_at),
                          result.published_at
                            ? t("cr_unpublished_toast")
                            : t("cr_published_toast"),
                        )
                      }
                    >
                      {result.published_at
                        ? t("cr_unpublish")
                        : t("cr_publish")}
                    </Button>
                    {confirmDelete ? (
                      <>
                        <Button
                          type="button"
                          size="sm"
                          variant="destructive"
                          onClick={() => {
                            setConfirmDelete(false);
                            run(() => del.mutateAsync(), t("cr_deleted_toast"));
                          }}
                        >
                          {t("cr_delete_yes")}
                        </Button>
                        <Button
                          type="button"
                          size="sm"
                          variant="ghost"
                          onClick={() => setConfirmDelete(false)}
                        >
                          {t("cr_cancel")}
                        </Button>
                      </>
                    ) : (
                      <Button
                        type="button"
                        size="sm"
                        variant="ghost"
                        onClick={() => setConfirmDelete(true)}
                      >
                        {t("cr_delete")}
                      </Button>
                    )}
                  </>
                )}
              </div>
            </div>
            {confirmDelete && (
              <p role="alert" className="text-sm text-destructive">
                {t("cr_delete_confirm")}
              </p>
            )}
            {viewing && <ResultRows examId={examId} />}
          </div>
        )}
      </section>

      {canWrite && (
        <section className="space-y-3" aria-label={t("cr_upload_title")}>
          <h3 className="text-sm font-semibold">{t("cr_upload_title")}</h3>
          <div className="space-y-1">
            <Label htmlFor="cr-file">{t("cr_file")}</Label>
            <input
              id="cr-file"
              type="file"
              accept=".csv,text/csv"
              onChange={(e) => {
                setFile(e.target.files?.[0] ?? null);
                resetPreview();
              }}
            />
          </div>
          <Button
            type="button"
            variant="outline"
            disabled={!file || busy}
            onClick={handlePreview}
          >
            {t("cr_preview")}
          </Button>

          {fileError && (
            <p role="alert" className="text-sm text-destructive">
              {fileError}
            </p>
          )}

          {preview && (
            <div className="space-y-2" aria-label={t("cr_preview")}>
              <p className="text-sm">
                <strong>{preview.matched}</strong> {t("cr_preview_matched")},{" "}
                <strong>{preview.errors.length}</strong> {t("cr_preview_bad")}.
              </p>
              {preview.result_published && (
                <p
                  role="alert"
                  className="rounded-md bg-gold-50 px-3 py-2 text-sm text-gold-700"
                >
                  {t("cr_published_warn")}
                </p>
              )}
              {preview.errors.length > 0 && (
                <table
                  className="w-full text-left text-xs"
                  aria-label={t("cr_errors_label")}
                >
                  <thead>
                    <tr>
                      <th>{t("cr_col_line")}</th>
                      <th>{t("cr_col_participant")}</th>
                      <th>{t("cr_col_issue")}</th>
                    </tr>
                  </thead>
                  <tbody>
                    {preview.errors.map((e, i) => (
                      <tr key={`${e.line}-${i}`}>
                        <td>{e.line}</td>
                        <td>{e.participant_no}</td>
                        <td>{reasonCopy(e.reason)}</td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              )}
              <Button
                type="button"
                disabled={!canConfirm}
                onClick={handleApply}
              >
                {t("cr_apply")}
              </Button>
            </div>
          )}
        </section>
      )}
    </section>
  );
}
