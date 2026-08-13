"use client";

import { useState } from "react";
import { Bell, Plus } from "lucide-react";
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
    <div className="space-y-6 fade-in">
      <AdminPageHeader
        icon={Bell}
        title={t("notifications_page_title")}
        description={t("notifications_page_description")}
        actionsAlign="end"
        actions={
          tab === "announcements" ? (
            <Button size="sm" onClick={handleCreate}>
              <Plus className="mr-1 size-4" />
              {t("create")}
            </Button>
          ) : null
        }
      />

      <Tabs value={tab} onValueChange={(v) => setTab(v as NotificationsTab)}>
        <TabsList className="mb-6">
          <TabsTrigger value="inbox" className="text-xs">
            {t("notification_tab_inbox")}
          </TabsTrigger>
          <TabsTrigger value="announcements" className="text-xs">
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
