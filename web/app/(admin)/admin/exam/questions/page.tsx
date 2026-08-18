"use client";

import { useEffect, useMemo, useState } from "react";
import { toast } from "sonner";
import { ChevronLeft, ChevronRight, Library, Plus, Search, Trash2, Upload } from "lucide-react";
import { AdminPageHeader } from "@/components/admin/AdminPageHeader";
import { QuestionEditor } from "@/components/admin/QuestionEditor";
import { QuestionPreview } from "@/components/admin/QuestionPreview";
import { TopicsModal } from "@/components/admin/TopicsModal";
import { Button } from "@/components/ui/button";
import { Skeleton } from "@/components/ui/skeleton";
import { Badge } from "@/components/ui/badge";
import { Input } from "@/components/ui/input";
import { DataTable, type DataTableColumn } from "@/components/ui/data-table";
import { useQueryClient } from "@tanstack/react-query";
import {
  useBankQuestions,
  useDeleteBankQuestion,
  adminBankQuestionsKeys,
} from "@/lib/hooks/admin-bank-questions";
import { useTopics } from "@/lib/hooks/admin-topics";
import { QuestionImportModal } from "@/components/admin/QuestionImportModal";
import { useTranslation } from "@/lib/i18n";
import { cn } from "@/lib/utils";
import { ApiError } from "@/lib/api";
import { stripHtmlToPlainText } from "@/lib/rich-text";
import { FORMAT_TONE, DIFFICULTY_TONE } from "@/lib/question-tone";
import type { BankQuestionListItem, QuestionFormat } from "@/lib/types";

const PAGE_SIZE = 25;

// short/fill_blank are retired from authoring (2026-07-31 merge into
// multi_blank) and are dropped from the filter too — legacy questions in
// those formats still appear under "Semua" and in search, and their row
// badges still render via FORMAT_LABELS below, which stays complete.
const ALL_FORMATS: Array<QuestionFormat | "all"> = [
  "all",
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
  }, [format, topicId, search]);

  const filters = useMemo(
    () => ({
      format: format === "all" ? undefined : format,
      topic_id: topicId || undefined,
      search: search.trim() || undefined,
      page,
      limit: PAGE_SIZE,
    }),
    [format, topicId, search, page]
  );

  const bank = useBankQuestions(filters);
  const topics = useTopics();
  const deleteQuestion = useDeleteBankQuestion();

  const rows = bank.data?.data ?? [];
  const total = bank.data?.total ?? 0;
  const pageCount = Math.max(1, Math.ceil(total / PAGE_SIZE));
  const topicOptions = topics.data?.data ?? [];

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
        <span className="font-mono text-xs text-ink-500">{item.question.question_number ?? "—"}</span>
      ),
    },
    {
      key: "question",
      header: t("question"),
      cell: (item) => stripHtmlToPlainText(item.question.body),
      className: "max-w-xs truncate",
    },
    {
      key: "used_in",
      header: t("used_in"),
      cell: (item) => item.attached_count,
    },
    {
      key: "topic",
      header: t("topic"),
      cell: (item) =>
        item.question.topic ? (
          <Badge variant="outline" className="border-transparent bg-brand-50 text-brand-700">
            {item.question.topic}
          </Badge>
        ) : (
          "—"
        ),
    },
    {
      key: "format",
      header: t("format"),
      cell: (item) => (
        <Badge variant="outline" className={cn("border-transparent", FORMAT_TONE[item.question.format])}>
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
          <Badge
            variant="outline"
            className={cn(
              "border-transparent",
              item.question.difficulty && DIFFICULTY_TONE[item.question.difficulty]
            )}
          >
            {t(difficultyKey)}
          </Badge>
        ) : (
          "—"
        );
      },
    },
    {
      key: "points",
      header: t("points"),
      cell: (item) => (
        <>
          <span className="font-semibold text-info">+{item.question.point_correct}</span>
          <span className="text-ink-500"> / {item.question.point_wrong}</span>
        </>
      ),
    },
    {
      key: "actions",
      header: t("th_actions"),
      cell: (item) => (
        <Button
          variant="ghost"
          size="icon"
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
      ),
    },
  ];

  return (
    <div className="space-y-6 fade-in">
      <AdminPageHeader
        icon={Library}
        title={t("question_bank_title")}
        description={t("question_bank_subtitle")}
        actionsAlign="end"
        actions={
          <div className="flex flex-wrap items-center gap-2">
            <Button variant="outline" onClick={() => setTopicsOpen(true)}>
              {t("manage_topics")}
            </Button>
            <Button variant="outline" onClick={handleCsvClick}>
              <Upload className="mr-1 size-4" />
              CSV
            </Button>
            <Button onClick={openCreate}>
              <Plus className="mr-1 size-4" />
              {t("create")}
            </Button>
          </div>
        }
      />

      <div className="flex flex-col gap-3 md:flex-row md:items-center md:justify-between">
        <select
          data-slot="select"
          value={format}
          onChange={(e) => setFormat(e.target.value as QuestionFormat | "all")}
          className="h-9 rounded-md border border-input bg-transparent px-3 text-sm"
        >
          {ALL_FORMATS.map((f) => (
            <option key={f} value={f}>
              {t(FORMAT_LABELS[f] as Parameters<typeof t>[0])}
            </option>
          ))}
        </select>

        <div className="flex flex-col gap-3 sm:flex-row">
          <select
            data-slot="select"
            value={topicId}
            onChange={(e) => setTopicId(e.target.value)}
            disabled={topics.isLoading}
            className="h-9 rounded-md border border-input bg-transparent px-3 text-sm"
          >
            <option value="">{t("all_topics")}</option>
            {topicOptions.map((topic) => (
              <option key={topic.id} value={topic.id}>
                {topic.name}
              </option>
            ))}
          </select>

          <div className="relative w-full sm:w-64">
            <Search className="absolute left-2.5 top-1/2 size-4 -translate-y-1/2 text-ink-400" />
            <Input
              value={search}
              onChange={(e) => setSearch(e.target.value)}
              placeholder={t("search")}
              className="pl-9"
            />
          </div>
        </div>
      </div>

      {bank.isLoading && !bank.data && (
        <div className="space-y-2">
          {Array.from({ length: 5 }).map((_, i) => (
            <Skeleton key={i} className="h-12 w-full" />
          ))}
        </div>
      )}

      {bank.isError && !bank.data && (
        <div className="rounded-lg border border-danger/20 bg-danger/10 p-4 text-danger">
          {errorMessage(bank.error)}
        </div>
      )}

      {!bank.isLoading && !bank.isError && (
        <DataTable
          columns={columns}
          rows={rows}
          rowKey={(item) => item.question.id}
          empty={t("tests_picker_empty")}
          onRowClick={handleRowClick}
          data-testid="exam-questions-table"
          footer={
            total > PAGE_SIZE ? (
              <div className="flex items-center justify-between border-t px-4 py-3 text-sm">
                <span className="text-ink-500">
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
                          <span className="px-1 text-ink-500">…</span>
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
