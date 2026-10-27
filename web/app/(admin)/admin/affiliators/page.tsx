"use client";

import { Handshake } from "lucide-react";
import { AdminPageHeader } from "@/components/admin/AdminPageHeader";
import { AffiliatorListTab } from "@/components/admin/AffiliatorListTab";
import { AffiliatorPromoteTab } from "@/components/admin/AffiliatorPromoteTab";
import { NoAccess } from "@/components/admin/NoAccess";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
import { useHasCapability } from "@/lib/hooks/use-capability";
import { useTranslation } from "@/lib/i18n";

export default function AdminAffiliatorsPage() {
  const { t } = useTranslation();
  const allowed = useHasCapability("system:admin");

  if (!allowed) return <NoAccess />;

  return (
    <div className="fade-in space-y-6">
      <AdminPageHeader icon={Handshake} title={t("aff_admin_title")} description={t("aff_admin_subtitle")} />

      <Tabs defaultValue="list">
        <TabsList aria-label={t("aff_admin_title")}>
          <TabsTrigger value="list">{t("aff_admin_tab_list")}</TabsTrigger>
          <TabsTrigger value="promote">{t("aff_admin_tab_promote")}</TabsTrigger>
        </TabsList>
        <TabsContent value="list" className="pt-4">
          <AffiliatorListTab />
        </TabsContent>
        <TabsContent value="promote" className="pt-4">
          <AffiliatorPromoteTab />
        </TabsContent>
      </Tabs>
    </div>
  );
}
