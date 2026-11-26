"use client";

import { useState } from "react";
import { Plus } from "lucide-react";
import { AdminPageHeader } from "@/components/admin/AdminPageHeader";
import { AnnouncementTable } from "@/components/admin/AnnouncementTable";
import { AnnouncementComposer } from "@/components/admin/AnnouncementComposer";
import { PurchaseNotificationFeed } from "@/components/admin/PurchaseNotificationFeed";
import { Button } from "@/components/ui/button";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
import { useTranslation } from "@/lib/i18n";
import type { Announcement } from "@/lib/hooks/admin-announcements";

type NotificationsTab = "inbox" | "announcements";

export default function NotificationsPage() {
  const { t } = useTranslation();
  const [tab, setTab] = useState<NotificationsTab>("inbox");
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
        actions={
          tab === "announcements" ? (
            <Button variant="secondary" onClick={handleCreate}>
              <Plus aria-hidden />
              {t("create")}
            </Button>
          ) : null
        }
      />

      <Tabs value={tab} onValueChange={(v) => setTab(v as NotificationsTab)}>
        <TabsList className="mb-5">
          <TabsTrigger value="inbox">
            {t("notification_tab_inbox")}
          </TabsTrigger>
          <TabsTrigger value="announcements">
            {t("notification_tab_announcements")}
          </TabsTrigger>
        </TabsList>

        <TabsContent value="inbox">
          <PurchaseNotificationFeed />
        </TabsContent>

        <TabsContent value="announcements">
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
