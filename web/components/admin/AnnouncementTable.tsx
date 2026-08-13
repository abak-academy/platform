"use client";

import { useMemo, useState } from "react";
import {
  Plus,
  Bell,
  Send,
  Clock,
  Users,
  Tag,
  MoreHorizontal,
  Edit,
  Trash2,
} from "lucide-react";
import { type DICT, useTranslation } from "@/lib/i18n";

type DictKey = keyof (typeof DICT)["id"];
import { StatCard } from "@/components/admin/StatCard";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import { Tabs, TabsList, TabsTrigger } from "@/components/ui/tabs";
import { cn } from "@/lib/utils";
import {
  useAdminAnnouncements,
  useDeleteAnnouncement,
  useSendAnnouncement,
} from "@/lib/hooks/admin-announcements";
import type { Announcement } from "@/lib/hooks/admin-announcements";

const TYPE_TONE: Record<string, string> = {
  announcement: "bg-info-bg text-info border-info",
  promo: "bg-success-bg text-success border-success",
  exam: "bg-warn-bg text-warn border-warn",
};

const TYPE_ICON: Record<string, React.ReactNode> = {
  announcement: <Bell className="size-4" />,
  promo: <Tag className="size-4" />,
  exam: <Clock className="size-4" />,
};

const TYPE_LABEL: Record<string, DictKey> = {
  announcement: "notification_announcement",
  promo: "notification_promo",
  exam: "notification_exam",
};

const RECIPIENT_LABEL: Record<string, DictKey> = {
  all: "notification_all_users",
  students: "notification_students",
  admins: "notification_admins",
};

const STATUS_TONE: Record<string, string> = {
  draft: "bg-ink-100 text-ink-600 border-line",
  scheduled: "bg-info-bg text-info border-info",
  sent: "bg-success-bg text-success border-success",
};

function fmtDateTime(iso?: string | null) {
  if (!iso) return "—";
  const d = new Date(iso);
  return d.toLocaleString("id-ID", {
    day: "2-digit",
    month: "short",
    year: "numeric",
    hour: "2-digit",
    minute: "2-digit",
  });
}

interface AnnouncementTableProps {
  onCreateClick: () => void;
  onEdit: (announcement: Announcement) => void;
}

