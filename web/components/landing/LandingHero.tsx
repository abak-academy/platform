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
    <section data-hero data-swarm-light className="relative isolate flex min-h-[min(980px,100svh)] flex-col justify-end overflow-hidden bg-black">
      <link rel="preload" as="image" href="/landing/hero-award.webp" fetchPriority="high" />
      <div data-hero-photo className="absolute inset-x-0 bottom-0 -z-20 h-[58%] md:inset-0 md:h-auto">
        <img
          src="/landing/hero-award.webp"
          alt=""
          loading="eager"
          fetchPriority="high"
          decoding="async"
          className="h-full w-full object-cover object-bottom"
        />
      </div>
      <div aria-hidden="true" className="absolute inset-0 -z-10 bg-linear-to-b from-black/65 via-black/45 to-black/60" />
      <MathFloaters light />
      <div data-hero-content className="mx-auto flex w-full max-w-public flex-col gap-6 px-6 pt-32 pb-[110px] md:pb-[170px]">
        <h1 className="type-display-hero min-h-[3em] max-w-[16ch] text-white">
          {t("land_hero_prefix")}{" "}
          <span className="[&>span]:text-ember-500">
            <RotatingWord words={eventNames} fallback={t("land_hero_word_fallback")} />
          </span>
        </h1>
        <p className="type-body max-w-[56ch] text-white/85">{t("land_hero_lede")}</p>
        <div className="flex flex-wrap gap-3">
          <Button asChild size="lg">
            <Link href="/event">{t("land_hero_cta_events")}</Link>
          </Button>
          <Button asChild size="lg" variant="line-light">
            <a href={waHref(t("land_hero_consult_wa"))} target="_blank" rel="noopener noreferrer">
              {t("land_hero_cta_consult")}
            </a>
          </Button>
        </div>
        <dl className="mt-8 flex border-t border-white/20 pt-6 flex-wrap gap-x-10 gap-y-4">
          {SITE_CONFIG.stats.map((s) => (
            <div key={s.labelKey} className="flex flex-col">
              <dd className="type-h1 order-1 text-white">{s.value}</dd>
              <dt className="type-body-sm order-2 text-white/80">{t(s.labelKey)}</dt>
            </div>
          ))}
        </dl>
      </div>
      <div aria-hidden="true" data-hero-fade className="absolute inset-x-0 bottom-0 h-24 bg-linear-to-b from-transparent to-surface md:h-36" />
    </section>
  );
}
