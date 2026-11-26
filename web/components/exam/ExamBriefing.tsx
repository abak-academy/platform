"use client";

import { useState, type ReactNode } from "react";
import { ArrowRight, ArrowUpRight, CalendarDays, ClipboardList, Clock, Eye, Info, LayoutGrid, Lock, MessageCircle, RotateCcw } from "lucide-react";

import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Card } from "@/components/ui/card";
import { PageHead } from "@/components/shell/PageHead";
import { isHttpUrl, isWhatsAppGroupUrl } from "@/lib/exam-event-info";
import { accessWindow, workingMinutes } from "@/lib/exam-schedule";
import { useTranslation } from "@/lib/i18n";
import type { RegistrationExam } from "@/lib/types";
import { cn } from "@/lib/utils";

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
  actions?: ReactNode;
}

export function ExamBriefing({ exam, backHref, backLabel, preview = false, status = "ready", afterHowTo, checkIn, gate, actions }: ExamBriefingProps) {
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

  return (
    <>
      <PageHead
        back={{ href: backHref, label: backLabel }}
        title={<span className="[overflow-wrap:anywhere]">{exam.title}</span>}
        sub={t("examdetail_subtitle")}
        actions={actions}
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

      <Card className="gap-3 p-6">
        <div className="flex flex-wrap items-center justify-between gap-3">
          <h2 className="type-h2 text-ink-900">{t("competition_detail_exam_info")}</h2>
          {!preview && <Badge variant={statusVariant}>{statusLabel}</Badge>}
        </div>
        <div className="grid grid-cols-1 gap-3 narrow:grid-cols-2">
          <section className={PANEL}>
            <IconTile tone="sky">
              <CalendarDays className="size-5" />
            </IconTile>
            <h3 className={PANEL_MONO}>{t("examdetail_access_title")}</h3>
            {access ? (
              access.to && access.sameDay ? (
                <>
                  <span className={PANEL_VALUE}>{access.from.date}</span>
                  <span data-testid="access-hours" className="flex items-baseline gap-1 text-ink-900">
                    <span>{access.from.time}</span>
                    <span aria-hidden="true">–</span>
                    <span>{access.to.time}</span>
                    <span>WIB</span>
                  </span>
                </>
              ) : (
                <>
                  <span className={PANEL_VALUE}>{access.from.date}</span>
                  <span className="text-ink-900">
                    {access.from.time}
                    {access.to && ` – ${access.to.date}, ${access.to.time}`} WIB
                  </span>
                </>
              )
            ) : (
              <span className={PANEL_VALUE}>—</span>
            )}
            <small className={PANEL_SMALL}>{t("examdetail_access_caption")}</small>
          </section>

          <section data-testid="duration-panel" className={PANEL}>
            <IconTile tone="ember">
              <Clock className="size-5" />
            </IconTile>
            <h3 className={PANEL_MONO}>{t("examdetail_time_title")}</h3>
            <span className={PANEL_VALUE}>
              {minutes != null ? (
                <>
                  <span>{minutes}</span> <span>{minutesUnit}</span>
                </>
              ) : (
                "—"
              )}
            </span>
            <span className="text-ink-900">{perTest ? t("examcard_timer_per_test") : t("examcard_timer_overall")}</span>
            <small className={PANEL_SMALL}>
              {perTest ? t("examdetail_time_caption_per_test") : t("examdetail_time_caption_overall")}
            </small>
          </section>
        </div>
        <p className="flex items-start gap-2 text-xs text-ink-600">
          <Lock className="mt-0.5 size-3.5 shrink-0" />
          {t("examdetail_access_note")}
        </p>
      </Card>

      {exam.whatsapp_group_url && isWhatsAppGroupUrl(exam.whatsapp_group_url) && (
        <Card className="flex-row flex-wrap items-center gap-3.5 bg-[linear-gradient(100deg,var(--color-teal-50),var(--color-surface))] p-6">
          <span className="grid size-12 shrink-0 place-items-center rounded-2xl bg-[#25d366] text-white">
            <MessageCircle className="size-5" />
          </span>
          <div className="min-w-0 flex-1 basis-60">
            <h2 className="text-[15px] font-bold text-ink-900">{t("exam_event_student_wa_title")}</h2>
            <p className="mt-0.5 text-[13px] text-ink-600">
              {t("exam_event_student_wa_body")} {t("exam_event_student_wa_optional")}
            </p>
          </div>
          <Button asChild variant="outline" className={BTN}>
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

      <Card className="gap-3 p-6">
        <h2 className="type-h2 text-ink-900">{t("examdetail_howto_title")}</h2>
        <ul className="flex flex-col gap-4">
          <HowTo
            tone="sky"
            icon={sequential ? <ArrowRight className="size-5" /> : <LayoutGrid className="size-5" />}
            title={sequential ? t("examcard_nav_sequential") : t("examcard_nav_free")}
          >
            <p className={HOWTO_P}>{sequential ? t("examcard_nav_sequential_hint") : t("examcard_nav_free_hint")}</p>
          </HowTo>
          <HowTo
            tone="ember"
            icon={<Clock className="size-5" />}
            title={perTest ? t("examcard_timer_per_test") : t("examcard_timer_overall")}
          >
            <p className={HOWTO_P}>
              {t(perTest ? "examdetail_timer_rule_per_test" : "examdetail_timer_rule_overall")
                .replace("{n}", String(minutes ?? "—"))
                .replace("{s}", String(exam.section_count))}
            </p>
          </HowTo>
          <HowTo
            tone="teal"
            icon={<ClipboardList className="size-5" />}
            title={t("examdetail_questions_item")
              .replace("{q}", String(exam.question_count))
              .replace("{s}", String(exam.section_count))}
          >
            <p className={HOWTO_P}>{t("examdetail_questions_hint")}</p>
            <h3 className="sr-only">{t("examdetail_sections_title")}</h3>
            {exam.sections.length === 0 ? (
              <p className={HOWTO_P}>{t("examdetail_sections_empty")}</p>
            ) : (
              <ol className="mt-2.5 flex flex-col gap-2">
                {exam.sections.map((section, i) => (
                  <li key={section.test_id} className="flex flex-wrap items-center gap-3 rounded-2xl bg-surface-sunken px-3.5 py-3">
                    <span className="grid size-7 shrink-0 place-items-center rounded-[10px] bg-[#1a2168] font-[family-name:var(--font-display)] text-[13px] font-extrabold text-white">
                      {i + 1}
                    </span>
                    <span className="min-w-0 flex-1 font-bold [overflow-wrap:anywhere] text-ink-900">{section.title}</span>
                    <span className="text-ink-600">
                      {t("examdetail_section_meta")
                        .replace("{n}", String(section.question_count))
                        .replace(
                          "{presentation}",
                          section.question_presentation === "all_on_page"
                            ? t("examdetail_pres_all")
                            : t("examdetail_pres_one"),
                        )}
                      {" · "}
                      {perTest
                        ? t("examdetail_minutes").replace("{n}", String(section.duration_minutes))
                        : t("examdetail_shared_timer")}
                    </span>
                  </li>
                ))}
              </ol>
            )}
          </HowTo>
          <HowTo tone="gold" icon={<RotateCcw className="size-5" />} title={attempts}>
            <p className={HOWTO_P}>{t("examdetail_attempts_hint")}</p>
          </HowTo>
          {(exam.additional_rules_note || (exam.additional_rules_url && isHttpUrl(exam.additional_rules_url))) && (
            <HowTo tone="berry" icon={<Info className="size-5" />} title={t("exam_event_student_rules_title")}>
              {exam.additional_rules_note && (
                <p className={cn(HOWTO_P, "whitespace-pre-line [overflow-wrap:anywhere]")}>{exam.additional_rules_note}</p>
              )}
              {exam.additional_rules_url && isHttpUrl(exam.additional_rules_url) && (
                <a
                  href={exam.additional_rules_url}
                  target="_blank"
                  rel="noopener noreferrer"
                  className="mt-1 inline-flex items-center gap-1 [overflow-wrap:anywhere] text-brand-ink hover:underline"
                >
                  {t("exam_event_rules_link")}
                  <ArrowUpRight className="size-4" />
                </a>
              )}
            </HowTo>
          )}
        </ul>
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
        <Card className="gap-0 border-2 border-ember-600 p-6">
          <h2 className="type-h2 text-ink-900">{resume ? t("continue_exam") : t("examdetail_ready_title")}</h2>
          {!resume && <p className="mt-1 text-sm text-ink-600">{t("examdetail_ready_body")}</p>}
          {!resume && (
            <label className="mt-3 flex items-start gap-2.5 rounded-2xl bg-surface-sunken px-4 py-3.5 text-sm leading-[1.55] font-semibold text-ink-900">
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
          <Button
            type="button"
            size="lg"
            className="mt-3.5 w-fit border-[1.5px] border-transparent px-6 font-[family-name:var(--font-display)] text-[15px] leading-none font-bold tracking-[-0.005em] has-[>svg]:px-6"
            disabled={startDisabled}
            onClick={gate?.onStart}
          >
            {preview
              ? t("examdetail_preview_start")
              : gate?.pending
                ? t("sys_loading")
                : resume
                  ? t("continue_exam")
                  : t("exam_start_button")}
            <ArrowRight />
          </Button>
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
  gold: "bg-gold-50 text-gold-700",
  berry: "bg-berry-50 text-berry-700",
} as const;

const BTN = "px-[18px] has-[>svg]:px-[18px]";
const PANEL = "flex min-w-0 flex-col gap-1 rounded-[20px] bg-surface-sunken p-[18px]";
const PANEL_MONO = "mt-2 font-mono text-[10px] leading-[1.4] font-semibold tracking-[0.12em] text-ink-600 uppercase";
const PANEL_VALUE = "font-[family-name:var(--font-display)] text-xl leading-[normal] font-extrabold text-ink-900";
const PANEL_SMALL = "mt-1.5 text-[12.5px] text-ink-600";
const HOWTO_P = "mt-1 text-sm leading-[1.55] text-ink-600";

function HowTo({ tone, icon, title, children }: { tone: keyof typeof TILE_TONE; icon: ReactNode; title: string; children: ReactNode }) {
  return (
    <li className="flex gap-3.5">
      <IconTile tone={tone}>{icon}</IconTile>
      <div className="min-w-0 flex-1">
        <h3 className="font-sans text-[15px] font-bold tracking-normal text-ink-900">{title}</h3>
        {children}
      </div>
    </li>
  );
}

function IconTile({ tone, children }: { tone: keyof typeof TILE_TONE; children: ReactNode }) {
  return <span className={`grid size-10 shrink-0 place-items-center rounded-[13px] ${TILE_TONE[tone]}`}>{children}</span>;
}
