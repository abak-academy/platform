"use client";

import {
  Activity, Library, ClipboardList, Calendar, BookOpen, AlertTriangle, ShieldAlert,
} from "lucide-react";
import { useAuthStore } from "@/stores/auth";
import { useMe } from "@/lib/hooks/auth";
import { useExamDashboard } from "@/lib/hooks/admin-dashboard";
import { useHasCapability } from "@/lib/hooks/use-capability";
import { NoAccess } from "@/components/admin/NoAccess";
import { MonitorCard } from "@/components/admin/MonitorCard";
import { DashboardHero } from "@/components/admin/DashboardHero";
import { QuickActionTiles, type QuickAction } from "@/components/admin/QuickActionTiles";
import { Skeleton } from "@/components/ui/skeleton";
import { useTranslation } from "@/lib/i18n";
import type { UserRole } from "@/lib/nav-config";

export default function ExamDashboardPage() {
  const { t, lang } = useTranslation();
  const user = useAuthStore((s) => s.user);
  const storeRole = user?.role as UserRole | undefined;
  const me = useMe({ enabled: !storeRole });
  const name = user?.name ?? me.data?.name ?? t("exam_home_default_name");
  const canRead = useHasCapability("sessions:read");

  const { data, isLoading, isError, refetch } = useExamDashboard();

  const quickActions: QuickAction[] = [
    { icon: Library, label: t("exam_action_new_question"), href: "/admin/exam/questions" },
    { icon: ClipboardList, label: t("exam_action_new_test"), href: "/admin/exam/tests" },
    { icon: Calendar, label: t("exam_action_new_package"), href: "/admin/exam/packages" },
    { icon: Activity, label: t("exam_action_monitor"), href: "/admin/exam/monitor" },
  ];

  if (!canRead) return <NoAccess />;

  const formatDateTime = (iso: string) =>
    new Intl.DateTimeFormat(lang === "en" ? "en-US" : "id-ID", {
      day: "numeric", month: "short", hour: "2-digit", minute: "2-digit", timeZone: "Asia/Jakarta",
    }).format(new Date(iso));

  return (
    <div className="fade-in">
      <DashboardHero
        icon={ShieldAlert}
        badge={t("exam_home_badge")}
        name={name}
        subtitle={t("exam_home_subtitle")}
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
        <div className="mb-8 grid grid-cols-1 gap-6 sm:grid-cols-2 xl:grid-cols-5">
          {[...Array(5)].map((_, i) => <Skeleton key={i} className="h-28 w-full" />)}
        </div>
      ) : (
        <>
          <div className="mb-8 grid grid-cols-1 gap-6 sm:grid-cols-2 xl:grid-cols-5">
            <MonitorCard
              testId="exam-active-sessions"
              href="/admin/exam/monitor"
              label={t("exam_home_active_sessions")}
              value={data?.active_sessions ?? 0}
              icon={Activity}
              accent={(data?.active_sessions ?? 0) > 0 ? "tertiary" : "secondary"}
            />
            <MonitorCard
              testId="exam-questions"
              href="/admin/exam/questions"
              label={t("exam_home_questions")}
              value={data?.counts.questions ?? 0}
              icon={Library}
            />
            <MonitorCard
              testId="exam-tests"
              href="/admin/exam/tests"
              label={t("exam_home_tests")}
              value={data?.counts.tests ?? 0}
              icon={ClipboardList}
            />
            <MonitorCard
              testId="exam-exams"
              href="/admin/exam/packages"
              label={t("exam_home_exams")}
              value={data?.counts.exams ?? 0}
              icon={Calendar}
            />
            {/* admin_exam also holds products(course):* and sections:* in rbac.go */}
            <MonitorCard
              testId="exam-courses"
              href="/admin/courses"
              label={t("exam_home_courses")}
              value={data?.counts.courses ?? 0}
              icon={BookOpen}
            />
          </div>

          <div className="mb-8 grid grid-cols-1 gap-6 lg:grid-cols-2">
            <div className="md-card-outlined">
              <h3 className="text-title-large mb-4">{t("admin_home_upcoming_exams")}</h3>
              {!data?.upcoming_exams.length ? (
                <div className="py-12 text-center text-ink-500">{t("admin_home_upcoming_empty")}</div>
              ) : (
                <div className="space-y-3">
                  {data.upcoming_exams.map((exam) => (
                    <div
                      key={exam.id}
                      className="flex items-center justify-between gap-3 rounded-[12px] p-3"
                      style={{ backgroundColor: "var(--md-sys-color-surface-container-high)" }}
                    >
                      <div>
                        <div className="text-body" style={{ fontWeight: 500 }}>{exam.title}</div>
                        <div className="text-label color-on-surface-variant">
                          {formatDateTime(exam.scheduled_at)}
                        </div>
                      </div>
                      <div className="text-label" style={{ color: "var(--md-sys-color-primary)" }}>
                        {t("admin_home_registrants").replace("{count}", String(exam.registrant_count))}
                      </div>
                    </div>
                  ))}
                </div>
              )}
            </div>

            <div className="md-card-outlined">
              <h3 className="text-title-large mb-4">{t("exam_home_violations")}</h3>
              {!data?.recent_violations.length ? (
                <div className="py-12 text-center text-ink-500">{t("exam_home_violations_empty")}</div>
              ) : (
                <div className="space-y-3">
                  {data.recent_violations.map((v) => (
                    <div key={v.session_id + v.occurred_at} className="flex items-center gap-3">
                      <div
                        className="flex size-8 shrink-0 items-center justify-center rounded-full"
                        style={{
                          backgroundColor: "var(--md-sys-color-error-container)",
                          color: "var(--md-sys-color-on-error-container)",
                        }}
                      >
                        <AlertTriangle size={16} />
                      </div>
                      <div>
                        <div className="text-body" style={{ fontWeight: 500 }}>{v.student_name}</div>
                        <div className="text-label color-on-surface-variant">
                          {v.exam_title} · {v.violation_type} · {formatDateTime(v.occurred_at)}
                        </div>
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
