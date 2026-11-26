"use client";

import { useEffect, useRef, useState } from "react";
import Link from "next/link";
import { usePathname } from "next/navigation";
import { ArrowUpRight, Menu, X } from "lucide-react";
import { AbakWordmark } from "./AbakWordmark";
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
import { AbakAvatar } from "@/components/ui/abak-avatar";

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
        <AbakAvatar name={name ?? "A"} size={34} ring={false} />
        {t("pub_nav_dashboard")}
        {role === "student" && <StudentCount />}
      </Link>
    );
  }

  return (
    <div
      className={cn(
        "inline-flex h-11 items-center rounded-[var(--radius-pill)] border p-1",
        light ? "border-white/30 bg-white/10" : "border-line bg-paper/70",
      )}
    >
      <Link
        href="/login"
        className={cn(
          "type-label inline-flex h-full items-center rounded-[var(--radius-pill)] px-3.5 transition-colors",
          light ? "text-white hover:bg-white/15" : "text-ink-700 hover:bg-surface-sunken hover:text-ink-900",
        )}
      >
        {t("pub_nav_login")}
      </Link>
      <Link
        href="/register"
        className="type-label group inline-flex h-full items-center gap-2 rounded-[var(--radius-pill)] bg-ember-600 pl-3.5 pr-1 text-on-ember shadow-[var(--shadow-cta)] transition-colors hover:bg-ember-500"
      >
        {t("pub_nav_signup")}
        <span
          aria-hidden
          className="grid size-7 place-items-center rounded-full bg-white/20 transition-transform duration-300 group-hover:rotate-45"
        >
          <ArrowUpRight className="size-4" />
        </span>
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
          "flex w-full items-center gap-5 rounded-[var(--radius-pill)] border transition-[max-width,padding,background-color,box-shadow] duration-500",
          top
            ? "max-w-[var(--container-public)] border-transparent bg-transparent px-0 py-3 text-white"
            : "max-w-[1040px] border-line bg-paper/80 py-1.5 pl-[18px] pr-1.5 text-ink-900 shadow-[var(--shadow-pop)] backdrop-blur-xl backdrop-saturate-150",
        )}
      >
        <Link href="/" aria-label={t("pub_nav_home")} className="shrink-0">
          <AbakWordmark light={top} hideTextOnPhone />
        </Link>

        <ul className="mx-auto hidden items-center gap-0.5 md:flex">
          {LINKS.map((l) => {
            const active = isActive(pathname, l.match);
            return (
              <li key={l.href}>
                <Link
                  href={l.href}
                  aria-current={active ? "page" : undefined}
                  className={cn(
                    "type-label group relative inline-flex items-center rounded-[var(--radius-pill)] px-3.5 py-2",
                    active ? (top ? "text-white" : "text-brand-ink") : top ? "text-white/85 hover:text-white" : "text-ink-700 hover:text-ink-900",
                  )}
                >
                  {t(l.key)}
                  <span
                    aria-hidden
                    className={cn(
                      "absolute bottom-0.5 left-1/2 size-[5px] -translate-x-1/2 rounded-full transition-transform duration-300",
                      active ? "scale-100 bg-ember-600" : "scale-0 bg-line-strong group-hover:scale-100",
                    )}
                  />
                </Link>
              </li>
            );
          })}
        </ul>

        <div className="hidden shrink-0 items-center gap-2 md:flex">
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
