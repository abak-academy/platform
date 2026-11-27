"use client";

import { useState, useEffect } from "react";
import { Send } from "lucide-react";
import { useTranslation } from "@/lib/i18n";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { cn } from "@/lib/utils";
import {
  useCreateAnnouncement,
  useUpdateAnnouncement,
} from "@/lib/hooks/admin-announcements";
import type { Announcement } from "@/lib/hooks/admin-announcements";

const NOTIFICATION_TYPES = ["announcement", "promo", "exam"] as const;
const RECIPIENT_GROUPS = ["all", "students", "admins"] as const;

export const TYPE_LABEL = {
  announcement: "notification_announcement",
  promo: "notification_promo",
  exam: "notification_exam",
} as const;

export const RECIPIENT_LABEL = {
  all: "notification_all_users",
  students: "notification_students",
  admins: "notification_admins",
} as const;

interface AnnouncementComposerProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  announcement?: Announcement | null;
  /** Prototype "Tulis pengumuman": the composer as a page card, reset after each submit. */
  inline?: boolean;
}

export function AnnouncementComposer({
  open,
  onOpenChange,
  announcement,
  inline = false,
}: AnnouncementComposerProps) {
  const { t } = useTranslation();
  const createMutation = useCreateAnnouncement();
  const updateMutation = useUpdateAnnouncement();
  // The form now stays filled until the server answers, so block repeat clicks meanwhile.
  const pending = createMutation.isPending || updateMutation.isPending;

  const [title, setTitle] = useState("");
  const [message, setMessage] = useState("");
  const [type, setType] = useState("announcement");
  const [recipients, setRecipients] = useState("all");
  const [scheduledAt, setScheduledAt] = useState("");

  const [resetKey, setResetKey] = useState(0);

  useEffect(() => {
    if (!open && !inline) return;
    if (announcement) {
      setTitle(announcement.title);
      setMessage(announcement.message);
      setType(announcement.type);
      setRecipients(announcement.recipients);
      setScheduledAt(announcement.scheduled_at ?? "");
    } else {
      setTitle("");
      setMessage("");
      setType("announcement");
      setRecipients("all");
      setScheduledAt("");
    }
  }, [open, inline, announcement, resetKey]);

  const handleClose = () => (inline ? setResetKey((k) => k + 1) : onOpenChange(false));
  const closeOnSuccess = { onSuccess: handleClose };

  const handleSaveDraft = () => {
    if (announcement) {
      updateMutation.mutate({
        id: announcement.id,
        input: { title, message, type, recipients },
      }, closeOnSuccess);
    } else {
      createMutation.mutate({
        title,
        message,
        type,
        recipients,
        status: "draft",
      }, closeOnSuccess);
    }
  };

  const handleSchedule = () => {
    const scheduledAtISO = new Date(scheduledAt).toISOString();
    if (announcement) {
      updateMutation.mutate({
        id: announcement.id,
        input: { title, message, type, recipients, scheduled_at: scheduledAtISO },
      }, closeOnSuccess);
    } else {
      createMutation.mutate({
        title,
        message,
        type,
        recipients,
        status: "scheduled",
        scheduled_at: scheduledAtISO,
      }, closeOnSuccess);
    }
  };

  const handleSendNow = () => {
    if (announcement) {
      updateMutation.mutate({
        id: announcement.id,
        input: { title, message, type, recipients },
      }, closeOnSuccess);
    } else {
      createMutation.mutate({
        title,
        message,
        type,
        recipients,
        status: "sent",
      }, closeOnSuccess);
    }
  };

  const canSchedule = scheduledAt !== "";

  const body = (
    <div className="space-y-4">
      <div className="grid gap-2">
        <Label>{t("notification_title")}</Label>
        <Input
          value={title}
          onChange={(e) => setTitle(e.target.value)}
          placeholder={t("notification_title")}
        />
      </div>
      <div className="grid gap-2">
        <Label>{t("notification_message")}</Label>
        <textarea
          value={message}
          onChange={(e: React.ChangeEvent<HTMLTextAreaElement>) =>
            setMessage(e.target.value)
          }
          placeholder={t("notification_message")}
          rows={4}
          className={cn(
            "min-h-[96px] w-full resize-y rounded-[14px] border-[1.5px] border-line bg-surface px-[15px] py-3 text-[15px] outline-none transition-[color,box-shadow]",
            "focus-visible:border-ring focus-visible:ring-[3px] focus-visible:ring-brand-300/50",
            "placeholder:text-ink-600 disabled:pointer-events-none disabled:cursor-not-allowed disabled:opacity-50"
          )}
        />
      </div>
      <div className="grid grid-cols-2 gap-3">
        <div className="grid gap-2">
          <Label>{t("notification_type")}</Label>
          <Select value={type} onValueChange={setType}>
            <SelectTrigger>
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              {NOTIFICATION_TYPES.map((tt) => (
                <SelectItem key={tt} value={tt}>
                  {t(TYPE_LABEL[tt])}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
        </div>
        <div className="grid gap-2">
          <Label>{t("notification_recipients")}</Label>
          <Select value={recipients} onValueChange={setRecipients}>
            <SelectTrigger>
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              {RECIPIENT_GROUPS.map((rg) => (
                <SelectItem key={rg} value={rg}>
                  {t(RECIPIENT_LABEL[rg])}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
        </div>
      </div>
      <div className="grid gap-2">
        <Label>{t("notification_scheduled_at")}</Label>
        <Input
          type="datetime-local"
          value={scheduledAt}
          onChange={(e) => setScheduledAt(e.target.value)}
          className="w-full"
          data-testid="scheduled-at-input"
        />
      </div>
      <div className="flex flex-wrap justify-end gap-2">
        {!inline && (
          <Button variant="outline" onClick={handleClose}>
            {t("cancel")}
          </Button>
        )}
        <Button variant="outline" onClick={handleSaveDraft} disabled={pending}>
          {t("save")} {t("notification_draft")}
        </Button>
        <Button
          variant="outline"
          onClick={handleSchedule}
          disabled={!canSchedule || pending}
        >
          {t("notification_schedule")}
        </Button>
        <Button onClick={handleSendNow} disabled={pending}>
          <Send className="mr-1 size-4" />
          {t("send")}
        </Button>
      </div>
    </div>
  );

  if (inline) {
    return (
      <section className="md-card-outlined flex flex-col gap-3.5" aria-labelledby="announcement-compose-title">
        <h2 id="announcement-compose-title" className="type-h2">{t("notification_compose_title")}</h2>
        {body}
      </section>
    );
  }

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="sm:max-w-[480px]">
        <DialogHeader>
          <DialogTitle>
            {(announcement ? t("update") : t("create")) + " " + t("notification")}
          </DialogTitle>
        </DialogHeader>
        {body}
      </DialogContent>
    </Dialog>
  );
}
