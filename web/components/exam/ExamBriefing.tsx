"use client";

import { useState, type ReactNode } from "react";
import Link from "next/link";
import { ArrowLeft, ArrowRight, BookOpen, CalendarDays, ClipboardList, Clock, Eye, Lock, Users } from "lucide-react";

import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Card } from "@/components/ui/card";
import { isHttpUrl, isWhatsAppGroupUrl } from "@/lib/exam-event-info";
import { accessWindow, workingMinutes } from "@/lib/exam-schedule";
import { useTranslation } from "@/lib/i18n";
import type { RegistrationExam } from "@/lib/types";

export interface BriefingGate {
  kind: "start" | "resume";
  disabled: boolean;
  hint?: string | null;
  pending: boolean;
  onStart: () => void;
}

interface ExamBriefingProps {
  exam: RegistrationExam;
  backHref: string;
  backLabel: string;
  preview?: boolean;
  status?: "ready" | "upcoming" | "checkin" | "expired" | "done";
  afterHowTo?: ReactNode;
  checkIn?: ReactNode;
  gate: BriefingGate | null;
}

export function ExamBriefing({ exam, backHref, backLabel, preview = false, status = "ready", afterHowTo, checkIn, gate }: ExamBriefingProps) {
  const { t, lang } = useTranslation();
  const [understood, setUnderstood] = useState(false);

  const access = accessWindow(exam.scheduled_at, exam.scheduled_end_at, lang, { weekday: true });
  const minutes = workingMinutes(exam);
  const perTest = exam.timer_mode === "per_test";
  const sequential = exam.navigation_policy === "sequential";
  const attempts =
    exam.max_attempts == null
      ? t("examdetail_attempts_unlimited")
      : t("examdetail_attempts_max").replace("{n}", String(Math.max(1, exam.max_attempts)));

  const minutesUnit = t(perTest ? "examdetail_minutes_total" : "examdetail_minutes").replace("{n}", "").trim();
  const resume = gate?.kind === "resume";
  const [statusLabel, statusClass] = resume
    ? [t("st_inprogress"), "bg-brand-50 text-brand-700"]
    : status === "upcoming"
      ? [t("examcard_upcoming_badge"), "border-line bg-surface-2 text-ink-700"]
      : status === "checkin"
        ? [t("st_checkin"), "bg-brand-50 text-brand-700"]
        : status === "expired"
          ? [t("st_expired"), "bg-warn-bg text-warn"]
          : status === "done"
            ? [t("examcard_status_done"), "bg-success-bg text-success"]
            : [t("examcard_status_ready"), "bg-success-bg text-success"];
  const startDisabled = preview || !gate || gate.disabled || gate.pending || (!resume && !understood);

  return (
    <div className="mx-auto max-w-3xl px-4 py-6 md:px-6 md:py-8">
      <Link
        href={backHref}
        className="mb-4 inline-flex items-center gap-1.5 text-sm font-medium text-ink-600 hover:text-ink-900"
      >
        <ArrowLeft className="size-4" />
        {backLabel}
      </Link>

      {preview && (
        <div className="mb-4 flex items-start gap-2 rounded-lg border border-info/30 bg-info/10 px-4 py-3 text-sm text-ink-700">
          <Eye className="mt-0.5 size-4 shrink-0 text-info" />
          {t("examdetail_preview_banner")}
        </div>
      )}

      <header className="mb-6">
        <h1 className="font-serif text-2xl font-bold [overflow-wrap:anywhere] text-ink-900 md:text-3xl">{exam.title}</h1>
        <p className="mt-2 text-sm text-ink-500">{t("examdetail_subtitle")}</p>
      </header>

      <Card className="space-y-4 p-5">
        <div className="flex items-center justify-between gap-3">
          <h2 className="font-serif text-xl font-semibold text-ink-900">{t("competition_detail_exam_info")}</h2>
          {!preview && <Badge className={`rounded-full border-transparent ${statusClass}`}>{statusLabel}</Badge>}
        </div>
        <div className="grid grid-cols-1 gap-4 sm:grid-cols-2">
          <section className="flex min-w-0 flex-col rounded-xl border border-line p-4">
            <h3 className="flex items-center gap-3 text-sm font-semibold text-ink-900">
              <IconChip>
                <CalendarDays className="size-5" />
              </IconChip>
              {t("examdetail_access_title")}
            </h3>
            <div className="mt-4 flex-1 text-ink-900">
              {access ? (
                access.to && access.sameDay ? (
                  <>
                    <div className="text-base font-semibold">{access.from.date}</div>
                    <div data-testid="access-hours" className="mt-1 flex items-baseline gap-2 text-2xl font-bold">
                      <span>{access.from.time}</span>
                      <span className="font-normal text-ink-500" aria-hidden="true">–</span>
                      <span>{access.to.time}</span>
                      <span className="text-xs font-medium text-ink-500">WIB</span>
                    </div>
                  </>
                ) : (
                  <div className="text-base font-semibold">
                    <div>
                      {access.from.date}, {access.from.time} WIB
                    </div>
                    {access.to && (
                      <div>
                        {access.to.date}, {access.to.time} WIB
                      </div>
                    )}
                  </div>
                )
              ) : (
                "—"
              )}
            </div>
            <p className="mt-4 text-xs text-ink-500">{t("examdetail_access_caption")}</p>
          </section>

          <section data-testid="duration-panel" className="flex min-w-0 flex-col rounded-xl border border-line p-4">
            <h3 className="flex items-center gap-3 text-sm font-semibold text-ink-900">
              <IconChip>
                <Clock className="size-5" />
              </IconChip>
              {t("examdetail_time_title")}
            </h3>
            <div className="mt-4 flex-1">
              {minutes != null ? (
                <div className="flex items-baseline gap-2 text-ink-900">
                  <span className="text-3xl leading-none font-bold">{minutes}</span>
                  <span className="text-sm font-medium">{minutesUnit}</span>
                </div>
              ) : (
                "—"
              )}
              <div className="mt-1 text-sm font-medium text-brand-600">
                {perTest ? t("examcard_timer_per_test") : t("examcard_timer_overall")}
              </div>
            </div>
            <p className="mt-4 text-xs text-ink-500">
              {perTest ? t("examdetail_time_caption_per_test") : t("examdetail_time_caption_overall")}
            </p>
          </section>
        </div>
        <p className="flex items-start gap-2 border-t border-line pt-4 text-xs text-ink-500">
          <Lock className="mt-0.5 size-3.5 shrink-0" />
          {t("examdetail_access_note")}
        </p>
      </Card>

      {exam.whatsapp_group_url && isWhatsAppGroupUrl(exam.whatsapp_group_url) && (
        <Card className="mt-4 space-y-3 p-5">
          <div className="flex items-start gap-3">
            <Users className="mt-0.5 size-5 shrink-0 text-ink-500" />
            <div className="min-w-0">
              <h2 className="font-serif text-lg font-semibold text-ink-900">{t("exam_event_student_wa_title")}</h2>
              <p className="mt-1 text-sm text-ink-500">{t("exam_event_student_wa_body")}</p>
              <p className="mt-1 text-xs text-ink-500">{t("exam_event_student_wa_optional")}</p>
            </div>
          </div>
          <Button asChild variant="outline">
            <a
              href={exam.whatsapp_group_url}
              target="_blank"
              rel="noopener noreferrer"
              aria-label={t("exam_event_student_wa_aria")}
            >
              {t("exam_event_student_wa_cta")}
            </a>
          </Button>
        </Card>
      )}

      <Card className="mt-4 space-y-4 p-5">
        <h2 className="font-serif text-lg font-semibold text-ink-900">{t("examdetail_howto_title")}</h2>
        <div className="flex items-start gap-3 text-sm">
          {sequential ? (
            <Lock className="mt-0.5 size-4 shrink-0 text-ink-500" />
          ) : (
            <ArrowRight className="mt-0.5 size-4 shrink-0 text-ink-500" />
          )}
          <div className="min-w-0">
            <div className="font-semibold text-ink-900">
              {sequential ? t("examcard_nav_sequential") : t("examcard_nav_free")}
            </div>
            <div className="text-ink-500">
              {sequential ? t("examcard_nav_sequential_hint") : t("examcard_nav_free_hint")}
            </div>
          </div>
        </div>
        <div className="flex items-start gap-3 text-sm">
          <Clock className="mt-0.5 size-4 shrink-0 text-ink-500" />
          <div className="min-w-0">
            <div className="font-semibold text-ink-900">
              {perTest ? t("examcard_timer_per_test") : t("examcard_timer_overall")}
            </div>
            <div className="text-ink-500">
              {t(perTest ? "examdetail_timer_rule_per_test" : "examdetail_timer_rule_overall")
                .replace("{n}", String(minutes ?? "—"))
                .replace("{s}", String(exam.section_count))}
            </div>
          </div>
        </div>
        <div className="flex items-start gap-3 text-sm">
          <ClipboardList className="mt-0.5 size-4 shrink-0 text-ink-500" />
          <div className="min-w-0">
            <div className="font-semibold text-ink-900">
              {t("examdetail_questions_item")
                .replace("{q}", String(exam.question_count))
                .replace("{s}", String(exam.section_count))}
            </div>
            <div className="text-ink-500">{t("examdetail_questions_hint")}</div>
          </div>
        </div>
        {(exam.additional_rules_note || (exam.additional_rules_url && isHttpUrl(exam.additional_rules_url))) && (
          <div className="flex items-start gap-3 text-sm">
            <BookOpen className="mt-0.5 size-4 shrink-0 text-ink-500" />
            <div className="min-w-0">
              <h3 className="font-semibold text-ink-900">{t("exam_event_student_rules_title")}</h3>
              {exam.additional_rules_note && (
                <p className="whitespace-pre-line [overflow-wrap:anywhere] text-ink-500">{exam.additional_rules_note}</p>
              )}
              {exam.additional_rules_url && isHttpUrl(exam.additional_rules_url) && (
                <a
                  href={exam.additional_rules_url}
                  target="_blank"
                  rel="noopener noreferrer"
                  className="mt-1 inline-block font-medium [overflow-wrap:anywhere] text-brand-700 hover:underline"
                >
                  {t("exam_event_rules_link")}
                </a>
              )}
            </div>
          </div>
        )}
        <div className="border-t border-line pt-4">
          <h3 className="text-sm font-semibold text-ink-900">{t("examdetail_sections_title")}</h3>
          {exam.sections.length === 0 ? (
            <p className="mt-2 text-sm text-ink-500">{t("examdetail_sections_empty")}</p>
          ) : (
            <ol className="mt-2 divide-y divide-line">
              {exam.sections.map((section, i) => (
                <li key={section.test_id} className="flex items-center justify-between gap-3 py-3">
                  <div className="flex min-w-0 items-center gap-3">
                    <Badge variant="secondary" className="size-6 shrink-0 justify-center rounded-full p-0">
                      {i + 1}
                    </Badge>
                    <div className="min-w-0">
                      <div className="text-sm font-semibold [overflow-wrap:anywhere] text-ink-900">{section.title}</div>
                      <div className="text-xs text-ink-500">
                        {t("examdetail_section_meta")
                          .replace("{n}", String(section.question_count))
                          .replace(
                            "{presentation}",
                            section.question_presentation === "all_on_page"
                              ? t("examdetail_pres_all")
                              : t("examdetail_pres_one"),
                          )}
                      </div>
                    </div>
                  </div>
                  <div className="shrink-0 text-sm font-semibold text-ink-900">
                    {perTest
                      ? t("examdetail_minutes").replace("{n}", String(section.duration_minutes))
                      : t("examdetail_shared_timer")}
                  </div>
                </li>
              ))}
            </ol>
          )}
        </div>
      </Card>

      {afterHowTo}

      {exam.requires_checkin && (
        <Card className="mt-4 border-warning/30 bg-warning-bg p-5">
          <h3 className="font-serif text-base font-semibold text-ink-900">
            {t("competition_detail_checkin_heading")}
          </h3>
          <p className="mt-2 text-sm text-ink-700">
            {t("competition_detail_checkin_body").replace("{N}", String(exam.check_in_window_minutes ?? 15))}
          </p>
        </Card>
      )}

      {preview && exam.requires_checkin ? (
        <Card className="mt-4 p-5">
          <h3 className="font-serif text-base font-semibold text-ink-900">{t("exam_checkin_title")}</h3>
          <Button type="button" className="mt-3" disabled>
            {t("examdetail_preview_checkin")}
          </Button>
        </Card>
      ) : (
        checkIn
      )}

      {(preview || gate) && (
        <Card className="mt-4 p-5">
          <h2 className="font-serif text-lg font-semibold text-ink-900">
            {resume ? t("continue_exam") : t("examdetail_ready_title")}
          </h2>
          {!resume && <p className="mt-1 text-sm text-ink-500">{t("examdetail_ready_body")}</p>}
          {!resume && (
            <label className="mt-4 flex items-start gap-2 text-sm text-ink-900">
              <input
                type="checkbox"
                className="mt-0.5 size-4 shrink-0"
                checked={understood}
                disabled={preview}
                onChange={(e) => setUnderstood(e.target.checked)}
              />
              {t("examdetail_ready_checkbox")}
            </label>
          )}
          <div className="mt-4 flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
            <span className="text-sm text-ink-500">{attempts}</span>
            <Button type="button" size="lg" className="shrink-0" disabled={startDisabled} onClick={gate?.onStart}>
              {preview
                ? t("examdetail_preview_start")
                : gate?.pending
                  ? t("sys_loading")
                  : resume
                    ? t("continue_exam")
                    : t("exam_start_button")}
            </Button>
          </div>
          {gate?.hint && <p className="mt-2 text-sm text-ink-500">{gate.hint}</p>}
          {!resume && !preview && !understood && (
            <p className="mt-2 text-xs text-ink-500">{t("examdetail_ready_hint")}</p>
          )}
        </Card>
      )}
    </div>
  );
}

function IconChip({ children }: { children: ReactNode }) {
  return <span className="flex size-10 shrink-0 items-center justify-center rounded-lg bg-brand-50 text-brand-600">{children}</span>;
}
