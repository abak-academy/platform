"use client";

import { useTranslation } from "@/lib/i18n";

export function AboutLines() {
  const { t } = useTranslation();
  return (
    <section id="tentang" data-reveal aria-label={t("land_about_label")} className="mx-auto w-full max-w-public px-6 py-24 text-center">
      <p className="type-display text-ink-900">
        <span className="block min-[900px]:whitespace-nowrap">{t("land_about_1")}</span>
        <span className="block">{t("land_about_2")}</span>
        <span className="block">{t("land_about_3")}</span>
      </p>
      <p className="type-overline mt-8 text-ink-500">{t("land_about_sig")}</p>
    </section>
  );
}
