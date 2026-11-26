"use client";

import { ChevronDown, MessageCircle } from "lucide-react";
import { Button } from "@/components/ui/button";
import { useTranslation } from "@/lib/i18n";
import { waHref } from "@/lib/site-config";

const QA = [
  ["land_faq_1_q", "land_faq_1_a"],
  ["land_faq_2_q", "land_faq_2_a"],
  ["land_faq_3_q", "land_faq_3_a"],
  ["land_faq_4_q", "land_faq_4_a"],
] as const;

export function Faq() {
  const { t } = useTranslation();
  return (
    <section data-reveal className="mx-auto w-full max-w-public-narrow px-6 py-20">
      <div className="mb-8">
        <span className="type-overline text-brand-ink">{t("land_faq_eyebrow")}</span>
        <h2 className="type-display mt-2 text-ink-900">{t("land_faq_title")}</h2>
      </div>
      <div className="divide-y divide-line rounded-[var(--radius-card)] border border-line bg-surface">
        {QA.map(([q, a]) => (
          <details key={q} className="group px-6 py-4">
            <summary className="type-h3 flex cursor-pointer list-none items-center justify-between gap-4 text-ink-900 [&::-webkit-details-marker]:hidden">
              {t(q)}
              <ChevronDown className="size-5 shrink-0 transition-transform group-open:rotate-180" aria-hidden="true" />
            </summary>
            <p className="type-body mt-3 text-ink-600">{t(a)}</p>
          </details>
        ))}
      </div>
      <div className="mt-6 flex flex-wrap items-center gap-4 rounded-[var(--radius-card)] bg-brand-50 p-6">
        <MessageCircle className="size-8 text-brand-ink" aria-hidden="true" />
        <div className="min-w-[200px] flex-1">
          <b className="type-h3 block text-ink-900">{t("land_faq_ask_title")}</b>
          <p className="type-body-sm text-ink-600">{t("land_faq_ask_text")}</p>
        </div>
        <Button asChild>
          <a href={waHref(t("land_faq_ask_wa"))} target="_blank" rel="noopener noreferrer">
            {t("land_faq_ask_cta")}
          </a>
        </Button>
      </div>
    </section>
  );
}
