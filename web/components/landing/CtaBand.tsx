"use client";

import Link from "next/link";
import { MathFloaters } from "@/components/brand/MathFloaters";
import { Button } from "@/components/ui/button";
import { useTranslation } from "@/lib/i18n";

export function CtaBand() {
  const { t } = useTranslation();
  return (
    <section data-reveal data-swarm-light className="relative isolate overflow-hidden bg-deep text-on-deep">
      <MathFloaters light />
      <div className="mx-auto flex w-full max-w-public flex-col items-center gap-5 px-6 py-24 text-center">
        <span className="type-overline opacity-80">{t("land_cta_eyebrow")}</span>
        <h2 className="type-display max-w-[20ch]">{t("land_cta_title")}</h2>
        <p className="type-body max-w-[48ch] opacity-85">{t("land_cta_text")}</p>
        <div className="mt-2 flex flex-wrap justify-center gap-3">
          <Button asChild size="lg" variant="light">
            <Link href="/register">{t("land_cta_signup")}</Link>
          </Button>
          <Button asChild size="lg" variant="line-light">
            <Link href="/event">{t("land_cta_events")}</Link>
          </Button>
        </div>
      </div>
    </section>
  );
}
