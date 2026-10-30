"use client";

import Link from "next/link";
import { AFFILIATE_TERMS_VERSION } from "@/lib/affiliate-terms";
import { useTranslation, type I18nKey } from "@/lib/i18n";

const CLAUSES: I18nKey[] = [
  "aff_terms_1",
  "aff_terms_2",
  "aff_terms_3",
  "aff_terms_4",
  "aff_terms_5",
  "aff_terms_6",
  "aff_terms_7",
  "aff_terms_8",
];

export default function AffiliateTermsPage() {
  const { t } = useTranslation();
  return (
    <main className="mx-auto w-full max-w-2xl px-6 py-12">
      <h1 className="font-serif text-3xl font-bold text-ink-900">{t("aff_terms_title")}</h1>
      <p className="mt-1 text-sm text-ink-500">{t("aff_terms_version").replace("{version}", AFFILIATE_TERMS_VERSION)}</p>
      <ol className="mt-6 list-decimal space-y-3 pl-5 text-[15px] leading-relaxed text-ink-700">
        {CLAUSES.map((key) => (
          <li key={key}>{t(key)}</li>
        ))}
      </ol>
      <Link href="/affiliate-register" className="mt-8 inline-block font-semibold text-success underline">
        {t("aff_terms_back")}
      </Link>
    </main>
  );
}
