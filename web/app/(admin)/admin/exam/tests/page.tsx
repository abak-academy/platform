"use client";

import { useEffect, useMemo, useState } from "react";
import { useRouter } from "next/navigation";
import { toast } from "sonner";
import { ClipboardList, Pencil, Search, Trash2 } from "lucide-react";
import { AdminPageHeader } from "@/components/admin/AdminPageHeader";
import { TestModal } from "@/components/admin/TestModal";
import { Button } from "@/components/ui/button";
import { Skeleton } from "@/components/ui/skeleton";
import { Badge } from "@/components/ui/badge";
import { Input } from "@/components/ui/input";
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

export default function TestsPage() {
  const { t } = useTranslation();
  const router = useRouter();
  const [modalOpen, setModalOpen] = useState(false);
  const [editingTest, setEditingTest] = useState<Test | null>(null);
  const [deletingId, setDeletingId] = useState<string | null>(null);

  const [subject, setSubject] = useState("");
  const [topic, setTopic] = useState("");
  const [search, setSearch] = useState("");
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
    <div className="space-y-6 fade-in">
      <AdminPageHeader
        icon={ClipboardList}
        title={t("tests_page_title")}
        description={t("tests_page_description")}
        actions={<Button onClick={openCreate}>{t("tests_new")}</Button>}
      />

      <div
        data-testid="exam-tests-toolbar"
        className="flex flex-col gap-3 md:flex-row md:items-center md:justify-between"
      >
        <select
          data-slot="select"
          data-testid="tests-subject-filter"
          value={subject}
          onChange={(e) => setSubject(e.target.value)}
          className="h-9 rounded-md border border-input bg-transparent px-3 text-sm"
        >
          <option value="">{t("tab_all")}</option>
          {subjectOptions.map((s) => (
            <option key={s} value={s}>
              {s}
            </option>
          ))}
        </select>

        <div className="flex flex-col gap-3 sm:flex-row">
          <select
            data-slot="select"
            data-testid="tests-topic-filter"
            value={topic}
            onChange={(e) => setTopic(e.target.value)}
            disabled={topicsForSubject.isLoading}
            className="h-9 rounded-md border border-input bg-transparent px-3 text-sm"
          >
            <option value="">{t("all_topics")}</option>
            {topicOptions.map((opt) => (
              <option key={opt.id} value={opt.name}>
                {opt.name}
              </option>
            ))}
          </select>

          <div className="relative w-full sm:w-64">
            <Search className="absolute left-2.5 top-1/2 size-4 -translate-y-1/2 text-ink-400" />
            <Input
              data-testid="tests-search-input"
              value={search}
              onChange={(e) => setSearch(e.target.value)}
              placeholder={t("search")}
              className="pl-9"
            />
          </div>
        </div>
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
        <div className="overflow-x-auto md-card-outlined">
          <table className="w-full text-sm">
            <thead className="bg-muted">
              <tr>
                <th className="px-4 py-3 text-left font-medium">{t("tests_col_test")}</th>
                <th className="px-4 py-3 text-left font-medium">{t("tests_col_subject_topic")}</th>
                <th className="px-4 py-3 text-left font-medium">{t("tests_col_questions")}</th>
                <th className="px-4 py-3 text-left font-medium">{t("tests_col_duration")}</th>
                <th className="px-4 py-3 text-left font-medium">{t("tests_col_content")}</th>
                <th className="px-4 py-3" />
              </tr>
            </thead>
            <tbody>
              {tests.map((test) => (
                <tr
                  key={test.id}
                  data-testid="test-row"
                  tabIndex={0}
                  onClick={() => router.push(`/admin/exam/tests/${test.id}`)}
                  onKeyDown={(e) => {
                    if (e.target !== e.currentTarget) return;
                    if (e.key === "Enter" || e.key === " ") {
                      e.preventDefault();
                      router.push(`/admin/exam/tests/${test.id}`);
                    }
                  }}
                  className="cursor-pointer border-t align-top transition-colors hover:bg-muted/40"
                >
                  <td className="max-w-[18rem] px-4 py-3 font-medium [overflow-wrap:anywhere]">{test.title}</td>
                  <td className="px-4 py-3">
                    <div>{test.subject}</div>
                    <div className="text-xs text-ink-500">{test.topic}</div>
                  </td>
                  <td className="px-4 py-3 tabular-nums">{test.question_count ?? 0}</td>
                  <td className="whitespace-nowrap px-4 py-3">{test.duration_minutes} min</td>
                  <td className="px-4 py-3">
                    <div className="flex flex-wrap gap-1">
                      {test.shared_material_html?.trim() && (
                        <Badge variant="secondary">{t("tests_content_material")}</Badge>
                      )}
                      {test.audio_url && <Badge variant="secondary">{t("tests_content_audio")}</Badge>}
                    </div>
                  </td>
                  <td className="px-4 py-3" onClick={(e) => e.stopPropagation()}>
                    <div className="flex items-center justify-end gap-1">
                      <Button
                        type="button"
                        size="sm"
                        variant="ghost"
                        onClick={() => openEdit(test)}
                        aria-label={t("action_edit")}
                      >
                        <Pencil className="mr-1 size-3.5" />
                        {t("action_edit")}
                      </Button>
                      <Button
                        type="button"
                        size="sm"
                        variant="ghost"
                        onClick={() => handleDelete(test.id)}
                        disabled={remove.isPending && deletingId === test.id}
                        aria-label={t("action_delete")}
                        className="text-danger hover:text-danger"
                      >
                        <Trash2 className="mr-1 size-3.5" />
                        {t("action_delete")}
                      </Button>
                    </div>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}

      {!isLoading && !isError && tests.length === 0 && (
        <div className="md-card-outlined px-4 py-8 text-center text-ink-500">
          {t("tests_empty")}
        </div>
      )}

      {!isLoading && !isError && tests.length > 0 && (
        <div className="flex items-center justify-between gap-3">
          <p className="text-label text-sm text-ink-500">
            {t("tests_shown_of_total")
              .replace("{shown}", String(tests.length))
              .replace("{total}", String(testsResp?.total ?? tests.length))}
          </p>
          {testsResp?.next_cursor && (
            <Button
              type="button"
              variant="outline"
              className="rounded-full"
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