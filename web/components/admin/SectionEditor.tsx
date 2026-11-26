"use client";

import { useState } from "react";
import { toast } from "sonner";
import {
  useAdminSections,
  useCreateSection,
  useUpdateSection,
  useDeleteSection,
  useReorderSections,
  useCreateLesson,
  useUpdateLesson,
  useDeleteLesson,
  useReorderLessons,
} from "@/lib/hooks/admin-courses";
import { LessonModal } from "@/components/admin/LessonModal";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Skeleton } from "@/components/ui/skeleton";
import { ChevronUp, ChevronDown, Plus, Trash2, Edit2, Check, X } from "lucide-react";
import type { CourseSection, Lesson, AdminCreateLessonInput, AdminUpdateLessonInput } from "@/lib/types";

interface SectionEditorProps {
  courseId: string;
}

function errorMessage(error: unknown): string {
  if (error instanceof Error) return error.message;
  return "Terjadi kesalahan.";
}

function formatDuration(seconds?: number): string {
  if (seconds == null || seconds <= 0) return "-";
  const m = Math.floor(seconds / 60);
  const s = seconds % 60;
  return `${m}:${s.toString().padStart(2, "0")}`;
}

function SectionLessons({
  courseId,
  sectionId,
  lessons,
}: {
  courseId: string;
  sectionId: string;
  lessons: Lesson[];
}) {
  const [editLesson, setEditLesson] = useState<Lesson | null>(null);
  const [createOpen, setCreateOpen] = useState(false);
  const createLesson = useCreateLesson(courseId, sectionId);
  const updateLesson = useUpdateLesson(courseId, sectionId);
  const deleteLesson = useDeleteLesson(courseId, sectionId);
  const reorderLessons = useReorderLessons(courseId, sectionId);

  async function handleCreate(input: AdminCreateLessonInput | AdminUpdateLessonInput) {
    try {
      await createLesson.mutateAsync(input as AdminCreateLessonInput);
      toast.success("Materi ditambahkan.");
      setCreateOpen(false);
    } catch (e) {
      toast.error(errorMessage(e));
    }
  }

  async function handleUpdate(input: AdminCreateLessonInput | AdminUpdateLessonInput) {
    if (!editLesson) return;
    try {
      await updateLesson.mutateAsync({ lessonId: editLesson.id, input: input as AdminUpdateLessonInput });
      toast.success("Materi diperbarui.");
      setEditLesson(null);
    } catch (e) {
      toast.error(errorMessage(e));
    }
  }

  async function handleDelete(lessonId: string) {
    if (!confirm("Hapus materi ini?")) return;
    try {
      await deleteLesson.mutateAsync(lessonId);
      toast.success("Materi dihapus.");
    } catch (e) {
      toast.error(errorMessage(e));
    }
  }

  async function handleReorder(next: Lesson[]) {
    try {
      await reorderLessons.mutateAsync({ lesson_ids: next.map((l) => l.id) });
    } catch (e) {
      toast.error(errorMessage(e));
    }
  }

  function moveUp(index: number) {
    if (index <= 0) return;
    const next = [...lessons];
    const temp = next[index - 1];
    next[index - 1] = next[index];
    next[index] = temp;
    void handleReorder(next);
  }

  function moveDown(index: number) {
    if (index >= lessons.length - 1) return;
    const next = [...lessons];
    const temp = next[index + 1];
    next[index + 1] = next[index];
    next[index] = temp;
    void handleReorder(next);
  }

  return (
    <>
      {lessons.length === 0 ? (
        <p className="text-sm text-ink-600">Belum ada materi.</p>
      ) : (
        <ul className="space-y-2">
          {lessons.map((lesson, index) => (
            <li key={lesson.id} className="flex items-center justify-between gap-2 rounded-2xl bg-surface-sunken px-3.5 py-3">
              <div className="flex flex-1 items-center gap-3 min-w-0">
                <span className="text-xs text-ink-600">#{lesson.position ?? index + 1}</span>
                <span className="text-sm truncate">{lesson.title}</span>
                <span className="text-xs text-ink-600">{formatDuration(lesson.duration_seconds)}</span>
              </div>
              <div className="flex items-center gap-1">
                <Button size="icon-xs" variant="ghost" onClick={() => setEditLesson(lesson)}>
                  <Edit2 className="size-3" />
                </Button>
                <Button size="icon-xs" variant="ghost" onClick={() => handleDelete(lesson.id)} disabled={deleteLesson.isPending}>
                  <Trash2 className="size-3" />
                </Button>
                <Button size="icon-xs" variant="ghost" onClick={() => moveUp(index)} disabled={index === 0 || reorderLessons.isPending}>
                  <ChevronUp className="size-3" />
                </Button>
                <Button
                  size="icon-xs"
                  variant="ghost"
                  onClick={() => moveDown(index)}
                  disabled={index === lessons.length - 1 || reorderLessons.isPending}
                >
                  <ChevronDown className="size-3" />
                </Button>
              </div>
            </li>
          ))}
        </ul>
      )}

      <Button variant="outline" size="sm" className="mt-4" onClick={() => setCreateOpen(true)} disabled={createLesson.isPending}>
        <Plus className="mr-1 size-4" />
        Tambah materi
      </Button>

      <LessonModal open={createOpen} onOpenChange={setCreateOpen} isPending={createLesson.isPending} onSubmit={handleCreate} />

      <LessonModal
        open={Boolean(editLesson)}
        onOpenChange={(open) => {
          if (!open) setEditLesson(null);
        }}
        lesson={editLesson}
        isPending={updateLesson.isPending}
        onSubmit={handleUpdate}
      />
    </>
  );
}

