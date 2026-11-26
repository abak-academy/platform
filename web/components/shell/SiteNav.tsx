"use client";

import { useEffect, useRef, useState } from "react";
import Link from "next/link";
import { usePathname } from "next/navigation";
import { Menu, X } from "lucide-react";
import { AbakLogo } from "@/components/brand/AbakLogo";
import { Avatar, AvatarFallback } from "@/components/ui/avatar";
import { useTranslation, type I18nKey } from "@/lib/i18n";
import { useAuthStore } from "@/stores/auth";
import { useResolvedRole } from "@/lib/hooks/use-capability";
import { useQuery } from "@tanstack/react-query";
import { authFetch } from "@/lib/api";
import { ordersKeys } from "@/lib/hooks/orders";
import type { Order, RegistrationListItem } from "@/lib/types";
import { kompetisiCount, pendingCount } from "@/lib/student-home";
import { redirectForRole } from "@/lib/auth-redirect";
import { cn } from "@/lib/utils";
import { LanguageToggle } from "./LanguageToggle";

const LINKS: { key: I18nKey; href: string; match: string[] }[] = [
  { key: "pub_nav_event", href: "/event", match: ["/event"] },
  { key: "pub_nav_program", href: "/program", match: ["/program"] },
  { key: "pub_nav_shop", href: "/catalog", match: ["/catalog"] },
];

const isActive = (pathname: string, match: string[]) =>
  match.some((m) => pathname === m || pathname.startsWith(`${m}/`));

function StudentCount() {
  const { t } = useTranslation();
  const orders = useQuery({
    queryKey: [...ordersKeys.list(), "site-nav"],
    queryFn: async () =>
      (await authFetch<{ data: Order[] }>("/orders", undefined, { redirectOn401: false })).data ?? [],
    retry: false,
  });
  const regs = useQuery({
    queryKey: ["site-nav", "registrations"],
    queryFn: async () =>
      (await authFetch<{ data: RegistrationListItem[] }>("/exam/registrations", undefined, { redirectOn401: false })).data ?? [],
    retry: false,
  });
  const n = pendingCount(orders.data ?? []) + kompetisiCount(regs.data ?? [], Date.now());
  if (n === 0) return null;
  return (
    <>
      <span className="sr-only">{t("pub_nav_attention").replace("{n}", String(n))}</span>
      <span aria-hidden className="rounded-full bg-ember-600 px-1.5 text-xs font-semibold text-on-ember">
        {n}
      </span>
    </>
  );
}

function AccountControl({ light }: { light: boolean }) {
  const { t } = useTranslation();
  const token = useAuthStore((s) => s.token);
  const name = useAuthStore((s) => s.user?.name);
  const { role } = useResolvedRole();

  if (token && role) {
    return (
      <Link
        href={redirectForRole(role)}
        className="inline-flex h-10 items-center gap-2 rounded-[var(--radius-pill)] border border-line bg-surface py-1 pl-1 pr-3 text-sm font-semibold text-ink-900"
      >
        <Avatar size="sm">
          <AvatarFallback>{(name ?? "A").slice(0, 1).toUpperCase()}</AvatarFallback>
        </Avatar>
        {t("pub_nav_dashboard")}
        {role === "student" && <StudentCount />}
      </Link>
    );
  }

  return (
    <div
      className={cn(
        "inline-flex h-10 items-center overflow-hidden rounded-[var(--radius-pill)] border text-sm font-semibold",
        light ? "border-white/40 text-white" : "border-line text-ink-900",
      )}
    >
      <Link href="/login" className="px-4 py-2.5 hover:underline">
        {t("pub_nav_login")}
      </Link>
      <Link href="/register" className="bg-ember-600 px-4 py-2.5 text-on-ember hover:bg-ember-700">
        {t("pub_nav_signup")}
      </Link>
    </div>
  );
}

