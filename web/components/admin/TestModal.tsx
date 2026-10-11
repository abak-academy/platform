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
import { RichTextEditor } from "@/components/admin/RichTextEditor";
import { AudioUploadInput } from "@/components/admin/AudioUploadInput";
import { materialHTMLOrNull } from "@/lib/rich-text";
import { useTranslation } from "@/lib/i18n";
import type { Test, AdminCreateTestInput, AdminUpdateTestInput } from "@/lib/types";

interface TestModalProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  test?: Test | null;
  onSubmit: (input: AdminCreateTestInput | AdminUpdateTestInput) => void;
  isPending: boolean;
}

export function TestModal({ open, onOpenChange, test, onSubmit, isPending }: TestModalProps) {
  const { t } = useTranslation();
  const isEdit = Boolean(test);
  const [sharedMaterial, setSharedMaterial] = useState("");
  const [instructions, setInstructions] = useState("");
  const [title, setTitle] = useState("");
  const [subject, setSubject] = useState("");
  const [topic, setTopic] = useState("");
  const [duration, setDuration] = useState("");
  const [audioUrl, setAudioUrl] = useState("");
  const [audioPlayLimit, setAudioPlayLimit] = useState("");
  const [sectionType, setSectionType] = useState<string | null>(null);

  useEffect(() => {
    if (!open) return;
    if (test) {
      setSharedMaterial(test.shared_material_html ?? "");
      setInstructions(test.instructions_html ?? "");
      setTitle(test.title ?? "");
      setSubject(test.subject ?? "");
      setTopic(test.topic ?? "");
      setDuration(test.duration_minutes != null ? String(test.duration_minutes) : "");
      setAudioUrl(test.audio_url ?? "");
      setAudioPlayLimit(test.audio_play_limit != null ? String(test.audio_play_limit) : "");
      setSectionType(test.section_type ?? null);
    } else {
      setSharedMaterial("");
      setInstructions("");
      setTitle("");
      setSubject("");
      setTopic("");
      setDuration("");
      setAudioUrl("");
      setAudioPlayLimit("");
      setSectionType(null);
    }
  }, [open, test]);

  const listeningRequiresAudio = sectionType === "listening" && audioUrl.trim() === "";
  const canSubmit =
    title.trim() !== "" &&
    subject.trim() !== "" &&
    topic.trim() !== "" &&
    duration !== "" &&
    Number(duration) > 0 &&
    !listeningRequiresAudio &&
    !isPending;

  function handleSubmit(e: React.FormEvent) {
    e.preventDefault();
    if (!canSubmit || isPending) return;

    const sectionTypePayload = sectionType ? { section_type: sectionType } : {};

    if (isEdit) {
      const input: AdminUpdateTestInput = {
        title: title.trim(),
        subject: subject.trim(),
        topic: topic.trim(),
        duration_minutes: Number(duration),
        ...(audioUrl.trim() ? { audio_url: audioUrl.trim() } : {}),
        ...(audioPlayLimit !== "" ? { audio_play_limit: Number(audioPlayLimit) } : {}),
        ...sectionTypePayload,
        shared_material_html: materialHTMLOrNull(sharedMaterial),
        instructions_html: materialHTMLOrNull(instructions),
      };
      onSubmit(input);
      return;
    }

    const input: AdminCreateTestInput = {
      title: title.trim(),
      subject: subject.trim(),
      topic: topic.trim(),
      duration_minutes: Number(duration),
      ...(audioUrl.trim() ? { audio_url: audioUrl.trim() } : {}),
      ...(audioPlayLimit !== "" ? { audio_play_limit: Number(audioPlayLimit) } : {}),
      ...sectionTypePayload,
        shared_material_html: materialHTMLOrNull(sharedMaterial),
        instructions_html: materialHTMLOrNull(instructions),
    };
    onSubmit(input);
  }

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="max-h-[90dvh] overflow-y-auto sm:max-w-2xl">
        <form onSubmit={handleSubmit}>
          <DialogHeader>
            <DialogTitle>{isEdit ? t("tests_edit") : t("tests_new")}</DialogTitle>
            <DialogDescription>
              {isEdit
                ? "Perbarui metadata tes."
                : "Tambahkan tes baru ke bank soal."}
            </DialogDescription>
          </DialogHeader>

          <div className="grid gap-4 py-4">
            <div className="grid gap-2">
              <Label htmlFor="test-title">{t("tests_field_title")}</Label>
              <Input
                id="test-title"
                value={title}
                onChange={(e) => setTitle(e.target.value)}
                placeholder={t("tests_field_title")}
                disabled={isPending}
              />
            </div>

            <div className="grid grid-cols-2 gap-4">
              <div className="grid gap-2">
                <Label htmlFor="test-subject">{t("tests_field_subject")}</Label>
                <Input
                  id="test-subject"
                  value={subject}
                  onChange={(e) => setSubject(e.target.value)}
                  placeholder={t("tests_field_subject")}
                  disabled={isPending}
                />
              </div>

              <div className="grid gap-2">
                <Label htmlFor="test-topic">{t("tests_field_topic")}</Label>
                <Input
                  id="test-topic"
                  value={topic}
                  onChange={(e) => setTopic(e.target.value)}
                  placeholder={t("tests_field_topic")}
                  disabled={isPending}
                />
              </div>
            </div>

            <div className="grid gap-2">
              <Label htmlFor="test-duration">{t("tests_field_duration")}</Label>
              <Input
                id="test-duration"
                type="number"
                min={1}
                value={duration}
                onChange={(e) => setDuration(e.target.value)}
                placeholder="60"
                disabled={isPending}
              />
            </div>

            <div className="grid gap-2">
              <Label htmlFor="test-shared-material">{t("format_shared_material")}</Label>
              <RichTextEditor id="test-shared-material" aria-label={t("format_shared_material")} value={sharedMaterial} onChange={setSharedMaterial} disabled={isPending} />
              <p className="text-xs text-ink-500">{t("format_reusable_material_hint")}</p>
            </div>
            <div className="grid gap-2">
              <Label htmlFor="test-instructions">{t("format_instructions")}</Label>
              <RichTextEditor id="test-instructions" aria-label={t("format_instructions")} value={instructions} onChange={setInstructions} disabled={isPending} />
            </div>
            <div className="grid gap-2">
              <Label>{t("tests_field_section_type")}</Label>
              <div className="grid grid-cols-4 gap-2">
                <label className="flex items-center gap-2 rounded-md border border-input px-3 py-2 text-sm">
                  <input
                    type="radio"
                    name="section_type"
                    value=""
                    checked={sectionType === null}
                    onChange={() => setSectionType(null)}
                    disabled={isPending}
                  />
                  <span>{t("tests_field_section_type_none")}</span>
                </label>
                <label className="flex items-center gap-2 rounded-md border border-input px-3 py-2 text-sm">
                  <input
                    type="radio"
                    name="section_type"
                    value="listening"
                    checked={sectionType === "listening"}
                    onChange={() => setSectionType("listening")}
                    disabled={isPending}
                  />
                  <span>{t("tests_field_section_type_listening")}</span>
                </label>
                <label className="flex items-center gap-2 rounded-md border border-input px-3 py-2 text-sm">
                  <input
                    type="radio"
                    name="section_type"
                    value="reading"
                    checked={sectionType === "reading"}
                    onChange={() => setSectionType("reading")}
                    disabled={isPending}
                  />
                  <span>{t("tests_field_section_type_reading")}</span>
                </label>
                <label className="flex items-center gap-2 rounded-md border border-input px-3 py-2 text-sm">
                  <input
                    type="radio"
                    name="section_type"
                    value="writing"
                    checked={sectionType === "writing"}
                    onChange={() => setSectionType("writing")}
                    disabled={isPending}
                  />
                  <span>{t("tests_field_section_type_writing")}</span>
                </label>
              </div>
              {sectionType === "listening" && (
                <p className="text-xs text-muted-foreground">
                  URL audio wajib diisi untuk sesi Listening.
                </p>
              )}
            </div>

            <div className="grid grid-cols-2 gap-4">
              <div className="grid gap-2">
                <Label htmlFor="test-audio-url">{t("tests_field_audio_url")}</Label>
                <AudioUploadInput
                  id="test-audio-url"
                  value={audioUrl}
                  onChange={setAudioUrl}
                  placeholder="https://..."
                  disabled={isPending}
                />
              </div>

              <div className="grid gap-2">
                <Label htmlFor="test-audio-play-limit">{t("tests_field_audio_play_limit")}</Label>
                <Input
                  id="test-audio-play-limit"
                  type="number"
                  min={0}
                  value={audioPlayLimit}
                  onChange={(e) => setAudioPlayLimit(e.target.value)}
                  placeholder="0"
                  disabled={isPending}
                />
              </div>
            </div>
          </div>

          <DialogFooter>
            <Button
              type="button"
              variant="outline"
              onClick={() => onOpenChange(false)}
              disabled={isPending}
            >
              {t("cancel")}
            </Button>
            <Button type="submit" disabled={!canSubmit || isPending}>
              {isPending ? t("saving") : t("save")}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  );
}