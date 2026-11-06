"use client";

import { useEffect, useLayoutEffect, useMemo, useRef, useState } from "react";
import { toast } from "sonner";
import { Check, Search } from "lucide-react";
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
import { Badge } from "@/components/ui/badge";
import { useBankQuestions } from "@/lib/hooks/admin-bank-questions";
import { useTopics } from "@/lib/hooks/admin-topics";
import { useDebouncedValue } from "@/lib/hooks/use-debounced-value";
import { useTranslation } from "@/lib/i18n";
import { stripHtmlToPlainText } from "@/lib/rich-text";
import type { QuestionFormat, QuestionWithOptions } from "@/lib/types";

const FORMAT_LABELS: Record<QuestionFormat, string> = {
  mcq: "fmt_mcq",
  multi_answer: "fmt_multi_answer",
  short: "fmt_short",
  fill_blank: "fmt_fill_blank",
  essay: "fmt_essay",
  multi_blank: "fmt_multi_blank",
  true_false: "fmt_true_false",
};

const DIFFICULTY_LABEL: Record<string, string> = {
  easy: "diff_easy",
  medium: "diff_medium",
  hard: "diff_hard",
};

// Was ["mcq","multi_answer","short","fill_blank","essay"] — stale twice over:
// it still offered the two retired formats and never offered multi_blank or
// true_false at all, so the picker couldn't filter by the formats that
// actually exist going forward.
const ALL_FORMATS: QuestionFormat[] = ["mcq", "multi_answer", "essay", "multi_blank", "true_false"];

interface QuestionPickerModalProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  testId: string;
  attached: QuestionWithOptions[];
  onAttach: (questionIds: string[]) => Promise<void>;
}

function SelectionBox({ selected }: { selected: boolean }) {
  return (
    <span
      aria-hidden="true"
      className={`flex size-5 flex-shrink-0 items-center justify-center rounded-md border ${
        selected
          ? "border-primary bg-primary text-primary-foreground"
          : "border-input bg-surface"
      }`}
    >
      {selected && <Check className="size-3" />}
    </span>
  );
}

function ClampedText({
  text,
  expanded,
  onOverflow,
}: {
  text: string;
  expanded: boolean;
  onOverflow: (overflowing: boolean) => void;
}) {
  const ref = useRef<HTMLParagraphElement>(null);

  useLayoutEffect(() => {
    const el = ref.current;
    if (!el || expanded) return;
    const measure = () => onOverflow(el.scrollHeight > el.clientHeight);
    measure();
    if (typeof ResizeObserver === "undefined") return;
    const observer = new ResizeObserver(measure);
    observer.observe(el);
    return () => observer.disconnect();
  }, [text, expanded, onOverflow]);

  return (
    <p
      ref={ref}
      className={`min-w-0 text-sm text-ink-900 [overflow-wrap:anywhere] ${expanded ? "" : "line-clamp-3"}`}
    >
      {text}
    </p>
  );
}

