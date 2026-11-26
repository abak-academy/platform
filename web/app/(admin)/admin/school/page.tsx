"use client";

import { Users, UserPlus, Calendar, Upload, FileText, ReceiptText, Trophy, RefreshCw } from "lucide-react";
import { useAuthStore } from "@/stores/auth";
import { useMe } from "@/lib/hooks/auth";
import { useSchoolDashboard } from "@/lib/hooks/admin-dashboard";
import { useHasCapability } from "@/lib/hooks/use-capability";
import { NoAccess } from "@/components/admin/NoAccess";
import { MonitorCard } from "@/components/admin/MonitorCard";
import { DashboardHero } from "@/components/admin/DashboardHero";
import { QuickActionTiles, type QuickAction } from "@/components/admin/QuickActionTiles";
import { Skeleton } from "@/components/ui/skeleton";
import { Badge, statusTone } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { EmptyState } from "@/components/ui/empty-state";
import { useTranslation } from "@/lib/i18n";
import { formatRupiah } from "@/lib/format";
import type { UserRole } from "@/lib/nav-config";

export default function SchoolDashboardPage() {
  const { t, lang } = useTranslation();
  const user = useAuthStore((s) => s.user);
  const storeRole = user?.role as UserRole | undefined;
  const me = useMe({ enabled: !storeRole });
  const name = user?.name ?? me.data?.name ?? t("school_home_default_name");
  const canRead = useHasCapability("students:read");

  const { data, isLoading, isError, refetch } = useSchoolDashboard();

  const quickActions: QuickAction[] = [
    { icon: Users, label: t("school_action_students"), href: "/admin/school/students" },
    { icon: Upload, label: t("school_action_import"), href: "/admin/school/students" },
    { icon: Calendar, label: t("school_action_bulk_order"), href: "/admin/exam/packages" },
    { icon: FileText, label: t("school_action_reports"), href: "/admin/school/reports" },
  ];

  if (!canRead) return <NoAccess />;

  const formatDateTime = (iso: string) =>
    new Intl.DateTimeFormat(lang === "en" ? "en-US" : "id-ID", {
      day: "numeric", month: "short", hour: "2-digit", minute: "2-digit", timeZone: "Asia/Jakarta",
    }).format(new Date(iso));

  return (
    <div className="fade-in flex flex-col gap-4">
      <DashboardHero
        badge={t("school_home_badge")}
        name={name}
        subtitle={t("school_home_subtitle")}
      />

      {isError ? (
        <EmptyState
          framed
          icon={RefreshCw}
          title={t("dash_load_failed")}
          action={<Button variant="outline" onClick={() => refetch()}>{t("admin_home_reload")}</Button>}
        />
      ) : isLoading ? (
        <div className="grid grid-cols-2 gap-3 xl:grid-cols-3">
          {[...Array(3)].map((_, i) => <Skeleton key={i} className="h-28 w-full rounded-[var(--radius-card)]" />)}
        </div>
      ) : (
        <>
          <div className="grid grid-cols-2 gap-3 xl:grid-cols-3">
            <MonitorCard
              testId="school-students"
              href="/admin/school/students"
              label={t("school_home_students")}
              value={data?.counts.students ?? 0}
              icon={Users}
            />
            <MonitorCard
              testId="school-new-students"
              href="/admin/school/students"
              label={t("school_home_new_students")}
              value={data?.counts.new_students_month ?? 0}
              icon={UserPlus}
            />
            <MonitorCard
              testId="school-orderable-exams"
              href="/admin/exam/packages"
              label={t("school_home_orderable_exams")}
              value={data?.orderable_exam_count ?? 0}
              icon={Calendar}
            />
          </div>

          <div className="grid grid-cols-1 gap-4 lg:grid-cols-2">
            <div className="md-card-outlined" data-testid="school-latest-bulk-order">
              <h2 className="type-h2 mb-3.5">{t("school_home_bulk_title")}</h2>
              {!data?.latest_bulk_order ? (
                <EmptyState icon={ReceiptText} title={t("school_home_bulk_empty")} />
              ) : (
                <div className="space-y-2">
                  <Badge variant={statusTone(data.latest_bulk_order.status)}>{data.latest_bulk_order.status}</Badge>
                  <div className="text-sm text-ink-600">
                    {t("school_home_bulk_participants").replace("{count}", String(data.latest_bulk_order.participant_count))}
                  </div>
                  <div className="text-sm text-ink-600">{formatRupiah(data.latest_bulk_order.total)}</div>
                  <div className="text-sm text-ink-600">{formatDateTime(data.latest_bulk_order.placed_at)}</div>
                </div>
              )}
            </div>

            <div className="md-card-outlined">
              <h2 className="type-h2 mb-3.5">{t("school_home_results")}</h2>
              {!data?.recent_results.length ? (
                <EmptyState icon={Trophy} title={t("school_home_results_empty")} />
              ) : (
                <div className="space-y-2">
                  {data.recent_results.map((r) => (
                    <div
                      key={r.session_id}
                      data-testid={`school-result-${r.session_id}`}
                      className="flex items-center justify-between gap-3 rounded-[20px] border border-line px-4 py-3.5"
                    >
                      <div>
                        <div className="font-bold text-ink-900">{r.student_name}</div>
                        <div className="text-xs text-ink-600">
                          {r.exam_title} · {formatDateTime(r.submitted_at)}
                        </div>
                      </div>
                      <div
                        data-testid={`school-result-${r.session_id}-score`}
                        className={r.score === null ? "text-sm text-ink-600" : "font-[family-name:var(--font-display)] text-xl font-extrabold text-ink-900"}
                      >
                        {r.score === null ? t("school_home_ungraded") : String(r.score)}
                      </div>
                    </div>
                  ))}
                </div>
              )}
            </div>
          </div>
        </>
      )}

      <QuickActionTiles title={t("admin_home_quick_actions")} actions={quickActions} />
    </div>
  );
}
