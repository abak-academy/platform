"use client";

import { CheckCircle2, PlayCircle } from "lucide-react";
import { useTranslation } from "@/lib/i18n";
import { Progress } from "@/components/ui/progress";
import type { CourseSection, Lesson } from "@/lib/types";

export interface LessonListProps {
  title: string;
  sections: (CourseSection & { lessons?: Lesson[] })[];
  activeLessonId?: string;
  onSelectLesson: (lessonId: string) => void;
  doneCount: number;
  totalCount: number;
  progressPct: number;
}

// Prototype .learn-side (.pad-sm + .cur-sec + .cur-item).
export function LessonList({
  title,
  sections,
  activeLessonId,
  onSelectLesson,
  doneCount,
  totalCount,
  progressPct,
}: LessonListProps) {
  const { t } = useTranslation();
  return (
    <aside className="min-w-0 rounded-[var(--radius-card)] border border-line bg-card p-1.5 shadow-card min-[1025px]:sticky min-[1025px]:top-5 min-[1025px]:max-h-[calc(100vh-40px)] min-[1025px]:overflow-y-auto">
      <div className="flex flex-col gap-2 p-4">
        <b className="text-[15px] font-bold text-ink-900">{title}</b>
        <div className="flex flex-wrap items-center justify-between gap-2 text-[13px] text-ink-600">
          <span>{t("course_done_of").replace("{x}", String(doneCount)).replace("{y}", String(totalCount))}</span>
          <b className="text-teal-700">{progressPct}%</b>
        </div>
        <Progress value={progressPct} />
      </div>
      {sections.map((section, si) => (
        <div key={section.id} className="border-t border-line py-1.5">
          <div className="px-3 py-2 font-mono text-[10.5px] leading-[normal] font-semibold tracking-[0.1em] text-ink-600 uppercase">
            {t("course_chapter").replace("{n}", String(si + 1))} · {section.title}
          </div>
          {section.lessons?.map((lesson) => {
            const isActive = lesson.id === activeLessonId;
            const isDone = Boolean(lesson.completed);
            const Icon = isDone ? CheckCircle2 : PlayCircle;
            return (
              <button
                key={lesson.id}
                type="button"
                onClick={() => onSelectLesson(lesson.id)}
                aria-current={isActive ? "true" : undefined}
                className={[
                  "flex w-full items-center gap-2 rounded-[14px] px-3 py-2.5 text-left text-[14px]",
                  isActive ? "bg-brand-50 font-bold text-brand-ink" : "text-ink-700 hover:bg-surface-sunken",
                ].join(" ")}
              >
                <Icon className={isDone ? "size-4 shrink-0 text-teal-700" : "size-4 shrink-0"} strokeWidth={1.75} />
                <span className="flex-1">{lesson.title}</span>
                {typeof lesson.duration_seconds === "number" && lesson.duration_seconds > 0 && (
                  <small className="shrink-0 text-[11.67px] text-ink-600">{Math.round(lesson.duration_seconds / 60)}′</small>
                )}
              </button>
            );
          })}
        </div>
      ))}
    </aside>
  );
}
