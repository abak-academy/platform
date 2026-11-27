"use client";

import Link from "next/link";
import { BookOpen, Play } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Progress } from "@/components/ui/progress";
import { fileUrl } from "@/lib/api";
import { useTranslation } from "@/lib/i18n";
import { courseRowText } from "@/lib/student-home";
import type { Dashboard } from "@/lib/types";
import { DashCard } from "./DashCard";
import { Spot } from "./Spot";

export function ContinueLearningCard({ courses }: { courses: Dashboard["enrolled_courses"] }) {
  const { t } = useTranslation();

  return (
    <DashCard icon={BookOpen} tone="sky" title={t("stu_home_learn_title")} more={{ href: "/courses", label: t("stu_home_events_view_all") }}>
      {courses.length === 0 ? (
        <p className="text-ink-600">{t("stu_home_learn_empty")}</p>
      ) : (
        <ul className="m-0 mb-2 flex list-none flex-col p-0">
          {courses.map((c) => {
            const row = courseRowText(c);
            const cover = fileUrl(c.cover);
            return (
              <li key={c.id} className="border-t border-line first:border-t-0">
                <Link href={`/courses/${c.id}`} className="group flex items-center gap-3 py-2.5 text-ink-900">
                  {/* No subject/tone on the dashboard payload: brand gradient + the prototype's fallback spot. */}
                  <span
                    data-testid="learn-cover"
                    className="relative grid h-[58px] w-[76px] flex-none place-items-center overflow-hidden rounded-[12px] bg-[radial-gradient(120%_110%_at_100%_0%,var(--color-brand-400),var(--color-brand-600))]"
                  >
                    {cover ? <img src={cover} alt="" className="size-full object-cover" /> : <Spot className="size-[46px]" />}
                    <span className="absolute right-[5px] bottom-[5px] grid size-5 place-items-center rounded-full bg-white text-[#14202b]">
                      <Play className="size-2.5" />
                    </span>
                  </span>
                  <div className="flex min-w-0 flex-1 flex-col gap-[5px]">
                    <div className="flex items-baseline gap-2.5">
                      <b className="min-w-0 flex-1 truncate font-[family-name:var(--font-display)] text-[13.5px] font-bold group-hover:text-brand-ink">
                        {c.title}
                      </b>
                      <span className="font-[family-name:var(--font-display)] text-[13px] font-extrabold text-teal-700">{row.percent}%</span>
                    </div>
                    <small className="truncate text-[11.5px] text-ink-600">
                      {row.done
                        ? t("stu_home_learn_done")
                        : t("stu_home_learn_lesson_of").replace("{x}", String(row.lesson)).replace("{y}", String(row.total))}
                    </small>
                    <Progress value={row.percent} className="h-1.5" />
                  </div>
                </Link>
              </li>
            );
          })}
        </ul>
      )}
      <Button asChild variant="ghost" size="sm" className="w-full px-[13px]">
        <Link href="/catalog">{t("stu_home_learn_add")}</Link>
      </Button>
    </DashCard>
  );
}
