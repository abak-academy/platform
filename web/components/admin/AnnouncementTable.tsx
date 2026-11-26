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
import { DataTable } from "@/components/ui/data-table";
import { EmptyState } from "@/components/ui/empty-state";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import { Tabs, TabsList, TabsTrigger } from "@/components/ui/tabs";
import {
  useAdminAnnouncements,
  useDeleteAnnouncement,
  useSendAnnouncement,
} from "@/lib/hooks/admin-announcements";
import type { Announcement } from "@/lib/hooks/admin-announcements";

const TYPE_TONE: Record<string, "sky" | "teal" | "gold"> = {
  announcement: "sky",
  promo: "teal",
  exam: "gold",
};

const STATUS_VARIANT: Record<string, "teal" | "sky" | "mute"> = { sent: "teal", scheduled: "sky", draft: "mute" };

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
    <section className="flex flex-col gap-4">
      <p className="text-xs text-ink-600">{t("notification_announcements_description")}</p>

      <div className="grid grid-cols-2 gap-3 lg:grid-cols-4">
        <StatCard label={t("tab_all")} value={String(stats.total)} tone="brand" icon={Bell} />
        <StatCard label={t("notification_sent")} value={String(stats.sent)} tone="teal" icon={Send} />
        <StatCard label={t("notification_scheduled")} value={String(stats.scheduled)} tone="sky" icon={Clock} />
        <StatCard label={t("notification_draft")} value={String(stats.draft)} tone="gold" icon={Edit} />
      </div>

      <Tabs value={tab} onValueChange={(v) => setTab(v)}>
        <TabsList>
          <TabsTrigger value="all">{t("tab_all")}</TabsTrigger>
          <TabsTrigger value="sent">{t("notification_sent")}</TabsTrigger>
          <TabsTrigger value="scheduled">{t("notification_scheduled")}</TabsTrigger>
          <TabsTrigger value="draft">{t("notification_draft")}</TabsTrigger>
        </TabsList>
      </Tabs>

      {rows.length === 0 ? (
        <EmptyState
          framed
          icon={Bell}
          title={tab === "all" ? t("notification_announcements_empty") : t("notification_announcements_empty_filtered")}
          text={tab === "all" ? t("notification_announcements_empty_hint") : undefined}
          action={tab === "all" ? (
            <Button size="sm" className="mt-2" onClick={onCreateClick}>
              <Plus className="mr-1 size-4" />
              {t("create")}
            </Button>
          ) : undefined}
        />
      ) : (
        <DataTable
          rows={rows}
          rowKey={(n) => n.id}
          empty=""
          columns={[
            {
              key: "title",
              header: t("notification"),
              cell: (n) => (
                <>
                  <div className="max-w-[280px] truncate font-bold text-ink-900">{n.title}</div>
                  <div className="font-mono text-[11px] text-ink-600">{n.id}</div>
                </>
              ),
            },
            {
              key: "type",
              header: t("notification_type"),
              cell: (n) => (
                <Badge variant={TYPE_TONE[n.type]}>
                  {TYPE_ICON[n.type]}
                  {t(TYPE_LABEL[n.type])}
                </Badge>
              ),
            },
            {
              key: "recipients",
              header: t("notification_recipients"),
              cell: (n) => (
                <span className="inline-flex items-center gap-1 text-xs text-ink-600">
                  <Users className="size-3" />
                  {t(RECIPIENT_LABEL[n.recipients])}
                </span>
              ),
            },
            { key: "status", header: t("notification_status"), cell: (n) => <Badge variant={STATUS_VARIANT[n.status] ?? "mute"} className="capitalize">{n.status}</Badge> },
            { key: "time", header: t("notification_time"), cell: (n) => (n.status === "sent" ? fmtDateTime(n.sent_at) : fmtDateTime(n.scheduled_at)) },
            { key: "count", header: t("notification_recipients_targeted"), cell: (n) => (n.recipient_count != null ? n.recipient_count.toLocaleString("id-ID") : "—") },
          ]}
          rowActions={(n) =>
            n.status === "sent" ? (
              <span data-testid="row-actions-sent" className="inline-flex items-center text-[11px] text-ink-600">
                —
              </span>
            ) : (
              <DropdownMenu>
                <DropdownMenuTrigger asChild>
                  <Button variant="ghost" size="icon-xs" data-testid="row-actions-dropdown">
                    <MoreHorizontal className="size-4 text-ink-600" />
                  </Button>
                </DropdownMenuTrigger>
                <DropdownMenuContent align="end">
                  <DropdownMenuItem onClick={() => onEdit(n)}>
                    <Edit className="mr-2 size-4" />
                    {t("action_edit")}
                  </DropdownMenuItem>
                  <DropdownMenuItem onClick={() => sendMutation.mutate(n.id)}>
                    <Send className="mr-2 size-4" />
                    {t("notification_send_now")}
                  </DropdownMenuItem>
                  <DropdownMenuItem className="text-danger" onClick={() => deleteMutation.mutate(n.id)}>
                    <Trash2 className="mr-2 size-4" />
                    {t("action_delete")}
                  </DropdownMenuItem>
                </DropdownMenuContent>
              </DropdownMenu>
            )
          }
        />
      )}
    </section>
  );
}
