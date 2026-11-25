"use client";

import { useEffect, useRef, useState } from "react";
import { usePathname } from "next/navigation";
import { X } from "lucide-react";
import { AppSidebar } from "./AppSidebar";
import { AppHeader } from "./AppHeader";
import { Button } from "@/components/ui/button";
import { cn } from "@/lib/utils";
import { useTranslation } from "@/lib/i18n";
import { ADMIN_ROLES, type UserRole } from "@/lib/nav-config";
import { useCart } from "@/lib/hooks/orders";
import { useCartStore } from "@/stores/cart";

function CartCountSync() {
  const { data: cart } = useCart();
  const setCount = useCartStore((s) => s.setCount);
  useEffect(() => {
    setCount(cart?.items?.length ?? 0);
  }, [cart?.items?.length, setCount]);
  return null;
}

function useIsLg() {
  const [lg, setLg] = useState(false);

  useEffect(() => {
    const mq = window.matchMedia("(min-width: 1024px)");
    function handler(event: MediaQueryListEvent | MediaQueryList) {
      setLg(event.matches);
    }
    handler(mq);
    mq.addEventListener("change", handler);
    return () => mq.removeEventListener("change", handler);
  }, []);

  return lg;
}

interface AppShellProps {
  role: UserRole;
  counts?: Record<string, number>;
  children: React.ReactNode;
}

export function AppShell({ role, counts, children }: AppShellProps) {
  const pathname = usePathname();
  const { t } = useTranslation();
  const [open, setOpen] = useState(false);
  const isLg = useIsLg();
  const menuButtonRef = useRef<HTMLButtonElement>(null);
  const drawerRef = useRef<HTMLElement>(null);
  const wasOpen = useRef(false);
  const isAdmin = ADMIN_ROLES.includes(role);
  // The affiliate portal shares the admin dashboard's look (cards, tables, filters).
  const adminLook = isAdmin || role === "affiliator";

  useEffect(() => {
    setOpen(false);
  }, [pathname, isLg]);

  useEffect(() => {
    if (open) drawerRef.current?.focus();
    else if (wasOpen.current) menuButtonRef.current?.focus();
    wasOpen.current = open;
  }, [open]);

  useEffect(() => {
    if (!open) return;
    function onKeyDown(e: KeyboardEvent) {
      if (e.key === "Escape") setOpen(false);
    }
    document.addEventListener("keydown", onKeyDown);
    return () => document.removeEventListener("keydown", onKeyDown);
  }, [open]);

  return (
    <div className={cn("flex min-h-screen bg-paper", adminLook && "admin-shell")}>
      <aside className="fixed left-0 top-0 z-30 hidden h-screen w-sidebar flex-col border-r border-line bg-surface lg:flex">
        <AppSidebar role={role} counts={counts} />
      </aside>

      {open && (
        <>
          <div
            data-testid="drawer-scrim"
            className="fixed inset-0 z-(--z-drawer) bg-scrim backdrop-blur-[6px]"
            onClick={() => setOpen(false)}
          />
          <aside
            ref={drawerRef}
            role="dialog"
            aria-modal="true"
            aria-label={t("nav_menu")}
            tabIndex={-1}
            className="fixed left-0 top-0 z-(--z-drawer) flex h-screen w-sidebar flex-col bg-surface shadow-pop outline-none"
          >
            <Button
              variant="ghost"
              size="icon-sm"
              onClick={() => setOpen(false)}
              aria-label={t("nav_close_menu")}
              className="absolute right-3 top-4"
            >
              <X className="size-5 text-ink-600" />
            </Button>
            <AppSidebar role={role} counts={counts} />
          </aside>
        </>
      )}

      <div className="flex min-w-0 flex-1 flex-col lg:pl-sidebar">
        {!isAdmin && role !== "affiliator" && <CartCountSync />}
        <AppHeader role={role} onMenuClick={() => setOpen(true)} menuButtonRef={menuButtonRef} />
        <main className="flex-1">
          <div
            key={pathname}
            className="fade-in mx-auto w-full max-w-page px-4 py-7 narrow:px-8 lg:py-8"
          >
            {children}
          </div>
        </main>
      </div>
    </div>
  );
}
