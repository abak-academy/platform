"use client";

import { useEffect, useMemo, useState } from "react";
import { toast } from "sonner";
import { ChevronLeft, ChevronRight, Eye, Plus, Search, Trash2, Upload } from "lucide-react";
import { AdminPageHeader } from "@/components/admin/AdminPageHeader";
import { QuestionEditor } from "@/components/admin/QuestionEditor";
import { QuestionPreview } from "@/components/admin/QuestionPreview";
import { TopicsModal } from "@/components/admin/TopicsModal";
import { Button } from "@/components/ui/button";
import { Skeleton } from "@/components/ui/skeleton";
import { Badge } from "@/components/ui/badge";
import { EmptyState } from "@/components/ui/empty-state";
import { Popover, PopoverContent, PopoverTrigger } from "@/components/ui/popover";
import {
  ActiveChip,
  FILTER_PANEL_CLASS,
  FilterGrid,
  FilterPanelFooter,
  FilterSelect,
  ToolbarCount,
  ToolbarFilterButton,
  ToolbarSearch,
} from "@/components/admin/ListToolbar";
import { DataTable, type DataTableColumn } from "@/components/ui/data-table";
import { useQueryClient } from "@tanstack/react-query";
import {
  useBankQuestions,
  useDeleteBankQuestion,
  adminBankQuestionsKeys,
} from "@/lib/hooks/admin-bank-questions";
import { useTopics } from "@/lib/hooks/admin-topics";
import { useDebouncedValue } from "@/lib/hooks/use-debounced-value";
import { QuestionImportModal } from "@/components/admin/QuestionImportModal";
import { useTranslation } from "@/lib/i18n";
import { cn } from "@/lib/utils";
import { FORMAT_BADGE_VARIANT } from "@/lib/question-tone";
import { ApiError } from "@/lib/api";
import { stripHtmlToPlainText } from "@/lib/rich-text";
import type { BankQuestionListItem, QuestionFormat } from "@/lib/types";

const PAGE_SIZE = 25;

const DIFFICULTY_TEXT: Record<string, string> = {
  easy: "text-teal-700",
  medium: "text-gold-700",
  hard: "text-danger-600",
};

// short/fill_blank are retired from authoring (2026-07-31 merge into
// multi_blank) and are dropped from the filter too — legacy questions in
// those formats still appear under "Semua" and in search, and their row
// badges still render via FORMAT_LABELS below, which stays complete.
const ALL_FORMATS: QuestionFormat[] = [
  "mcq",
  "multi_answer",
  "essay",
  "multi_blank",
  "true_false",
];

const FORMAT_LABELS: Record<QuestionFormat | "all", string> = {
  all: "tab_all",
  mcq: "fmt_mcq",
  multi_answer: "fmt_multi_answer",
  short: "fmt_short",
  fill_blank: "fmt_fill_blank",
  essay: "fmt_essay",
  multi_blank: "fmt_multi_blank",
  true_false: "fmt_true_false",
};

const DIFFICULTY_LABELS: Record<string, "diff_easy" | "diff_medium" | "diff_hard"> = {
  easy: "diff_easy",
  medium: "diff_medium",
  hard: "diff_hard",
};

