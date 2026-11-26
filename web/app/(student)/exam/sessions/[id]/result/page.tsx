"use client";

import { useParams } from "next/navigation";
import type { LucideIcon } from "lucide-react";
import { AlertCircle, Award, ChevronDown, Clock, Crown, EyeOff, Lightbulb, Lock } from "lucide-react";

import { useSessionResult } from "@/lib/hooks/exam";
import { useTranslation, DICT } from "@/lib/i18n";
type I18nKey = keyof typeof DICT.id;
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import { Card } from "@/components/ui/card";
import { EmptyState } from "@/components/ui/empty-state";
import { PageHead } from "@/components/shell/PageHead";
import { Progress } from "@/components/ui/progress";
import { Skeleton } from "@/components/ui/skeleton";
import { RichContent } from "@/components/admin/RichContent";
import { formatChoiceAnswer } from "@/lib/option-key";
import type { ResultPembahasanItem } from "@/lib/types";

function formatDate(iso: string): string {
  const d = new Date(iso);
  if (Number.isNaN(d.getTime())) return iso;
  return new Intl.DateTimeFormat("id-ID", {
    day: "numeric",
    month: "short",
    year: "numeric",
    hour: "2-digit",
    minute: "2-digit",
  }).format(d);
}

export default function SessionResultPage() {
  const { t } = useTranslation();
  const params = useParams<{ id: string }>();
  const sessionId = params?.id ?? "";

  const {
    data: result,
    isLoading,
    isError,
    error,
    refetch,
  } = useSessionResult(sessionId);

  // ── Error state ────────────────────────────────────────────────────────

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

  // ── Loading state ──────────────────────────────────────────────────────

  if (isLoading || !result) {
    return (
      <>
        <p className="text-sm text-ink-600">{t("sys_loading")}</p>
        <Skeleton className="h-40 w-full rounded-[30px]" />
        <Skeleton className="h-24 w-full rounded-[22px]" />
      </>
    );
  }

  // ── Configured end screen (FR-38) ──────────────────────────────────────
  // Shown after submit whenever an admin has set both an image and a promo
  // text for this exam — replaces the plain result screen below regardless
  // of result-visibility state (FR-39: falls back to plain when unset).

  if (result.end_screen_image_url && result.end_screen_promo_text) {
    return (
      <div className="mx-auto w-full max-w-2xl text-center">
        <img
          src={result.end_screen_image_url}
          alt={t("result_end_screen_image_alt")}
          data-testid="end-screen-image"
          className="mx-auto mb-6 max-h-80 w-full rounded-lg object-contain"
        />
        <p className="whitespace-pre-wrap text-ink-800">
          {result.end_screen_promo_text}
        </p>
      </div>
    );
  }

  // ── Non-result states (hidden / grading / locked) ─────────────────────

  if (result.state === "hidden") {
    return <NonResultCard icon={EyeOff} title={t("result_hidden")} certificateUrl={result.certificate_url} />;
  }

  if (result.state === "grading") {
    return <NonResultCard icon={Clock} title={t("result_grading")} certificateUrl={result.certificate_url} />;
  }

  if (result.state === "locked") {
    return (
      <NonResultCard
        icon={Lock}
        title={t("result_locked").replace(
          "{t}",
          formatDate(result.result_release_at),
        )}
        certificateUrl={result.certificate_url}
      />
    );
  }

  // ── Result state ────────────────────────────────────────────────────────

  const sections = result.result_config === "score_pembahasan" ? result.breakdown : [];
  const maxTotal = sections.reduce((n, r) => n + r.max, 0);
  const pct = Math.round(
    Math.min(100, Math.max(0, maxTotal > 0 ? (result.score / maxTotal) * 100 : result.score)),
  );

  return (
    <>
      <PageHead
        back={{ href: "/exam", label: t("competition_detail_back") }}
        title={t("exam_admin_tab_results")}
        actions={
          <>
            <Button asChild variant="outline">
              <a href={`/exam/sessions/${sessionId}/leaderboard`}>
                <Crown />
                {t("view_leaderboard")}
              </a>
            </Button>
            {result.certificate_url && (
              <Button asChild variant="secondary">
                <a href={result.certificate_url} target="_blank" rel="noreferrer">
                  <Award />
                  {t("certificate")}
                </a>
              </Button>
            )}
          </>
        }
      />

      <div className="grid gap-4 lg:grid-cols-[minmax(0,1.4fr)_minmax(0,1fr)]">
        <section
          data-testid="score-hero"
          className="grid items-center gap-5 rounded-[30px] bg-[radial-gradient(120%_140%_at_100%_0%,#4cc3b2,var(--color-teal-600)_50%,#0f4d46)] p-[30px] text-white max-narrow:p-6 sm:grid-cols-[1fr_auto]"
        >
          <div>
            <p className="font-mono text-[11.5px] font-semibold tracking-[0.14em] uppercase opacity-90">
              {t("result_your_score")}
            </p>
            <h2 className="mt-2 text-[30px] leading-[1.1] font-extrabold [font-family:var(--font-display)]">
              {t("result_rank")} <span>#{result.rank}</span>
            </h2>
            <div className="mt-3.5 flex gap-[22px]">
              {(
                [
                  [result.correct_count, "result_correct"],
                  [result.wrong_count, "result_incorrect"],
                  [result.empty_count, "result_empty"],
                ] as const
              ).map(([n, key]) => (
                <div key={key}>
                  <b className="block text-[26px] font-extrabold [font-family:var(--font-display)]">{n}</b>
                  <span className="text-xs opacity-85">{t(key)}</span>
                </div>
              ))}
            </div>
          </div>
          <div
            role="img"
            aria-label={`${t("result_your_score")} ${result.score}`}
            style={{ background: `conic-gradient(#fff ${pct}%, rgba(255,255,255,.2) 0)` }}
            className="relative grid size-[150px] place-items-center rounded-full max-narrow:size-[120px]"
          >
            <span aria-hidden className="absolute inset-3 rounded-full bg-teal-600" />
            <b className="relative text-[46px] leading-none font-extrabold [font-family:var(--font-display)]">{result.score}</b>
          </div>
        </section>

        {sections.length > 0 && (
          <Card className="gap-0 p-6">
            <h2 className="mb-3.5 type-h2 text-ink-900">{t("result_by_topic")}</h2>
            <div className="space-y-3.5">
              {sections.map((row) => {
                const rowPct = row.max > 0 ? Math.min(100, Math.max(0, (row.earned / row.max) * 100)) : 0;
                return (
                  <div key={row.test_id}>
                    <div className="mb-1.5 flex items-center justify-between gap-2 text-sm">
                      <span className="flex items-center gap-2 text-ink-900">
                        {row.title}
                        {row.section_type && (
                          <Badge variant="mute" className="h-5 px-2 text-[11px]">
                            {t(`section_type_${row.section_type}` as I18nKey)}
                          </Badge>
                        )}
                      </span>
                      <span className="font-bold text-ink-900">
                        {row.earned}/{row.max}
                      </span>
                    </div>
                    <Progress value={rowPct} variant={rowPct >= 70 ? "teal" : "hatch"} />
                  </div>
                );
              })}
            </div>
          </Card>
        )}
      </div>

      {result.result_config === "score_pembahasan" && (
        <section>
          <h2 className="mb-3.5 type-h2 text-ink-900">{t("result_pembahasan")}</h2>
          <div className="flex flex-col gap-2.5">
            {result.pembahasan.map((item, i) => (
              <PembahasanItem key={item.question_id} index={i} item={item} t={t} />
            ))}
          </div>
        </section>
      )}
    </>
  );
}

