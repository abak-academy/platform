"use client";

import { useState, useCallback } from "react";
import { Activity, BarChart, CheckCircle2, Clock, Eye, RefreshCw, AlertTriangle, RotateCcw, Users, XCircle } from "lucide-react";
import { toast } from "sonner";
import { AdminPageHeader } from "@/components/admin/AdminPageHeader";
import {
  useAvailableExamsForMonitor,
  useSessionMonitor,
  useReopenSession,
  useForceSubmitSession,
} from "@/lib/hooks/admin-sessions";
import { useTranslation } from "@/lib/i18n";
import { Button } from "@/components/ui/button";
import { Badge, type badgeVariants } from "@/components/ui/badge";
import type { VariantProps } from "class-variance-authority";
import { Progress } from "@/components/ui/progress";
import { StatCard } from "@/components/admin/StatCard";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { EmptyState } from "@/components/ui/empty-state";
import { Skeleton } from "@/components/ui/skeleton";
import { DataTable, type DataTableColumn } from "@/components/ui/data-table";
import { AbakAvatar } from "@/components/ui/abak-avatar";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import type { ExamMonitorAvailable, SessionMonitorRow, SessionMonitorStatus } from "@/lib/types";

type BadgeVariant = NonNullable<VariantProps<typeof badgeVariants>["variant"]>;

// ── Status label key map (i18n keys for each derived status) ──
// Explicit mapping because status "checked_in" has no underscore in its key "st_checkedin"

const STATUS_LABEL_KEY: Record<SessionMonitorStatus, string> = {
  registered: "st_registered",
  checked_in: "st_checkedin",
  in_progress: "st_inprogress",
  overdue: "st_overdue",
  submitted: "st_submitted",
};

// ── Status badge map ──

const STATUS_BADGE: Record<SessionMonitorStatus, BadgeVariant> = {
  registered: "mute",
  checked_in: "brand",
  in_progress: "teal",
  overdue: "danger",
  submitted: "sky",
};

// ── Date formatting helpers ──

function formatRemaining(seconds: number): string {
  const m = Math.floor(seconds / 60);
  const s = seconds % 60;
  return `${String(m).padStart(2, "0")}:${String(s).padStart(2, "0")}`;
}

function formatTime(iso: string | null): string {
  if (!iso) return "—";
  const d = new Date(iso);
  return d.toLocaleTimeString("id-ID", { hour: "2-digit", minute: "2-digit" });
}

function formatWindow(scheduledAt: string | null, scheduledEndAt: string | null): string {
  if (!scheduledAt) return "—";
  const dateOpts: Intl.DateTimeFormatOptions = {
    day: "2-digit",
    month: "short",
    hour: "2-digit",
    minute: "2-digit",
  };
  const start = new Date(scheduledAt).toLocaleString("id-ID", dateOpts);
  if (!scheduledEndAt) return start;
  const end = new Date(scheduledEndAt).toLocaleString("id-ID", dateOpts);
  return `${start} – ${end}`;
}

// ── Available-exams state badge map ──

const AVAILABLE_STATE_BADGE: Record<ExamMonitorAvailable["state"], { variant: BadgeVariant; labelKey: string }> = {
  live: { variant: "live", labelKey: "monitor_state_live" },
  ended: { variant: "mute", labelKey: "monitor_state_ended" },
};

// ── Page component ──

