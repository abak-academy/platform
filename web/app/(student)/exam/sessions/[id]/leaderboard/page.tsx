"use client";

import { useEffect, useState } from "react";
import { useParams } from "next/navigation";
import { AlertCircle, Medal } from "lucide-react";

import { useSessionLeaderboard } from "@/lib/hooks/exam";
import type { ExamLeaderboardEntry } from "@/lib/types";
import { ApiError } from "@/lib/api";
import { useTranslation } from "@/lib/i18n";
import { Button } from "@/components/ui/button";
import { Card } from "@/components/ui/card";
import { EmptyState } from "@/components/ui/empty-state";
import { Skeleton } from "@/components/ui/skeleton";
import { PageHead } from "@/components/shell/PageHead";

export default function SessionLeaderboardPage() {
  const { t } = useTranslation();
  const params = useParams<{ id: string }>();
  const sessionId = params?.id ?? "";

  const [entries, setEntries] = useState<ExamLeaderboardEntry[]>([]);
  const [cursor, setCursor] = useState<string | undefined>(undefined);

  const {
    data,
    isLoading,
    isError,
    error,
    refetch,
    isFetching,
  } = useSessionLeaderboard(sessionId, cursor ? { cursor } : undefined);

  // Reset pagination when navigating to a different session
  useEffect(() => {
    setEntries([]);
    setCursor(undefined);
  }, [sessionId]);

  // Accumulate leaderboard pages
  useEffect(() => {
    if (!data) return;
    if (!cursor) {
      setEntries(data.data);
    } else {
      setEntries((prev) => [...prev, ...data.data]);
    }
  }, [data]);

  // ── 403: leaderboard not available ─────────────────────────────────────────

  if (
    isError &&
    error instanceof ApiError &&
    error.status === 403 &&
    error.code === "leaderboard_not_available"
  ) {
    return (
      <EmptyState framed icon={Medal} title={t("result_leaderboard_not_available")} />
    );
  }

  // ── Generic error ──────────────────────────────────────────────────────────

  if (isError) {
    return (
      <>
        <Card className="gap-0 border-danger/30 bg-danger-bg px-5 py-4">
          <div className="flex items-center gap-3">
            <AlertCircle className="size-5 text-danger" />
            <div className="flex-1 text-sm text-ink-700">
              {t("sys_error_load")}
              {error instanceof Error && error.message
                ? ` ${error.message}`
                : ""}
            </div>
            <Button variant="outline" size="sm" onClick={() => refetch()}>
              {t("retry")}
            </Button>
          </div>
        </Card>
      </>
    );
  }

  // ── Loading (first page only — keep accumulated rows during load-more) ────

  if (entries.length === 0 && (isLoading || !data)) {
    return (
      <>
        <p className="text-sm text-ink-600">{t("sys_loading")}</p>
        <Skeleton className="h-12 w-full rounded-[22px]" />
        <Skeleton className="h-40 w-full rounded-[22px]" />
      </>
    );
  }

  const head = (
    <PageHead
      back={{ href: `/exam/sessions/${sessionId}/result`, label: t("competition_detail_back") }}
      title={t("view_leaderboard")}
    />
  );

  // ── Empty state ───────────────────────────────────────────────────────────

  if (entries.length === 0) {
    return (
      <>
        {head}
        <Card className="gap-0 p-6 text-center text-sm text-ink-600">
          {t("admin_exam_leaderboard_empty")}
        </Card>
      </>
    );
  }

  // ── Leaderboard ──────────────────────────────────────────────────────────

  const top = entries.slice(0, 3);
  const rest = entries.slice(3);
  const podium = [1, 0, 2].filter((i) => top[i]);

  return (
    <>
      {head}

      <section className="mx-auto grid w-full max-w-[720px] grid-cols-3 items-end gap-3.5 max-narrow:gap-2" data-testid="podium">
        {podium.map((i) => {
          const entry = top[i];
          const tier = POD[i];
          return (
            <div key={entry.session_id} className="flex min-w-0 flex-col items-center gap-1.5 text-center">
              <Initial name={entry.student_name} className={i === 0 ? "size-[84px] ring-4 ring-[#f3cd6b]" : "size-16"} />
              <b className="max-w-full truncate type-label text-ink-900">{entry.student_name}</b>
              <div
                className={`flex w-full flex-col items-center gap-1.5 rounded-[22px_22px_8px_8px] pt-4 ${tier.bar}`}
              >
                <span className={`grid h-6 min-w-6 place-items-center rounded-full px-1.5 text-[11px] font-extrabold ${tier.medal}`}>
                  {entry.rank}
                </span>
                <b className="text-[30px] leading-none font-extrabold max-narrow:text-[22px] [font-family:var(--font-display)]">{entry.score}</b>
              </div>
            </div>
          );
        })}
      </section>

      {rest.length > 0 && (
        <Card className="gap-0 p-2">
          <ol>
            {rest.map((entry) => (
              <li
                key={entry.session_id}
                className="flex items-center gap-3 rounded-[14px] px-3 py-2.5 [&+&]:border-t [&+&]:border-line"
              >
                <span className="w-[34px] text-center text-sm font-extrabold text-ink-600 [font-family:var(--font-display)]">
                  {entry.rank}
                </span>
                <Initial name={entry.student_name} className="size-[30px] text-xs" />
                <span className="min-w-0 flex-1 truncate font-bold text-ink-900">{entry.student_name}</span>
                <b className="font-extrabold text-ink-900 tabular-nums [font-family:var(--font-display)]">{entry.score}</b>
              </li>
            ))}
          </ol>
        </Card>
      )}

      {/* Load more */}
      {data?.next_cursor && (
        <div className="flex justify-center">
          <Button
            type="button"
            variant="outline"
            onClick={() => setCursor(data.next_cursor)}
            disabled={isFetching}
          >
            {isFetching ? t("sys_loading") : t("sys_load_more")}
          </Button>
        </div>
      )}
    </>
  );
}

const POD = [
  { bar: "h-[170px] bg-[linear-gradient(180deg,#f3cd6b,var(--color-gold))] text-[#3d2b08]", medal: "bg-[#f3cd6b] text-[#5b3f05]" },
  { bar: "h-[130px] bg-[linear-gradient(180deg,#c4cdd4,#8a979b)] text-white", medal: "bg-[#d5dbe0] text-[#3a4650]" },
  { bar: "h-[104px] bg-[linear-gradient(180deg,#f59a6b,var(--color-ember-600))] text-white", medal: "bg-[#f2b48e] text-[#6b2f13]" },
] as const;

function Initial({ name, className }: { name: string; className?: string }) {
  return (
    <span
      aria-hidden
      className={`grid shrink-0 place-items-center rounded-full bg-brand-100 font-extrabold text-brand-800 ring-2 ring-surface ${className ?? ""}`}
    >
      {name.trim().charAt(0).toUpperCase()}
    </span>
  );
}
