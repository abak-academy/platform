"use client";

import { useEffect, useMemo, useState } from "react";
import Link from "next/link";
import { useRouter } from "next/navigation";
import { toast } from "sonner";
import { Pencil, Search, Trash2 } from "lucide-react";
import { AdminPageHeader } from "@/components/admin/AdminPageHeader";
import { TestModal } from "@/components/admin/TestModal";
import { Button } from "@/components/ui/button";
import { Skeleton } from "@/components/ui/skeleton";
import { Badge } from "@/components/ui/badge";
import { EmptyState } from "@/components/ui/empty-state";
import { ToolbarCount, ToolbarFilterButton, ToolbarSearch } from "@/components/admin/ListToolbar";
import { cn } from "@/lib/utils";
import {
  useAdminTests,
  useCreateTest,
  useUpdateTest,
  useDeleteTest,
} from "@/lib/hooks/admin-tests";
import { useTopics } from "@/lib/hooks/admin-topics";
import { useDebouncedValue } from "@/lib/hooks/use-debounced-value";
import { useTranslation } from "@/lib/i18n";
import type { Test, AdminCreateTestInput, AdminUpdateTestInput } from "@/lib/types";

const FILTER_SELECT =
  "h-10 rounded-full border-[1.5px] border-line bg-surface px-3.5 text-[13px] font-bold text-ink-900 outline-none focus-visible:border-brand-600 focus-visible:ring-4 focus-visible:ring-brand-50 disabled:opacity-45";
const ICON_BTN =
  "grid size-8 place-items-center rounded-full text-ink-700 transition-colors hover:bg-brand-50 hover:text-brand-ink disabled:opacity-45";

