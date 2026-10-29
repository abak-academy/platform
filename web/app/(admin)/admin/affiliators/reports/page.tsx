"use client";

import { useState } from "react";
import { BarChart3 } from "lucide-react";
import {
  AFFILIATE_REPORT_TABS,
  AffiliatorsReport,
  ProductsReport,
  PromosReport,
  SchoolsReport,
  TrendReport,
} from "@/components/admin/AffiliateReports";
import { AdminPageHeader } from "@/components/admin/AdminPageHeader";
import { NoAccess } from "@/components/admin/NoAccess";
import { PeriodBar, type PeriodPreset, type PeriodRange } from "@/components/admin/PeriodBar";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
import { useHasCapability } from "@/lib/hooks/use-capability";
import { useTranslation } from "@/lib/i18n";
import { queryRange } from "@/lib/period";

export default function AffiliateReportsPage() {
  const { t } = useTranslation();
  const allowed = useHasCapability("system:admin");
  const [preset, setPreset] = useState<PeriodPreset>("30d");
  const [range, setRange] = useState<PeriodRange>({});
  // Hold the query back while a custom range is half typed, like the dashboards.
  const [resolved, setResolved] = useState<PeriodRange>(() => queryRange("30d", {}));

  if (!allowed) return <NoAccess />;

  function handlePeriod(next: PeriodPreset, nextRange: PeriodRange) {
    setPreset(next);
    setRange(nextRange);
    if (next !== "custom" || (nextRange.from && nextRange.to)) setResolved(queryRange(next, nextRange));
  }

  return (
    <div className="fade-in space-y-6">
      <AdminPageHeader icon={BarChart3} title={t("aff_report_title")} description={t("aff_report_subtitle")} />
      <PeriodBar preset={preset} range={range} onChange={handlePeriod} allowAllTime />

      <Tabs defaultValue="affiliators">
        <TabsList aria-label={t("aff_report_title")} className="h-auto flex-wrap">
          {AFFILIATE_REPORT_TABS.map((tab) => (
            <TabsTrigger key={tab.id} value={tab.id}>
              {t(tab.labelKey)}
            </TabsTrigger>
          ))}
        </TabsList>
        <TabsContent value="affiliators" className="pt-4"><AffiliatorsReport period={resolved} /></TabsContent>
        <TabsContent value="schools" className="pt-4"><SchoolsReport period={resolved} /></TabsContent>
        <TabsContent value="products" className="pt-4"><ProductsReport period={resolved} /></TabsContent>
        <TabsContent value="promos" className="pt-4"><PromosReport period={resolved} /></TabsContent>
        <TabsContent value="trend" className="pt-4"><TrendReport period={resolved} /></TabsContent>
      </Tabs>
    </div>
  );
}
