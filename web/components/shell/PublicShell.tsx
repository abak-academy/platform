"use client";

import { useLayoutEffect } from "react";
import { useUIStore } from "@/stores/ui";
import { PublicMotion } from "@/components/motion/PublicMotion";
import { useTranslation } from "@/lib/i18n";
import { SiteFooter } from "./SiteFooter";
import { SiteNav } from "./SiteNav";

export function PublicShell({ children, guestOnly }: { children: React.ReactNode; guestOnly?: boolean }) {
  const { t } = useTranslation();
  useLayoutEffect(() => {
    const { enterPublic, leavePublic } = useUIStore.getState();
    enterPublic();
    return leavePublic;
  }, []);
  return (
    <div data-public-shell data-guest-only={guestOnly ? "" : undefined} onContextMenu={(e) => e.preventDefault()}>
      <a
        href="#main"
        className="sr-only focus:not-sr-only focus:fixed focus:left-4 focus:top-4 focus:z-[60] focus:rounded-[var(--radius-pill)] focus:bg-surface focus:px-4 focus:py-2 focus:text-ink-900"
      >
        {t("pub_nav_skip")}
      </a>
      <SiteNav />
      <main id="main">{children}</main>
      <SiteFooter />
      <PublicMotion />
    </div>
  );
}