export default function QuestionBankPage() {
  const { t } = useTranslation();
  const [format, setFormat] = useState<QuestionFormat | "all">("all");
  const [topicId, setTopicId] = useState<string>("");
  const [search, setSearch] = useState("");
  // Search is a server filter, so it must be debounced — otherwise every
  // keystroke fires a new paginated request.
  const debouncedSearch = useDebouncedValue(search);

  const [previewOpen, setPreviewOpen] = useState(false);
  const [previewItem, setPreviewItem] = useState<BankQuestionListItem | null>(null);

  const [editorOpen, setEditorOpen] = useState(false);
  const [editorItem, setEditorItem] = useState<BankQuestionListItem | null>(null);

  const [topicsOpen, setTopicsOpen] = useState(false);
  const [importOpen, setImportOpen] = useState(false);

  const queryClient = useQueryClient();

  const [page, setPage] = useState(1);
  // A filter change re-scopes the list; staying on page 5 of the old scope
  // would show an arbitrary window of the new one.
  useEffect(() => {
    setPage(1);
  }, [format, topicId, debouncedSearch]);

  const filters = useMemo(
    () => ({
      format: format === "all" ? undefined : format,
      topic_id: topicId || undefined,
      search: debouncedSearch.trim() || undefined,
      page,
      limit: PAGE_SIZE,
    }),
    [format, topicId, debouncedSearch, page]
  );

  const bank = useBankQuestions(filters);
  const topics = useTopics();
  const deleteQuestion = useDeleteBankQuestion();

  const rows = bank.data?.data ?? [];
  const total = bank.data?.total ?? 0;
  const pageCount = Math.max(1, Math.ceil(total / PAGE_SIZE));
  const topicOptions = topics.data?.data ?? [];
  const formatOptions = ALL_FORMATS.map((f) => ({ value: f, label: t(FORMAT_LABELS[f] as Parameters<typeof t>[0]) }));
  const topicSelectOptions = topicOptions.map((topic) => ({ value: topic.id, label: topic.name }));
  const activeChips = [
    ...(format !== "all"
      ? [{ key: "format", label: `${t("format")}: ${formatOptions.find((o) => o.value === format)?.label ?? format}`, clear: () => setFormat("all") }]
      : []),
    ...(topicId
      ? [{ key: "topic", label: `${t("topic")}: ${topicSelectOptions.find((o) => o.value === topicId)?.label ?? topicId}`, clear: () => setTopicId("") }]
      : []),
  ];
  const resetFilters = () => {
    setFormat("all");
    setTopicId("");
  };

  function openCreate() {
    setEditorItem(null);
    setEditorOpen(true);
  }

  function openEdit(item: BankQuestionListItem) {
    setEditorItem(item);
    setEditorOpen(true);
  }

  function openPreview(item: BankQuestionListItem) {
    setPreviewItem(item);
    setPreviewOpen(true);
  }

  function handleRowClick(item: BankQuestionListItem) {
    openPreview(item);
  }

  function handleEditFromPreview() {
    setPreviewOpen(false);
    if (previewItem) {
      setEditorItem(previewItem);
      setEditorOpen(true);
    }
  }

  function handleSaved() {
    setEditorOpen(false);
    setEditorItem(null);
    toast.success(t("tests_save_success"));
  }

  function handleCsvClick() {
    setImportOpen(true);
  }

  function handleImportSuccess() {
    queryClient.invalidateQueries({ queryKey: adminBankQuestionsKeys.lists() });
  }

  function errorMessage(err: unknown): string {
    if (err instanceof Error) return err.message;
    return t("error_generic");
  }

  async function handleDelete(item: BankQuestionListItem) {
    if (!confirm(t("questions_confirm_delete"))) return;
    try {
      await deleteQuestion.mutateAsync(item.question.id);
      toast.success(t("questions_deleted"));
    } catch (e) {
      if (e instanceof ApiError && e.code === "question_in_published_exam") {
        toast.error(t("question_in_published_exam_reason"));
      } else {
        toast.error(errorMessage(e) || t("questions_delete_failed"));
      }
    }
  }

  const columns: DataTableColumn<BankQuestionListItem>[] = [
    {
      key: "id",
      header: "ID",
      cell: (item) => (
        <span className="font-mono text-xs text-ink-600">{item.question.question_number ?? "—"}</span>
      ),
    },
    {
      key: "question",
      header: t("question"),
      cell: (item) => <span className="line-clamp-2">{stripHtmlToPlainText(item.question.body)}</span>,
      className: "max-w-[420px] !whitespace-normal",
    },
    {
      key: "topic",
      header: t("topic"),
      cell: (item) => item.question.topic || "—",
    },
    {
      key: "format",
      header: t("format"),
      cell: (item) => (
        <Badge variant={FORMAT_BADGE_VARIANT[item.question.format]}>
          {t(FORMAT_LABELS[item.question.format] as Parameters<typeof t>[0])}
        </Badge>
      ),
    },
    {
      key: "difficulty",
      header: t("difficulty"),
      cell: (item) => {
        const difficultyKey = item.question.difficulty
          ? DIFFICULTY_LABELS[item.question.difficulty]
          : undefined;
        return difficultyKey ? (
          <span className={cn("text-xs font-bold", DIFFICULTY_TEXT[item.question.difficulty ?? ""])}>
            {t(difficultyKey)}
          </span>
        ) : (
          "—"
        );
      },
    },
    {
      key: "used_in",
      header: t("used_in"),
      align: "right",
      cell: (item) => item.attached_count,
    },
    {
      key: "points",
      header: t("points"),
      cell: (item) => (
        <>
          <span className="font-semibold text-info">+{item.question.point_correct}</span>
          <span className="text-ink-600"> / {item.question.point_wrong}</span>
        </>
      ),
    },
    {
      key: "actions",
      header: t("th_actions"),
      align: "right",
      cell: (item) => (
        <div className="flex items-center justify-end gap-1">
          <Button
            variant="ghost"
            size="icon-sm"
            aria-label={t("action_view")}
            onClick={() => handleRowClick(item)}
          >
            <Eye className="size-4" />
          </Button>
          <Button
            variant="ghost"
            size="icon-sm"
            aria-label={t("action_delete")}
            title={item.in_live_exam ? t("question_in_published_exam_reason") : undefined}
            disabled={item.in_live_exam}
            onClick={(e) => {
              e.stopPropagation();
              handleDelete(item);
            }}
          >
            <Trash2 className="size-4" />
          </Button>
        </div>
      ),
    },
  ];

  return (
    <div className="flex flex-col gap-4 fade-in">
      <AdminPageHeader
        title={t("question_bank_title")}
        description={t("question_bank_subtitle")}
        actions={
          <>
            <Button variant="outline" onClick={() => setTopicsOpen(true)}>
              {t("manage_topics")}
            </Button>
            <Button variant="outline" onClick={handleCsvClick}>
              <Upload className="size-4" />
              CSV
            </Button>
            <Button variant="dark" onClick={openCreate}>
              <Plus className="size-4" />
              {t("create")}
            </Button>
          </>
        }
      />

      <div className="md-card-flush">
          <div className="flex flex-wrap items-center gap-2.5 border-b border-line px-4 py-3.5">
            <ToolbarSearch value={search} onChange={setSearch} placeholder={t("search")} clearLabel={t("list_toolbar_clear_search")} />
            <Popover>
              <PopoverTrigger asChild>
                <ToolbarFilterButton label={t("list_toolbar_filter")} count={activeChips.length} />
              </PopoverTrigger>
              <PopoverContent align="end" className={FILTER_PANEL_CLASS} aria-label={t("list_toolbar_filter")}>
                <div className="flex flex-col gap-3.5">
                  <FilterGrid>
                    <FilterSelect
                      label={t("format")}
                      value={format === "all" ? "" : format}
                      options={formatOptions}
                      onChange={(v) => setFormat((v || "all") as QuestionFormat | "all")}
                    />
                    <FilterSelect label={t("topic")} value={topicId} options={topicSelectOptions} onChange={setTopicId} />
                  </FilterGrid>
                  <FilterPanelFooter
                    onReset={resetFilters}
                    showLabel={t("list_toolbar_show_n").replace("{n}", String(total))}
                  />
                </div>
              </PopoverContent>
            </Popover>
            <ToolbarCount>{total} {t("question")}</ToolbarCount>
          </div>
          {activeChips.length > 0 && (
            <div className="flex flex-wrap items-center gap-1.5 border-b border-line px-4 py-2.5" role="group" aria-label={t("list_toolbar_active")}>
              {activeChips.map((c) => (
                <ActiveChip key={c.key} label={c.label} removeLabel={`${t("list_toolbar_remove")}: ${c.label}`} onRemove={c.clear} />
              ))}
              <Button variant="ghost" size="sm" onClick={resetFilters}>{t("list_toolbar_clear_all")}</Button>
            </div>
          )}
          {bank.isLoading && !bank.data ? (
            <div className="space-y-2 p-4">
              {Array.from({ length: 5 }).map((_, i) => (
                <Skeleton key={i} className="h-12 w-full" />
              ))}
            </div>
          ) : bank.isError && !bank.data ? (
            <div className="m-4 rounded-lg border border-danger/20 bg-danger/10 p-4 text-danger">
              {errorMessage(bank.error)}
            </div>
          ) : (
          <DataTable
            surface="plain"
            columns={columns}
            rows={rows}
            rowKey={(item) => item.question.id}
            empty={<EmptyState icon={Search} title={t("tests_picker_empty")} />}
            data-testid="exam-questions-table"
            footer={
              total > PAGE_SIZE ? (
                <div className="flex flex-wrap items-center justify-between gap-3 border-t border-line px-4 py-3 text-[13px]">
                  <span className="text-ink-600">
                    {t("pagination_summary")
                      .replace("{page}", String(page))
                      .replace("{pages}", String(pageCount))
                      .replace("{total}", String(total))}
                  </span>
                  <div className="flex items-center gap-1">
                    <Button
                      variant="outline"
                      size="sm"
                      disabled={page <= 1}
                      onClick={() => setPage((p) => Math.max(1, p - 1))}
                    >
                      <ChevronLeft className="size-4" />
                      {t("pagination_prev")}
                    </Button>
                    {Array.from({ length: pageCount }, (_, i) => i + 1)
                      .filter((n) => n === 1 || n === pageCount || Math.abs(n - page) <= 2)
                      .map((n, idx, arr) => (
                        <span key={n} className="flex items-center">
                          {idx > 0 && arr[idx - 1] !== n - 1 && (
                            <span className="px-1 text-ink-600">…</span>
                          )}
                          <Button
                            variant={n === page ? "default" : "ghost"}
                            size="sm"
                            onClick={() => setPage(n)}
                          >
                            {n}
                          </Button>
                        </span>
                      ))}
                    <Button
                      variant="outline"
                      size="sm"
                      disabled={page >= pageCount}
                      onClick={() => setPage((p) => Math.min(pageCount, p + 1))}
                    >
                      {t("pagination_next")}
                      <ChevronRight className="size-4" />
                    </Button>
                  </div>
                </div>
              ) : undefined
            }
          />
          )}
      </div>

      <QuestionPreview
        item={previewItem}
        open={previewOpen}
        onOpenChange={setPreviewOpen}
        onEdit={handleEditFromPreview}
      />

      <TopicsModal open={topicsOpen} onOpenChange={setTopicsOpen} />

      <QuestionImportModal
        open={importOpen}
        onOpenChange={setImportOpen}
        onSuccess={handleImportSuccess}
      />

      {editorOpen && (
        <QuestionEditor
          question={editorItem ?? undefined}
          onCancel={() => {
            setEditorOpen(false);
            setEditorItem(null);
          }}
          onSaved={handleSaved}
        />
      )}
    </div>
  );
}
