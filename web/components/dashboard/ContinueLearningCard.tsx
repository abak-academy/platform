"use client";

import Link from "next/link";
import { BookOpen, Play } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Card } from "@/components/ui/card";
import { Progress } from "@/components/ui/progress";
import { fileUrl } from "@/lib/api";
import { useTranslation } from "@/lib/i18n";
import { courseRowText } from "@/lib/student-home";
import type { Dashboard } from "@/lib/types";

export function ContinueLearningCard({ courses }: { courses: Dashboard["enrolled_courses"] }) {
  const { t } = useTranslation();
  const addButton = (
    <Button asChild variant="ghost" size="sm" className="w-full">
      <Link href="/catalog">{t("stu_home_learn_add")}</Link>
    </Button>
  );

  return (
    <Card className="flex flex-col gap-3.5 border-line p-5">
      <div className="flex items-center gap-2.5">
        <span className="flex size-9 items-center justify-center rounded-xl bg-sky-50 text-sky-600">
          <BookOpen className="size-5" />
        </span>
        <h2 className="flex-1 text-[15px] font-extrabold text-ink-900">{t("stu_home_learn_title")}</h2>
        <Link href="/courses" className="text-sm font-medium text-brand-700 hover:text-brand-800">
          {t("stu_home_events_view_all")}
        </Link>
      </div>
      {courses.length === 0 ? (
        <p className="text-sm text-ink-500">{t("stu_home_learn_empty")}</p>
      ) : (
        <ul className="m-0 flex list-none flex-col p-0">
          {courses.map((c) => {
            const row = courseRowText(c);
            const cover = fileUrl(c.cover);
            return (
              <li key={c.id} className="border-t border-line first:border-t-0">
                <Link href={`/courses/${c.id}`} className="flex items-center gap-3 py-2.5">
                  <span
                    data-testid="learn-cover"
                    className="relative flex h-[58px] w-[76px] flex-none items-center justify-center overflow-hidden rounded-xl bg-brand-50"
                  >
                    {cover && <img src={cover} alt="" className="size-full object-cover" />}
                    <span className="absolute bottom-1 right-1 flex size-5 items-center justify-center rounded-full bg-surface text-ink-900">
                      <Play className="size-2.5" />
                    </span>
                  </span>
                  <div className="flex min-w-0 flex-1 flex-col gap-1">
                    <div className="flex items-baseline gap-2.5">
                      <b className="min-w-0 flex-1 truncate text-[13.5px] font-bold text-ink-900">{c.title}</b>
                      <span className="text-[13px] font-extrabold text-teal-700">{row.percent}%</span>
                    </div>
                    <small className="truncate text-[11.5px] text-ink-500">
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
      {addButton}
    </Card>
  );
}