export function SiteNav() {
  const { t } = useTranslation();
  const pathname = usePathname();
  const landing = pathname === "/";
  const [scrolled, setScrolled] = useState(false);
  const [hidden, setHidden] = useState(false);
  const [open, setOpen] = useState(false);
  const lastY = useRef(0);

  useEffect(() => {
    lastY.current = window.scrollY;
    const onScroll = () => {
      const y = window.scrollY;
      const limit = landing
        ? (document.querySelector<HTMLElement>("[data-hero]")?.offsetHeight ?? 600)
        : 120;
      setScrolled(y > 8);
      if (y <= limit) {
        setHidden(false);
        lastY.current = y;
      } else if (y > lastY.current) {
        setHidden(true);
        lastY.current = y;
      } else if (lastY.current - y >= 5) {
        setHidden(false);
        lastY.current = y;
      }
    };
    onScroll();
    window.addEventListener("scroll", onScroll, { passive: true });
    return () => window.removeEventListener("scroll", onScroll);
  }, [landing]);

  useEffect(() => setOpen(false), [pathname]);

  const top = landing && !scrolled;

  return (
    <header
      data-state={top ? "top" : "pill"}
      data-hidden={hidden ? "true" : "false"}
      onFocus={() => setHidden(false)}
      className={cn(
        "fixed inset-x-0 top-0 z-50 flex justify-center px-3 pt-3 transition-transform duration-300",
        hidden && "-translate-y-[150%]",
      )}
    >
      <nav
        aria-label={t("pub_nav_label")}
        className={cn(
          "flex w-full max-w-[var(--container-public)] items-center gap-3 rounded-[var(--radius-pill)] px-4 py-2 transition-colors",
          top ? "bg-transparent text-white" : "border border-line bg-surface/80 text-ink-900 backdrop-blur-md",
        )}
      >
        <Link href="/" aria-label={t("pub_nav_home")} className="flex items-center gap-2 font-semibold">
          <AbakLogo size={32} />
        </Link>

        <ul className="ml-4 hidden items-center gap-1 md:flex">
          {LINKS.map((l) => {
            const active = isActive(pathname, l.match);
            return (
              <li key={l.href}>
                <Link
                  href={l.href}
                  aria-current={active ? "page" : undefined}
                  className={cn(
                    "relative inline-flex items-center gap-1.5 rounded-[var(--radius-pill)] px-3 py-2 text-sm font-semibold",
                    active ? (top ? "text-white" : "text-brand-ink") : top ? "text-white/80 hover:text-white" : "text-ink-600 hover:text-ink-900",
                  )}
                >
                  {active && <span aria-hidden className="size-1.5 rounded-full bg-ember-600" />}
                  {t(l.key)}
                </Link>
              </li>
            );
          })}
        </ul>

        <div className="ml-auto hidden items-center gap-3 md:flex">
          <LanguageToggle onDark={top} className={top ? "border-white/40 text-white" : undefined} />
          <AccountControl light={top} />
        </div>

        <button
          type="button"
          className="ml-auto inline-flex size-10 items-center justify-center rounded-full md:hidden"
          aria-expanded={open}
          aria-controls="site-nav-panel"
          aria-label={t(open ? "pub_nav_menu_close" : "pub_nav_menu_open")}
          onClick={() => setOpen((o) => !o)}
        >
          {open ? <X className="size-5" /> : <Menu className="size-5" />}
        </button>
      </nav>

      {open && (
        <div
          id="site-nav-panel"
          className="absolute inset-x-3 top-[calc(100%-0.25rem)] space-y-4 rounded-2xl border border-line bg-surface p-4 text-ink-900 shadow-lg md:hidden"
        >
          <ul className="space-y-1">
            {LINKS.map((l) => (
              <li key={l.href}>
                <Link
                  href={l.href}
                  aria-current={isActive(pathname, l.match) ? "page" : undefined}
                  className="block rounded-lg px-3 py-2 font-semibold"
                >
                  {t(l.key)}
                </Link>
              </li>
            ))}
          </ul>
          <AccountControl light={false} />
          <div className="flex items-center justify-between text-sm">
            <span>{t("pub_nav_lang_row")}</span>
            <LanguageToggle />
          </div>
        </div>
      )}
    </header>
  );
}
