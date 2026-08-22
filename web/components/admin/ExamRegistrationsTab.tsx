"use client";

import { useEffect, useMemo, useRef, useState, type ReactNode } from "react";
import { ArrowUpDown, CheckCircle, Download, Eye, EyeOff, Loader2 } from "lucide-react";
import { toast } from "sonner";
import { useQueryClient } from "@tanstack/react-query";
import { useTranslation } from "@/lib/i18n";
import { ParticipantPicker } from "@/components/admin/ParticipantPicker";
import { SnapCheckout } from "@/components/cart/SnapCheckout";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { DataTable, type DataTableColumn } from "@/components/ui/data-table";
import { formatRupiah } from "@/lib/format";
import {
  usePreviewBulkExamOrder,
  useCreateBulkExamOrder,
  type BulkExamOrderPreview,
} from "@/lib/hooks/admin-bulk-exam-orders";
import {
  useGrantExamAccess,
  usePresignExamGrantBulkUpload,
  useEnqueueExamGrantBulk,
} from "@/lib/hooks/admin-exam-grants";
import { putFileToPresignedURL } from "@/lib/hooks/admin-students-bulk";
import { useJobStatus } from "@/lib/hooks/jobs";
import { adminExamsKeys, exportExamRoster, useExamRoster } from "@/lib/hooks/admin-exams";
import { useAuthStore } from "@/stores/auth";
import type { ExamRosterEntry } from "@/lib/types";

const EXAM_GRANT_BULK_TEMPLATE = "username\nandi123\nbudi456\n";

function downloadExamGrantBulkTemplate(): void {
  const blob = new Blob([EXAM_GRANT_BULK_TEMPLATE], { type: "text/csv;charset=utf-8" });
  const url = URL.createObjectURL(blob);
  const a = document.createElement("a");
  a.href = url;
  a.download = "exam_grant_bulk_template.csv";
  document.body.appendChild(a);
  a.click();
  document.body.removeChild(a);
  URL.revokeObjectURL(url);
}

interface ExamRegistrationsTabProps {
  examId: string;
  examName: string;
}

