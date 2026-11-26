"use client";

import Link from "next/link";
import { RotatingWord } from "@/components/landing/RotatingWord";
import { MathFloaters } from "@/components/brand/MathFloaters";
import { Button } from "@/components/ui/button";
import { useTranslation } from "@/lib/i18n";
import { SITE_CONFIG, waHref } from "@/lib/site-config";

export function LandingHero({ eventNames }: { eventNames: string[] }) {
  const { t } = useTranslation();
  return (
    <section data-hero data-swarm-light className="relative isolate flex min-h-[100svh] flex-col justify-end overflow-hidden bg-paper">
      <div data-hero-photo className="absolute inset-0 -z-20">
        <img
          src="/landing/hero-award.webp"
          alt=""
          loading="eager"
          fetchPriority="high"
          decoding="async"
          className="h-full w-full object-cover object-bottom"
        />
      </div>
      <div aria-hidden="true" className="absolute inset-0 -z-10 bg-linear-to-b from-paper/70 via-paper/60 to-paper" />
      <MathFloaters />
      <div data-hero-content className="mx-auto flex w-full max-w-public flex-col gap-6 px-6 pt-32 pb-12">
        <h1 className="type-display-hero max-w-[16ch] text-ink-900">
          {t("land_hero_prefix")} <RotatingWord words={eventNames} fallback={t("land_hero_word_fallback")} />
        </h1>
        <p className="type-body max-w-[56ch] text-ink-700">{t("land_hero_lede")}</p>
        <div className="flex flex-wrap gap-3">
          <Button asChild size="lg">
            <Link href="/event">{t("land_hero_cta_events")}</Link>
          </Button>
          <Button asChild size="lg" variant="outline">
            <a href={waHref(t("land_hero_consult_wa"))} target="_blank" rel="noopener noreferrer">
              {t("land_hero_cta_consult")}
            </a>
          </Button>
        </div>
        <dl className="mt-8 flex flex-wrap gap-x-10 gap-y-4">
          {SITE_CONFIG.stats.map((s) => (
            <div key={s.labelKey} className="flex flex-col">
              <dd className="type-h1 order-1 text-ink-900">{s.value}</dd>
              <dt className="type-body-sm order-2 text-ink-600">{t(s.labelKey)}</dt>
            </div>
          ))}
        </dl>
      </div>
    </section>
  );
}
