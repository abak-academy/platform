"use client";

import { useMemo, useState } from "react";
import { ArrowUpDown, CheckCircle, Loader2 } from "lucide-react";
import { toast } from "sonner";
import { useTranslation } from "@/lib/i18n";
import { ParticipantPicker } from "@/components/admin/ParticipantPicker";
import { SnapCheckout } from "@/components/cart/SnapCheckout";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import { formatRupiah } from "@/lib/format";
import {
  usePreviewBulkExamOrder,
  useCreateBulkExamOrder,
} from "@/lib/hooks/admin-bulk-exam-orders";
import { useGrantExamAccess } from "@/lib/hooks/admin-exam-grants";
import { useExamRoster } from "@/lib/hooks/admin-exams";
import { useAuthStore } from "@/stores/auth";
import type { ExamRosterEntry } from "@/lib/types";

interface ExamRegistrationsTabProps {
  examId: string;
  examName: string;
}

// csvField quotes a CSV field only when it needs it (contains a comma, quote,
// or newline), doubling embedded quotes per RFC 4180 — student names can
// contain commas.
//
// A field whose first character is one a spreadsheet reads as the start of a
// formula is additionally prefixed with a single quote and force-quoted:
// student names are attacker-supplied at registration, so without this a name
// like `=HYPERLINK(...)` executes when an admin opens the export in Excel or
// Sheets.
const FORMULA_LEAD = /^[=+\-@\t\r]/;