function ExamRosterSection({ examId, action }: { examId: string; action?: ReactNode }) {
  const { t } = useTranslation();
  const [sortDir, setSortDir] = useState<"asc" | "desc">("asc");
  const [cursor, setCursor] = useState<string | undefined>();
  const [nextCursor, setNextCursor] = useState<string | undefined>();
  const [rows, setRows] = useState<ExamRosterEntry[]>([]);
  const [exporting, setExporting] = useState(false);
  const [revealedTokens, setRevealedTokens] = useState<Set<string>>(new Set());
  const query = useExamRoster(examId, { cursor, limit: 20, sort: sortDir });

  useEffect(() => {
    if (!query.data) return;
    const page = query.data.data ?? [];
    setRows((previous) => {
      if (!cursor) return page;
      const ids = new Set(previous.map((row) => row.registration_id));
      return [...previous, ...page.filter((row) => !ids.has(row.registration_id))];
    });
    setNextCursor(query.data.next_cursor);
  }, [cursor, query.data]);

  const toggleSort = () => {
    setRows([]);
    setCursor(undefined);
    setNextCursor(undefined);
    setSortDir((direction) => direction === "asc" ? "desc" : "asc");
  };

  const handleExport = async () => {
    setExporting(true);
    try {
      await exportExamRoster(examId);
    } catch {
      toast.error(t("exam_roster_export_failed"));
    } finally {
      setExporting(false);
    }
  };

  const toggleToken = (registrationId: string) => {
    setRevealedTokens((prev) => {
      const next = new Set(prev);
      if (next.has(registrationId)) {
        next.delete(registrationId);
      } else {
        next.add(registrationId);
      }
      return next;
    });
  };

  const columns: DataTableColumn<ExamRosterEntry>[] = [
    {
      key: "participant_no",
      header: (
        <button
          type="button"
          className="flex items-center gap-1"
          onClick={toggleSort}
        >
          {t("exam_roster_th_participant_no")}
          <ArrowUpDown className="size-3.5" />
        </button>
      ),
      cell: (r) => (
        <span data-testid="roster-participant-no" className="font-medium text-ink-900">
          {r.participant_no || "—"}
        </span>
      ),
    },
    {
      key: "name",
      header: t("exam_roster_th_name"),
      cell: (r) => <span className="text-ink-900">{r.student_name}</span>,
    },
    {
      key: "username",
      header: t("exam_roster_th_username"),
      cell: (r) => (
        <span className="text-ink-500">
          {r.student_username ? `@${r.student_username}` : "—"}
        </span>
      ),
    },
    {
      key: "status",
      header: t("exam_roster_th_status"),
      cell: (r) => r.status,
    },
    {
      key: "checked_in",
      header: t("exam_roster_th_checked_in"),
      cell: (r) => (r.checked_in_at ? "✓" : "—"),
    },
    {
      key: "token",
      header: t("exam_roster_th_token"),
      cell: (r) => {
        const revealed = revealedTokens.has(r.registration_id);
        return (
          <div className="flex items-center gap-2 font-mono text-xs">
            <span className="select-all">{revealed ? r.token : "••••••••"}</span>
            <Button
              type="button"
              size="xs"
              variant="ghost"
              onClick={() => toggleToken(r.registration_id)}
              aria-label={revealed ? t("exam_roster_hide_token") : t("exam_roster_show_token")}
            >
              {revealed ? <EyeOff className="size-3.5" /> : <Eye className="size-3.5" />}
            </Button>
          </div>
        );
      },
    },
  ];

  return (
    <div className="space-y-3">
      <div className="flex items-center justify-between">
        <h3 className="font-serif text-base font-semibold text-ink-900">
          {t("exam_roster_title")}
        </h3>
        <div className="flex items-center gap-2">
          {action}
          <Button
            variant="outline"
            size="sm"
            className="rounded-full"
            disabled={rows.length === 0 || exporting}
            onClick={handleExport}
          >
            {t("exam_roster_export_csv")}
          </Button>
        </div>
      </div>

      {query.isError && <p className="text-sm text-danger">{t("exam_roster_load_failed")}</p>}

      {!query.isError && query.isLoading && rows.length === 0 && <p className="text-sm text-ink-500">…</p>}

      {!query.isError && (!query.isLoading || rows.length > 0) && (
        <DataTable
          columns={columns}
          rows={rows}
          rowKey={(r) => r.registration_id}
          empty={t("exam_roster_empty")}
          footer={
            nextCursor && (
              <div className="border-t border-line px-4 py-3 text-center">
                <Button
                  variant="outline"
                  size="sm"
                  onClick={() => setCursor(nextCursor)}
                  disabled={query.isFetching}
                >
                  {query.isFetching ? t("sys_loading") : t("sys_load_more")}
                </Button>
              </div>
            )
          }
        />
      )}
    </div>
  );
}