export function QuestionPickerModal({
  open,
  onOpenChange,
  attached,
  onAttach,
}: QuestionPickerModalProps) {
  const { t } = useTranslation();
  const [search, setSearch] = useState("");
  // Search is a server filter, so it must be debounced — otherwise every
  // keystroke fires a new request for a full bank page.
  const debouncedSearch = useDebouncedValue(search);
  const [format, setFormat] = useState<QuestionFormat | "all">("all");
  const [topicId, setTopicId] = useState<string>("all");
  const [selected, setSelected] = useState<Set<string>>(new Set());
  const [isAttaching, setIsAttaching] = useState(false);
  const [expanded, setExpanded] = useState<Set<string>>(new Set());
  const [overflowing, setOverflowing] = useState<Set<string>>(new Set());

  const filters = useMemo(
    () => ({
      search: debouncedSearch.trim() || undefined,
      format: format === "all" ? undefined : format,
      topic_id: topicId === "all" ? undefined : topicId,
      limit: 50,
    }),
    [debouncedSearch, format, topicId]
  );

  const bank = useBankQuestions(filters);
  const topics = useTopics();

  const attachedIds = useMemo(
    () => new Set(attached.map((a) => a.question.id)),
    [attached]
  );

  useEffect(() => {
    if (!open) {
      setSearch("");
      setFormat("all");
      setTopicId("all");
      setSelected(new Set());
      setExpanded(new Set());
    }
  }, [open]);

  function markOverflow(id: string, value: boolean) {
    setOverflowing((prev) => {
      if (prev.has(id) === value) return prev;
      const next = new Set(prev);
      if (value) next.add(id);
      else next.delete(id);
      return next;
    });
  }

  function toggleExpanded(id: string) {
    setExpanded((prev) => {
      const next = new Set(prev);
      if (next.has(id)) next.delete(id);
      else next.add(id);
      return next;
    });
  }

  function toggle(id: string) {
    setSelected((prev) => {
      const next = new Set(prev);
      if (next.has(id)) next.delete(id);
      else next.add(id);
      return next;
    });
  }

  async function handleAttach() {
    if (selected.size === 0) return;
    setIsAttaching(true);
    try {
      await onAttach(Array.from(selected));
      toast.success(t("tests_save_success"));
      onOpenChange(false);
    } catch (e) {
      toast.error(e instanceof Error ? e.message : t("error_generic"));
    } finally {
      setIsAttaching(false);
    }
  }

  const rows = bank.data?.data ?? [];
  const filtersActive = search !== "" || format !== "all" || topicId !== "all";

  function resetFilters() {
    setSearch("");
    setFormat("all");
    setTopicId("all");
  }

  const topicOptions = topics.data?.data ?? [];

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="max-w-[720px] sm:!max-w-[720px]" style={{ maxWidth: 720 }}>
        <DialogHeader>
          <DialogTitle>{t("tests_picker_title")}</DialogTitle>
          <DialogDescription>{t("tests_picker_subtitle")}</DialogDescription>
        </DialogHeader>

        <div className="min-w-0 space-y-3 py-1">
          <div className="grid gap-2">
            <Label htmlFor="picker-search">{t("tests_picker_search_label")}</Label>
            <div className="flex h-9 items-center gap-2 rounded-lg border border-input bg-surface-2 px-3 focus-within:ring-2 focus-within:ring-ring/40">
              <Search className="size-4 text-muted-foreground" />
              <Input
                id="picker-search"
                value={search}
                onChange={(e) => setSearch(e.target.value)}
                placeholder={t("tests_picker_search")}
                className="h-8 flex-1 border-0 bg-transparent px-0 shadow-none focus-visible:ring-0"
              />
            </div>
          </div>

          <div className="grid grid-cols-1 gap-3 sm:grid-cols-2">
            <div className="grid gap-2">
              <Label htmlFor="picker-topic">{t("tests_picker_topic_label")}</Label>
              <select
                id="picker-topic"
                data-slot="select"
                value={topicId}
                onChange={(e) => setTopicId(e.target.value)}
                disabled={topics.isLoading}
                className="h-9 w-full rounded-md border border-input bg-transparent px-2 text-sm"
              >
                <option value="all">{t("tests_picker_topic_all")}</option>
                {topicOptions.map((topic) => (
                  <option key={topic.id} value={topic.id}>
                    {topic.name}
                  </option>
                ))}
              </select>
            </div>
            <div className="grid gap-2">
              <Label htmlFor="picker-format">{t("tests_picker_format_label")}</Label>
              <select
                id="picker-format"
                data-slot="select"
                value={format}
                onChange={(e) => setFormat(e.target.value as QuestionFormat | "all")}
                className="h-9 w-full rounded-md border border-input bg-transparent px-2 text-sm"
              >
                <option value="all">{t("tab_all")}</option>
                {ALL_FORMATS.map((f) => (
                  <option key={f} value={f}>
                    {t(FORMAT_LABELS[f] as Parameters<typeof t>[0])}
                  </option>
                ))}
              </select>
            </div>
          </div>

          <div className="flex items-center justify-between gap-3 text-xs text-muted-foreground">
            <span>{!bank.isLoading && t("tests_picker_count").replace("{n}", String(rows.length))}</span>
            {filtersActive && (
              <button type="button" onClick={resetFilters} className="font-medium text-primary hover:underline">
                {t("tests_picker_reset")}
              </button>
            )}
          </div>

          <div className="max-h-[320px] overflow-y-auto overflow-x-hidden rounded-md border border-input">
            {bank.isLoading && (
              <div className="p-4 text-center text-muted-foreground">{t("sys_loading")}</div>
            )}
            {bank.isError && (
              <div className="space-y-2 p-4 text-center text-danger">
                <p>{t("tests_picker_error")}</p>
                <Button type="button" size="sm" variant="outline" onClick={() => bank.refetch()}>
                  {t("retry")}
                </Button>
              </div>
            )}
            {!bank.isLoading && !bank.isError && rows.length === 0 && (
              <div className="p-4 text-center text-muted-foreground">{t("tests_picker_empty")}</div>
            )}
            {rows.map((row) => {
              const isAttached = attachedIds.has(row.question.id);
              const isSelected = selected.has(row.question.id);
              const body = stripHtmlToPlainText(row.question.body);
              const isExpanded = expanded.has(row.question.id);
              return (
                <div
                  key={row.question.id}
                  className={`border-b last:border-b-0 ${
                    isAttached
                      ? "opacity-50"
                      : isSelected
                        ? "bg-brand-50"
                        : "hover:bg-muted/30"
                  }`}
                >
                  <button
                    type="button"
                    disabled={isAttached}
                    onClick={() => toggle(row.question.id)}
                    className={`flex w-full items-center gap-3 px-3 py-2 text-left transition-colors ${
                      isAttached ? "cursor-default" : ""
                    }`}
                  >
                    <SelectionBox selected={isSelected} />
                    <div className="min-w-0 flex-1">
                      <ClampedText
                        text={body}
                        expanded={isExpanded}
                        onOverflow={(value) => markOverflow(row.question.id, value)}
                      />
                      <div className="mt-1 flex flex-wrap items-center gap-1.5">
                        <span className="max-w-[140px] truncate rounded bg-muted px-1.5 py-0.5 font-mono text-[10px] text-muted-foreground">
                          {row.question.id}
                        </span>
                        <Badge variant="outline">
                          {t(FORMAT_LABELS[row.question.format] as Parameters<typeof t>[0])}
                        </Badge>
                        {row.question.topic && (
                          <span className="max-w-[160px] truncate rounded bg-muted px-1.5 py-0.5 text-[10px] text-muted-foreground">
                            {row.question.topic}
                          </span>
                        )}
                        {row.question.difficulty &&
                          DIFFICULTY_LABEL[row.question.difficulty] && (
                            <Badge variant="secondary">
                              {t(DIFFICULTY_LABEL[row.question.difficulty] as Parameters<typeof t>[0])}
                            </Badge>
                          )}
                      </div>
                    </div>
                    {isAttached && (
                      <span className="flex-shrink-0 text-[11px] text-muted-foreground">
                        {t("tests_picker_attached_badge")}
                      </span>
                    )}
                  </button>
                  {(isExpanded || overflowing.has(row.question.id)) && (
                    <button
                      type="button"
                      onClick={() => toggleExpanded(row.question.id)}
                      className="mb-2 ml-11 text-xs font-medium text-primary hover:underline"
                    >
                      {t(isExpanded ? "tests_picker_less" : "tests_picker_more")}
                    </button>
                  )}
                </div>
              );
            })}
          </div>

          {bank.data?.next_cursor && (
            <p className="text-xs text-muted-foreground">
              {t("tests_picker_partial").replace("{n}", String(rows.length))}
            </p>
          )}

        </div>

        <DialogFooter className="items-center sm:justify-between">
          <p className="text-sm text-muted-foreground">
            {t("tests_picker_selected").replace("{n}", String(selected.size))}
          </p>
          <div className="flex gap-2">
            <Button type="button" variant="outline" onClick={() => onOpenChange(false)} disabled={isAttaching}>
              {t("cancel")}
            </Button>
            <Button type="button" onClick={handleAttach} disabled={selected.size === 0 || isAttaching}>
              {isAttaching
                ? t("saving")
                : t("tests_picker_confirm").replace("{n}", String(selected.size))}
            </Button>
          </div>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