function NonResultCard({
  icon,
  title,
  certificateUrl,
}: {
  icon: LucideIcon;
  title: string;
  certificateUrl?: string | null;
}) {
  const { t } = useTranslation();
  return (
    <EmptyState
      framed
      icon={icon}
      title={title}
      action={
        certificateUrl && (
          <Button variant="outline" asChild>
            <a href={certificateUrl} target="_blank" rel="noreferrer">
              <Award />
              {t("certificate")}
            </a>
          </Button>
        )
      }
    />
  );
}

function PembahasanItem({
  index,
  item,
  t,
}: {
  index: number;
  item: ResultPembahasanItem;
  t: (key: I18nKey) => string;
}) {
  return (
    <details className="group rounded-[var(--radius-card)] border border-line bg-card shadow-card">
      <summary className="flex cursor-pointer list-none items-center gap-3 px-5 py-4 [&::-webkit-details-marker]:hidden">
        <span className="shrink-0 type-label text-ink-900">{index + 1}.</span>
        <span className="min-w-0 flex-1 text-sm font-medium text-ink-900">
          <RichContent html={item.body} />
        </span>
        <Badge variant={item.is_correct ? "teal" : "danger"}>
          {item.is_correct ? t("result_correct") : t("result_incorrect")}
        </Badge>
        <ChevronDown className="size-4 shrink-0 text-ink-600 transition-transform group-open:rotate-180" aria-hidden />
      </summary>
      <div className="flex flex-col gap-3.5 px-5 pb-5 text-sm">
        <AnswerRow
          label={t("result_your_answer")}
          value={formatChoiceAnswer(item.your_answer, item.format) || "—"}
          tone={item.is_correct ? "right" : "wrong"}
        />
        {!item.is_correct && item.correct_answer != null && (
          <AnswerRow
            label={t("result_correct_answer")}
            value={formatChoiceAnswer(item.correct_answer, item.format)}
            tone="right"
          />
        )}
        {item.explanation && (
          <div className="flex gap-3 rounded-2xl bg-gold-50 p-4">
            <Lightbulb className="mt-0.5 size-4 shrink-0 text-gold-700" aria-hidden />
            <p className="whitespace-pre-wrap text-ink-700">{item.explanation}</p>
          </div>
        )}
      </div>
    </details>
  );
}

function AnswerRow({ label, value, tone }: { label: string; value: string; tone: "right" | "wrong" }) {
  return (
    <p
      className={`flex min-h-12 items-center gap-3 rounded-[18px] border-[1.5px] px-4 py-2 ${
        tone === "right" ? "border-teal-600 bg-teal-50" : "border-danger bg-danger-50"
      }`}
    >
      <span className="text-xs text-ink-600">{label}:</span>
      <span className="font-bold text-ink-900">{value}</span>
    </p>
  );
}