// admin_school orders (Midtrans-paid); super_admin grants directly (no order,
// no payment). Same participant pool, submit action branches by role.
export function ExamRegistrationsTab({ examId, examName }: ExamRegistrationsTabProps) {
  const { t } = useTranslation();
  const queryClient = useQueryClient();
  const role = useAuthStore((s) => s.user?.role);
  const schoolId = useAuthStore((s) => s.user?.school_id);
  const isSuperAdmin = role === "super_admin";
  const isAdminExam = role === "admin_exam";

  const [modalOpen, setModalOpen] = useState(false);
  const [selectedStudentIds, setSelectedStudentIds] = useState<string[]>([]);
  const [createdOrderId, setCreatedOrderId] = useState<string | null>(null);
  const [previewResult, setPreviewResult] = useState<BulkExamOrderPreview | null>(null);
  const [grantResult, setGrantResult] = useState<{
    granted_count: number;
    granted_students: Array<{ id: string; name: string; username: string }>;
  } | null>(null);
  const [grantMode, setGrantMode] = useState<"manual" | "csv">("manual");
  const [csvFile, setCsvFile] = useState<File | null>(null);
  const [csvJobId, setCsvJobId] = useState<string | null>(null);
  const flowGenerationRef = useRef(0);

  const previewMutation = usePreviewBulkExamOrder();
  const createMutation = useCreateBulkExamOrder();
  const grantMutation = useGrantExamAccess();
  const csvPresignMutation = usePresignExamGrantBulkUpload(examId);
  const csvEnqueueMutation = useEnqueueExamGrantBulk();
  const csvJob = useJobStatus(csvJobId);

  const previewInput = useMemo(() => {
    if (selectedStudentIds.length === 0) return null;
    return { exam_id: examId, student_ids: selectedStudentIds };
  }, [examId, selectedStudentIds]);

  const handlePreview = () => {
    if (selectedStudentIds.length === 0) {
      toast.error(t("bulk_exam_order_empty_students"));
      return;
    }
    const flowGeneration = flowGenerationRef.current;
    previewMutation.mutate(
      { exam_id: examId, student_ids: selectedStudentIds },
      {
        onSuccess: (result) => {
          if (flowGeneration !== flowGenerationRef.current) return;
          setPreviewResult(result);
        },
        onError: () => {
          if (flowGeneration !== flowGenerationRef.current) return;
          toast.error(t("bulk_exam_order_preview_failed"));
        },
      },
    );
  };

  const handleCreateOrder = () => {
    if (!previewInput) return;
    const flowGeneration = flowGenerationRef.current;
    createMutation.mutate(previewInput, {
      onSuccess: (order) => {
        if (flowGeneration !== flowGenerationRef.current) return;
        setCreatedOrderId(order.id);
        toast.success(t("bulk_exam_order_created"));
        queryClient.invalidateQueries({ queryKey: [...adminExamsKeys.rosters(), examId] });
      },
      onError: (err) => {
        if (flowGeneration !== flowGenerationRef.current) return;
        const msg =
          err instanceof Error ? err.message : t("bulk_exam_order_creating_failed");
        toast.error(msg);
      },
    });
  };

  const handleGrant = () => {
    if (selectedStudentIds.length === 0) {
      toast.error(t("exam_grant_empty_students"));
      return;
    }
    const flowGeneration = flowGenerationRef.current;
    grantMutation.mutate(
      { exam_id: examId, student_ids: selectedStudentIds },
      {
        onSuccess: (result) => {
          if (flowGeneration !== flowGenerationRef.current) return;
          setGrantResult(result);
          toast.success(t("exam_grant_success"));
          queryClient.invalidateQueries({ queryKey: [...adminExamsKeys.rosters(), examId] });
        },
        onError: (err) => {
          if (flowGeneration !== flowGenerationRef.current) return;
          const msg = err instanceof Error ? err.message : t("error_generic");
          toast.error(msg);
        },
      },
    );
  };

  const handleCsvSubmit = async () => {
    if (!csvFile) {
      toast.error(t("exam_grant_bulk_no_file"));
      return;
    }
    const flowGeneration = flowGenerationRef.current;
    try {
      const presignResp = await csvPresignMutation.mutateAsync({
        filename: csvFile.name,
        contentType: csvFile.type || "text/csv",
      });
      if (flowGeneration !== flowGenerationRef.current) return;
      try {
        await putFileToPresignedURL(presignResp.url, csvFile, csvFile.type || "text/csv");
      } catch (err) {
        if (flowGeneration !== flowGenerationRef.current) return;
        toast.error(err instanceof Error ? err.message : t("exam_grant_bulk_put_failed"));
        return;
      }
      if (flowGeneration !== flowGenerationRef.current) return;
      const enqueueResp = await csvEnqueueMutation.mutateAsync({
        examId,
        fileKey: presignResp.key,
      });
      if (flowGeneration !== flowGenerationRef.current) return;
      setCsvJobId(enqueueResp.job_id);
    } catch (err) {
      if (flowGeneration !== flowGenerationRef.current) return;
      toast.error(err instanceof Error ? err.message : t("exam_grant_bulk_enqueue_failed"));
    }
  };

  const csvJobData = csvJob.data;
  const csvIsTerminalSuccess = csvJobData?.status === "succeeded";
  const csvIsTerminalFailed = csvJobData?.status === "failed";

  useEffect(() => {
    if (csvIsTerminalSuccess) {
      queryClient.invalidateQueries({ queryKey: [...adminExamsKeys.rosters(), examId] });
    }
    // Only re-run when the job transitions to success, not on every roster/queryClient identity change.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [csvIsTerminalSuccess, examId]);

  const resetFlowState = () => {
    setSelectedStudentIds([]);
    setCreatedOrderId(null);
    setPreviewResult(null);
    setGrantResult(null);
    setGrantMode("manual");
    setCsvFile(null);
    setCsvJobId(null);
    previewMutation.reset();
    createMutation.reset();
    grantMutation.reset();
  };

  const handleReset = () => {
    resetFlowState();
  };

  const handleModalOpenChange = (open: boolean) => {
    setModalOpen(open);
    flowGenerationRef.current += 1;
    if (!open) {
      resetFlowState();
    }
  };

  const modalTitle = isSuperAdmin
    ? t("exam_registrations_grant_modal_title")
    : t("bulk_exam_order_pick_participants");

  // ── Main roster + action modal ──────────────────────────────────────────

  return (
    <div className="space-y-6">
      <ExamRosterSection
        examId={examId}
        action={
          !isAdminExam ? (
            <Button
              variant="outline"
              size="sm"
              className="rounded-full"
              data-testid={isSuperAdmin ? "open-grant-modal" : "open-order-modal"}
              onClick={() => setModalOpen(true)}
            >
              {isSuperAdmin ? t("exam_grant_grant") : t("bulk_exam_order_pick_participants")}
            </Button>
          ) : undefined
        }
      />

      {isAdminExam && (
        <div className="md-card-outlined p-5 text-sm text-ink-600">
          {t("exam_registrations_manual_notice")}
        </div>
      )}

      {!isAdminExam && (
        <Dialog open={modalOpen} onOpenChange={handleModalOpenChange}>
          <DialogContent className="sm:max-w-2xl">
            <DialogHeader>
              <DialogTitle>{modalTitle}</DialogTitle>
            </DialogHeader>

            {createdOrderId ? (
              <div className="text-center">
                <CheckCircle className="mx-auto mb-4 size-12 text-success" />
                <h2 className="font-serif text-xl font-bold text-ink-900">
                  {t("bulk_exam_order_created")}
                </h2>
                <p className="mt-2 text-sm text-ink-500">{t("bulk_exam_order_created_desc")}</p>
                <p className="mt-1 text-sm text-ink-500">
                  {examName} &middot;{" "}
                  {t("bulk_exam_order_students_count").replace(
                    "{n}",
                    String(selectedStudentIds.length),
                  )}
                </p>

                <div className="mt-8 flex flex-col items-center gap-4 sm:flex-row sm:justify-center">
                  <SnapCheckout orderId={createdOrderId} basePath="/admin/bulk-exam-orders" />
                  <Button variant="outline" className="rounded-full" onClick={handleReset}>
                    {t("bulk_exam_order_reset")}
                  </Button>
                </div>
              </div>
            ) : grantResult ? (
              <div className="text-center">
                <CheckCircle className="mx-auto mb-4 size-12 text-success" />
                <h2 className="font-serif text-xl font-bold text-ink-900">
                  {t("exam_grant_success_title")}
                </h2>
                <p className="mt-2 text-sm text-ink-500">
                  {t("exam_grant_success_desc_count").replace(
                    "{n}",
                    String(grantResult.granted_count),
                  )}{" "}
                  &middot; {examName}
                </p>

                {grantResult.granted_students.length > 0 && (
                  <div className="mx-auto mt-6 max-h-[200px] max-w-sm overflow-y-auto rounded-lg border border-line p-2 text-left">
                    {grantResult.granted_students.map((s) => (
                      <div key={s.id} className="flex items-center gap-2 px-2 py-1.5 text-sm">
                        <span className="font-medium text-ink-900">{s.name}</span>
                        <span className="text-ink-500">@{s.username}</span>
                      </div>
                    ))}
                  </div>
                )}

                <div className="mt-8 flex justify-center">
                  <Button variant="outline" className="rounded-full" onClick={handleReset}>
                    {t("exam_grant_grant_again")}
                  </Button>
                </div>
              </div>
            ) : (
              <>
                {isSuperAdmin && (
                  <div className="flex gap-2" data-testid="grant-mode-toggle">
                    <Button
                      type="button"
                      variant={grantMode === "manual" ? "default" : "outline"}
                      size="sm"
                      className="rounded-full"
                      data-testid="grant-mode-manual"
                      onClick={() => setGrantMode("manual")}
                    >
                      {t("exam_grant_mode_manual")}
                    </Button>
                    <Button
                      type="button"
                      variant={grantMode === "csv" ? "default" : "outline"}
                      size="sm"
                      className="rounded-full"
                      data-testid="grant-mode-csv"
                      onClick={() => setGrantMode("csv")}
                    >
                      {t("exam_grant_mode_csv")}
                    </Button>
                  </div>
                )}

                {isSuperAdmin && grantMode === "csv" ? (
                  <div className="space-y-6">
                    <section>
                      <h3 className="text-sm font-semibold text-ink-900">
                        1. {t("exam_grant_bulk_download_template")}
                      </h3>
                      <div className="mt-2">
                        <Button
                          type="button"
                          variant="outline"
                          size="sm"
                          className="rounded-full"
                          data-testid="csv-download-template"
                          onClick={downloadExamGrantBulkTemplate}
                        >
                          <Download className="mr-2 size-4" />
                          {t("exam_grant_bulk_download_template")}
                        </Button>
                      </div>
                    </section>

                    <section>
                      <h3 className="text-sm font-semibold text-ink-900">
                        2. {t("exam_grant_bulk_upload")}
                      </h3>
                      <div className="mt-2 space-y-3">
                        <div className="grid gap-2">
                          <Label htmlFor="exam-grant-bulk-file">
                            {t("exam_grant_bulk_choose_file")}
                          </Label>
                          <Input
                            id="exam-grant-bulk-file"
                            data-testid="csv-file-input"
                            type="file"
                            accept=".csv,text/csv"
                            onChange={(e) => setCsvFile(e.target.files?.[0] ?? null)}
                            disabled={csvPresignMutation.isPending || csvEnqueueMutation.isPending}
                          />
                          {csvFile && (
                            <p className="text-sm text-muted-foreground">{csvFile.name}</p>
                          )}
                        </div>

                        <Button
                          type="button"
                          className="rounded-full"
                          data-testid="csv-upload-submit"
                          onClick={handleCsvSubmit}
                          disabled={
                            !csvFile || csvPresignMutation.isPending || csvEnqueueMutation.isPending
                          }
                        >
                          {csvPresignMutation.isPending || csvEnqueueMutation.isPending ? (
                            <Loader2 className="mr-2 size-4 animate-spin" />
                          ) : null}
                          {t("exam_grant_bulk_upload")}
                        </Button>
                      </div>
                    </section>

                    {csvJobData && (
                      <section className="md-card-outlined space-y-3 p-4">
                        {csvIsTerminalSuccess && (
                          <div className="flex items-center gap-2">
                            <CheckCircle className="size-5 text-success" />
                            <h4 className="text-sm font-semibold text-ink-900">
                              {t("exam_grant_bulk_success")}
                            </h4>
                          </div>
                        )}

                        {!csvIsTerminalSuccess && !csvIsTerminalFailed && (
                          <div className="space-y-2">
                            <p className="text-sm font-medium text-ink-900">
                              {t("exam_grant_bulk_progress").replace(
                                "{pct}",
                                String(Math.round(csvJobData.progress ?? 0)),
                              )}
                            </p>
                            <div className="h-2 w-full overflow-hidden rounded-full bg-surface-2">
                              <div
                                className="h-full bg-primary transition-all"
                                style={{
                                  width: `${Math.max(0, Math.min(100, csvJobData.progress ?? 0))}%`,
                                }}
                              />
                            </div>
                          </div>
                        )}

                        {csvIsTerminalFailed && (
                          <div className="space-y-2">
                            <h4 className="text-sm font-semibold text-danger">
                              {t("exam_grant_bulk_failed")}
                            </h4>
                            {csvJobData.error && (
                              <p className="text-sm text-danger">{csvJobData.error}</p>
                            )}
                          </div>
                        )}

                        {(csvIsTerminalSuccess || csvIsTerminalFailed) && csvJobData.result_url && (
                          <a
                            href={csvJobData.result_url}
                            className="inline-flex items-center gap-2 text-sm font-medium text-primary underline-offset-4 hover:underline"
                            download="exam_grant_bulk_result.csv"
                          >
                            <Download className="size-4" />
                            {t("exam_grant_bulk_download_result")}
                          </a>
                        )}
                      </section>
                    )}
                  </div>
                ) : (
                  <div>
                    <ParticipantPicker
                      examId={examId}
                      schoolId={isSuperAdmin ? undefined : schoolId}
                      selected={selectedStudentIds}
                      onChange={setSelectedStudentIds}
                    />
                  </div>
                )}

                {selectedStudentIds.length > 0 && isSuperAdmin && grantMode === "manual" && (
                  <div className="space-y-4">
                    <Button size="lg" className="rounded-full" onClick={handleGrant} disabled={grantMutation.isPending}>
                      {grantMutation.isPending ? (
                        <Loader2 className="mr-2 size-4 animate-spin" />
                      ) : null}
                      {grantMutation.isPending ? t("exam_grant_granting") : t("exam_grant_grant")}
                    </Button>

                    {grantMutation.isError && (
                      <p className="text-sm text-danger">{t("error_generic")}</p>
                    )}
                  </div>
                )}

                {selectedStudentIds.length > 0 && !isSuperAdmin && (
                  <div className="space-y-4">
                    <Button size="lg" className="rounded-full" onClick={handlePreview} disabled={previewMutation.isPending}>
                      {previewMutation.isPending ? (
                        <Loader2 className="mr-2 size-4 animate-spin" />
                      ) : null}
                      {t("bulk_exam_order_preview")}
                    </Button>

                    {previewResult && (
                      <div className="md-card-outlined space-y-4 p-5">
                        <h4 className="font-serif text-base font-semibold text-ink-900">
                          {t("bulk_exam_order_preview_title")}
                        </h4>
                        <div className="flex items-center gap-2">
                          <Badge variant="outline">
                            {previewResult.net_new_count}{" "}
                            {t("bulk_exam_order_students_count").replace(
                              "{n}",
                              String(previewResult.net_new_count),
                            )}
                          </Badge>
                        </div>

                        {previewResult.excluded.length > 0 && (
                          <div className="max-h-[160px] overflow-y-auto rounded-lg border border-line p-2">
                            {previewResult.excluded.map((s) => (
                              <div
                                key={s.student_id}
                                className="flex items-center gap-2 px-2 py-1.5 text-sm"
                              >
                                <span className="font-medium text-ink-900">{s.name}</span>
                                <span className="text-ink-500">({s.reason})</span>
                              </div>
                            ))}
                          </div>
                        )}

                        <div className="border-t border-line pt-3">
                          <div className="flex items-center justify-between text-sm">
                            <span className="font-semibold text-ink-900">
                              {t("bulk_exam_order_total")}
                            </span>
                            <span className="font-serif text-lg font-bold text-success">
                              {formatRupiah(previewResult.total)}
                            </span>
                          </div>
                        </div>

                        <Button
                          size="lg"
                          className="w-full rounded-full"
                          onClick={handleCreateOrder}
                          disabled={createMutation.isPending}
                        >
                          {createMutation.isPending ? (
                            <Loader2 className="mr-2 size-4 animate-spin" />
                          ) : null}
                          {createMutation.isPending
                            ? t("bulk_exam_order_confirming")
                            : t("bulk_exam_order_confirm")}
                        </Button>
                      </div>
                    )}

                    {previewMutation.isError && (
                      <p className="text-sm text-danger">{t("bulk_exam_order_preview_failed")}</p>
                    )}
                  </div>
                )}
              </>
            )}
          </DialogContent>
        </Dialog>
      )}
    </div>
  );
}
