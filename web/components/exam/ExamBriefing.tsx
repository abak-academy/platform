"use client";

import { useState, type ReactNode } from "react";
import { ArrowRight, ArrowUpRight, BookOpen, CalendarDays, ClipboardList, Clock, Eye, Lock, Users } from "lucide-react";

import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Card } from "@/components/ui/card";
import { PageHead } from "@/components/shell/PageHead";
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

type BadgeVariant = NonNullable<React.ComponentProps<typeof Badge>["variant"]>;

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
  const [statusLabel, statusVariant]: [string, BadgeVariant] = resume
    ? [t("st_inprogress"), "live"]
    : status === "upcoming"
      ? [t("examcard_upcoming_badge"), "mute"]
      : status === "checkin"
        ? [t("st_checkin"), "gold"]
        : status === "expired"
          ? [t("st_expired"), "danger"]
          : status === "done"
            ? [t("examcard_status_done"), "sky"]
            : [t("examcard_status_ready"), "teal"];
  const startDisabled = preview || !gate || gate.disabled || gate.pending || (!resume && !understood);

  const DISPLAY = "[font-family:var(--font-display)]";
  return (
    <>
      <PageHead
        back={{ href: backHref, label: backLabel }}
        title={<span className="[overflow-wrap:anywhere]">{exam.title}</span>}
        sub={t("examdetail_subtitle")}
      />

      {preview && (
        <div className="flex items-start gap-2 rounded-lg border border-info/30 bg-info/10 px-4 py-3 text-sm text-ink-700">
          <Eye className="mt-0.5 size-4 shrink-0 text-info" />
          {t("examdetail_preview_banner")}
        </div>
      )}

      {preview && exam.requires_checkin ? (
        <Card className="gap-0 p-6">
          <h3 className="type-h3 text-ink-900">{t("exam_checkin_title")}</h3>
          <Button type="button" className="mt-3 w-fit" disabled>
            {t("examdetail_preview_checkin")}
          </Button>
        </Card>
      ) : (
        checkIn
      )}

      <Card className="gap-4 p-6">
        <div className="flex flex-wrap items-center justify-between gap-3">
          <h2 className="type-h2 text-ink-900">{t("competition_detail_exam_info")}</h2>
          {!preview && <Badge variant={statusVariant}>{statusLabel}</Badge>}
        </div>
        <div className="grid grid-cols-1 gap-3.5 sm:grid-cols-2">
          <section className="flex min-w-0 flex-col gap-1 rounded-[20px] bg-surface-sunken p-[18px]">
            <IconTile tone="sky">
              <CalendarDays className="size-5" />
            </IconTile>
            <h3 className="mt-2 font-mono text-[10px] font-semibold tracking-[0.1em] text-ink-600 uppercase">
              {t("examdetail_access_title")}
            </h3>
            <div className="text-ink-900">
              {access ? (
                access.to && access.sameDay ? (
                  <>
                    <div className={`${DISPLAY} text-xl font-extrabold`}>{access.from.date}</div>
                    <div data-testid="access-hours" className="flex items-baseline gap-2 text-sm font-semibold text-ink-600">
                      <span>{access.from.time}</span>
                      <span aria-hidden="true">–</span>
                      <span>{access.to.time}</span>
                      <span className="text-xs font-medium">WIB</span>
                    </div>
                  </>
                ) : (
                  <div className={`${DISPLAY} text-base font-extrabold`}>
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
            <p className="mt-1.5 text-xs text-ink-600">{t("examdetail_access_caption")}</p>
          </section>

          <section data-testid="duration-panel" className="flex min-w-0 flex-col gap-1 rounded-[20px] bg-surface-sunken p-[18px]">
            <IconTile tone="ember">
              <Clock className="size-5" />
            </IconTile>
            <h3 className="mt-2 font-mono text-[10px] font-semibold tracking-[0.1em] text-ink-600 uppercase">
              {t("examdetail_time_title")}
            </h3>
            <div>
              {minutes != null ? (
                <div className={`${DISPLAY} flex items-baseline gap-2 text-ink-900`}>
                  <span className="text-xl font-extrabold">{minutes}</span>
                  <span className="text-sm font-bold">{minutesUnit}</span>
                </div>
              ) : (
                "—"
              )}
              <div className="text-sm font-semibold text-ink-600">
                {perTest ? t("examcard_timer_per_test") : t("examcard_timer_overall")}
              </div>
            </div>
            <p className="mt-1.5 text-xs text-ink-600">
              {perTest ? t("examdetail_time_caption_per_test") : t("examdetail_time_caption_overall")}
            </p>
          </section>
        </div>
        <p className="flex items-start gap-2 text-xs text-ink-600">
          <Lock className="mt-0.5 size-3.5 shrink-0" />
          {t("examdetail_access_note")}
        </p>
      </Card>

      {exam.whatsapp_group_url && isWhatsAppGroupUrl(exam.whatsapp_group_url) && (
        <Card className="flex-row flex-wrap items-center gap-4 p-6">
          <IconTile tone="teal">
            <Users className="size-5" />
          </IconTile>
          <div className="min-w-0 flex-1 basis-60">
            <h2 className="type-h3 text-ink-900">{t("exam_event_student_wa_title")}</h2>
            <p className="mt-1 text-sm text-ink-600">{t("exam_event_student_wa_body")}</p>
            <p className="mt-1 text-xs text-ink-600">{t("exam_event_student_wa_optional")}</p>
          </div>
          <Button asChild variant="outline">
            <a
              href={exam.whatsapp_group_url}
              target="_blank"
              rel="noopener noreferrer"
              aria-label={t("exam_event_student_wa_aria")}
            >
              {t("exam_event_student_wa_cta")}
              <ArrowUpRight />
            </a>
          </Button>
        </Card>
      )}

      <Card className="gap-4 p-6">
        <h2 className="type-h2 text-ink-900">{t("examdetail_howto_title")}</h2>
        <div className="flex items-start gap-4 text-sm">
          <IconTile tone="sky">{sequential ? <Lock className="size-5" /> : <ArrowRight className="size-5" />}</IconTile>
          <div className="min-w-0">
            <div className="font-bold text-ink-900">
              {sequential ? t("examcard_nav_sequential") : t("examcard_nav_free")}
            </div>
            <div className="text-ink-600">
              {sequential ? t("examcard_nav_sequential_hint") : t("examcard_nav_free_hint")}
            </div>
          </div>
        </div>
        <div className="flex items-start gap-4 text-sm">
          <IconTile tone="ember">
            <Clock className="size-5" />
          </IconTile>
          <div className="min-w-0">
            <div className="font-bold text-ink-900">
              {perTest ? t("examcard_timer_per_test") : t("examcard_timer_overall")}
            </div>
            <div className="text-ink-600">
              {t(perTest ? "examdetail_timer_rule_per_test" : "examdetail_timer_rule_overall")
                .replace("{n}", String(minutes ?? "—"))
                .replace("{s}", String(exam.section_count))}
            </div>
          </div>
        </div>
        <div className="flex items-start gap-4 text-sm">
          <IconTile tone="teal">
            <ClipboardList className="size-5" />
          </IconTile>
          <div className="min-w-0">
            <div className="font-bold text-ink-900">
              {t("examdetail_questions_item")
                .replace("{q}", String(exam.question_count))
                .replace("{s}", String(exam.section_count))}
            </div>
            <div className="text-ink-600">{t("examdetail_questions_hint")}</div>
          </div>
        </div>
        {(exam.additional_rules_note || (exam.additional_rules_url && isHttpUrl(exam.additional_rules_url))) && (
          <div className="flex items-start gap-4 text-sm">
            <IconTile tone="berry">
              <BookOpen className="size-5" />
            </IconTile>
            <div className="min-w-0">
              <h3 className="font-bold text-ink-900">{t("exam_event_student_rules_title")}</h3>
              {exam.additional_rules_note && (
                <p className="whitespace-pre-line [overflow-wrap:anywhere] text-ink-600">{exam.additional_rules_note}</p>
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
          <h3 className="type-h3 text-ink-900">{t("examdetail_sections_title")}</h3>
          {exam.sections.length === 0 ? (
            <p className="mt-2 text-sm text-ink-600">{t("examdetail_sections_empty")}</p>
          ) : (
            <ol className="mt-2 divide-y divide-line">
              {exam.sections.map((section, i) => (
                <li key={section.test_id} className="flex items-center justify-between gap-3 py-3">
                  <div className="flex min-w-0 items-center gap-3">
                    <span className="grid size-[26px] shrink-0 place-items-center rounded-lg bg-deep text-xs font-extrabold text-on-deep">
                      {i + 1}
                    </span>
                    <div className="min-w-0">
                      <div className="text-sm font-bold [overflow-wrap:anywhere] text-ink-900">{section.title}</div>
                      <div className="text-xs text-ink-600">
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
        <Card className="gap-0 border-warning/30 bg-warning-bg p-6">
          <h3 className="type-h3 text-ink-900">{t("competition_detail_checkin_heading")}</h3>
          <p className="mt-2 text-sm text-ink-700">
            {t("competition_detail_checkin_body").replace("{N}", String(exam.check_in_window_minutes ?? 15))}
          </p>
        </Card>
      )}

      {(preview || gate) && (
        <Card className="gap-0 p-6">
          <h2 className="type-h2 text-ink-900">{resume ? t("continue_exam") : t("examdetail_ready_title")}</h2>
          {!resume && <p className="mt-1 text-sm text-ink-600">{t("examdetail_ready_body")}</p>}
          {!resume && (
            <label className="mt-4 flex items-center gap-3 rounded-2xl bg-surface-sunken px-4 py-3.5 text-sm font-semibold text-ink-900">
              <input
                type="checkbox"
                className="size-[19px] shrink-0 accent-brand-600"
                checked={understood}
                disabled={preview}
                onChange={(e) => setUnderstood(e.target.checked)}
              />
              {t("examdetail_ready_checkbox")}
            </label>
          )}
          <div className="mt-4 flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
            <span className="text-sm text-ink-600">{attempts}</span>
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
          {gate?.hint && <p className="mt-2 text-sm text-ink-600">{gate.hint}</p>}
          {!resume && !preview && !understood && (
            <p className="mt-2 text-xs text-ink-600">{t("examdetail_ready_hint")}</p>
          )}
        </Card>
      )}
    </>
  );
}

const TILE_TONE = {
  sky: "bg-sky-50 text-sky-700",
  ember: "bg-ember-50 text-ember-700",
  teal: "bg-teal-50 text-teal-700",
  berry: "bg-berry-50 text-berry-700",
} as const;

function IconTile({ tone, children }: { tone: keyof typeof TILE_TONE; children: ReactNode }) {
  return <span className={`grid size-10 shrink-0 place-items-center rounded-[13px] ${TILE_TONE[tone]}`}>{children}</span>;
}
