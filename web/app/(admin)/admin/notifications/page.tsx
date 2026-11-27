"use client";

import { useState } from "react";
import { Megaphone } from "lucide-react";
import { AdminPageHeader } from "@/components/admin/AdminPageHeader";
import { AnnouncementTable } from "@/components/admin/AnnouncementTable";
import { AnnouncementComposer, RECIPIENT_LABEL, TYPE_LABEL } from "@/components/admin/AnnouncementComposer";
import { PurchaseNotificationFeed } from "@/components/admin/PurchaseNotificationFeed";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
import { useTranslation } from "@/lib/i18n";
import { useAdminAnnouncements, type Announcement } from "@/lib/hooks/admin-announcements";

type NotificationsTab = "inbox" | "announcements";

export default function NotificationsPage() {
  const { t, lang } = useTranslation();
  const shortDate = new Intl.DateTimeFormat(lang === "en" ? "en-US" : "id-ID", { day: "numeric", month: "short", timeZone: "Asia/Jakarta" });
  const [tab, setTab] = useState<NotificationsTab>("announcements");
  const { data: announcements } = useAdminAnnouncements();
  const sent = (announcements ?? []).filter((a) => a.status === "sent");
  const [composerOpen, setComposerOpen] = useState(false);
  const [editing, setEditing] = useState<Announcement | null>(null);

  const handleCreate = () => {
    setEditing(null);
    setComposerOpen(true);
  };

  const handleEdit = (ann: Announcement) => {
    setEditing(ann);
    setComposerOpen(true);
  };

  const handleClose = () => {
    setComposerOpen(false);
    setEditing(null);
  };

  return (
    <div className="flex flex-col gap-5 fade-in">
      <AdminPageHeader
        title={t("notifications_page_title")}
        description={t("notifications_page_description")}
      />

      <Tabs value={tab} onValueChange={(v) => setTab(v as NotificationsTab)}>
        <TabsList className="mb-5">
          <TabsTrigger value="announcements">
            {t("notification_tab_announcements")}
          </TabsTrigger>
          <TabsTrigger value="inbox">
            {t("notification_tab_inbox")}
          </TabsTrigger>
        </TabsList>

        <TabsContent value="inbox">
          <PurchaseNotificationFeed />
        </TabsContent>

        <TabsContent value="announcements" className="flex flex-col gap-4">
          <div className="grid items-start gap-4 lg:grid-cols-[minmax(0,1fr)_340px]">
            <AnnouncementComposer inline open onOpenChange={() => {}} />
            <section className="md-card-outlined flex flex-col gap-3.5" aria-labelledby="announcements-sent-title">
              <h2 id="announcements-sent-title" className="type-h2">{t("notification_sent_title")}</h2>
              {sent.length === 0 ? (
                <p className="text-[13px] text-ink-600">{t("notification_sent_empty")}</p>
              ) : (
                <div className="flex flex-col gap-1.5">
                  {sent.map((a) => (
                    <div key={a.id} className="flex items-start gap-3 rounded-2xl p-2.5">
                      <span className="grid size-[38px] shrink-0 place-items-center rounded-[12px] bg-ember-50 text-ember-700">
                        <Megaphone aria-hidden className="size-4" />
                      </span>
                      <div className="min-w-0 flex-1">
                        <b className="block font-[family-name:var(--font-display)] text-sm font-bold">{a.title}</b>
                        <p className="my-0.5 line-clamp-2 text-[13px] text-ink-600">{a.message}</p>
                        <small className="text-xs text-ink-600">
                          {a.recipients in RECIPIENT_LABEL ? t(RECIPIENT_LABEL[a.recipients as keyof typeof RECIPIENT_LABEL]) : a.recipients} · {a.type in TYPE_LABEL ? t(TYPE_LABEL[a.type as keyof typeof TYPE_LABEL]) : a.type}{a.sent_at ? ` · ${shortDate.format(new Date(a.sent_at))}` : ""}
                        </small>
                      </div>
                    </div>
                  ))}
                </div>
              )}
            </section>
          </div>
          <AnnouncementTable onCreateClick={handleCreate} onEdit={handleEdit} />
        </TabsContent>
      </Tabs>

      <AnnouncementComposer
        open={composerOpen}
        onOpenChange={(open) => {
          if (!open) handleClose();
        }}
        announcement={editing}
      />
    </div>
  );
}