export default function TestsPage() {
  const { t } = useTranslation();
  const router = useRouter();
  const [modalOpen, setModalOpen] = useState(false);
  const [editingTest, setEditingTest] = useState<Test | null>(null);
  const [deletingId, setDeletingId] = useState<string | null>(null);

  const [subject, setSubject] = useState("");
  const [topic, setTopic] = useState("");
  const [search, setSearch] = useState("");
  const [filtersOpen, setFiltersOpen] = useState(false);
  const q = useDebouncedValue(search).trim();

  const allTopics = useTopics();
  const topicsForSubject = useTopics(subject || undefined);
  const topicOptions = topicsForSubject.data?.data ?? [];

  const subjectOptions = useMemo(() => {
    const set = new Set<string>();
    for (const item of allTopics.data?.data ?? []) {
      if (item.subject) set.add(item.subject);
    }
    return Array.from(set).sort();
  }, [allTopics.data]);

  // A topic name that doesn't exist under the newly selected subject must not linger.
  useEffect(() => {
    if (topic && !topicOptions.some((opt) => opt.name === topic)) setTopic("");
  }, [topic, topicsForSubject.data]);

  const activeFilters = (subject ? 1 : 0) + (topic ? 1 : 0);

  const filterKey = `${subject}\u0000${topic}\u0000${q}`;
  const [page, setPage] = useState<{ key: string; cursor?: string }>({ key: filterKey });
  const cursor = page.key === filterKey ? page.cursor : undefined;
  const [entries, setEntries] = useState<Test[]>([]);

  const {
    data: testsResp,
    isLoading,
    isError,
    error,
    isFetching,
    refetch,
  } = useAdminTests({
    subject: subject || undefined,
    topic: topic || undefined,
    q: q || undefined,
    cursor,
  });
  const tests = entries;
  const create = useCreateTest();
  const update = useUpdateTest(editingTest?.id ?? "");
  const remove = useDeleteTest(deletingId ?? "");

  // Reset accumulated rows when filters change
  useEffect(() => {
    setEntries([]);
  }, [subject, topic, q]);

  // Accumulate pages
  useEffect(() => {
    if (!testsResp) return;
    if (!cursor) {
      setEntries(testsResp.data ?? []);
    } else {
      setEntries((prev) => [...prev, ...(testsResp.data ?? [])]);
    }
  }, [testsResp]);

  function handleLoadMore() {
    if (testsResp?.next_cursor) {
      setPage({ key: filterKey, cursor: testsResp.next_cursor });
    }
  }

  function openCreate() {
    setEditingTest(null);
    setModalOpen(true);
  }

  function openEdit(test: Test) {
    setEditingTest(test);
    setModalOpen(true);
  }

  function errorMessage(err: unknown): string {
    if (err instanceof Error) return err.message;
    return t("error_generic");
  }

  async function handleSubmit(input: AdminCreateTestInput | AdminUpdateTestInput) {
    try {
      if (editingTest) {
        await update.mutateAsync(input as AdminUpdateTestInput);
        toast.success(t("tests_update_success"));
      } else {
        const created = await create.mutateAsync(input as AdminCreateTestInput);
        toast.success(t("tests_create_success"));
        setModalOpen(false);
        router.push(`/admin/exam/tests/${created.id}`);
        return;
      }
      setModalOpen(false);
      setEditingTest(null);
    } catch (e) {
      toast.error(errorMessage(e));
    }
  }

  async function handleDelete(id: string) {
    if (!confirm(t("tests_confirm_delete"))) return;
    setDeletingId(id);
    try {
      await remove.mutateAsync();
      toast.success(t("tests_delete_success"));
    } catch (e) {
      toast.error(errorMessage(e));
    } finally {
      setDeletingId(null);
    }
  }

  return (
    <div className="flex flex-col gap-5 fade-in">
      <AdminPageHeader
        title={t("tests_page_title")}
        description={t("tests_page_description")}
        actions={<Button onClick={openCreate}>{t("tests_new")}</Button>}
      />

      <div
        data-testid="exam-tests-toolbar"
        className="flex flex-col gap-2.5 rounded-[var(--radius-card)] border border-line bg-surface px-3.5 py-3 shadow-card"
      >
        <div className="flex flex-wrap items-center gap-2.5">
          <ToolbarSearch
            value={search}
            onChange={setSearch}
            placeholder={t("tests_search_placeholder")}
            clearLabel={t("exam_packages_search_clear")}
          />
          <ToolbarFilterButton
            type="button"
            label={t("tests_filter")}
            count={activeFilters}
            aria-expanded={filtersOpen}
            aria-controls={filtersOpen ? "tests-filter-panel" : undefined}
            onClick={() => setFiltersOpen((open) => !open)}
          />
          <ToolbarCount>
            <span data-testid="tests-count">
              {t("tests_count").replace("{n}", String(testsResp?.total ?? tests.length))}
            </span>
          </ToolbarCount>
        </div>
        {filtersOpen && (
          <div id="tests-filter-panel" className="flex flex-col gap-3 border-t border-line pt-3 sm:flex-row sm:items-center">
            <select
              data-slot="select"
              data-testid="tests-subject-filter"
              aria-label={t("tests_field_subject")}
              value={subject}
              onChange={(e) => setSubject(e.target.value)}
              className={FILTER_SELECT}
            >
              <option value="">{t("tab_all")}</option>
              {subjectOptions.map((s) => (
                <option key={s} value={s}>
                  {s}
                </option>
              ))}
            </select>
            <select
              data-slot="select"
              data-testid="tests-topic-filter"
              aria-label={t("tests_field_topic")}
              value={topic}
              onChange={(e) => setTopic(e.target.value)}
              disabled={topicsForSubject.isLoading}
              className={FILTER_SELECT}
            >
              <option value="">{t("all_topics")}</option>
              {topicOptions.map((opt) => (
                <option key={opt.id} value={opt.name}>
                  {opt.name}
                </option>
              ))}
            </select>
            {activeFilters > 0 && (
              <Button
                type="button"
                variant="ghost"
                size="sm"
                onClick={() => {
                  setSubject("");
                  setTopic("");
                }}
              >
                {t("tests_filter_reset")}
              </Button>
            )}
          </div>
        )}
      </div>

      {isLoading && tests.length === 0 && (
        <div className="space-y-2">
          {Array.from({ length: 5 }).map((_, i) => (
            <Skeleton key={i} className="h-12 w-full" />
          ))}
        </div>
      )}

      {isError && (
        <div className="rounded-lg border border-danger/20 bg-danger/10 p-4 text-danger">
          <p>{errorMessage(error)}</p>
          <Button type="button" size="sm" variant="outline" className="mt-2" onClick={() => refetch()}>
            {t("retry")}
          </Button>
        </div>
      )}

      {!isError && tests.length > 0 && (
        <div className="grid grid-cols-1 gap-3.5 sm:grid-cols-2 xl:grid-cols-3">
          {tests.map((test) => (
            <article
              key={test.id}
              data-testid="test-row"
              onClick={() => router.push(`/admin/exam/tests/${test.id}`)}
              className="md-card-outlined flex cursor-pointer flex-col gap-2.5 transition-transform duration-300 ease-spring hover:-translate-y-1"
            >
              <div className="flex items-start justify-between gap-2">
                <div className="flex flex-wrap gap-1.5">
                  <Badge variant="mute">{t("tests_questions_n").replace("{n}", String(test.question_count ?? 0))}</Badge>
                  <Badge variant="sky">{t("tests_minutes_n").replace("{n}", String(test.duration_minutes))}</Badge>
                </div>
                <div className="-mt-1 -mr-1 flex items-center gap-1" onClick={(e) => e.stopPropagation()}>
                  <button
                    type="button"
                    onClick={() => openEdit(test)}
                    aria-label={t("action_edit")}
                    className={ICON_BTN}
                  >
                    <Pencil className="size-3.5" aria-hidden />
                  </button>
                  <button
                    type="button"
                    onClick={() => handleDelete(test.id)}
                    disabled={remove.isPending && deletingId === test.id}
                    aria-label={t("action_delete")}
                    className={cn(ICON_BTN, "text-danger-600 hover:text-danger-600")}
                  >
                    <Trash2 className="size-3.5" aria-hidden />
                  </button>
                </div>
              </div>
              <h3 className="type-h3 [overflow-wrap:anywhere]">
                <Link
                  href={`/admin/exam/tests/${test.id}`}
                  onClick={(e) => {
                    e.stopPropagation();
                    if (e.metaKey || e.ctrlKey || e.shiftKey || e.altKey || e.button !== 0) return;
                    e.preventDefault();
                    router.push(`/admin/exam/tests/${test.id}`);
                  }}
                  className="hover:underline"
                >
                  {test.title}
                </Link>
              </h3>
              <div className="flex flex-wrap items-center gap-1.5">
                <Badge variant="sky">{test.subject}</Badge>
                <span className="text-xs text-ink-600">{test.topic}</span>
              </div>
              <div className="mt-auto text-xs text-ink-600">
                <div>
                  {test.shared_material_html?.trim()
                    ? t("tests_content_material")
                    : t("tests_content_no_material")}
                </div>
                {test.audio_url && <div>{t("tests_content_audio")}</div>}
              </div>
            </article>
          ))}
        </div>
      )}

      {!isLoading && !isError && tests.length === 0 && (
        <EmptyState framed icon={Search} title={t("tests_empty")} />
      )}

      <p className="text-xs text-ink-600">{t("tests_list_footnote")}</p>

      {!isLoading && !isError && tests.length > 0 && (
        <div className="flex items-center justify-between gap-3">
          <p className="text-sm text-ink-600">
            {t("tests_shown_of_total")
              .replace("{shown}", String(tests.length))
              .replace("{total}", String(testsResp?.total ?? tests.length))}
          </p>
          {testsResp?.next_cursor && (
            <Button
              type="button"
              variant="outline"
              onClick={handleLoadMore}
              disabled={isFetching}
            >
              {isFetching ? t("sys_loading") : t("sys_load_more")}
            </Button>
          )}
        </div>
      )}

      <TestModal
        open={modalOpen}
        onOpenChange={setModalOpen}
        test={editingTest}
        onSubmit={handleSubmit}
        isPending={create.isPending || update.isPending}
      />
    </div>
  );
}