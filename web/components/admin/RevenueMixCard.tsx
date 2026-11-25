"use client";

import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { Ring } from "@/components/ui/ring";
import { formatRupiah } from "@/lib/format";
import { useTranslation } from "@/lib/i18n";

type Amount = { value: number; prev?: number };

const positive = (n: number) => Math.max(0, n);
const share = (digital: number, physical: number) => {
  const d = positive(digital);
  const total = d + positive(physical);
  return total > 0 ? Math.round((d / total) * 100) : 0;
};

export function RevenueMixCard({ digital, physical }: { digital: Amount; physical: Amount }) {
  const { t } = useTranslation();
  const hasRevenue = positive(digital.value) + positive(physical.value) > 0;
  const digitalShare = share(digital.value, physical.value);
  const physicalShare = hasRevenue ? 100 - digitalShare : 0;

  const prevDigital = digital.prev ?? 0;
  const prevPhysical = physical.prev ?? 0;
  const delta = positive(prevDigital) + positive(prevPhysical) > 0 ? digitalShare - share(prevDigital, prevPhysical) : null;
  const changeKey =
    delta === null ? null : delta > 0 ? "admin_home_mix_share_up" : delta < 0 ? "admin_home_mix_share_down" : "admin_home_mix_share_same";

  const rows = [
    { id: "digital", name: t("admin_home_series_digital"), hint: t("admin_home_mix_digital_hint"), amount: digital.value, pct: digitalShare, dot: "bg-brand-600" },
    { id: "physical", name: t("admin_home_series_physical"), hint: t("admin_home_mix_physical_hint"), amount: physical.value, pct: physicalShare, dot: "bg-berry-600" },
  ];

  return (
    <Card>
      <CardHeader>
        <CardTitle>{t("admin_home_chart_mix_title")}</CardTitle>
        <CardDescription>{t("admin_home_chart_mix_sub")}</CardDescription>
      </CardHeader>
      <CardContent>
        {digital.value === 0 && physical.value === 0 ? (
          <div className="py-12 text-center text-ink-600">{t("admin_home_chart_empty")}</div>
        ) : (
          <div className="flex flex-col gap-4">
            <div className="flex flex-col items-center gap-6 sm:flex-row">
              <Ring value={digitalShare} track="var(--color-berry-600)" label={`${t("admin_home_series_digital")} ${digitalShare}%`}>
                <span className="type-h1">{digitalShare}%</span>
              </Ring>
              <ul className="flex w-full min-w-0 flex-1 flex-col gap-3">
                {rows.map((r) => (
                  <li key={r.id} className="flex items-center gap-3">
                    <span aria-hidden className={`size-3 shrink-0 rounded-full ${r.dot}`} />
                    <div className="min-w-0 flex-1">
                      <div className="type-label">{r.name}</div>
                      <div className="type-caption text-ink-600">{r.hint}</div>
                    </div>
                    <div className="text-right">
                      <div className="type-label">{formatRupiah(r.amount)}</div>
                      <div data-testid={`mix-${r.id}-share`} className="type-caption text-ink-600">{r.pct}%</div>
                    </div>
                  </li>
                ))}
              </ul>
            </div>
            {changeKey && delta !== null && (
              <p data-testid="mix-change" className="type-body-sm text-ink-600">
                {t(changeKey).replace("{n}", String(Math.abs(delta)))}
              </p>
            )}
          </div>
        )}
      </CardContent>
    </Card>
  );
}
