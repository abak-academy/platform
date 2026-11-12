"use client";

import Link from "next/link";
import { AFFILIATE_TERMS_VERSION } from "@/lib/affiliate-terms";
import { useTranslation, type I18nKey } from "@/lib/i18n";

// The terms cover both programs: clauses 1-2 apply to everyone, 3-6 to
// Affiliators and 7-11 to Mitra.
const SECTIONS: { heading: I18nKey; start: number; clauses: I18nKey[] }[] = [
  { heading: "aff_terms_h_general", start: 1, clauses: ["aff_terms_1", "aff_terms_2"] },
  { heading: "aff_terms_h_affiliate", start: 3, clauses: ["aff_terms_3", "aff_terms_4", "aff_terms_5", "aff_terms_6"] },
  { heading: "aff_terms_h_partner", start: 7, clauses: ["aff_terms_7", "aff_terms_8", "aff_terms_9", "aff_terms_10", "aff_terms_11"] },
];

export default function AffiliateTermsPage() {
  const { t } = useTranslation();
  return (
    <main className="mx-auto w-full max-w-2xl px-6 py-12">
      <h1 className="font-serif text-3xl font-bold text-ink-900">{t("aff_terms_title")}</h1>
      <p className="mt-1 text-sm text-ink-500">{t("aff_terms_version").replace("{version}", AFFILIATE_TERMS_VERSION)}</p>
      {SECTIONS.map((section) => (
        <section key={section.heading} className="mt-6">
          <h2 className="font-serif text-lg font-bold text-ink-900">{t(section.heading)}</h2>
          <ol start={section.start} className="mt-2 list-decimal space-y-3 pl-5 text-[15px] leading-relaxed text-ink-700">
            {section.clauses.map((key) => (
              <li key={key}>{t(key)}</li>
            ))}
          </ol>
        </section>
      ))}
      <Link href="/affiliate-register" className="mt-8 inline-block font-semibold text-success underline">
        {t("aff_terms_back")}
      </Link>
    </main>
  );
}
