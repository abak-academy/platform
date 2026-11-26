"use client";

import Link from "next/link";
import { FileText, Monitor, Ticket, Users } from "lucide-react";
import { Button } from "@/components/ui/button";
import { useTranslation } from "@/lib/i18n";
import { waHref } from "@/lib/site-config";

const TILES = [
  { icon: Users, key: "land_school_tile_1" },
  { icon: Ticket, key: "land_school_tile_2" },
  { icon: Monitor, key: "land_school_tile_3" },
  { icon: FileText, key: "land_school_tile_4" },
] as const;

export function SchoolBand() {
  const { t } = useTranslation();
  return (
    <section id="sekolah" data-reveal className="mx-auto w-full max-w-public px-6 pt-[60px] md:pt-[88px]">
      <div className="grid gap-8 rounded-[var(--radius-panel)] bg-brand-50 p-8 md:grid-cols-2 md:p-12">
        <div className="flex flex-col gap-4">
          <span className="type-overline text-brand-ink">{t("land_school_eyebrow")}</span>
          <h2 className="type-display text-ink-900">{t("land_school_title")}</h2>
          <p className="type-body text-ink-700">{t("land_school_text")}</p>
          <div className="mt-2 flex flex-wrap gap-3">
            <Button asChild size="lg">
              <a href={waHref(t("land_school_wa"))} target="_blank" rel="noopener noreferrer">
                {t("land_school_cta")}
              </a>
            </Button>
            <Button asChild size="lg" variant="outline">
              <Link href="/event">{t("land_school_schedule")}</Link>
            </Button>
          </div>
        </div>
        <div data-reveal-grid className="grid grid-cols-2 gap-4">
          {TILES.map((tile) => (
            <div key={tile.key} className="flex flex-col justify-between gap-6 rounded-[var(--radius-card)] bg-surface p-5 shadow-card">
              <tile.icon className="size-7 text-brand-ink" aria-hidden="true" />
              <span className="type-label text-ink-900">{t(tile.key)}</span>
            </div>
          ))}
        </div>
      </div>
    </section>
  );
}
