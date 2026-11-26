"use client";

import { Clock } from "lucide-react";
import { Card } from "@/components/ui/card";
import { Donut } from "@/components/dashboard/Donut";
import { useTranslation } from "@/lib/i18n";
import type { DashboardStudySummary } from "@/lib/types";

export function StudySummaryCard({ study }: { study: DashboardStudySummary }) {
  const { t, lang } = useTranslation();
  const total = study.total_lectures;
  const visited = study.visited_lectures;
  const hours = (study.total_minutes / 60).toLocaleString(lang === "id" ? "id-ID" : "en-US", {
    minimumFractionDigits: 1,
    maximumFractionDigits: 1,
  });
  const legend = [
    { label: t("stu_home_study_legend_watched"), value: String(visited), dot: "bg-brand-600" },
    { label: t("stu_home_study_legend_remaining"), value: String(Math.max(0, total - visited)), dot: "bg-brand-100" },
    {
      label: t("stu_home_study_legend_courses"),
      value: `${study.completed_courses}/${study.enrolled_courses_count}`,
      dot: "bg-ink-400",
    },
  ];

  return (
    <Card className="flex flex-col gap-3.5 border-line p-5">
      <div className="flex items-center gap-2.5">
        <span className="flex size-9 items-center justify-center rounded-xl bg-teal-50 text-teal-700">
          <Clock className="size-5" />
        </span>
        <h3 className="flex-1 text-[15px] font-extrabold text-ink-900">{t("stu_home_study_title")}</h3>
      </div>

      {total === 0 ? (
        <div className="flex flex-1 items-center justify-center py-8 text-center">
          <p className="text-sm text-ink-500">{t("dash_study_empty")}</p>
        </div>
      ) : (
        <div className="flex items-center gap-[18px]">
          <Donut size={116} thickness={16} value={visited / total} centerLabel={hours} centerSub={t("dash_hours")} />
          <ul className="m-0 flex flex-1 list-none flex-col gap-2 p-0 text-[13px] text-ink-500">
            {legend.map((l) => (
              <li key={l.label} className="flex items-center gap-2">
                <i className={`size-2.5 rounded-[3px] ${l.dot}`} />
                {l.label}
                <b className="ml-auto text-ink-900">{l.value}</b>
              </li>
            ))}
          </ul>
        </div>
      )}
    </Card>
  );
}
