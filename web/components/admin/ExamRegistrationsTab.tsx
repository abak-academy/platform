"use client";

import { useEffect, useMemo, useRef, useState, type ReactNode } from "react";
import { ArrowUpDown, CheckCircle, Download, Eye, EyeOff, Loader2, Search } from "lucide-react";
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
  useRevokeExamAccess,
  usePresignExamRevokeBulkUpload,
  useEnqueueExamRevokeBulk,
  type RevokeExamAccessResponse,
} from "@/lib/hooks/admin-exam-grants";
import { putFileToPresignedURL } from "@/lib/hooks/admin-students-bulk";
import { useJobStatus } from "@/lib/hooks/jobs";
import { adminExamsKeys, exportExamRoster, useExamRegistrationCounts, useExamRoster } from "@/lib/hooks/admin-exams";
import { useDebouncedValue } from "@/lib/hooks/use-debounced-value";
import { hasCapability } from "@/lib/hooks/use-capability";
import { useAuthStore } from "@/stores/auth";
import type { ExamRosterEntry } from "@/lib/types";

const EXAM_GRANT_BULK_TEMPLATE = "username\nandi123\nbudi456\n";

function downloadExamBulkTemplate(filename: string): void {
  const blob = new Blob([EXAM_GRANT_BULK_TEMPLATE], { type: "text/csv;charset=utf-8" });
  const url = URL.createObjectURL(blob);
  const a = document.createElement("a");
  a.href = url;
  a.download = filename;
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
  const [accumulated, setAccumulated] = useState<ExamRosterEntry[]>([]);
  const [activeCursor, setActiveCursor] = useState<string | undefined>(undefined);
  const [nextCursor, setNextCursor] = useState<string | undefined>(undefined);
  const [exporting, setExporting] = useState(false);
  const [revealedTokens, setRevealedTokens] = useState<Set<string>>(new Set());
  const [search, setSearch] = useState("");
  const debouncedSearch = useDebouncedValue(search);
  const counts = useExamRegistrationCounts(examId);

  const filterKey = `${examId}:${sortDir}:${debouncedSearch}`;
  const filterKeyRef = useRef(filterKey);

  // A search, sort or exam change restarts the keyset walk — accumulated rows
  // and the cursor no longer belong to the new filter. The ref (not the effect
  // order) is what keeps cached first-page data from being wiped when the
  // filter is cleared.
  useEffect(() => {
    if (filterKey !== filterKeyRef.current) {
      setAccumulated([]);
      setActiveCursor(undefined);
      setNextCursor(undefined);
      filterKeyRef.current = filterKey;
    }
  }, [filterKey]);

  const query = useExamRoster(examId, {
    cursor: activeCursor,
    limit: 20,
    sort: sortDir,
    q: debouncedSearch || undefined,
  });

  useEffect(() => {
    if (!query.data) return;
    if (filterKey !== filterKeyRef.current) return;

    setAccumulated((prev) => {
      const rows = query.data!.data ?? [];
      if (activeCursor === undefined) return rows;
      const ids = new Set(prev.map((r) => r.registration_id));
      const fresh = rows.filter((r) => !ids.has(r.registration_id));
      return [...prev, ...fresh];
    });
    setNextCursor(query.data.next_cursor);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [query.data, filterKey]);

  const toggleSort = () => {
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
    <div className="space-y-4">
      {counts.data && (
        <div className="grid grid-cols-2 gap-4 sm:grid-cols-3 xl:grid-cols-5" data-testid="roster-counts">
          <RosterSummaryCard testId="roster-count-total" label={t("exam_roster_stat_total")} value={counts.data.total} />
          <RosterSummaryCard testId="roster-count-registered" label={t("exam_roster_stat_registered")} value={counts.data.registered} />
          <RosterSummaryCard testId="roster-count-checked-in" label={t("exam_roster_stat_checkedin")} value={counts.data.checked_in} />
          <RosterSummaryCard testId="roster-count-submitted" label={t("exam_roster_stat_submitted")} value={counts.data.submitted} />
          <RosterSummaryCard testId="roster-count-revoked" label={t("exam_roster_stat_revoked")} value={counts.data.revoked} />
        </div>
      )}

      <div className="flex flex-wrap items-center justify-between gap-3">
        <div className="relative w-full max-w-xs">
          <Search className="pointer-events-none absolute left-3 top-1/2 size-4 -translate-y-1/2 text-ink-400" />
          <Input
            value={search}
            onChange={(e) => setSearch(e.target.value)}
            placeholder={t("exam_roster_search_placeholder")}
            className="h-9 pl-9 text-xs"
            data-testid="roster-search"
          />
        </div>
        <div className="flex flex-wrap items-center gap-2">
          {action}
          <Button
            variant="outline"
            size="sm"
            className="rounded-full"
            disabled={accumulated.length === 0 || exporting}
            onClick={handleExport}
          >
            {t("exam_roster_export_csv")}
          </Button>
        </div>
      </div>

      {query.isError && <p className="text-sm text-danger">{t("exam_roster_load_failed")}</p>}

      {!query.isError && query.isLoading && accumulated.length === 0 && <p className="text-sm text-ink-500">…</p>}

      {!query.isError && (!query.isLoading || accumulated.length > 0) && (
        <DataTable
          columns={columns}
          rows={accumulated}
          rowKey={(r) => r.registration_id}
          empty={t("exam_roster_empty")}
          footer={
            nextCursor && (
              <div className="border-t border-line px-4 py-3 text-center">
                <Button
                  variant="outline"
                  size="sm"
                  onClick={() => setActiveCursor(nextCursor)}
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

// Summary card in the exam results-workspace style (ResultsWorkspaceTab's
// SummaryCard markup) so both tabs read the same.
function RosterSummaryCard({ testId, label, value }: { testId: string; label: string; value: number }) {
  return (
    <div data-testid={testId} className="rounded-xl border border-line bg-card p-4 shadow-sm">
      <div className="text-sm text-ink-600">{label}</div>
      <div className="mt-1 text-2xl font-bold text-ink-950">{value}</div>
    </div>
  );
}

// admin_school orders (Midtrans-paid); super_admin and admin_exam grant directly (no order,
// no payment). Same participant pool, submit action branches by role.
export function ExamRegistrationsTab({ examId, examName }: ExamRegistrationsTabProps) {
  const { t } = useTranslation();
  const queryClient = useQueryClient();
  const role = useAuthStore((s) => s.user?.role);
  const schoolId = useAuthStore((s) => s.user?.school_id);
  const canGrant = hasCapability(role, "exam-grants:write");
  const canBulkGrant = hasCapability(role, "exam-grants-bulk:write");
  const canRegister = canGrant || hasCapability(role, "bulk-exam-orders:write");

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
  const [revokeOpen, setRevokeOpen] = useState(false);
  const [revokeMode, setRevokeMode] = useState<"manual" | "csv">("manual");
  const [revokeSelectedIds, setRevokeSelectedIds] = useState<string[]>([]);
  const [revokeResult, setRevokeResult] = useState<RevokeExamAccessResponse | null>(null);
  const [revokeConfirmOpen, setRevokeConfirmOpen] = useState(false);
  const [revokeCsvFile, setRevokeCsvFile] = useState<File | null>(null);
  const [revokeJobId, setRevokeJobId] = useState<string | null>(null);
  const flowGenerationRef = useRef(0);

  const previewMutation = usePreviewBulkExamOrder();
  const createMutation = useCreateBulkExamOrder();
  const grantMutation = useGrantExamAccess();
  const revokeMutation = useRevokeExamAccess();
  const grantPresignMutation = usePresignExamGrantBulkUpload(examId);
  const grantEnqueueMutation = useEnqueueExamGrantBulk();
  const revokePresignMutation = usePresignExamRevokeBulkUpload(examId);
  const revokeEnqueueMutation = useEnqueueExamRevokeBulk();
  const csvPresignMutation = grantPresignMutation;
  const csvEnqueueMutation = grantEnqueueMutation;
  const csvJob = useJobStatus(csvJobId);
  const revokeJob = useJobStatus(revokeJobId);

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

  const handleRevokeCsvSubmit = async () => {
    if (!revokeCsvFile) {
      toast.error(t("exam_revoke_bulk_no_file"));
      return;
    }
    const flowGeneration = flowGenerationRef.current;
    try {
      const presignResp = await revokePresignMutation.mutateAsync({
        filename: revokeCsvFile.name,
        contentType: revokeCsvFile.type || "text/csv",
      });
      if (flowGeneration !== flowGenerationRef.current) return;
      try {
        await putFileToPresignedURL(presignResp.url, revokeCsvFile, revokeCsvFile.type || "text/csv");
      } catch (err) {
        if (flowGeneration !== flowGenerationRef.current) return;
        toast.error(err instanceof Error ? err.message : t("exam_revoke_bulk_put_failed"));
        return;
      }
      if (flowGeneration !== flowGenerationRef.current) return;
      const enqueueResp = await revokeEnqueueMutation.mutateAsync({
        examId,
        fileKey: presignResp.key,
      });
      if (flowGeneration !== flowGenerationRef.current) return;
      setRevokeJobId(enqueueResp.job_id);
    } catch (err) {
      if (flowGeneration !== flowGenerationRef.current) return;
      toast.error(err instanceof Error ? err.message : t("exam_revoke_bulk_enqueue_failed"));
    }
  };

  const handleRevokeConfirm = () => {
    if (revokeSelectedIds.length === 0) return;
    revokeMutation.mutate(
      { exam_id: examId, student_ids: revokeSelectedIds },
      {
        onSuccess: (result) => {
          setRevokeConfirmOpen(false);
          setRevokeResult(result);
          setRevokeSelectedIds([]);
          if (result.revoked_count > 0) {
            toast.success(t("exam_roster_revoke_success").replace("{n}", String(result.revoked_count)));
          }
          queryClient.invalidateQueries({ queryKey: [...adminExamsKeys.rosters(), examId] });
        },
        onError: (err) => {
          setRevokeConfirmOpen(false);
          toast.error(err instanceof Error ? err.message : t("exam_roster_revoke_failed"));
        },
      },
    );
  };

  const csvJobData = csvJob.data;
  const csvIsTerminalSuccess = csvJobData?.status === "succeeded";
  const csvIsTerminalFailed = csvJobData?.status === "failed";
  const revokeJobData = revokeJob.data;
  const revokeIsTerminalSuccess = revokeJobData?.status === "succeeded";
  const revokeIsTerminalFailed = revokeJobData?.status === "failed";
  const previewExcluded = previewResult?.excluded ?? [];

  useEffect(() => {
    if (csvIsTerminalSuccess) {
      queryClient.invalidateQueries({ queryKey: [...adminExamsKeys.rosters(), examId] });
    }
    // Only re-run when the job transitions to success, not on every roster/queryClient identity change.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [csvIsTerminalSuccess, examId]);

  useEffect(() => {
    if (revokeIsTerminalSuccess) {
      queryClient.invalidateQueries({ queryKey: [...adminExamsKeys.rosters(), examId] });
    }
    // Only re-run when the job transitions to success, not on every roster/queryClient identity change.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [revokeIsTerminalSuccess, examId]);

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

  const resetRevokeState = () => {
    setRevokeMode("manual");
    setRevokeSelectedIds([]);
    setRevokeResult(null);
    setRevokeConfirmOpen(false);
    setRevokeCsvFile(null);
    setRevokeJobId(null);
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

  const handleRevokeModalOpenChange = (open: boolean) => {
    setRevokeOpen(open);
    flowGenerationRef.current += 1;
    if (!open) {
      resetRevokeState();
    }
  };

  const modalTitle = canGrant
    ? t("exam_registrations_grant_modal_title")
    : t("bulk_exam_order_pick_participants");

  // ── Main roster + action modal ──────────────────────────────────────────

  return (
    <div className="space-y-6">
      <ExamRosterSection
        examId={examId}
        action={
          canRegister ? (
            <>
              <Button
                variant="outline"
                size="sm"
                className="rounded-full"
                data-testid={canGrant ? "open-grant-modal" : "open-order-modal"}
                onClick={() => setModalOpen(true)}
              >
                {canGrant ? t("exam_grant_grant") : t("bulk_exam_order_pick_participants")}
              </Button>
              {canGrant && (
                <Button
                  variant="outline"
                  size="sm"
                  className="rounded-full"
                  data-testid="open-revoke-modal"
                  onClick={() => setRevokeOpen(true)}
                >
                  {t("exam_roster_revoke")}
                </Button>
              )}
            </>
          ) : undefined
        }
      />

      {canRegister && (
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
                {canBulkGrant && (
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

                {canBulkGrant && grantMode === "csv" ? (
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
                          onClick={() => downloadExamBulkTemplate("exam_grant_bulk_template.csv")}
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
                      schoolId={canGrant ? undefined : schoolId}
                      selected={selectedStudentIds}
                      onChange={setSelectedStudentIds}
                    />
                  </div>
                )}

                {selectedStudentIds.length > 0 && canGrant && grantMode === "manual" && (
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

                {selectedStudentIds.length > 0 && !canGrant && (
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

                        {previewExcluded.length > 0 && (
                          <div className="max-h-[160px] overflow-y-auto rounded-lg border border-line p-2">
                            {previewExcluded.map((s) => (
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

      {canGrant && (
        <Dialog open={revokeOpen} onOpenChange={handleRevokeModalOpenChange}>
          <DialogContent className="sm:max-w-2xl">
            <DialogHeader>
              <DialogTitle>{t("exam_revoke_modal_title")}</DialogTitle>
            </DialogHeader>

            {revokeResult ? (
              <div className="space-y-3">
                <div className="flex items-center gap-2">
                  {revokeResult.revoked_count > 0 ? (
                    <CheckCircle className="size-5 text-success" />
                  ) : null}
                  <h4 className="text-sm font-semibold text-ink-900">
                    {t("exam_roster_revoke_result_title")}
                  </h4>
                </div>
                {revokeResult.revoked_count > 0 && (
                  <p className="text-sm text-ink-500">
                    {t("exam_roster_revoke_success").replace("{n}", String(revokeResult.revoked_count))}
                  </p>
                )}
                <div className="max-h-[240px] space-y-1 overflow-y-auto rounded-lg border border-line p-2">
                  {revokeResult.results.map((r) => (
                    <div key={r.student_id} className="flex items-center gap-2 px-2 py-1.5 text-sm">
                      <span className="font-medium text-ink-900">{r.name || r.student_id}</span>
                      <span className="text-ink-500">{r.username ? `@${r.username}` : ""}</span>
                      <span className="ml-auto text-xs text-ink-500">
                        {r.status}
                        {r.message ? ` — ${r.message}` : ""}
                      </span>
                    </div>
                  ))}
                </div>
                <div className="flex justify-end">
                  <Button variant="outline" className="rounded-full" onClick={resetRevokeState}>
                    {t("exam_revoke_again")}
                  </Button>
                </div>
              </div>
            ) : (
              <>
                {canBulkGrant && (
                  <div className="flex gap-2" data-testid="revoke-mode-toggle">
                    <Button
                      type="button"
                      variant={revokeMode === "manual" ? "default" : "outline"}
                      size="sm"
                      className="rounded-full"
                      data-testid="revoke-mode-manual"
                      onClick={() => setRevokeMode("manual")}
                    >
                      {t("exam_grant_mode_manual")}
                    </Button>
                    <Button
                      type="button"
                      variant={revokeMode === "csv" ? "default" : "outline"}
                      size="sm"
                      className="rounded-full"
                      data-testid="revoke-mode-csv"
                      onClick={() => setRevokeMode("csv")}
                    >
                      {t("exam_grant_mode_csv")}
                    </Button>
                  </div>
                )}

                {revokeMode === "csv" ? (
                  <div className="space-y-6">
                    <section>
                      <h3 className="text-sm font-semibold text-ink-900">
                        1. {t("exam_revoke_bulk_download_template")}
                      </h3>
                      <div className="mt-2">
                        <Button
                          type="button"
                          variant="outline"
                          size="sm"
                          className="rounded-full"
                          data-testid="revoke-csv-download-template"
                          onClick={() => downloadExamBulkTemplate("exam_revoke_bulk_template.csv")}
                        >
                          <Download className="mr-2 size-4" />
                          {t("exam_revoke_bulk_download_template")}
                        </Button>
                      </div>
                    </section>

                    <section>
                      <h3 className="text-sm font-semibold text-ink-900">
                        2. {t("exam_revoke_bulk_upload")}
                      </h3>
                      <div className="mt-2 space-y-3">
                        <div className="grid gap-2">
                          <Label htmlFor="exam-revoke-bulk-file">
                            {t("exam_revoke_bulk_choose_file")}
                          </Label>
                          <Input
                            id="exam-revoke-bulk-file"
                            data-testid="revoke-csv-file-input"
                            type="file"
                            accept=".csv,text/csv"
                            onChange={(e) => setRevokeCsvFile(e.target.files?.[0] ?? null)}
                            disabled={revokePresignMutation.isPending || revokeEnqueueMutation.isPending}
                          />
                          {revokeCsvFile && (
                            <p className="text-sm text-muted-foreground">{revokeCsvFile.name}</p>
                          )}
                        </div>

                        <Button
                          type="button"
                          className="rounded-full"
                          data-testid="revoke-csv-upload-submit"
                          onClick={handleRevokeCsvSubmit}
                          disabled={
                            !revokeCsvFile || revokePresignMutation.isPending || revokeEnqueueMutation.isPending
                          }
                        >
                          {revokePresignMutation.isPending || revokeEnqueueMutation.isPending ? (
                            <Loader2 className="mr-2 size-4 animate-spin" />
                          ) : null}
                          {t("exam_revoke_bulk_upload")}
                        </Button>
                      </div>
                    </section>

                    {revokeJobData && (
                      <section className="md-card-outlined space-y-3 p-4">
                        {revokeIsTerminalSuccess && (
                          <div className="flex items-center gap-2">
                            <CheckCircle className="size-5 text-success" />
                            <h4 className="text-sm font-semibold text-ink-900">
                              {t("exam_revoke_bulk_success")}
                            </h4>
                          </div>
                        )}

                        {!revokeIsTerminalSuccess && !revokeIsTerminalFailed && (
                          <div className="space-y-2">
                            <p className="text-sm font-medium text-ink-900">
                              {t("exam_revoke_bulk_progress").replace(
                                "{pct}",
                                String(Math.round(revokeJobData.progress ?? 0)),
                              )}
                            </p>
                            <div className="h-2 w-full overflow-hidden rounded-full bg-surface-2">
                              <div
                                className="h-full bg-primary transition-all"
                                style={{
                                  width: `${Math.max(0, Math.min(100, revokeJobData.progress ?? 0))}%`,
                                }}
                              />
                            </div>
                          </div>
                        )}

                        {revokeIsTerminalFailed && (
                          <div className="space-y-2">
                            <h4 className="text-sm font-semibold text-danger">
                              {t("exam_revoke_bulk_failed")}
                            </h4>
                            {revokeJobData.error && (
                              <p className="text-sm text-danger">{revokeJobData.error}</p>
                            )}
                          </div>
                        )}

                        {(revokeIsTerminalSuccess || revokeIsTerminalFailed) && revokeJobData.result_url && (
                          <a
                            href={revokeJobData.result_url}
                            className="inline-flex items-center gap-2 text-sm font-medium text-primary underline-offset-4 hover:underline"
                            download="exam_revoke_bulk_result.csv"
                          >
                            <Download className="size-4" />
                            {t("exam_revoke_bulk_download_result")}
                          </a>
                        )}
                      </section>
                    )}
                  </div>
                ) : (
                  <div>
                    <ParticipantPicker
                      examId={examId}
                      mode="revoke"
                      selected={revokeSelectedIds}
                      onChange={setRevokeSelectedIds}
                    />
                  </div>
                )}

                {revokeMode === "manual" && revokeSelectedIds.length > 0 && (
                  <Button
                    size="lg"
                    className="rounded-full"
                    data-testid="revoke-submit"
                    onClick={() => setRevokeConfirmOpen(true)}
                    disabled={revokeMutation.isPending}
                  >
                    {t("exam_roster_revoke")}
                  </Button>
                )}
              </>
            )}
          </DialogContent>
        </Dialog>
      )}

      {canGrant && (
        <Dialog open={revokeConfirmOpen} onOpenChange={setRevokeConfirmOpen}>
          <DialogContent className="sm:max-w-md">
            <DialogHeader>
              <DialogTitle>{t("exam_roster_revoke_confirm_title")}</DialogTitle>
            </DialogHeader>
            <p className="text-sm text-ink-600">
              {t("exam_roster_revoke_confirm_desc").replace("{n}", String(revokeSelectedIds.length))}
            </p>
            <div className="mt-4 flex justify-end gap-2">
              <Button
                variant="outline"
                className="rounded-full"
                data-testid="revoke-confirm-cancel"
                disabled={revokeMutation.isPending}
                onClick={() => setRevokeConfirmOpen(false)}
              >
                {t("exam_roster_revoke_cancel")}
              </Button>
              <Button
                className="rounded-full"
                data-testid="revoke-confirm"
                disabled={revokeMutation.isPending}
                onClick={handleRevokeConfirm}
              >
                {revokeMutation.isPending ? (
                  <Loader2 className="mr-2 size-4 animate-spin" />
                ) : null}
                {revokeMutation.isPending ? t("sys_loading") : t("exam_roster_revoke")}
              </Button>
            </div>
          </DialogContent>
        </Dialog>
      )}
    </div>
  );
}
