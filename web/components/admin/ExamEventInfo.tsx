"use client";

import { useState } from "react";
import { BookOpen, Users } from "lucide-react";
import { toast } from "sonner";
import { Button } from "@/components/ui/button";
import { Label } from "@/components/ui/label";
import { Input } from "@/components/ui/input";
import { useUpdateExam } from "@/lib/hooks/admin-exams";
import { isHttpUrl, isWhatsAppGroupUrl } from "@/lib/exam-event-info";
import { useTranslation } from "@/lib/i18n";
import type { ExamDetail } from "@/lib/types";

// Mirrors maxAdditionalRulesNote in backend/internal/service/exam.go.
const MAX_RULES_NOTE = 500;

export function ExamWhatsAppCard({ url, canManage = true }: { url?: string | null; canManage?: boolean }) {
  const { t } = useTranslation();
  const validUrl = url && isWhatsAppGroupUrl(url) ? url : null;
  return (
    <section className="md-card-outlined min-w-0 space-y-3 p-6" aria-labelledby="exam-wa-card-title">
      <div className="flex items-start gap-3">
        <Users className="mt-0.5 size-5 shrink-0 text-ink-500" />
        <div>
          <h2 id="exam-wa-card-title" className="text-title-medium font-semibold">
            {t("exam_event_wa_card_title")}
          </h2>
          <p className="text-sm text-ink-500">{t("exam_event_wa_card_subtitle")}</p>
        </div>
      </div>
      {validUrl ? (
        <div className="space-y-1 text-sm">
          <span className="inline-block rounded-full bg-success-bg px-2 py-0.5 text-xs font-medium text-success">
            {t("exam_event_wa_available")}
          </span>
          <a
            href={validUrl}
            target="_blank"
            rel="noopener noreferrer"
            className="block font-medium [overflow-wrap:anywhere] text-brand-700 hover:underline"
          >
            {validUrl} ↗
          </a>
          <p className="text-ink-500">{t("exam_event_wa_available_hint")}</p>
        </div>
      ) : (
        <div className="space-y-1 text-sm">
          <p className="font-semibold">{t("exam_event_wa_missing")}</p>
          <p className="text-ink-500">{t("exam_event_wa_missing_hint")}</p>
        </div>
      )}
      {canManage && <p className="border-t border-line pt-3 text-xs text-ink-500">{t("exam_event_wa_manage")}</p>}
    </section>
  );
}

