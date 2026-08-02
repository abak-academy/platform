"use client";

import { useEffect, useRef, useState } from "react";
import { Download, Loader2 } from "lucide-react";
import { useTranslation } from "@/lib/i18n";
import { Button } from "@/components/ui/button";
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
import {
  useAdminResults,
  useAdminResultDetail,
  exportAdminResults,
} from "@/lib/hooks/admin-results";
import { useAdminSchools } from "@/lib/hooks/admin-schools";
import { useAuthStore } from "@/stores/auth";
import type { AdminResultRow, AdminResultDetail } from "@/lib/types";

interface ExamResultsTabProps {
  examId: string;
}

// Radix Select forbids an empty-string item value, so "every school" needs its
// own sentinel; it maps back to "" (no school_id param, meaning "all schools").
const ALL_SCHOOLS_VALUE = "_all_";

export function ExamResultsTab({ examId }: ExamResultsTabProps) {
  const { t, lang } = useTranslation();
  const dateLocale = lang === "en" ? "en-US" : "id-ID";

  const role = useAuthStore((s) => s.user?.role);
  const isSuperAdmin = role === "super_admin";

  const { data: schoolsData } = useAdminSchools();
  const [selectedSchoolId, setSelectedSchoolId] = useState<string>("");

  const [search, setSearch] = useState("");
  const [accumulated, setAccumulated] = useState<AdminResultRow[]>([]);
  const [activeCursor, setActiveCursor] = useState<string | undefined>(undefined);
  const [nextCursor, setNextCursor] = useState<string | undefined>(undefined);

  const filterKey = `${examId}:${search}:${selectedSchoolId}`;
  const filterKeyRef = useRef(filterKey);

  useEffect(() => {
    if (filterKey !== filterKeyRef.current) {
      setAccumulated([]);
      setActiveCursor(undefined);
      setNextCursor(undefined);
      filterKeyRef.current = filterKey;
    }
  }, [filterKey]);

  const query = useAdminResults({
    examId,
    q: search || undefined,
    cursor: activeCursor,
    limit: 20,
    ...(isSuperAdmin && selectedSchoolId ? { schoolId: selectedSchoolId } : {}),
    enabled: Boolean(examId),
  });

  useEffect(() => {
    if (!query.data) return;
    if (filterKey !== filterKeyRef.current) return;

    setAccumulated((prev) => {
      const rows = query.data!.data ?? [];
      if (activeCursor === undefined) return rows;
      const ids = new Set(prev.map((r) => r.session_id));
      const fresh = rows.filter((r) => !ids.has(r.session_id));
      return [...prev, ...fresh];
    });
    setNextCursor(query.data.next_cursor);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [query.data]);

  const [selectedSessionId, setSelectedSessionId] = useState<string>("");
  const detailResult = useAdminResultDetail(
    selectedSessionId,
    isSuperAdmin ? selectedSchoolId : undefined,
  );

  const [exporting, setExporting] = useState(false);
  const handleExport = async () => {
    if (!examId) return;
    setExporting(true);
    try {
      await exportAdminResults(examId, isSuperAdmin ? selectedSchoolId : undefined);
    } catch {
      // Export errors handled silently — the CSV download is best-effort
    } finally {
      setExporting(false);
    }
  };

  const handleLoadMore = () => {
    if (nextCursor) setActiveCursor(nextCursor);
  };

  return (
    <div className="space-y-4">
      <div className="flex flex-wrap items-end justify-between gap-3">
        {isSuperAdmin && (
          <div>
            <p className="text-xs text-ink-500">{t("select_school")}</p>
            <Select
              value={selectedSchoolId || ALL_SCHOOLS_VALUE}
              onValueChange={(v) => setSelectedSchoolId(v === ALL_SCHOOLS_VALUE ? "" : v)}
            >
              <SelectTrigger className="mt-1 h-9 w-[240px] text-xs" aria-label={t("select_school")}>
                <SelectValue placeholder={t("students_all_schools")} />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value={ALL_SCHOOLS_VALUE}>{t("students_all_schools")}</SelectItem>
                {(schoolsData?.data ?? []).map((s) => (
                  <SelectItem key={s.id} value={s.id}>
                    {s.name}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
          </div>
        )}
        <Button
          size="sm"
          onClick={handleExport}
          disabled={exporting || !examId}
        >
          {exporting ? (
            <Loader2 className="mr-1 size-4 animate-spin" />
          ) : (
            <Download className="mr-1 size-4" />
          )}
          {exporting ? t("school_reports_export_loading") : t("school_reports_export")}
        </Button>
      </div>

      {query.isLoading && accumulated.length === 0 ? (
        <div className="py-12 text-center text-ink-500">{t("sys_loading_data")}</div>
      ) : query.isError && accumulated.length === 0 ? (
        <div className="py-12 text-center text-ink-500">{t("sys_error_load")}</div>
      ) : (
        <div className="md-card-outlined">
          <div className="overflow-x-auto">
            <table className="w-full text-sm">
              <thead className="bg-surface-2 text-left text-xs font-semibold text-ink-600">
                <tr>
                  <th className="px-4 py-3">{t("th_name")}</th>
                  <th className="px-4 py-3">{t("schools_th_school")}</th>
                  <th className="px-4 py-3">{t("school_reports_col_score")}</th>
                  <th className="px-4 py-3">{t("school_reports_col_submitted")}</th>
                </tr>
              </thead>
              <tbody className="divide-y divide-line">
                {accumulated.length === 0 && (
                  <tr>
                    <td colSpan={4} className="px-4 py-8 text-center text-sm text-ink-500">
                      {t("school_reports_empty")}
                    </td>
                  </tr>
                )}
                {accumulated.map((row) => (
                  <tr
                    key={row.session_id}
                    className="cursor-pointer group hover:bg-surface-2"
                    onClick={() => setSelectedSessionId(row.session_id)}
                  >
                    <td className="px-4 py-3 font-medium text-ink-900">{row.student_name}</td>
                    <td className="px-4 py-3 text-xs text-ink-600">{row.school_name || "-"}</td>
                    <td className="px-4 py-3 text-xs text-ink-600">{row.score}</td>
                    <td className="px-4 py-3 text-xs text-ink-600">
                      {new Date(row.submitted_at).toLocaleString(dateLocale, {
                        day: "2-digit",
                        month: "short",
                        year: "numeric",
                      })}
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>

          {nextCursor && (
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
          )}
        </div>
      )}

      <Dialog
        open={selectedSessionId !== ""}
        onOpenChange={(open) => {
          if (!open) setSelectedSessionId("");
        }}
      >
        <DialogContent className="sm:max-w-lg">
          <DialogHeader>
            <DialogTitle className="font-serif">{t("school_reports_detail_title")}</DialogTitle>
          </DialogHeader>
          {detailResult.isLoading ? (
            <div className="py-8 text-center text-ink-500">{t("sys_loading_data")}</div>
          ) : detailResult.data ? (
            <ResultDetailContent
              detail={detailResult.data}
              t={t as unknown as (key: string) => string}
              dateLocale={dateLocale}
            />
          ) : null}
          <DialogFooter className="mt-4">
            <Button onClick={() => setSelectedSessionId("")}>{t("cancel")}</Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
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
      <div className="text-sm text-ink-600">
        <p>
          <span className="font-semibold text-ink-900">{detail.student_name}</span> · Username:{" "}
          {detail.username ?? "-"}
        </p>
      </div>

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

      <div className="text-xs text-ink-500">
        {t("school_reports_col_submitted")}:{" "}
        {new Date(detail.submitted_at).toLocaleString(dateLocale, {
          day: "2-digit",
          month: "short",
          year: "numeric",
        })}
      </div>

      {detail.breakdown && detail.breakdown.length > 0 && (
        <div>
          <h4 className="mb-2 text-sm font-semibold text-ink-900">{t("result_by_topic")}</h4>
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
    </div>
  );
}
