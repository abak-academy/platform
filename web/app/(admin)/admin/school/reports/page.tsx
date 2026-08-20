"use client";

import { useEffect, useRef, useState } from "react";
import {
  FileText,
  Search,
  Download,
  Loader2,
  Eye,
} from "lucide-react";
import { useTranslation } from "@/lib/i18n";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { DataTable, type DataTableColumn } from "@/components/ui/data-table";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogFooter,
} from "@/components/ui/dialog";
import { AdminPageHeader } from "@/components/admin/AdminPageHeader";
import { useProducts } from "@/lib/hooks/products";
import {
  useAdminResults,
  useAdminResultDetail,
  exportAdminResults,
} from "@/lib/hooks/admin-results";
import { useSchoolOptions } from "@/lib/hooks/admin-schools";
import { useAuthStore } from "@/stores/auth";
import { RichContent } from "@/components/admin/RichContent";
import { formatChoiceAnswer } from "@/lib/option-key";
import type { AdminResultRow, AdminResultDetail, ProductType } from "@/lib/types";

export default function SchoolReportsPage() {
  const { t, lang } = useTranslation();
  const dateLocale = lang === "en" ? "en-US" : "id-ID";

  // Role-gated school picker (super_admin only)
  const currentRole = useAuthStore((s) => s.user?.role);
  const isSuperAdmin = currentRole === "super_admin";
  const { data: schoolsData, isLoading: schoolsLoading } = useSchoolOptions();
  const [selectedSchoolId, setSelectedSchoolId] = useState<string>("");

  // Exam picker
  const { data: examProducts = [], isLoading: examsLoading } = useProducts("exam" as ProductType);
  const [selectedExamId, setSelectedExamId] = useState<string>("");

  // Search
  const [search, setSearch] = useState("");

  // Cursor pagination
  const [accumulated, setAccumulated] = useState<AdminResultRow[]>([]);
  const [activeCursor, setActiveCursor] = useState<string | undefined>(undefined);
  const [nextCursor, setNextCursor] = useState<string | undefined>(undefined);

  // Guard: reset pagination on filter change
  const filterKey = `${selectedExamId}:${search}:${selectedSchoolId}`;
  const pageFilterKeyRef = useRef(filterKey);

  useEffect(() => {
    if (filterKey !== pageFilterKeyRef.current) {
      setAccumulated([]);
      setActiveCursor(undefined);
      setNextCursor(undefined);
      pageFilterKeyRef.current = filterKey;
    }
  }, [filterKey]);

  const query = useAdminResults({
    examId: selectedExamId,
    q: search || undefined,
    cursor: activeCursor,
    limit: 20,
    ...(isSuperAdmin && selectedSchoolId ? { schoolId: selectedSchoolId } : {}),
    enabled: Boolean(selectedExamId) && (!isSuperAdmin || Boolean(selectedSchoolId)),
  });

  // Accumulate pages as they arrive
  useEffect(() => {
    if (!query.data || !selectedExamId) return;
    if (filterKey !== pageFilterKeyRef.current) return;

    setAccumulated((prev) => {
      const data = query.data!.data ?? [];
      if (activeCursor === undefined) return data;
      const ids = new Set(prev.map((s) => s.session_id));
      const fresh = data.filter((s) => !ids.has(s.session_id));
      return [...prev, ...fresh];
    });
    setNextCursor(query.data.next_cursor);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [query.data, selectedExamId]);

  // Drill-down dialog
  const [selectedSessionId, setSelectedSessionId] = useState<string>("");
  const detailResult = useAdminResultDetail(
    selectedSessionId,
    isSuperAdmin ? selectedSchoolId : undefined,
  );

  // Export
  const [exporting, setExporting] = useState(false);

  const handleRowClick = (sessionId: string) => {
    setSelectedSessionId(sessionId);
  };

  const handleCloseDetail = () => {
    setSelectedSessionId("");
  };

  const handleExport = async () => {
    if (!selectedExamId) return;
    setExporting(true);
    try {
      await exportAdminResults(selectedExamId, isSuperAdmin ? selectedSchoolId : undefined);
    } catch {
      // Export errors handled silently — the CSV download is best-effort
    } finally {
      setExporting(false);
    }
  };

  const handleLoadMore = () => {
    if (nextCursor) {
      setActiveCursor(nextCursor);
    }
  };

  const reportColumns: DataTableColumn<AdminResultRow>[] = [
    {
      key: "name",
      header: t("th_name"),
      cell: (row) => <span className="font-medium text-ink-900">{row.student_name}</span>,
    },
    {
      key: "username",
      header: t("school_reports_col_username"),
      cell: (row) => <span className="text-xs text-ink-600">{row.username ?? "-"}</span>,
    },
    {
      key: "score",
      header: t("school_reports_col_score"),
      cell: (row) => <span className="text-xs text-ink-600">{row.score}</span>,
    },
    {
      key: "submitted",
      header: t("school_reports_col_submitted"),
      cell: (row) => (
        <span className="text-xs text-ink-600">
          {new Date(row.submitted_at).toLocaleString(dateLocale, {
            day: "2-digit",
            month: "short",
            year: "numeric",
          })}
        </span>
      ),
    },
    {
      key: "actions",
      header: t("th_actions"),
      align: "right",
      cell: (row) => (
        <Button
          variant="ghost"
          size="icon"
          aria-label={t("action_view")}
          onClick={() => handleRowClick(row.session_id)}
        >
          <Eye className="size-4" />
        </Button>
      ),
    },
  ];

  // No exam selected — show empty state
  if (!selectedExamId) {
    return (
      <div className="mx-auto max-w-6xl px-4 py-8 md:px-6 md:py-10 fade-in">
        <AdminPageHeader
          icon={FileText}
          title={t("school_reports_title")}
          description={t("school_reports_subtitle")}
          actions={
            <Button
              size="sm"
              disabled={true}
            >
              <Download className="mr-1 size-4" />
              {t("school_reports_export")}
            </Button>
          }
        />
        {isSuperAdmin && (
          <SchoolPicker
            schoolsData={schoolsData?.data ?? []}
            selectedSchoolId={selectedSchoolId}
            onSelect={setSelectedSchoolId}
            isLoading={schoolsLoading}
            label={t("select_school")}
          />
        )}
        <ExamPicker
          examProducts={examProducts}
          selectedExamId={selectedExamId}
          onSelect={setSelectedExamId}
          examsLoading={examsLoading}
          label={t("school_reports_select_exam")}
        />
        <div className="py-12 text-center text-ink-500">
          {t("school_reports_no_exam")}
        </div>
      </div>
    );
  }

  // Loading state (first page only)
  if (query.isLoading && accumulated.length === 0) {
    return (
      <div className="mx-auto max-w-6xl px-4 py-8 md:px-6 md:py-10 fade-in">
        <AdminPageHeader
          icon={FileText}
          title={t("school_reports_title")}
          description={t("school_reports_subtitle")}
          actions={
            <Button
              size="sm"
              onClick={handleExport}
              disabled={exporting || !selectedExamId}
            >
              {exporting ? (
                <Loader2 className="mr-1 size-4 animate-spin" />
              ) : (
                <Download className="mr-1 size-4" />
              )}
              {exporting ? t("school_reports_export_loading") : t("school_reports_export")}
            </Button>
          }
        />
        {isSuperAdmin && (
          <SchoolPicker
            schoolsData={schoolsData?.data ?? []}
            selectedSchoolId={selectedSchoolId}
            onSelect={setSelectedSchoolId}
            isLoading={schoolsLoading}
            label={t("select_school")}
          />
        )}
        <ExamPicker
          examProducts={examProducts}
          selectedExamId={selectedExamId}
          onSelect={setSelectedExamId}
          examsLoading={examsLoading}
          label={t("school_reports_select_exam")}
        />
        <div className="py-12 text-center text-ink-500">
          {t("sys_loading_data")}
        </div>
      </div>
    );
  }

  if (query.error && accumulated.length === 0) {
    return (
      <div className="mx-auto max-w-6xl px-4 py-8 md:px-6 md:py-10 fade-in">
        <AdminPageHeader
          icon={FileText}
          title={t("school_reports_title")}
          description={t("school_reports_subtitle")}
          actions={
            <Button
              size="sm"
              onClick={handleExport}
              disabled={exporting || !selectedExamId}
            >
              {exporting ? (
                <Loader2 className="mr-1 size-4 animate-spin" />
              ) : (
                <Download className="mr-1 size-4" />
              )}
              {exporting ? t("school_reports_export_loading") : t("school_reports_export")}
            </Button>
          }
        />
        {isSuperAdmin && (
          <SchoolPicker
            schoolsData={schoolsData?.data ?? []}
            selectedSchoolId={selectedSchoolId}
            onSelect={setSelectedSchoolId}
            isLoading={schoolsLoading}
            label={t("select_school")}
          />
        )}
        <ExamPicker
          examProducts={examProducts}
          selectedExamId={selectedExamId}
          onSelect={setSelectedExamId}
          examsLoading={examsLoading}
          label={t("school_reports_select_exam")}
        />
        <div className="py-12 text-center text-ink-500">
          {t("sys_error_load")}
        </div>
      </div>
    );
  }

  return (
    <div className="mx-auto max-w-6xl px-4 py-8 md:px-6 md:py-10 fade-in">
      <AdminPageHeader
        icon={FileText}
        title={t("school_reports_title")}
        description={t("school_reports_subtitle")}
        actions={
          <Button
            size="sm"
            onClick={handleExport}
            disabled={exporting || !selectedExamId}
          >
            {exporting ? (
              <Loader2 className="mr-1 size-4 animate-spin" />
            ) : (
              <Download className="mr-1 size-4" />
            )}
            {exporting ? t("school_reports_export_loading") : t("school_reports_export")}
          </Button>
        }
      />

      {isSuperAdmin && (
        <SchoolPicker
          schoolsData={schoolsData?.data ?? []}
          selectedSchoolId={selectedSchoolId}
          onSelect={setSelectedSchoolId}
          isLoading={schoolsLoading}
          label={t("select_school")}
        />
      )}

      <ExamPicker
        examProducts={examProducts}
        selectedExamId={selectedExamId}
        onSelect={setSelectedExamId}
        examsLoading={examsLoading}
        label={t("school_reports_select_exam")}
      />

      {/* Search */}
      <div className="mb-4 flex items-center gap-2">
        <Search className="size-4 text-ink-400" />
        <Input
          value={search}
          onChange={(e) => setSearch(e.target.value)}
          placeholder={t("students_search_placeholder")}
          className="h-9 w-[220px] text-xs"
        />
      </div>

      {/* Results Table */}
      <DataTable
        data-testid="school-reports-table"
        columns={reportColumns}
        rows={accumulated}
        rowKey={(row) => row.session_id}
        empty={t("school_reports_empty")}
        footer={
          nextCursor && (
            <div className="border-t border-line px-4 py-3 text-center">
              <Button
                variant="outline"
                size="sm"
                onClick={handleLoadMore}
                disabled={query.isFetching}
              >
                {query.isFetching ? t("sys_loading") : t("load_more")}
              </Button>
            </div>
          )
        }
      />

      {/* Drill-down dialog */}
      <Dialog
        open={selectedSessionId !== ""}
        onOpenChange={(open) => {
          if (!open) handleCloseDetail();
        }}
      >
        <DialogContent className="sm:max-w-lg">
          <DialogHeader>
            <DialogTitle className="font-serif">
              {t("school_reports_detail_title")}
            </DialogTitle>
          </DialogHeader>
          {detailResult.isLoading ? (
            <div className="py-8 text-center text-ink-500">
              {t("sys_loading_data")}
            </div>
          ) : detailResult.data ? (
            <ResultDetailContent
              detail={detailResult.data}
              t={t as unknown as (key: string) => string}
              dateLocale={dateLocale}
            />
          ) : null}
          <DialogFooter className="mt-4">
            <Button onClick={handleCloseDetail}>
              {t("cancel")}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  );
}

function SchoolPicker({
  schoolsData,
  selectedSchoolId,
  onSelect,
  isLoading,
  label,
}: {
  schoolsData: { id: string; name: string }[];
  selectedSchoolId: string;
  onSelect: (id: string) => void;
  isLoading: boolean;
  label: string;
}) {
  if (isLoading) {
    return (
      <div className="mb-4">
        <p className="text-xs text-ink-500">{label}</p>
        <div className="mt-1 h-9 w-[240px] animate-pulse rounded-md bg-surface-2" />
      </div>
    );
  }

  return (
    <div className="mb-4">
      <p className="text-xs text-ink-500">{label}</p>
      <Select value={selectedSchoolId} onValueChange={onSelect}>
        <SelectTrigger className="mt-1 h-9 w-[240px] text-xs" aria-label={label}>
          <SelectValue placeholder={label} />
        </SelectTrigger>
        <SelectContent>
          {schoolsData.map((s) => (
            <SelectItem key={s.id} value={s.id}>
              {s.name}
            </SelectItem>
          ))}
        </SelectContent>
      </Select>
    </div>
  );
}

function ExamPicker({
  examProducts,
  selectedExamId,
  onSelect,
  examsLoading,
  label,
}: {
  examProducts: { id: string; name: string }[];
  selectedExamId: string;
  onSelect: (id: string) => void;
  examsLoading: boolean;
  label: string;
}) {
  if (examsLoading) {
    return (
      <div className="mb-6">
        <p className="text-xs text-ink-500">{label}</p>
        <div className="mt-1 h-9 w-[240px] animate-pulse rounded-md bg-surface-2" />
      </div>
    );
  }

  return (
    <div className="mb-6">
      <p className="text-xs text-ink-500">{label}</p>
      <Select value={selectedExamId} onValueChange={onSelect}>
        <SelectTrigger className="mt-1 h-9 w-[240px] text-xs">
          <SelectValue placeholder={label} />
        </SelectTrigger>
        <SelectContent>
          {examProducts.map((p) => (
            <SelectItem key={p.id} value={p.id}>
              {p.name}
            </SelectItem>
          ))}
        </SelectContent>
      </Select>
    </div>
  );
}

function ResultDetailContent({
  detail,
  t,
  dateLocale,
}: {
  detail: AdminResultDetail;
  t: (key: string) => string;
  dateLocale: string;
}) {
  return (
    <div className="space-y-4">
      {/* Student info */}
      <div className="text-sm text-ink-600">
        <p><span className="font-semibold text-ink-900">{detail.student_name}</span> · Username: {detail.username ?? "-"}</p>
      </div>

      {/* Score summary */}
      <div className="grid grid-cols-4 gap-2 text-center text-xs">
        <div className="rounded-lg bg-surface-2 p-2">
          <div className="text-lg font-bold text-ink-900">{detail.score}</div>
          <div className="text-ink-500">{t("school_reports_detail_score")}</div>
        </div>
        <div className="rounded-lg bg-success-bg p-2">
          <div className="text-lg font-bold text-success">{detail.correct_count}</div>
          <div className="text-ink-500">{t("school_reports_detail_correct")}</div>
        </div>
        <div className="rounded-lg bg-danger-bg p-2">
          <div className="text-lg font-bold text-danger">{detail.wrong_count}</div>
          <div className="text-ink-500">{t("school_reports_detail_wrong")}</div>
        </div>
        <div className="rounded-lg bg-surface-2 p-2">
          <div className="text-lg font-bold text-ink-900">{detail.empty_count}</div>
          <div className="text-ink-500">{t("school_reports_detail_empty")}</div>
        </div>
      </div>

      {/* Submitted date */}
      <div className="text-xs text-ink-500">
        {t("school_reports_col_submitted")}:{" "}
        {new Date(detail.submitted_at).toLocaleString(dateLocale, {
          day: "2-digit",
          month: "short",
          year: "numeric",
        })}
      </div>

      {/* Breakdown — only for score_pembahasan */}
      {detail.result_config === "score_pembahasan" && detail.breakdown && (
        <div>
          <h4 className="mb-2 text-sm font-semibold text-ink-900">
            {t("result_by_topic")}
          </h4>
          <div className="space-y-1">
            {detail.breakdown.map((b) => (
              <div
                key={b.test_id}
                className="flex items-center justify-between rounded-md bg-surface-2 px-3 py-2 text-xs"
              >
                <span className="text-ink-700">{b.title}</span>
                <span className="font-semibold text-ink-900">
                  {b.earned}/{b.max}
                </span>
              </div>
            ))}
          </div>
        </div>
      )}

      {/* Pembahasan — only for score_pembahasan */}
      {detail.result_config === "score_pembahasan" && detail.pembahasan && (
        <div>
          <h4 className="mb-2 text-sm font-semibold text-ink-900">
            {t("result_pembahasan")}
          </h4>
          <div className="space-y-2 max-h-64 overflow-y-auto">
            {detail.pembahasan.map((p) => (
              <div
                key={p.question_id}
                className="rounded-md border border-line bg-surface-2 px-3 py-2 text-xs"
              >
                <p className="font-medium text-ink-900">
                  <RichContent html={p.body} />
                </p>
                <p className="mt-1 text-ink-600">
                  {t("result_your_answer")}:{" "}
                  {formatChoiceAnswer(p.your_answer, p.format) || "—"}
                </p>
                <p className="text-ink-600">
                  {t("result_correct_answer")}:{" "}
                  {formatChoiceAnswer(p.correct_answer, p.format) || "—"}
                </p>
                {p.explanation && (
                  <p className="mt-1 text-ink-500 italic">{p.explanation}</p>
                )}
              </div>
            ))}
          </div>
        </div>
      )}
    </div>
  );
}
