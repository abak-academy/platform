"use client";

import { Building, Users, UserPlus, Calendar, Upload, FileText } from "lucide-react";
import { useAuthStore } from "@/stores/auth";
import { useMe } from "@/lib/hooks/auth";
import { useSchoolDashboard } from "@/lib/hooks/admin-dashboard";
import { useHasCapability } from "@/lib/hooks/use-capability";
import { NoAccess } from "@/components/admin/NoAccess";
import { MonitorCard } from "@/components/admin/MonitorCard";
import { DashboardHero } from "@/components/admin/DashboardHero";
import { QuickActionTiles, type QuickAction } from "@/components/admin/QuickActionTiles";
import { Skeleton } from "@/components/ui/skeleton";
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

  const formatResultDate = (iso: string) =>
    new Intl.DateTimeFormat(lang === "en" ? "en-US" : "id-ID", {
      day: "numeric", month: "short", timeZone: "Asia/Jakarta",
    }).format(new Date(iso));

  return (
    <div className="fade-in">
      <DashboardHero
        icon={Building}
        badge={t("school_home_badge")}
        name={name}
        subtitle={t("school_home_subtitle")}
      />

      {isError ? (
        <div className="md-card-outlined mb-8">
          <div className="py-12 text-center text-ink-500">
            <p className="mb-4">{t("dash_load_failed")}</p>
            <button type="button" className="md-btn-tonal" onClick={() => refetch()}>
              {t("admin_home_reload")}
            </button>
          </div>
        </div>
      ) : isLoading ? (
        <div className="mb-8 grid grid-cols-1 gap-6 sm:grid-cols-2 xl:grid-cols-3">
          {[...Array(3)].map((_, i) => <Skeleton key={i} className="h-28 w-full" />)}
        </div>
      ) : (
        <>
          <div className="mb-8 grid grid-cols-1 gap-6 sm:grid-cols-2 xl:grid-cols-3">
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

          <div className="mb-8 grid grid-cols-1 gap-6 lg:grid-cols-2">
            <div className="md-card-outlined" data-testid="school-latest-bulk-order">
              <h3 className="text-title-large mb-4">{t("school_home_bulk_title")}</h3>
              {!data?.latest_bulk_order ? (
                <div className="py-12 text-center text-ink-500">{t("school_home_bulk_empty")}</div>
              ) : (
                <div className="space-y-2">
                  <div className="text-body" style={{ fontWeight: 500 }}>{data.latest_bulk_order.status}</div>
                  <div className="text-label color-on-surface-variant">
                    {t("school_home_bulk_participants").replace("{count}", String(data.latest_bulk_order.participant_count))}
                  </div>
                  <div className="text-label color-on-surface-variant">{formatRupiah(data.latest_bulk_order.total)}</div>
                  <div className="text-label color-on-surface-variant">{formatDateTime(data.latest_bulk_order.placed_at)}</div>
                </div>
              )}
            </div>

            <div className="md-card-outlined">
              <h3 className="text-title-large mb-4">{t("school_home_results")}</h3>
              {!data?.recent_results.length ? (
                <div className="py-12 text-center text-ink-500">{t("school_home_results_empty")}</div>
              ) : (
                <div className="space-y-3">
                  {data.recent_results.map((r) => (
                    <div
                      key={r.session_id}
                      data-testid={`school-result-${r.session_id}`}
                      className="flex items-center justify-between gap-3 rounded-[12px] p-3"
                      style={{ backgroundColor: "var(--md-sys-color-surface-container-high)" }}
                    >
                      <div>
                        <div className="text-body" style={{ fontWeight: 500 }}>{r.student_name}</div>
                        <div className="text-label color-on-surface-variant">
                          {r.exam_title} · {formatResultDate(r.submitted_at)}
                        </div>
                      </div>
                      <div
                        className="text-label"
                        style={{ color: r.score === null ? "var(--md-sys-color-outline)" : "var(--md-sys-color-primary)" }}
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
