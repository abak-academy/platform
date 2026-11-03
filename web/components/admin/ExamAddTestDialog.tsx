"use client";

import { Plus, Search } from "lucide-react";
import { Button } from "@/components/ui/button";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { Input } from "@/components/ui/input";
import { Skeleton } from "@/components/ui/skeleton";
import { useTranslation } from "@/lib/i18n";
import type { Test } from "@/lib/types";

interface ExamAddTestDialogProps {
  open: boolean;
  onClose: () => void;
  tests: Test[];
  attachedIds: string[];
  search: string;
  onSearch: (value: string) => void;
  loading: boolean;
  fetching: boolean;
  total?: number;
  hasMore: boolean;
  onLoadMore: () => void;
  onAdd: (testId: string) => void;
}

export function ExamAddTestDialog({
  open,
  onClose,
  tests,
  attachedIds,
  search,
  onSearch,
  loading,
  fetching,
  total,
  hasMore,
  onLoadMore,
  onAdd,
}: ExamAddTestDialogProps) {
  const { t } = useTranslation();
  const attached = new Set(attachedIds);

  return (
    <Dialog open={open} onOpenChange={(next) => { if (!next) onClose(); }}>
      <DialogContent className="sm:max-w-xl">
        <DialogHeader>
          <DialogTitle className="font-serif">{t("exam_admin_add_dialog_title")}</DialogTitle>
          <DialogDescription>{t("exam_admin_add_dialog_desc")}</DialogDescription>
        </DialogHeader>

        <div className="relative">
          <Search className="absolute left-2.5 top-1/2 size-4 -translate-y-1/2 text-ink-400" />
          <Input
            value={search}
            onChange={(e) => onSearch(e.target.value)}
            placeholder={t("search")}
            className="pl-9"
          />
        </div>

        <div className="max-h-[50vh] overflow-y-auto">
          {loading && tests.length === 0 ? (
            <div className="space-y-2">
              {Array.from({ length: 3 }).map((_, i) => (
                <Skeleton key={i} className="h-12 w-full" />
              ))}
            </div>
          ) : tests.length === 0 ? (
            <div className="rounded-md border border-dashed p-6 text-center text-sm text-ink-500">
              {t("exam_admin_add_dialog_empty")}
            </div>
          ) : (
            <ul className="space-y-2">
              {tests.map((test) => (
                <li
                  key={test.id}
                  className="flex items-center justify-between gap-3 rounded-md border border-line p-3 text-sm"
                >
                  <div className="min-w-0">
                    <div className="truncate font-medium">{test.title}</div>
                    <div className="text-xs text-ink-500">
                      {test.subject} · {test.topic}
                      {test.question_count != null &&
                        ` · ${t("exam_admin_section_questions").replace("{n}", String(test.question_count))}`}{" "}
                      · {test.duration_minutes} {t("minutes")}
                    </div>
                  </div>
                  {attached.has(test.id) ? (
                    <span className="shrink-0 rounded-full bg-muted px-2 py-0.5 text-xs font-medium text-ink-600">
                      {t("exam_admin_add_dialog_added")}
                    </span>
                  ) : (
                    <Button
                      type="button"
                      size="sm"
                      variant="outline"
                      className="shrink-0 rounded-full"
                      onClick={() => onAdd(test.id)}
                    >
                      <Plus className="size-3" />
                      {t("admin_exam_detail_tests_add")}
                    </Button>
                  )}
                </li>
              ))}
            </ul>
          )}
        </div>

        <div className="text-xs text-ink-500">
          {t("tests_shown_of_total")
            .replace("{shown}", String(tests.length))
            .replace("{total}", String(total ?? tests.length))}
        </div>
        {hasMore && (
          <div className="flex justify-end">
            <Button
              type="button"
              size="sm"
              variant="outline"
              className="rounded-full"
              onClick={onLoadMore}
              disabled={fetching}
            >
              {fetching ? t("sys_loading") : t("sys_load_more")}
            </Button>
          </div>
        )}

        <DialogFooter className="sm:items-center sm:justify-between">
          <span className="text-xs text-ink-500">{t("exam_admin_add_dialog_hint")}</span>
          <Button type="button" className="rounded-full" onClick={onClose}>
            {t("exam_admin_add_dialog_done")}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
