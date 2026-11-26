"use client";

import { useEffect, useState } from "react";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import type { Lesson, AdminCreateLessonInput, AdminUpdateLessonInput } from "@/lib/types";

interface LessonModalProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  lesson?: Lesson | null;
  onSubmit: (input: AdminCreateLessonInput | AdminUpdateLessonInput) => void | Promise<void>;
  isPending: boolean;
}

export function LessonModal({ open, onOpenChange, lesson, onSubmit, isPending }: LessonModalProps) {
  const isEdit = Boolean(lesson);
  const [title, setTitle] = useState("");
  const [videoUrl, setVideoUrl] = useState("");
  const [duration, setDuration] = useState("");

  useEffect(() => {
    if (open) {
      if (lesson) {
        setTitle(lesson.title ?? "");
        setVideoUrl(lesson.video_url ?? "");
        setDuration(lesson.duration_seconds != null ? String(lesson.duration_seconds) : "");
      } else {
        setTitle("");
        setVideoUrl("");
        setDuration("");
      }
    }
  }, [open, lesson]);

  const canSubmit = title.trim() !== "";

  function handleSubmit(e: React.FormEvent) {
    e.preventDefault();
    if (!canSubmit || isPending) return;

    const base = {
      title: title.trim(),
      video_url: videoUrl.trim() || undefined,
      duration: duration ? Number(duration) : undefined,
    };

    if (isEdit) {
      const input: AdminUpdateLessonInput = {
        ...base,
      };
      onSubmit(input);
      return;
    }

    const input: AdminCreateLessonInput = base;
    onSubmit(input);
  }

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="sm:max-w-[480px]">
        <form onSubmit={handleSubmit}>
          <DialogHeader>
            <DialogTitle>{isEdit ? "Edit materi" : "Tambah materi"}</DialogTitle>
            <DialogDescription>
              {isEdit ? "Perbarui detail materi." : "Tambahkan materi baru ke bab ini."}
            </DialogDescription>
          </DialogHeader>

          <div className="grid gap-4 py-4">
            <div className="grid gap-2">
              <Label htmlFor="lesson-title">Judul</Label>
              <Input
                id="lesson-title"
                value={title}
                onChange={(e) => setTitle(e.target.value)}
                placeholder="Judul materi"
                disabled={isPending}
              />
            </div>

            <div className="grid gap-2">
              <Label htmlFor="lesson-video">URL Video</Label>
              <Input
                id="lesson-video"
                value={videoUrl}
                onChange={(e) => setVideoUrl(e.target.value)}
                placeholder="https://..."
                disabled={isPending}
              />
            </div>

            <div className="grid gap-2">
              <Label htmlFor="lesson-duration">Durasi (detik)</Label>
              <Input
                id="lesson-duration"
                type="number"
                min={0}
                value={duration}
                onChange={(e) => setDuration(e.target.value)}
                placeholder="0"
                disabled={isPending}
              />
            </div>
          </div>

          <DialogFooter>
            <Button
              type="button"
              variant="outline"
              onClick={() => onOpenChange(false)}
              disabled={isPending}
            >
              Batal
            </Button>
            <Button type="submit" disabled={!canSubmit || isPending}>
              {isPending ? "Menyimpan..." : "Simpan"}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  );
}
