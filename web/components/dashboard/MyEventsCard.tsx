"use client";

import { useState } from "react";
import Link from "next/link";
import { Trophy } from "lucide-react";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Card } from "@/components/ui/card";
import { Skeleton } from "@/components/ui/skeleton";
import { useSessionResult } from "@/lib/hooks/exam";
import { useTranslation } from "@/lib/i18n";
import { eventRows, minutesFor, type EventRow } from "@/lib/student-home";
import type { RegistrationListItem } from "@/lib/types";
import { ExternalResultsDialog } from "./ExternalResultsDialog";

const TZ = "Asia/Jakarta";

function DateTile({ iso, lang }: { iso: string | null; lang: string }) {
  const locale = lang === "id" ? "id-ID" : "en-US";
  const d = iso ? new Date(iso) : null;
  return (
    <div className="flex size-11 shrink-0 flex-col items-center justify-center rounded-lg bg-surface-sunken text-center">
      {d ? (
        <>
          <b className="type-label leading-none text-ink-900">
            {d.toLocaleDateString(locale, { day: "numeric", timeZone: TZ })}
          </b>
          <small className="type-label-xs uppercase text-ink-600">
            {d.toLocaleDateString(locale, { month: "short", timeZone: TZ })}
          </small>
        </>
      ) : (
        <b className="type-label text-ink-600">—</b>
      )}
    </div>
  );
}

function EventListRow({ row }: { row: EventRow }) {
  const { t, lang } = useTranslation();
  const { reg, live, action } = row;
  const [open, setOpen] = useState(false);
  const detailHref = `/exam/${reg.id}`;
  const needsResult = (action === "result" || (action === "external" && reg.status === "submitted")) && !!reg.session_id;
  const result = useSessionResult(needsResult ? reg.session_id! : undefined);
  const leaderboardHref = `/exam/sessions/${reg.session_id}/leaderboard`;
  const released = result.data?.state === "result";
  const minutes = minutesFor(reg);

  let actionNode: React.ReactNode = null;
  if (action === "external") {
    actionNode = (
      <>
        <Button size="sm" variant="outline" onClick={() => setOpen(true)}>
          {t("stu_home_events_results")}
        </Button>
        <ExternalResultsDialog
          open={open}
          onOpenChange={setOpen}
          examTitle={reg.exam_title}
          results={reg.external_results ?? []}
          leaderboardHref={released ? leaderboardHref : null}
        />
      </>
    );
  } else if (action === "result") {
    if (result.isPending) {
      actionNode = <Skeleton data-testid="event-result-loading" className="h-8 w-20" />;
    } else if (result.isError) {
      actionNode = (
        <Button asChild size="sm" variant="outline">
          <Link href={detailHref}>{t("stu_home_events_detail")}</Link>
        </Button>
      );
    } else if (result.data.state === "grading" || result.data.state === "locked") {
      actionNode = <span className="type-label-sm text-ink-600">{t("stu_home_events_awaiting")}</span>;
    } else if (result.data.state === "hidden") {
      actionNode = <Badge variant="mute">{t("stu_home_events_done")}</Badge>;
    } else {
      actionNode = (
        <Button asChild size="sm" variant="outline">
          <Link href={leaderboardHref}>{t("stu_home_events_results")}</Link>
        </Button>
      );
    }
  } else if (action === "expired") {
    actionNode = <span className="type-label-sm text-ink-600">{t("stu_home_events_missed")}</span>;
  } else if (action === "start") {
    actionNode = (
      <Button asChild size="sm">
        <Link href={detailHref}>{t("stu_home_events_start")}</Link>
      </Button>
    );
  } else {
    actionNode = (
      <Button asChild size="sm" variant="outline">
        <Link href={detailHref}>{t("stu_home_events_detail")}</Link>
      </Button>
    );
  }

  return (
    <li className="flex items-center gap-3">
      <DateTile iso={reg.scheduled_at} lang={lang} />
      <div className="min-w-0 flex-1">
        <p className="truncate type-label text-ink-900">{reg.exam_title}</p>
        <p className="type-body-sm text-ink-600">
          {minutes != null && t("stu_home_events_minutes").replace("{n}", String(minutes))}
          {live && (
            <>
              {minutes != null && " · "}
              <span className="text-ember-700">{t("stu_home_events_live")}</span>
            </>
          )}
        </p>
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
    <Card className="gap-4 p-6">
      <div className="flex items-center gap-2">
        <Trophy className="size-5 text-gold-700" aria-hidden />
        <h2 className="type-h3 flex-1 text-ink-900">{t("stu_home_events_title")}</h2>
        <Link href="/exam" className="type-label-sm text-brand-ink underline-offset-4 hover:underline">
          {t("stu_home_events_view_all")}
        </Link>
      </div>
      {isLoading ? (
        <div className="flex flex-col gap-3" data-testid="events-loading">
          {[0, 1, 2].map((i) => (
            <Skeleton key={i} className="h-11 w-full" />
          ))}
        </div>
      ) : isError ? (
        <p className="type-body-sm text-ink-600">{t("dash_load_failed")}</p>
      ) : rows.length ? (
        <ul className="flex flex-col gap-3">
          {rows.map((row) => (
            <EventListRow key={row.reg.id} row={row} />
          ))}
        </ul>
      ) : (
        <div className="flex flex-col items-start gap-3">
          <p className="type-body-sm text-ink-600">{t("stu_home_events_empty")}</p>
          <Button asChild size="sm" variant="outline">
            <Link href="/catalog">{t("stu_home_events_browse")}</Link>
          </Button>
        </div>
      )}
    </Card>
  );
}
