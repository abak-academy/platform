"use client";

import { useState } from "react";
import Link from "next/link";
import { Timer, Trophy } from "lucide-react";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Skeleton } from "@/components/ui/skeleton";
import { useSessionResult } from "@/lib/hooks/exam";
import { useTranslation } from "@/lib/i18n";
import { eventRows, minutesFor, type EventRow } from "@/lib/student-home";
import type { RegistrationListItem } from "@/lib/types";
import { DashCard } from "./DashCard";
import { CompetitionResultDialog } from "./CompetitionResultDialog";

const TZ = "Asia/Jakarta";
// Prototype .ab-btn.sm: 34px, 13px side padding.
const SM = "px-[13px]";

function DateTile({ iso, lang }: { iso: string | null; lang: string }) {
  const locale = lang === "id" ? "id-ID" : "en-US";
  const d = iso ? new Date(iso) : null;
  return (
    <span
      data-testid="event-date"
      className="w-10 flex-none rounded-[10px] bg-surface-sunken py-1 text-center font-mono text-[10.5px] font-bold text-ink-600 uppercase"
    >
      <b className="block font-[family-name:var(--font-display)] text-base leading-[1.1] font-extrabold text-ink-900">
        {d ? d.toLocaleDateString(locale, { day: "numeric", timeZone: TZ }) : "—"}
      </b>
      {d && d.toLocaleDateString(locale, { month: "short", timeZone: TZ })}
    </span>
  );
}

function EventListRow({ row }: { row: EventRow }) {
  const { t, lang } = useTranslation();
  const { reg, live, action } = row;
  const [open, setOpen] = useState(false);
  const detailHref = `/exam/${reg.id}`;
  const needsResult = (action === "result" || (action === "competition" && reg.status === "submitted")) && !!reg.session_id;
  const result = useSessionResult(needsResult ? reg.session_id! : undefined);
  const leaderboardHref = `/exam/sessions/${reg.session_id}/leaderboard`;
  const released = result.data?.state === "result";
  const minutes = minutesFor(reg);

  let actionNode: React.ReactNode = null;
  if (action === "competition" && reg.competition_result) {
    actionNode = (
      <>
        <Button size="sm" className={SM} onClick={() => setOpen(true)}>
          {t("stu_home_events_results")}
        </Button>
        <CompetitionResultDialog
          open={open}
          onOpenChange={setOpen}
          examTitle={reg.exam_title}
          result={reg.competition_result}
          leaderboardHref={released ? leaderboardHref : null}
        />
      </>
    );
  } else if (action === "result") {
    if (result.isPending) {
      actionNode = <Skeleton data-testid="event-result-loading" className="h-[34px] w-20 rounded-full" />;
    } else if (result.isError) {
      actionNode = (
        <Button asChild size="sm" variant="outline" className={SM}>
          <Link href={detailHref}>{t("stu_home_events_detail")}</Link>
        </Button>
      );
    } else if (result.data.state === "grading" || result.data.state === "locked") {
      actionNode = <Badge variant="mute">{t("stu_home_events_awaiting")}</Badge>;
    } else if (result.data.state === "hidden") {
      actionNode = <Badge variant="mute">{t("stu_home_events_done")}</Badge>;
    } else {
      actionNode = (
        <Button asChild size="sm" className={SM}>
          <Link href={leaderboardHref}>{t("stu_home_events_results")}</Link>
        </Button>
      );
    }
  } else if (action === "expired") {
    actionNode = <Badge variant="mute">{t("stu_home_events_missed")}</Badge>;
  } else if (action === "start") {
    actionNode = (
      <Button asChild size="sm" variant="outline" className={SM}>
        <Link href={detailHref}>{t("stu_home_events_start")}</Link>
      </Button>
    );
  } else {
    actionNode = (
      <Button asChild size="sm" variant="outline" className={SM}>
        <Link href={detailHref}>{t("stu_home_events_detail")}</Link>
      </Button>
    );
  }

  return (
    <li className="flex items-center gap-2.5 border-t border-line py-[9px] first:border-t-0">
      <DateTile iso={reg.scheduled_at} lang={lang} />
      <div className="min-w-0 flex-1">
        <b className="block truncate font-[family-name:var(--font-display)] text-[13px] font-bold text-ink-900">{reg.exam_title}</b>
        <small className="block truncate text-[11.5px] text-ink-600">
          {minutes != null && (
            <>
              <Timer className="mr-[3px] inline size-4 align-[-2px]" aria-hidden />
              {t("stu_home_events_minutes").replace("{n}", String(minutes))}
            </>
          )}
          {live && (
            <>
              {minutes != null && " · "}
              <span className="font-bold text-ember-600">{t("stu_home_events_live")}</span>
            </>
          )}
        </small>
      </div>
      {actionNode}
    </li>
  );
}

export function MyEventsCard({
  registrations,
  now,
  isLoading = false,
  isError = false,
}: {
  registrations: RegistrationListItem[];
  now: number;
  isLoading?: boolean;
  isError?: boolean;
}) {
  const { t } = useTranslation();
  const rows = eventRows(registrations, now);
  return (
    <DashCard icon={Trophy} tone="gold" title={t("stu_home_events_title")} more={{ href: "/exam", label: t("stu_home_events_view_all") }}>
      {isLoading ? (
        <div className="flex flex-col gap-3" data-testid="events-loading">
          {[0, 1, 2].map((i) => (
            <Skeleton key={i} className="h-11 w-full" />
          ))}
        </div>
      ) : isError ? (
        <p className="type-body-sm text-ink-600">{t("dash_load_failed")}</p>
      ) : rows.length ? (
        <ul className="m-0 flex list-none flex-col p-0">
          {rows.map((row) => (
            <EventListRow key={row.reg.id} row={row} />
          ))}
        </ul>
      ) : (
        <div className="flex flex-col items-start gap-3">
          <p className="text-ink-600">{t("stu_home_events_empty")}</p>
          <Button asChild size="sm" variant="outline" className={SM}>
            <Link href="/catalog">{t("stu_home_events_browse")}</Link>
          </Button>
        </div>
      )}
    </DashCard>
  );
}