export function AnnouncementTable({ onCreateClick, onEdit }: AnnouncementTableProps) {
  const { t } = useTranslation();
  const { data: announcements = [], isLoading } = useAdminAnnouncements();
  const deleteMutation = useDeleteAnnouncement();
  const sendMutation = useSendAnnouncement();

  const [tab, setTab] = useState("all");

  const rows = useMemo(() => {
    if (tab === "all") return announcements;
    return announcements.filter((n) => n.status === tab);
  }, [tab, announcements]);

  const stats = useMemo(() => {
    return {
      total: announcements.length,
      sent: announcements.filter((n) => n.status === "sent").length,
      scheduled: announcements.filter((n) => n.status === "scheduled").length,
      draft: announcements.filter((n) => n.status === "draft").length,
    };
  }, [announcements]);

  if (isLoading) {
    return (
      <div className="p-4 text-center text-ink-600">{t("sys_loading")}</div>
    );
  }

  return (
    <section className="space-y-4">
      <p className="text-xs text-ink-600">{t("notification_announcements_description")}</p>

      <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-4">
        <StatCard label={t("tab_all")} value={String(stats.total)} accent="primary" />
        <StatCard label={t("notification_sent")} value={String(stats.sent)} accent="secondary" />
        <StatCard label={t("notification_scheduled")} value={String(stats.scheduled)} accent="tertiary" />
        <StatCard label={t("notification_draft")} value={String(stats.draft)} accent="primary" />
      </div>

      <Tabs value={tab} onValueChange={(v) => setTab(v)}>
        <TabsList>
          <TabsTrigger value="all" className="text-xs">{t("tab_all")}</TabsTrigger>
          <TabsTrigger value="sent" className="text-xs">{t("notification_sent")}</TabsTrigger>
          <TabsTrigger value="scheduled" className="text-xs">{t("notification_scheduled")}</TabsTrigger>
          <TabsTrigger value="draft" className="text-xs">{t("notification_draft")}</TabsTrigger>
        </TabsList>
      </Tabs>

      {rows.length === 0 ? (
        <div className="flex flex-col items-center gap-2 rounded-xl border border-line bg-surface px-6 py-12 text-center">
          <span className="flex size-12 items-center justify-center rounded-full bg-surface-2 text-ink-500">
            <Bell className="size-6" />
          </span>
          <p className="text-sm font-medium text-ink-900">
            {tab === "all"
              ? t("notification_announcements_empty")
              : t("notification_announcements_empty_filtered")}
          </p>
          {tab === "all" && (
            <>
              <p className="max-w-sm text-xs text-ink-600">
                {t("notification_announcements_empty_hint")}
              </p>
              <Button size="sm" className="mt-2" onClick={onCreateClick}>
                <Plus className="mr-1 size-4" />
                {t("create")}
              </Button>
            </>
          )}
        </div>
      ) : (
        <div className="md-card-outlined overflow-hidden">
          <div className="overflow-x-auto">
            <table className="w-full text-sm">
              <thead className="bg-surface-2 text-left text-xs font-semibold text-ink-600">
                <tr>
                  <th className="px-4 py-3">{t("notification")}</th>
                  <th className="px-4 py-3">{t("notification_type")}</th>
                  <th className="px-4 py-3">{t("notification_recipients")}</th>
                  <th className="px-4 py-3">{t("notification_status")}</th>
                  <th className="px-4 py-3">{t("notification_time")}</th>
                  <th className="px-4 py-3">{t("notification_recipients_targeted")}</th>
                  <th className="px-4 py-3 text-right" />
                </tr>
              </thead>
              <tbody className="divide-y divide-line">
                {rows.map((n) => (
                  <tr key={n.id} className="group hover:bg-surface-2">
                    <td className="px-4 py-3">
                      <div className="max-w-[280px] truncate font-medium text-ink-900">
                        {n.title}
                      </div>
                      <div className="font-mono text-[11px] text-ink-500">{n.id}</div>
                    </td>
                    <td className="px-4 py-3">
                      <Badge
                        variant="outline"
                        className={cn("text-[11px] font-semibold", TYPE_TONE[n.type])}
                      >
                        <span className="mr-1">{TYPE_ICON[n.type]}</span>
                        {t(TYPE_LABEL[n.type])}
                      </Badge>
                    </td>
                    <td className="px-4 py-3">
                      <span className="inline-flex items-center gap-1 text-xs text-ink-600">
                        <Users className="size-3" />
                        {t(RECIPIENT_LABEL[n.recipients])}
                      </span>
                    </td>
                    <td className="px-4 py-3">
                      <Badge
                        variant="outline"
                        className={cn(
                          "text-[11px] font-semibold capitalize",
                          STATUS_TONE[n.status]
                        )}
                      >
                        {n.status}
                      </Badge>
                    </td>
                    <td className="px-4 py-3 text-xs text-ink-600">
                      {n.status === "sent"
                        ? fmtDateTime(n.sent_at)
                        : fmtDateTime(n.scheduled_at)}
                    </td>
                    <td className="px-4 py-3 text-xs text-ink-600">
                      {n.recipient_count != null
                        ? n.recipient_count.toLocaleString("id-ID")
                        : "—"}
                    </td>
                    <td className="px-4 py-3 text-right">
                      {n.status === "sent" ? (
                        <span
                          data-testid="row-actions-sent"
                          className="inline-flex items-center text-[11px] text-ink-400"
                        >
                          —
                        </span>
                      ) : (
                        <DropdownMenu>
                          <DropdownMenuTrigger asChild>
                            <Button
                              variant="ghost"
                              size="icon-xs"
                              data-testid="row-actions-dropdown"
                            >
                              <MoreHorizontal className="size-4 text-ink-500" />
                            </Button>
                          </DropdownMenuTrigger>
                          <DropdownMenuContent align="end">
                            <DropdownMenuItem onClick={() => onEdit(n)}>
                              <Edit className="mr-2 size-4" />
                              {t("action_edit")}
                            </DropdownMenuItem>
                            <DropdownMenuItem
                              onClick={() => sendMutation.mutate(n.id)}
                            >
                              <Send className="mr-2 size-4" />
                              {t("notification_send_now")}
                            </DropdownMenuItem>
                            <DropdownMenuItem
                              className="text-danger"
                              onClick={() => deleteMutation.mutate(n.id)}
                            >
                              <Trash2 className="mr-2 size-4" />
                              {t("action_delete")}
                            </DropdownMenuItem>
                          </DropdownMenuContent>
                        </DropdownMenu>
                      )}
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        </div>
      )}
    </section>
  );
}
