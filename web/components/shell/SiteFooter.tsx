"use client";

import Link from "next/link";
import { AbakWordmark } from "./AbakWordmark";
import { useTranslation } from "@/lib/i18n";
import { SITE_CONFIG, waHref } from "@/lib/site-config";
import type { I18nKey } from "@/lib/i18n";

const COLUMNS: { title: I18nKey; links: { key: I18nKey; href: string }[] }[] = [
  {
    title: "pub_foot_col_explore",
    links: [
      { key: "pub_foot_link_events", href: "/event" },
      { key: "pub_foot_link_programs", href: "/program" },
      { key: "pub_foot_link_shop", href: "/catalog" },
    ],
  },
  {
    title: "pub_foot_col_account",
    links: [
      { key: "pub_foot_link_login", href: "/login" },
      { key: "pub_foot_link_signup", href: "/register" },
      { key: "pub_foot_link_affiliate", href: "/affiliate-register" },
    ],
  },
  { title: "pub_foot_col_team", links: [{ key: "pub_foot_link_admin", href: "/admin/login" }] },
];

export function SiteFooter() {
  const { t } = useTranslation();
  const social = SITE_CONFIG.social.filter((s) => s.url);
  const linkCls = "text-white/75 hover:text-white";

  return (
    <footer data-swarm-light className="bg-brand-900 text-white">
      <div className="mx-auto max-w-[var(--container-public)] px-4 md:px-6">
        <div data-footer-divider className="border-t border-white/15" />
      </div>
      <div className="mx-auto grid max-w-[var(--container-public)] gap-10 px-4 py-14 md:grid-cols-[1.6fr_1fr_1fr_1fr] md:px-6">
        <div className="space-y-4">
          <AbakWordmark light size={40} />
          <p className="max-w-sm text-white/75">{t("pub_foot_blurb")}</p>
          <p className="text-sm text-white/75">
            <span className="block font-semibold text-white">{t("pub_foot_address_label")}</span>
            {SITE_CONFIG.address}
          </p>
          <p className="text-sm text-white/75">
            <span className="block font-semibold text-white">{t("pub_foot_whatsapp_label")}</span>
            <a href={waHref(t("pub_foot_whatsapp_msg"))} target="_blank" rel="noreferrer" className={linkCls}>
              {SITE_CONFIG.whatsappDisplay}
            </a>
          </p>
          {social.length > 0 && (
            <div>
              <h2 className="sr-only">{t("pub_foot_social_label")}</h2>
              <ul className="flex gap-4 text-sm">
                {social.map((s) => (
                  <li key={s.key}>
                    <a href={s.url!} target="_blank" rel="noreferrer" className={linkCls}>
                      {s.label}
                    </a>
                  </li>
                ))}
              </ul>
            </div>
          )}
        </div>
        {COLUMNS.map((c) => (
          <nav key={c.title} aria-labelledby={c.title}>
            <h2 id={c.title} className="mb-3 text-sm font-semibold">
              {t(c.title)}
            </h2>
            <ul className="space-y-2 text-sm">
              {c.links.map((l) => (
                <li key={l.href}>
                  <Link href={l.href} className={linkCls}>
                    {t(l.key)}
                  </Link>
                </li>
              ))}
            </ul>
          </nav>
        ))}
      </div>
      <div className="border-t border-white/15 px-4 py-6 text-center text-sm text-white/60">
        <span className="font-semibold text-white">abak academy</span> · {t("pub_foot_copyright")}
      </div>
    </footer>
  );
}
