"use client";

import { CheckCircle2, PlayCircle } from "lucide-react";
import { Progress } from "@/components/ui/progress";
import type { CourseSection, Lesson } from "@/lib/types";

export interface LessonListProps {
  sections: (CourseSection & { lessons?: Lesson[] })[];
  activeLessonId?: string;
  onSelectLesson: (lessonId: string) => void;
  doneCount: number;
  totalCount: number;
  progressPct: number;
}

export function LessonList({
  sections,
  activeLessonId,
  onSelectLesson,
  doneCount,
  totalCount,
  progressPct,
}: LessonListProps) {
  return (
    <div className="flex max-h-[calc(100vh-40px)] flex-col overflow-y-auto rounded-[var(--radius-card)] border border-line bg-card p-1.5 shadow-card lg:sticky lg:top-5">
      <div className="px-3.5 pt-3 pb-3.5">
        <h3 className="type-label text-ink-900">Daftar Pelajaran</h3>
        <div className="mt-1 mb-2 flex items-center justify-between text-xs text-ink-600">
          <span>
            {doneCount}/{totalCount}
          </span>
          <b className="text-teal-700">{progressPct}%</b>
        </div>
        <Progress value={progressPct} />
      </div>
      {sections.map((section) => (
        <div key={section.id} className="border-t border-line py-1.5">
          <div className="px-3 py-1.5 font-mono text-[10.5px] font-semibold tracking-[0.1em] text-ink-600 uppercase">
            {section.title}
          </div>
          {section.lessons?.map((lesson) => {
            const isActive = lesson.id === activeLessonId;
            const isDone = Boolean(lesson.completed);
            return (
              <button
                key={lesson.id}
                type="button"
                onClick={() => onSelectLesson(lesson.id)}
                className={[
                  "flex w-full items-center gap-2 rounded-[14px] px-3 py-2.5 text-left text-sm transition-colors",
                  isActive ? "bg-brand-50 font-bold text-brand-ink" : "text-ink-700 hover:bg-surface-sunken",
                ].join(" ")}
              >
                {isDone ? (
                  <CheckCircle2 className="size-4 shrink-0 text-teal-600" />
                ) : (
                  <PlayCircle className="size-4 shrink-0 text-ink-600" />
                )}
                <span className="flex-1">{lesson.title}</span>
                {typeof lesson.duration_seconds === "number" && lesson.duration_seconds > 0 && (
                  <small className="shrink-0 text-xs text-ink-600">{Math.round(lesson.duration_seconds / 60)}′</small>
                )}
              </button>
            );
          })}
        </div>
      ))}
    </div>
  );
}
