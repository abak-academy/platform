"use client";

import { useEffect, useState } from "react";
import { toast } from "sonner";
import { Plus, Trash2 } from "lucide-react";
import {
  Dialog,
  DialogContent,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import {
  useTopics,
  useCreateTopic,
  useDeleteTopic,
} from "@/lib/hooks/admin-topics";
import { useTranslation } from "@/lib/i18n";

interface TopicsModalProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
}

export function TopicsModal({ open, onOpenChange }: TopicsModalProps) {
  const { t } = useTranslation();
  const [name, setName] = useState("");
  const [subject, setSubject] = useState("");
  const [deletingId, setDeletingId] = useState<string | null>(null);

  const topics = useTopics();
  const create = useCreateTopic();
  const remove = useDeleteTopic();

  useEffect(() => {
    if (!open) {
      setName("");
      setSubject("");
      setDeletingId(null);
    }
  }, [open]);

  async function handleAdd(e: React.FormEvent) {
    e.preventDefault();
    if (!name.trim() || !subject.trim()) {
      toast.error(t("tests_validation_topic_required"));
      return;
    }
    try {
      await create.mutateAsync({ name: name.trim(), subject: subject.trim() });
      toast.success(t("changes_saved"));
      setName("");
      setSubject("");
    } catch (e) {
      toast.error(e instanceof Error ? e.message : t("error_generic"));
    }
  }

  async function handleDelete(id: string) {
    setDeletingId(id);
    try {
      await remove.mutateAsync(id);
      toast.success(t("changes_saved"));
    } catch (e) {
      toast.error(t("topic_delete_blocked"));
    } finally {
      setDeletingId(null);
    }
  }

  const rows = topics.data?.data ?? [];

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="sm:max-w-[480px]">
        <DialogHeader>
          <DialogTitle>{t("manage_topics")}</DialogTitle>
        </DialogHeader>

        <form onSubmit={handleAdd} className="space-y-3 py-2">
          <div className="grid gap-2">
            <Label htmlFor="topic-name">{t("topic_name")}</Label>
            <Input
              id="topic-name"
              value={name}
              onChange={(e) => setName(e.target.value)}
              placeholder={t("topic_name")}
              disabled={create.isPending}
            />
          </div>
          <div className="grid gap-2">
            <Label htmlFor="topic-subject">{t("subject")}</Label>
            <Input
              id="topic-subject"
              value={subject}
              onChange={(e) => setSubject(e.target.value)}
              placeholder={t("subject")}
              disabled={create.isPending}
            />
          </div>
          <Button
            type="submit"
            variant="outline"
            disabled={create.isPending || !name.trim() || !subject.trim()}
          >
            <Plus className="mr-1 size-4" />
            {t("add_topic")}
          </Button>
        </form>

        <div className="max-h-[320px] overflow-y-auto rounded-lg border">
          {topics.isLoading && (
            <div className="p-4 text-center text-muted-foreground">{t("sys_loading")}</div>
          )}
          {!topics.isLoading && rows.length === 0 && (
            <div className="p-4 text-center text-muted-foreground">
              {t("tests_picker_empty")}
            </div>
          )}
          {rows.map((topic) => {
            const hasQuestions = (topic.question_count ?? 0) > 0;
            return (
              <div
                key={topic.id}
                className="flex items-center justify-between gap-3 border-b p-3 last:border-b-0"
              >
                <div className="min-w-0 flex-1">
                  <p className="truncate text-sm font-medium">{topic.name}</p>
                  <p className="truncate text-xs text-muted-foreground">
                    {topic.subject} · {topic.question_count ?? 0} {t("questions_in_topic")}
                  </p>
                </div>
                <Button
                  type="button"
                  size="icon-xs"
                  variant="ghost"
                  onClick={() => handleDelete(topic.id)}
                  disabled={hasQuestions || (remove.isPending && deletingId === topic.id)}
                  aria-label={t("action_delete")}
                  title={hasQuestions ? t("topic_delete_blocked") : t("action_delete")}
                >
                  <Trash2 className="size-4" />
                </Button>
              </div>
            );
          })}
        </div>

        <DialogFooter>
          <Button type="button" variant="outline" onClick={() => onOpenChange(false)}>
            {t("cancel")}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