export default function ExamMonitorPage() {
  const { t } = useTranslation();
  const [selectedExamId, setSelectedExamId] = useState<string>("");

  const {
    data: availableData,
    isLoading: availableLoading,
    isError: availableIsError,
    error: availableErr,
    refetch: refetchAvailable,
  } = useAvailableExamsForMonitor();
  const availableExams = availableData?.data ?? [];

  const {
    data: monitorData,
    isLoading: monitorLoading,
    isError: monitorError,
    error: monitorErr,
    refetch: refetchMonitor,
  } = useSessionMonitor(selectedExamId || undefined);

  const reopen = useReopenSession();
  const forceSubmit = useForceSubmitSession();

  const rows = monitorData?.rows ?? [];
  const violations = monitorData?.violations_recent ?? [];
  const selectedCandidate = availableExams.find((e) => e.id === selectedExamId);
  const examTitle = selectedCandidate?.title ?? monitorData?.exam?.title ?? "";

  const handleReopen = useCallback(
    (sessionId: string) => {
      const minutes = prompt(t("monitor_reopen_prompt"));
      if (!minutes) return;
      const n = Number(minutes);
      if (!Number.isInteger(n) || n <= 0) return;
      reopen.mutate(
        { sessionId, extend_minutes: n },
        {
          onSuccess: () => {
            toast.success(t("monitor_reopened_success"));
            refetchMonitor();
          },
          onError: () => toast.error(t("error_generic")),
        },
      );
    },
    [reopen, refetchMonitor, t],
  );

  const handleForceSubmit = useCallback(
    (sessionId: string) => {
      if (!confirm(t("monitor_force_submit_confirm"))) return;
      forceSubmit.mutate(sessionId, {
        onSuccess: () => {
          toast.success(t("monitor_force_submitted_success"));
          refetchMonitor();
        },
        onError: () => toast.error(t("error_generic")),
      });
    },
    [forceSubmit, refetchMonitor, t],
  );

  // ── Available-exams table ──

  const availableColumns: DataTableColumn<ExamMonitorAvailable>[] = [
    { key: "title", header: t("monitor_th_exam"), cell: (row) => row.title },
    {
      key: "window",
      header: t("monitor_th_window"),
      cell: (row) => (
        <span className="text-ink-600">{formatWindow(row.scheduled_at, row.scheduled_end_at)}</span>
      ),
    },
    {
      key: "state",
      header: t("th_status"),
      cell: (row) => {
        const badge = AVAILABLE_STATE_BADGE[row.state];
        return (
          <Badge variant={badge.variant}>
            {t(badge.labelKey as any)}
          </Badge>
        );
      },
    },
    {
      key: "not_started",
      header: t("monitor_th_not_started"),
      align: "right",
      cell: (row) => String(row.not_started_count),
    },
    {
      key: "active",
      header: t("monitor_th_active"),
      align: "right",
      cell: (row) => String(row.active_count),
    },
    {
      key: "total",
      header: t("monitor_th_total_registered"),
      align: "right",
      cell: (row) => String(row.total_registered),
    },
    {
      key: "actions",
      header: <span className="inline-block size-px overflow-hidden whitespace-nowrap [clip-path:inset(50%)]">{t("th_actions")}</span>,
      align: "right",
      cell: (row) => (
        <Button variant="outline" size="sm" onClick={() => setSelectedExamId(row.id)}>
          <Eye />
          {t("monitor_actions_view")}
        </Button>
      ),
    },
  ];

  const availableSection = (
    <div className="flex flex-col gap-3">
      <h2 className="type-h2">{t("monitor_pick_exam")}</h2>
      {availableLoading ? (
        <div className="space-y-2">
          {Array.from({ length: 3 }).map((_, i) => (
            <Skeleton key={i} className="h-12 w-full" />
          ))}
        </div>
      ) : availableIsError ? (
        <div className="rounded-lg border border-danger/20 bg-danger/10 p-4 text-danger">
          <p className="flex items-center gap-2 font-medium">
            <AlertTriangle size={16} />
            {availableErr instanceof Error ? availableErr.message : t("error_generic")}
          </p>
          <Button variant="outline" size="sm" className="mt-2" onClick={() => refetchAvailable()}>
            <RefreshCw size={14} /> {t("retry")}
          </Button>
        </div>
      ) : (
        <DataTable
          columns={availableColumns}
          rows={availableExams}
          rowKey={(row) => row.id}
          data-testid="exam-monitor-available-table"
          empty={
            <EmptyState icon={BarChart} title={t("monitor_available_empty")} />
          }
        />
      )}
    </div>
  );

  // ── Session table columns (detail section) ──

  const columns: DataTableColumn<SessionMonitorRow>[] = [
    {
      key: "name",
      header: t("th_name"),
      nowrap: true,
      cell: (row) => (
        <span className="inline-flex items-center gap-2.5 text-ink-900">
          <AbakAvatar name={row.student_name} size="xs" ring={false} />
          {row.student_name}
        </span>
      ),
    },
    {
      key: "school",
      header: t("school"),
      cell: (row) => (
        <span className="text-ink-600">{row.school_name ?? "—"}</span>
      ),
    },
    {
      key: "status",
      header: t("th_status"),
      cell: (row) => {
        return (
          <Badge variant={STATUS_BADGE[row.status]}>
            {t(STATUS_LABEL_KEY[row.status] as any)}
          </Badge>
        );
      },
    },
    {
      key: "active_section",
      header: t("monitor_th_active_section"),
      className: "text-xs",
      cell: (row) =>
        row.active_section_title ? (
          <div className="flex flex-col gap-0.5">
            <span className="font-medium">{row.active_section_title}</span>
            <span className="text-ink-600">
              {formatRemaining(row.active_section_remaining_seconds ?? 0)}
            </span>
          </div>
        ) : (
          <span className="text-ink-600">—</span>
        ),
    },
    {
      key: "progress",
      header: t("monitor_th_progress"),
      cell: (row) => {
        const progressPct =
          row.total_questions > 0 ? Math.round((row.answers_saved / row.total_questions) * 100) : 0;
        return (
          <div className="flex min-w-[140px] items-center gap-2">
            <Progress
              value={progressPct}
              variant={row.status === "submitted" ? "teal" : "ember"}
              className="flex-1"
            />
            <span className="text-xs text-ink-600">
              {row.answers_saved}/{row.total_questions}
            </span>
          </div>
        );
      },
    },
    {
      key: "checked_in",
      header: t("monitor_th_checked_in"),
      cell: (row) => (
        <span className="text-ink-600">{formatTime(row.checked_in_at)}</span>
      ),
    },
    {
      key: "last_activity",
      header: t("monitor_th_last_activity"),
      cell: (row) => (
        <span className="text-ink-600">{formatTime(row.last_saved_at)}</span>
      ),
    },
    {
      key: "actions",
      header: <span className="inline-block size-px overflow-hidden whitespace-nowrap [clip-path:inset(50%)]">{t("th_actions")}</span>,
      cell: (row) =>
        (row.status === "in_progress" || row.status === "overdue") && row.session_id ? (
          <div className="flex items-center gap-1">
            <Button
              variant="ghost"
              size="sm"
              onClick={() => handleReopen(row.session_id!)}
              disabled={reopen.isPending}
            >
              <RotateCcw />
              {t("monitor_actions_reopen")}
            </Button>
            {row.status === "overdue" && <Button
              variant="ghost"
              size="sm"
              onClick={() => handleForceSubmit(row.session_id!)}
              disabled={forceSubmit.isPending}
            >
              <XCircle />
              {t("monitor_actions_force_submit")}
            </Button>}
          </div>
        ) : null,
    },
  ];

  // ── Detail section (session table + violations sidebar) ──

  let detailSection: React.ReactNode = null;

  if (selectedExamId && monitorLoading) {
    detailSection = (
      <div className="flex flex-col gap-4 lg:flex-row">
        <div className="flex-1 space-y-2">
          {Array.from({ length: 5 }).map((_, i) => (
            <Skeleton key={i} className="h-12 w-full" />
          ))}
        </div>
        <div className="w-full space-y-2 lg:w-72">
          <Skeleton className="h-48 w-full" />
        </div>
      </div>
    );
  } else if (monitorError) {
    detailSection = (
      <div className="rounded-lg border border-danger/20 bg-danger/10 p-4 text-danger">
        <p className="flex items-center gap-2 font-medium">
          <AlertTriangle size={16} />
          {monitorErr instanceof Error ? monitorErr.message : t("error_generic")}
        </p>
        <Button variant="outline" size="sm" className="mt-2" onClick={() => refetchMonitor()}>
          <RefreshCw size={14} /> {t("retry")}
        </Button>
      </div>
    );
  } else if (selectedExamId) {
    detailSection = (
      <div className="flex flex-col gap-4 lg:flex-row">
        <div className="flex min-w-0 flex-1 flex-col gap-3">
          <div className="grid grid-cols-2 gap-3 min-[1200px]:grid-cols-4" data-testid="exam-monitor-kpis">
            <StatCard tone="sky" icon={Users} label={t("st_registered")} value={String(rows.length)} />
            <StatCard
              tone="ember"
              icon={Activity}
              label={t("st_inprogress")}
              value={String(rows.filter((r) => r.status === "in_progress" || r.status === "overdue").length)}
            />
            <StatCard
              tone="teal"
              icon={CheckCircle2}
              label={t("st_submitted")}
              value={String(rows.filter((r) => r.status === "submitted").length)}
            />
            <StatCard
              tone="gold"
              icon={Clock}
              label={t("monitor_th_not_started")}
              value={String(rows.filter((r) => r.status === "registered" || r.status === "checked_in").length)}
            />
          </div>
          <DataTable
            columns={columns}
            rows={rows}
            rowKey={(row) => row.registration_id}
            data-testid="exam-monitor-table"
            empty={
              <EmptyState icon={BarChart} title={t("monitor_empty")} />
            }
          />
        </div>

        <div className="w-full lg:w-72 lg:shrink-0">
          <div className="md-card-outlined !p-4">
            <h3 className="mb-3 type-h3">{t("monitor_sidebar_title")}</h3>
            {violations.length === 0 ? (
              <p className="text-sm text-ink-600">
                {t("monitor_no_violations")}
              </p>
            ) : (
              <ul className="space-y-3">
                {violations.map((v, i) => (
                  <li key={`${v.session_id}-${i}`} className="text-sm">
                    <div className="flex items-center justify-between">
                      <span className="font-medium">{v.student_name}</span>
                      <span className="text-xs text-ink-600">
                        ×{v.count}
                      </span>
                    </div>
                    <div className="flex items-center gap-1 text-xs text-ink-600">
                      <AlertTriangle size={10} />
                      <span>{v.latest_type}</span>
                    </div>
                    <p className="text-xs text-ink-600">
                      {v.latest_occurred_at
                        ? new Date(v.latest_occurred_at).toLocaleTimeString("id-ID", {
                            hour: "2-digit",
                            minute: "2-digit",
                          })
                        : "—"}
                    </p>
                  </li>
                ))}
              </ul>
            )}
          </div>
        </div>
      </div>
    );
  }

  // ── Main content ──

  return (
    <div className="flex flex-col gap-5 fade-in">
      <AdminPageHeader
        title={t("exam_monitor_title")}
        description={t("exam_monitor_subtitle")}
        actions={
          availableExams.length > 0 ? (
            <Select value={selectedExamId} onValueChange={setSelectedExamId}>
              <SelectTrigger size="pill" aria-label={t("monitor_pick_exam")}>
                <SelectValue placeholder={t("monitor_pick_exam")} />
              </SelectTrigger>
              <SelectContent>
                {availableExams.map((e) => (
                  <SelectItem key={e.id} value={e.id}>
                    {e.title}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
          ) : undefined
        }
      />
      {availableSection}
      <Dialog open={Boolean(selectedExamId)} onOpenChange={(open) => !open && setSelectedExamId("")}>
        <DialogContent className="sm:max-w-6xl">
          <DialogHeader>
            <div className="flex flex-wrap items-center gap-3 pr-8">
              <DialogTitle>{examTitle || t("exam_monitor_title")}</DialogTitle>
              {selectedCandidate && (
                <Badge variant={AVAILABLE_STATE_BADGE[selectedCandidate.state].variant}>
                  {t(AVAILABLE_STATE_BADGE[selectedCandidate.state].labelKey as any)}
                </Badge>
              )}
            </div>
            <DialogDescription>
              {selectedCandidate
                ? formatWindow(selectedCandidate.scheduled_at, selectedCandidate.scheduled_end_at)
                : t("exam_monitor_subtitle")}
            </DialogDescription>
          </DialogHeader>
          {detailSection}
        </DialogContent>
      </Dialog>
    </div>
  );
}
