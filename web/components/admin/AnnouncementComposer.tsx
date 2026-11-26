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

const TYPE_LABEL = {
  announcement: "notification_announcement",
  promo: "notification_promo",
  exam: "notification_exam",
} as const;

const RECIPIENT_LABEL = {
  all: "notification_all_users",
  students: "notification_students",
  admins: "notification_admins",
} as const;

interface AnnouncementComposerProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  announcement?: Announcement | null;
}

export function AnnouncementComposer({
  open,
  onOpenChange,
  announcement,
}: AnnouncementComposerProps) {
  const { t } = useTranslation();
  const createMutation = useCreateAnnouncement();
  const updateMutation = useUpdateAnnouncement();

  const [title, setTitle] = useState("");
  const [message, setMessage] = useState("");
  const [type, setType] = useState("announcement");
  const [recipients, setRecipients] = useState("all");
  const [scheduledAt, setScheduledAt] = useState("");

  useEffect(() => {
    if (!open) return;
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
  }, [open, announcement]);

  const handleClose = () => onOpenChange(false);

  const handleSaveDraft = () => {
    if (announcement) {
      updateMutation.mutate({
        id: announcement.id,
        input: { title, message, type, recipients },
      });
    } else {
      createMutation.mutate({
        title,
        message,
        type,
        recipients,
        status: "draft",
      });
    }
    handleClose();
  };

  const handleSchedule = () => {
    const scheduledAtISO = new Date(scheduledAt).toISOString();
    if (announcement) {
      updateMutation.mutate({
        id: announcement.id,
        input: { title, message, type, recipients, scheduled_at: scheduledAtISO },
      });
    } else {
      createMutation.mutate({
        title,
        message,
        type,
        recipients,
        status: "scheduled",
        scheduled_at: scheduledAtISO,
      });
    }
    handleClose();
  };

  const handleSendNow = () => {
    if (announcement) {
      updateMutation.mutate({
        id: announcement.id,
        input: { title, message, type, recipients },
      });
    } else {
      createMutation.mutate({
        title,
        message,
        type,
        recipients,
        status: "sent",
      });
    }
    handleClose();
  };

  const canSchedule = scheduledAt !== "";

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="sm:max-w-[480px]">
        <DialogHeader>
          <DialogTitle>
            {(announcement ? t("update") : t("create")) + " " + t("notification")}
          </DialogTitle>
        </DialogHeader>
        <div className="space-y-4">
          <div>
            <Label>{t("notification_title")}</Label>
            <Input
              value={title}
              onChange={(e) => setTitle(e.target.value)}
              placeholder={t("notification_title")}
            />
          </div>
          <div>
            <Label>{t("notification_message")}</Label>
            <textarea
              value={message}
              onChange={(e: React.ChangeEvent<HTMLTextAreaElement>) =>
                setMessage(e.target.value)
              }
              placeholder={t("notification_message")}
              rows={4}
              className={cn(
                "min-h-[96px] w-full rounded-md border border-input bg-transparent px-3 py-2 text-sm shadow-xs outline-none transition-[color,box-shadow]",
                "focus-visible:border-ring focus-visible:ring-[3px] focus-visible:ring-brand-300/50",
                "placeholder:text-ink-600 disabled:pointer-events-none disabled:cursor-not-allowed disabled:opacity-50"
              )}
            />
          </div>
          <div className="grid grid-cols-2 gap-3">
            <div>
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
            <div>
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
          <div>
            <Label>{t("notification_scheduled_at")}</Label>
            <Input
              type="datetime-local"
              value={scheduledAt}
              onChange={(e) => setScheduledAt(e.target.value)}
              className="w-full"
              data-testid="scheduled-at-input"
            />
          </div>
          <div className="flex justify-end gap-2">
            <Button variant="outline" onClick={handleClose}>
              {t("cancel")}
            </Button>
            <Button variant="outline" onClick={handleSaveDraft}>
              {t("save")} {t("notification_draft")}
            </Button>
            <Button
              variant="outline"
              onClick={handleSchedule}
              disabled={!canSchedule}
            >
              {t("notification_schedule")}
            </Button>
            <Button onClick={handleSendNow}>
              <Send className="mr-1 size-4" />
              {t("send")}
            </Button>
          </div>
        </div>
      </DialogContent>
    </Dialog>
  );
}
