"use client";

import { useState } from "react";
import { useRouter } from "next/navigation";
import { toast } from "sonner";
import { useAdminCourses, useCreateCourse } from "@/lib/hooks/admin-courses";
import { useTranslation } from "@/lib/i18n";
import { Library, Plus } from "lucide-react";
import { AdminPageHeader } from "@/components/admin/AdminPageHeader";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { EmptyState } from "@/components/ui/empty-state";
import { ListToolbar, type ListToolbarValue } from "@/components/admin/ListToolbar";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Skeleton } from "@/components/ui/skeleton";
import type { Course, AdminCreateCourseInput, EventIllustration } from "@/lib/types";
import { Spot } from "@/components/dashboard/Spot";
import { AbakAvatar } from "@/components/ui/abak-avatar";
import type { DICT } from "@/lib/i18n";

// Prototype SUBJ tones and GLYPHS, keyed by subject.
const TONES = {
  brand: { a: "#3d4ddb", b: "#6e7cee" }, teal: { a: "#1e978a", b: "#4cc3b2" }, sky: { a: "#2f6fde", b: "#6fa2f2" },
  berry: { a: "#c23b6b", b: "#ec7ea4" }, gold: { a: "#e3a131", b: "#f5c868" }, navy: { a: "#22315b", b: "#3c5390" },
};
const SUBJECT_TONE: Record<string, keyof typeof TONES> = {
  Matematika: "brand", Penalaran: "brand", Sains: "teal", Fisika: "teal", Kimia: "teal",
  "Bahasa Inggris": "sky", Literasi: "berry", "Bahasa Indonesia": "berry", Coding: "gold", AI: "navy",
};
const GLYPH: Record<string, string> = {
  Matematika: "∑", Penalaran: "∞", Sains: "⚛", Fisika: "F", Kimia: "K", "Bahasa Inggris": "Aa", Literasi: "“", "Bahasa Indonesia": "“", Coding: "</>", AI: "◎",
};
const SUBJECT_SPOT: Record<string, EventIllustration> = {
  Matematika: "math", Penalaran: "math", Sains: "science", Fisika: "science", Kimia: "science",
  "Bahasa Inggris": "language", Literasi: "language", "Bahasa Indonesia": "language", Coding: "coding", AI: "coding",
};
const tone = (course: Course) => TONES[SUBJECT_TONE[course.subject ?? ""] ?? "gold"];
const subjectSpot = (subject?: string) => (subject ? SUBJECT_SPOT[subject] : undefined);

type TFunc = (key: keyof (typeof DICT)["id"]) => string;

function CreateCourseModal({
  open,
  onOpenChange,
  onSubmit,
  isPending,
  t,
}: {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  onSubmit: (input: AdminCreateCourseInput) => void;
  isPending: boolean;
  t: TFunc;
}) {
  const [title, setTitle] = useState("");
  const [level, setLevel] = useState("");
  const [subject, setSubject] = useState("");
  const [instructorName, setInstructorName] = useState("");

  function handleSubmit(e: React.FormEvent) {
    e.preventDefault();
    if (!title.trim() || isPending) return;
    onSubmit({
      title: title.trim(),
      level: level.trim() || undefined,
      subject: subject.trim() || undefined,
      instructor_name: instructorName.trim() || undefined,
    });
  }

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="sm:max-w-[480px]">
        <form onSubmit={handleSubmit}>
          <DialogHeader>
            <DialogTitle>{t("courses_create")}</DialogTitle>
            <DialogDescription>{t("courses_create_desc")}</DialogDescription>
          </DialogHeader>

          <div className="grid gap-4 py-4">
            <div className="grid gap-2">
              <Label htmlFor="course-title">{t("th_title")}</Label>
              <Input
                id="course-title"
                value={title}
                onChange={(e) => setTitle(e.target.value)}
                placeholder={t("course_title_placeholder")}
                disabled={isPending}
              />
            </div>

            <div className="grid grid-cols-2 gap-4">
              <div className="grid gap-2">
                <Label htmlFor="course-level">{t("th_level")}</Label>
                <Input
                  id="course-level"
                  value={level}
                  onChange={(e) => setLevel(e.target.value)}
                  placeholder={t("course_level_placeholder")}
                  disabled={isPending}
                />
              </div>

              <div className="grid gap-2">
                <Label htmlFor="course-subject">{t("subject")}</Label>
                <Input
                  id="course-subject"
                  value={subject}
                  onChange={(e) => setSubject(e.target.value)}
                  placeholder={t("course_subject_placeholder")}
                  disabled={isPending}
                />
              </div>
            </div>

            <div className="grid gap-2">
              <Label htmlFor="course-instructor">{t("th_instructor")}</Label>
              <Input
                id="course-instructor"
                value={instructorName}
                onChange={(e) => setInstructorName(e.target.value)}
                placeholder={t("course_instructor_placeholder")}
                disabled={isPending}
              />
            </div>
          </div>

          <DialogFooter>
            <Button type="button" variant="outline" onClick={() => onOpenChange(false)} disabled={isPending}>
              {t("cancel")}
            </Button>
            <Button type="submit" disabled={!title.trim() || isPending}>
              {isPending ? t("saving") : t("save")}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  );
}