export function ExamAdditionalRulesCard({
  exam,
  summary,
  footnote,
}: {
  exam: ExamDetail;
  summary?: React.ReactNode;
  footnote?: React.ReactNode;
}) {
  const { t } = useTranslation();
  const update = useUpdateExam(exam.id);

  const [stored, setStored] = useState({ note: exam.additional_rules_note ?? "", url: exam.additional_rules_url ?? "" });
  const [syncedFrom, setSyncedFrom] = useState({ note: exam.additional_rules_note, url: exam.additional_rules_url });
  const hasStored = stored.note !== "" || stored.url !== "";
  const [editing, setEditing] = useState(false);
  const [enabled, setEnabled] = useState(hasStored);
  const [note, setNote] = useState(stored.note);
  const [url, setUrl] = useState(stored.url);
  const [error, setError] = useState<string | null>(null);

  if (syncedFrom.note !== exam.additional_rules_note || syncedFrom.url !== exam.additional_rules_url) {
    setSyncedFrom({ note: exam.additional_rules_note, url: exam.additional_rules_url });
    const next = { note: exam.additional_rules_note ?? "", url: exam.additional_rules_url ?? "" };
    setStored(next);
    if (!editing) resetDraft(next);
  }

  function resetDraft(from: { note: string; url: string }) {
    setNote(from.note);
    setUrl(from.url);
    setEnabled(from.note !== "" || from.url !== "");
    setError(null);
  }

  function handleCancel() {
    resetDraft(stored);
    setEditing(false);
  }

  async function handleSave() {
    const nextNote = enabled ? note.trim() : "";
    const nextUrl = enabled ? url.trim() : "";
    if (enabled && nextUrl !== "" && !isHttpUrl(nextUrl)) {
      setError(t("exam_event_rules_url_invalid"));
      return;
    }
    if (enabled && nextNote === "" && nextUrl === "") {
      setError(t("exam_event_rules_empty"));
      return;
    }
    setError(null);
    try {
      await update.mutateAsync({
        additional_rules_note: nextNote || null,
        additional_rules_url: nextUrl || null,
      });
      const next = { note: nextNote, url: nextUrl };
      setStored(next);
      resetDraft(next);
      setEditing(false);
      toast.success(t("exam_event_rules_saved"));
    } catch (err) {
      toast.error(err instanceof Error ? err.message : t("error_generic"));
    }
  }

  const showForm = editing || !hasStored;
  const formOpen = enabled && showForm;

  return (
    <section className="md-card-outlined min-w-0 space-y-3 p-4" aria-labelledby="exam-rules-card-title">
      <h3 id="exam-rules-card-title" className="text-title-medium font-semibold">
        {t("exam_event_rules_title")}
      </h3>

      {summary}

      {!showForm ? (
        <div className="flex items-start gap-3 text-sm">
          <BookOpen className="mt-0.5 size-4 shrink-0 text-ink-500" />
          <div className="min-w-0 flex-1 space-y-1">
            <div className="flex items-center justify-between gap-2">
              <strong>{t("exam_event_rules_saved_title")}</strong>
              <Button
                type="button"
                variant="ghost"
                size="sm"
                onClick={() => {
                  resetDraft(stored);
                  setEditing(true);
                }}
              >
                {t("exam_event_rules_edit")}
              </Button>
            </div>
            {stored.note && <p className="whitespace-pre-line [overflow-wrap:anywhere] text-ink-600">{stored.note}</p>}
            {stored.url && isHttpUrl(stored.url) && (
              <a
                href={stored.url}
                target="_blank"
                rel="noopener noreferrer"
                className="block font-medium [overflow-wrap:anywhere] text-brand-700 hover:underline"
              >
                {t("exam_event_rules_link")}
              </a>
            )}
          </div>
        </div>
      ) : (
        <form
          noValidate
          className="space-y-3"
          onSubmit={(e) => {
            e.preventDefault();
            void handleSave();
          }}
        >
          <label className="flex items-center gap-2 text-sm">
            <input
              type="checkbox"
              checked={enabled}
              onChange={(e) => {
                setEnabled(e.target.checked);
                setError(null);
              }}
              disabled={update.isPending}
            />
            <span>{t("exam_event_rules_toggle")}</span>
          </label>
          {formOpen && (
            <div className="space-y-3">
              <p className="text-xs text-ink-500">{t("exam_event_rules_fields_hint")}</p>
              <div className="grid gap-2">
                <Label htmlFor="exam-rules-note">{t("exam_event_rules_note_label")}</Label>
                <textarea
                  id="exam-rules-note"
                  data-slot="textarea"
                  value={note}
                  onChange={(e) => setNote(e.target.value)}
                  maxLength={MAX_RULES_NOTE}
                  rows={3}
                  placeholder={t("exam_event_rules_note_placeholder")}
                  disabled={update.isPending}
                  className="flex w-full rounded-md border border-input bg-transparent px-3 py-2 text-sm shadow-xs focus-visible:border-ring focus-visible:ring-[3px] focus-visible:ring-brand-300/50 disabled:pointer-events-none disabled:opacity-50"
                />
              </div>
              <div className="grid gap-2">
                <Label htmlFor="exam-rules-url">{t("exam_event_rules_url_label")}</Label>
                <Input
                  id="exam-rules-url"
                  type="url"
                  value={url}
                  onChange={(e) => setUrl(e.target.value)}
                  placeholder="https://..."
                  disabled={update.isPending}
                />
              </div>
            </div>
          )}
          {error && (
            <p role="alert" className="text-sm text-danger">
              {error}
            </p>
          )}
          {(enabled || hasStored) && (
            <div className="flex flex-wrap gap-2">
              <Button type="submit" size="sm" className="rounded-full" disabled={update.isPending}>
                {update.isPending ? t("saving") : t("exam_event_rules_save")}
              </Button>
              <Button type="button" variant="outline" size="sm" className="rounded-full" onClick={handleCancel} disabled={update.isPending}>
                {t("cancel")}
              </Button>
            </div>
          )}
          <p className="text-xs text-ink-500">{t("exam_event_rules_separate_save")}</p>
        </form>
      )}

      {footnote && <p className="border-t border-line pt-3 text-xs text-ink-500">{footnote}</p>}
    </section>
  );
}
