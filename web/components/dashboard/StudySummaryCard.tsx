"use client";

import { Award, Clock } from "lucide-react";
import { useTranslation } from "@/lib/i18n";
import type { DashboardStudySummary } from "@/lib/types";
import { DashCard } from "./DashCard";

export function StudySummaryCard({ study }: { study: DashboardStudySummary }) {
  const { t, lang } = useTranslation();
  const total = study.total_lectures;
  const visited = study.visited_lectures;
  const pct = total ? Math.round((visited / total) * 100) : 0;
  const hours = (study.total_minutes / 60).toLocaleString(lang === "id" ? "id-ID" : "en-US", {
    minimumFractionDigits: 1,
    maximumFractionDigits: 1,
  });
  const legend = [
    { label: t("stu_home_study_legend_watched"), value: String(visited), mark: <i className="size-2.5 rounded-[3px] bg-teal-600" /> },
    {
      label: t("stu_home_study_legend_remaining"),
      value: String(Math.max(0, total - visited)),
      mark: <i className="size-2.5 rounded-[3px] border border-line bg-surface-sunken" />,
    },
    {
      label: t("stu_home_study_legend_courses"),
      value: `${study.completed_courses}/${study.enrolled_courses_count}`,
      mark: <Award className="size-4" aria-hidden />,
    },
  ];

  return (
    <DashCard icon={Clock} tone="teal" title={t("stu_home_study_title")}>
      {total === 0 ? (
        <div className="flex flex-1 items-center justify-center py-8 text-center">
          <p className="text-sm text-ink-600">{t("dash_study_empty")}</p>
        </div>
      ) : (
        <div className="flex items-center gap-[18px]">
          <div
            role="img"
            aria-label={`${pct}%`}
            data-testid="study-ring"
            className="relative grid size-[116px] flex-none place-items-center rounded-full before:absolute before:inset-3.5 before:rounded-full before:bg-surface before:content-['']"
            style={{ background: `conic-gradient(var(--color-teal-600) ${pct}%, var(--color-surface-sunken) 0)` }}
          >
            <b className="relative block text-center font-[family-name:var(--font-display)] text-[26px] leading-none font-extrabold text-ink-900">{hours}</b>
            <span className="relative -mt-3.5 block text-center text-[11px] text-ink-600">{t("dash_hours")}</span>
          </div>
          <ul className="m-0 flex flex-1 list-none flex-col gap-2 p-0 text-[13px] text-ink-600">
            {legend.map((l) => (
              <li key={l.label} className="flex items-center gap-[7px]">
                {l.mark}
                {l.label}
                <b className="ml-auto text-ink-900">{l.value}</b>
              </li>
            ))}
          </ul>
        </div>
      )}
    </DashCard>
  );
}
