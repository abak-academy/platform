"use client";

import Link from "next/link";
import { ArrowRight } from "lucide-react";
import { Button } from "@/components/ui/button";
import { useTranslation } from "@/lib/i18n";
import { PROGRAMS } from "@/lib/programs";
import { waHref } from "@/lib/site-config";
import type { EventTone } from "@/lib/types";

const TONE: Record<EventTone, string> = {
  brand: "bg-brand-100 text-brand-ink",
  ember: "bg-ember-50 text-ember-700",
  teal: "bg-teal-50 text-teal-700",
  gold: "bg-gold-50 text-gold-700",
  sky: "bg-sky-50 text-sky-700",
  berry: "bg-berry-50 text-berry-700",
  navy: "bg-brand-100 text-brand-900",
};

export function ProgramGrid() {
  const { t } = useTranslation();
  return (
    <section data-reveal className="mx-auto w-full max-w-public px-6 py-20">
      <div className="mb-10 flex flex-wrap items-end justify-between gap-6">
        <div className="max-w-[60ch]">
          <span className="type-overline text-brand-ink">{t("land_prog_eyebrow")}</span>
          <h2 className="type-display mt-2 text-ink-900">{t("land_prog_title")}</h2>
          <p className="type-body mt-3 text-ink-600">{t("land_prog_lede")}</p>
        </div>
        <Button asChild variant="outline">
          <a href={waHref(t("land_prog_ask_wa"))} target="_blank" rel="noopener noreferrer">
            {t("land_prog_ask")}
          </a>
        </Button>
      </div>
      <div data-reveal-grid className="grid gap-5 sm:grid-cols-2 lg:grid-cols-3">
        {PROGRAMS.map((p) => (
          <article
            key={p.id}
            data-tilt
            className="flex flex-col gap-3 rounded-[var(--radius-card)] border border-line bg-surface p-6 shadow-card"
          >
            <span className={`flex size-11 items-center justify-center rounded-[var(--r-md)] ${TONE[p.tone]}`}>
              <p.icon className="size-5" aria-hidden="true" />
            </span>
            <h3 className="type-h2 text-ink-900">{t(p.titleKey)}</h3>
            <p className="type-body flex-1 text-ink-600">{t(p.shortKey)}</p>
            <Link href={`/program#${p.id}`} className="type-label inline-flex items-center gap-1.5 text-brand-ink hover:underline">
              {t("land_prog_more")}
              <span className="sr-only"> {t(p.titleKey)}</span>
              <ArrowRight className="size-4" aria-hidden="true" />
            </Link>
          </article>
        ))}
      </div>
    </section>
  );
}
