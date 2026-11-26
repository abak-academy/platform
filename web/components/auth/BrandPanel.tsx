"use client";

import { AbakWordmark } from "@/components/shell/AbakWordmark";
import { SITE_CONFIG } from "@/lib/site-config";
import { useTranslation } from "@/lib/i18n";

export function BrandPanel() {
  const { t } = useTranslation();
  return (
    <aside className="relative m-3 flex min-h-[260px] flex-col overflow-hidden rounded-[32px] bg-deep p-7 text-white max-[899px]:m-2 max-[679px]:min-h-[200px]">
      <img
        src="/landing/hero-award.webp"
        alt=""
        className="absolute inset-0 h-full w-full object-cover opacity-50"
      />
      <div
        aria-hidden
        className="absolute inset-0 bg-[linear-gradient(180deg,rgba(26,33,104,.25),rgba(21,24,58,.94)_70%)]"
      />
      <div className="relative z-[1]">
        <AbakWordmark light />
      </div>
      <div className="relative z-[1] mt-auto flex flex-col gap-2.5">
        <span className="text-[11.5px] font-bold uppercase tracking-[0.06em] text-[#f59a6b]">
          {t("auth_brand_eyebrow")}
        </span>
        <h2 className="font-[family-name:var(--font-display)] text-[38px] font-extrabold leading-[1.05] tracking-[-0.04em] text-white max-[679px]:text-[28px]">
          {t("auth_brand_title")}
        </h2>
        <p className="max-w-[420px] text-white/80 max-[679px]:hidden">{t("auth_brand_text")}</p>
        <p className="mt-1.5 font-semibold text-white max-[679px]:hidden">
          {SITE_CONFIG.stats[0].value} {t("auth_brand_proof")}
        </p>
      </div>
    </aside>
  );
}