export function SectionEditor({ courseId }: SectionEditorProps) {
  const [newSectionTitle, setNewSectionTitle] = useState("");
  const [editingSectionId, setEditingSectionId] = useState<string | null>(null);
  const [editingSectionTitle, setEditingSectionTitle] = useState("");

  const { data: sections, isLoading, isError, error } = useAdminSections(courseId);
  const createSection = useCreateSection(courseId);
  const updateSection = useUpdateSection(courseId);
  const deleteSection = useDeleteSection(courseId);
  const reorderSections = useReorderSections(courseId);

  async function handleAddSection(e: React.FormEvent) {
    e.preventDefault();
    const title = newSectionTitle.trim();
    if (!title) return;
    try {
      await createSection.mutateAsync({ title });
      toast.success("Bab ditambahkan.");
      setNewSectionTitle("");
    } catch (e) {
      toast.error(errorMessage(e));
    }
  }

  function startEditSection(section: CourseSection) {
    setEditingSectionId(section.id);
    setEditingSectionTitle(section.title);
  }

  function cancelEditSection() {
    setEditingSectionId(null);
    setEditingSectionTitle("");
  }

  async function saveEditSection(sectionId: string) {
    const title = editingSectionTitle.trim();
    if (!title) return;
    try {
      await updateSection.mutateAsync({ sectionId, input: { title } });
      toast.success("Bab diperbarui.");
      setEditingSectionId(null);
      setEditingSectionTitle("");
    } catch (e) {
      toast.error(errorMessage(e));
    }
  }

  async function handleDeleteSection(sectionId: string) {
    if (!confirm("Hapus bab ini beserta materi-nya?")) return;
    try {
      await deleteSection.mutateAsync(sectionId);
      toast.success("Bab dihapus.");
    } catch (e) {
      toast.error(errorMessage(e));
    }
  }

  async function handleReorderSections(newOrder: CourseSection[]) {
    try {
      await reorderSections.mutateAsync({ section_ids: newOrder.map((s) => s.id) });
    } catch (e) {
      toast.error(errorMessage(e));
    }
  }

  function moveSectionUp(index: number) {
    if (!sections || index <= 0) return;
    const next = [...sections];
    const temp = next[index - 1];
    next[index - 1] = next[index];
    next[index] = temp;
    void handleReorderSections(next);
  }

  function moveSectionDown(index: number) {
    if (!sections || index >= sections.length - 1) return;
    const next = [...sections];
    const temp = next[index + 1];
    next[index + 1] = next[index];
    next[index] = temp;
    void handleReorderSections(next);
  }

  if (isLoading) {
    return (
      <div className="space-y-4">
        {Array.from({ length: 3 }).map((_, i) => (
          <Skeleton key={i} className="h-24 w-full" />
        ))}
      </div>
    );
  }

  if (isError) {
    return (
      <div className="rounded-lg border border-destructive/20 bg-destructive/10 p-4 text-destructive">
        Gagal memuat kurikulum: {errorMessage(error)}
      </div>
    );
  }

  const sectionList = sections ?? [];

  return (
    <div className="space-y-6">
      <form onSubmit={handleAddSection} className="flex items-end gap-2">
        <div className="flex-1">
          <label htmlFor="new-section-title" className="mb-1 block text-sm font-medium">
            Bab baru
          </label>
          <Input
            id="new-section-title"
            value={newSectionTitle}
            onChange={(e) => setNewSectionTitle(e.target.value)}
            placeholder="Judul bab"
            disabled={createSection.isPending}
          />
        </div>
        <Button type="submit" disabled={!newSectionTitle.trim() || createSection.isPending}>
          <Plus className="mr-1 size-4" />
          Tambah bab
        </Button>
      </form>

      {sectionList.length === 0 && (
        <div className="rounded-2xl border border-line p-8 text-center text-ink-600">
          Belum ada bab. Tambahkan bab pertama di atas.
        </div>
      )}

      <div className="space-y-4">
        {sectionList.map((section, index) => {
          const isEditing = editingSectionId === section.id;
          return (
            <div key={section.id} className="rounded-[var(--radius-card)] border border-line bg-surface shadow-card">
              <div className="flex items-center justify-between gap-2 border-b border-line p-4">
                <div className="flex flex-1 items-center gap-2 min-w-0">
                  <span className="text-sm text-ink-600">#{section.position ?? index + 1}</span>
                  {isEditing ? (
                    <Input
                      value={editingSectionTitle}
                      onChange={(e) => setEditingSectionTitle(e.target.value)}
                      className="h-8"
                    />
                  ) : (
                    <h3 className="type-h3 truncate">{section.title}</h3>
                  )}
                </div>

                <div className="flex items-center gap-1">
                  {isEditing ? (
                    <>
                      <Button
                        size="icon-xs"
                        variant="ghost"
                        onClick={() => saveEditSection(section.id)}
                        disabled={!editingSectionTitle.trim() || updateSection.isPending}
                      >
                        <Check className="size-3" />
                      </Button>
                      <Button size="icon-xs" variant="ghost" onClick={cancelEditSection}>
                        <X className="size-3" />
                      </Button>
                    </>
                  ) : (
                    <>
                      <Button size="icon-xs" variant="ghost" onClick={() => startEditSection(section)}>
                        <Edit2 className="size-3" />
                      </Button>
                      <Button
                        size="icon-xs"
                        variant="ghost"
                        onClick={() => handleDeleteSection(section.id)}
                        disabled={deleteSection.isPending}
                      >
                        <Trash2 className="size-3" />
                      </Button>
                      <Button
                        size="icon-xs"
                        variant="ghost"
                        onClick={() => moveSectionUp(index)}
                        disabled={index === 0 || reorderSections.isPending}
                      >
                        <ChevronUp className="size-3" />
                      </Button>
                      <Button
                        size="icon-xs"
                        variant="ghost"
                        onClick={() => moveSectionDown(index)}
                        disabled={index === sectionList.length - 1 || reorderSections.isPending}
                      >
                        <ChevronDown className="size-3" />
                      </Button>
                    </>
                  )}
                </div>
              </div>

              <div className="p-4">
                <SectionLessons courseId={courseId} sectionId={section.id} lessons={section.lessons ?? []} />
              </div>
            </div>
          );
        })}
      </div>
    </div>
  );
}