function csvField(value: string): string {
  const neutralized = FORMULA_LEAD.test(value) ? `'${value}` : value;
  if (neutralized !== value || /[",\n]/.test(neutralized)) {
    return `"${neutralized.replace(/"/g, '""')}"`;
  }
  return neutralized;
}

function downloadRosterCSV(rows: ExamRosterEntry[]): void {
  const header = ["No. Peserta", "Nama", "Username", "Status", "Checked In"];
  const lines = rows.map((r) =>
    [
      r.participant_no || "",
      r.student_name,
      r.student_username ?? "",
      r.status,
      r.checked_in_at ? "yes" : "no",
    ]
      .map(csvField)
      .join(","),
  );
  const csv = [header.join(","), ...lines].join("\n") + "\n";
  const blob = new Blob([csv], { type: "text/csv;charset=utf-8" });
  const url = URL.createObjectURL(blob);
  const a = document.createElement("a");
  a.href = url;
  a.download = "roster.csv";
  document.body.appendChild(a);
  a.click();
  document.body.removeChild(a);
  URL.revokeObjectURL(url);
}

// ExamRosterSection is the FR-32 read-only participant roster: a sortable
// "No. Peserta" table (rows without a stored participant_number sort last and
// render "—" rather than a bogus number) plus a client-side CSV export.
function ExamRosterSection({ examId }: { examId: string }) {
  const { t } = useTranslation();
  const [sortDir, setSortDir] = useState<"asc" | "desc">("asc");
  const { data, isLoading, isError } = useExamRoster(examId);

  const rows = useMemo(() => {
    const list = data?.data ?? [];
    const sorted = [...list].sort((a, b) => {
      const an = a.participant_number ?? Number.MAX_SAFE_INTEGER;
      const bn = b.participant_number ?? Number.MAX_SAFE_INTEGER;
      return an - bn;
    });
    return sortDir === "asc" ? sorted : sorted.reverse();
  }, [data, sortDir]);

  return (
    <section className="md-card-outlined space-y-3 p-5">
      <div className="flex items-center justify-between">
        <h3 className="font-serif text-base font-semibold text-ink-900">
          {t("exam_roster_title")}
        </h3>
        <Button
          variant="outline"
          size="sm"
          className="rounded-full"
          disabled={rows.length === 0}
          onClick={() => downloadRosterCSV(rows)}
        >
          {t("exam_roster_export_csv")}
        </Button>
      </div>

      {isError && <p className="text-sm text-danger">{t("exam_roster_load_failed")}</p>}

      {!isError && isLoading && (
        <p className="text-sm text-ink-500">…</p>
      )}

      {!isError && !isLoading && rows.length === 0 && (
        <p className="text-sm text-ink-500">{t("exam_roster_empty")}</p>
      )}

      {!isError && !isLoading && rows.length > 0 && (
        <div className="overflow-x-auto">
          <table className="w-full text-left text-sm">
            <thead>
              <tr className="border-b border-line text-ink-500">
                <th className="py-2 pr-3 font-medium">
                  <button
                    type="button"
                    className="flex items-center gap-1"
                    onClick={() => setSortDir((d) => (d === "asc" ? "desc" : "asc"))}
                  >
                    {t("exam_roster_th_participant_no")}
                    <ArrowUpDown className="size-3.5" />
                  </button>
                </th>
                <th className="py-2 pr-3 font-medium">{t("exam_roster_th_name")}</th>
                <th className="py-2 pr-3 font-medium">{t("exam_roster_th_username")}</th>
                <th className="py-2 pr-3 font-medium">{t("exam_roster_th_status")}</th>
                <th className="py-2 font-medium">{t("exam_roster_th_checked_in")}</th>
              </tr>
            </thead>
            <tbody>
              {rows.map((r) => (
                <tr key={r.registration_id} className="border-b border-line/50">
                  <td
                    data-testid="roster-participant-no"
                    className="py-2 pr-3 font-medium text-ink-900"
                  >
                    {r.participant_no || "—"}
                  </td>
                  <td className="py-2 pr-3 text-ink-900">{r.student_name}</td>
                  <td className="py-2 pr-3 text-ink-500">
                    {r.student_username ? `@${r.student_username}` : "—"}
                  </td>
                  <td className="py-2 pr-3">{r.status}</td>
                  <td className="py-2">{r.checked_in_at ? "✓" : "—"}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
    </section>
  );
}

// admin_school orders (Midtrans-paid); super_admin grants directly (no order,
// no payment). Same participant pool, submit action branches by role.
export function ExamRegistrationsTab({ examId, examName }: ExamRegistrationsTabProps) {
  const { t } = useTranslation();
  const role = useAuthStore((s) => s.user?.role);
  const schoolId = useAuthStore((s) => s.user?.school_id);
  const isSuperAdmin = role === "super_admin";

  const [selectedStudentIds, setSelectedStudentIds] = useState<string[]>([]);
  const [createdOrderId, setCreatedOrderId] = useState<string | null>(null);
  const [grantResult, setGrantResult] = useState<{
    granted_count: number;
    granted_students: Array<{ id: string; name: string; username: string }>;
  } | null>(null);

  const previewMutation = usePreviewBulkExamOrder();
  const createMutation = useCreateBulkExamOrder();
  const grantMutation = useGrantExamAccess();

  const previewInput = useMemo(() => {
    if (selectedStudentIds.length === 0) return null;
    return { exam_id: examId, student_ids: selectedStudentIds };
  }, [examId, selectedStudentIds]);

  const handlePreview = () => {
    if (selectedStudentIds.length === 0) {
      toast.error(t("bulk_exam_order_empty_students"));
      return;
    }
    previewMutation.mutate(
      { exam_id: examId, student_ids: selectedStudentIds },
      { onError: () => toast.error(t("bulk_exam_order_preview_failed")) },
    );
  };

  const handleCreateOrder = () => {
    if (!previewInput) return;
    createMutation.mutate(previewInput, {
      onSuccess: (order) => {
        setCreatedOrderId(order.id);
        toast.success(t("bulk_exam_order_created"));
      },
      onError: (err) => {
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
    grantMutation.mutate(
      { exam_id: examId, student_ids: selectedStudentIds },
      {
        onSuccess: (result) => {
          setGrantResult(result);
          toast.success(t("exam_grant_success"));
        },
        onError: (err) => {
          const msg = err instanceof Error ? err.message : t("error_generic");
          toast.error(msg);
        },
      },
    );
  };

  const handleReset = () => {
    setSelectedStudentIds([]);
    setCreatedOrderId(null);
    setGrantResult(null);
    previewMutation.reset();
    createMutation.reset();
    grantMutation.reset();
  };

  // ── Success states ──────────────────────────────────────────────────────

  if (createdOrderId) {
    return (
      <div className="md-card-outlined p-6 text-center">
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
    );
  }

  if (grantResult) {
    return (
      <div className="md-card-outlined p-6 text-center">
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
    );
  }

  // ── Main picker + action ────────────────────────────────────────────────

  return (
    <div className="space-y-6">
      <ExamRosterSection examId={examId} />

      <section>
        <h3 className="font-serif text-base font-semibold text-ink-900">
          {t("bulk_exam_order_pick_participants")}
        </h3>
        <div className="mt-3">
          <ParticipantPicker
            schoolId={isSuperAdmin ? undefined : schoolId}
            selected={selectedStudentIds}
            onChange={setSelectedStudentIds}
          />
        </div>
      </section>

      {selectedStudentIds.length > 0 && isSuperAdmin && (
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

          {previewMutation.data && (
            <div className="md-card-outlined space-y-4 p-5">
              <h4 className="font-serif text-base font-semibold text-ink-900">
                {t("bulk_exam_order_preview_title")}
              </h4>
              <div className="flex items-center gap-2">
                <Badge variant="outline">
                  {previewMutation.data.net_new_count}{" "}
                  {t("bulk_exam_order_students_count").replace(
                    "{n}",
                    String(previewMutation.data.net_new_count),
                  )}
                </Badge>
              </div>

              {previewMutation.data.excluded.length > 0 && (
                <div className="max-h-[160px] overflow-y-auto rounded-lg border border-line p-2">
                  {previewMutation.data.excluded.map((s) => (
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
                    {formatRupiah(previewMutation.data.total)}
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
    </div>
  );
}
