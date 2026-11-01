"use client";

import { useEffect, useState } from "react";
import { usePathname } from "next/navigation";
import { AppSidebar } from "./AppSidebar";
import { AppHeader } from "./AppHeader";
import { cn } from "@/lib/utils";
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
  children: React.ReactNode;
}

export function AppShell({ role, children }: AppShellProps) {
  const pathname = usePathname();
  const [expanded, setExpanded] = useState(false);
  const isLg = useIsLg();
  const collapsed = !isLg && !expanded;
  const isAdmin = ADMIN_ROLES.includes(role);
  // The affiliate portal shares the admin dashboard's look (cards, tables, filters).
  const adminLook = isAdmin || role === "affiliator";

  useEffect(() => {
    if (isLg) setExpanded(false);
  }, [isLg]);

  function toggleSidebar() {
    setExpanded((prev) => !prev);
  }

  return (
    <div className={cn("flex min-h-screen bg-paper", adminLook && "admin-shell")}>
      <aside
        className={cn(
          "fixed left-0 top-0 z-30 flex h-screen flex-col border-r border-line bg-surface transition-all duration-200 ease-out",
          collapsed ? "w-16" : "w-[252px]"
        )}
      >
        <AppSidebar role={role} collapsed={collapsed} />
      </aside>

      <div
        className={cn(
          "flex min-w-0 flex-1 flex-col transition-[padding] duration-200 ease-out",
          collapsed ? "pl-16" : "pl-[252px]"
        )}
      >
        {!isAdmin && role !== "affiliator" && <CartCountSync />}
        <AppHeader onMenuClick={toggleSidebar} />
        <main className="flex-1">
          <div
            key={pathname}
            className="fade-in mx-auto w-full max-w-[1340px] px-7 py-7 lg:px-8 lg:py-8"
          >
            {children}
          </div>
        </main>
      </div>
    </div>
  );
}