export default function CoursesPage() {
  const { t } = useTranslation();
  const router = useRouter();
  const [modalOpen, setModalOpen] = useState(false);
  const { data: courses, isLoading, isError, error } = useAdminCourses();
  const create = useCreateCourse();

  function errorMessage(error: unknown): string {
    if (error instanceof Error) return error.message;
    return t("error_generic");
  }

  async function handleCreate(input: AdminCreateCourseInput) {
    try {
      await create.mutateAsync(input);
      toast.success(t("courses_created"));
      setModalOpen(false);
    } catch (e) {
      toast.error(errorMessage(e));
    }
  }

  function openCourse(course: Course) {
    router.push(`/admin/courses/${course.id}`);
  }

  const [toolbar, setToolbar] = useState<ListToolbarValue>({ q: "", filters: {} });
  const all = courses ?? [];
  const distinct = (pick: (c: Course) => string | undefined) =>
    [...new Set(all.map(pick).filter((v): v is string => !!v))].sort().map((v) => ({ value: v, label: v }));
  const shown = all.filter((c) => {
    const q = toolbar.q.trim().toLowerCase();
    return (
      (!q || c.title.toLowerCase().includes(q) || (c.instructor_name ?? "").toLowerCase().includes(q)) &&
      (!toolbar.filters.level || c.level === toolbar.filters.level) &&
      (!toolbar.filters.subject || c.subject === toolbar.filters.subject)
    );
  });

  return (
    <div className="flex flex-col gap-5 fade-in">
      <AdminPageHeader
        title={t("courses_page_title")}
        description={`${all.length} ${t("courses_unit")}`}
        actions={<Button variant="dark" onClick={() => setModalOpen(true)}><Plus aria-hidden />{t("courses_create")}</Button>}
      />

      {!isLoading && !isError && (
        <ListToolbar
          variant="standalone"
          value={toolbar}
          onChange={setToolbar}
          count={shown.length}
          unit={t("courses_unit")}
          searchPlaceholder={t("courses_search")}
          filters={[
            { key: "level", label: t("th_level"), options: distinct((c) => c.level) },
            { key: "subject", label: t("subject"), options: distinct((c) => c.subject) },
          ]}
        />
      )}

      {isLoading && (
        <div className="flex flex-col gap-2">
          {Array.from({ length: 5 }).map((_, i) => (
            <Skeleton key={i} className="h-12 w-full" />
          ))}
        </div>
      )}

      {isError && (
        <div className="rounded-lg border border-destructive/20 bg-destructive/10 p-4 text-destructive">
          {t("courses_load_failed")}: {errorMessage(error)}
        </div>
      )}

      {!isLoading && !isError && shown.length === 0 && <EmptyState framed icon={Library} title={t("empty_courses")} />}

      {!isLoading && !isError && shown.length > 0 && (
        <div className="grid gap-[18px] [grid-template-columns:repeat(auto-fill,minmax(255px,1fr))]">
          {shown.map((course) => (
            <button
              key={course.id}
              type="button"
              onClick={() => openCourse(course)}
              className="md-card-flush group flex flex-col text-left transition-all duration-300 ease-spring hover:-translate-y-[5px] hover:shadow-pop focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-brand-600"
            >
              <span
                className="relative grid h-[120px] place-items-center overflow-hidden"
                style={{ background: `radial-gradient(120% 110% at 100% 0%, ${tone(course).b}, ${tone(course).a})` }}
              >
                <Spot illustration={subjectSpot(course.subject)} className="relative z-[1] size-[84px] drop-shadow-[0_10px_14px_rgba(0,0,0,0.18)]" />
                <span aria-hidden className="absolute -right-2.5 -bottom-[34px] font-[family-name:var(--font-display)] text-[150px] leading-none font-extrabold tracking-[-0.06em] text-white/16">
                  {GLYPH[course.subject ?? ""] ?? course.title.charAt(0)}
                </span>
                {course.subject && <Badge variant="teal" className="absolute top-3 left-3 z-[2]">{course.subject}</Badge>}
              </span>
              <span className="flex flex-1 flex-col gap-2 px-[18px] pt-4 pb-[18px]">
                <h3 className="font-[family-name:var(--font-display)] text-base leading-[1.35] font-bold tracking-[-0.02em] text-ink-900">{course.title}</h3>
                <span className="flex items-center gap-2 text-[13px] text-ink-600">
                  <AbakAvatar name={course.instructor_name || "Pengajar"} size={24} />
                  <span className="truncate">{course.instructor_name ?? "—"}</span>
                </span>
                <span className="flex items-center justify-between gap-2 text-[13px] text-ink-600">
                  <span>{course.level ?? "—"}</span>
                </span>
              </span>
            </button>
          ))}
        </div>
      )}

      <CreateCourseModal
        open={modalOpen}
        onOpenChange={setModalOpen}
        onSubmit={handleCreate}
        isPending={create.isPending}
        t={t}
      />
    </div>
  );
}
