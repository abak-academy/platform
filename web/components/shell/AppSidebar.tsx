"use client";

import { useMemo, useState } from "react";
import Link from "next/link";
import { usePathname, useRouter } from "next/navigation";
import { ChevronDown, LogOut } from "lucide-react";
import { useAuthStore } from "@/stores/auth";
import { useLogout } from "@/lib/hooks/auth";
import { fileUrl } from "@/lib/api";
import { Button } from "@/components/ui/button";
import { Avatar, AvatarImage, AvatarFallback } from "@/components/ui/avatar";
import { Badge } from "@/components/ui/badge";
import { Separator } from "@/components/ui/separator";
import { AbakMark } from "./AbakMark";
import {
  NAV_CONFIG,
  ADMIN_ROLES,
  roleLabelKey,
  type UserRole,
  type NavItem,
} from "@/lib/nav-config";
import { useTranslation, type DICT } from "@/lib/i18n";
import { loginPathForRole } from "@/lib/auth-redirect";
import { cn } from "@/lib/utils";

interface AppSidebarProps {
  role: UserRole;
  collapsed?: boolean;
}

function isNavActive(pathname: string, item: NavItem): boolean {
  if (item.exact) return pathname === item.href;
  return pathname === item.href || pathname.startsWith(item.href + "/");
}

function NavItemLink({ item, active }: { item: NavItem; active: boolean }) {
  const { t } = useTranslation();
  return (
    <Link
      href={item.href}
      title={t(item.labelKey)}
      className={cn(
        "flex items-center gap-3 rounded-lg px-3 py-2 text-sm transition-colors",
        active
          ? "bg-brand-50 font-semibold text-brand-700"
          : "font-medium text-ink-600 hover:bg-brand-50/60 hover:text-brand-700"
      )}
    >
      <item.icon
        className={cn(
          "size-[18px] shrink-0",
          active ? "text-brand-600" : "text-ink-400"
        )}
      />
      <span className="truncate">{t(item.labelKey)}</span>
      {item.comingSoon && (
        <Badge
          variant="outline"
          className="ml-auto px-1.5 py-0 text-[10px] font-normal text-ink-400"
        >
          Akan datang
        </Badge>
      )}
    </Link>
  );
}

function RailItemLink({ item, active }: { item: NavItem; active: boolean }) {
  return (
    <Link
      href={item.href}
      title={item.labelKey}
      className={cn(
        "flex size-10 items-center justify-center rounded-lg transition-colors",
        active
          ? "bg-brand-50 text-brand-600"
          : "text-ink-400 hover:bg-brand-50/60 hover:text-brand-700"
      )}
    >
      <item.icon className="size-[18px]" />
    </Link>
  );
}

export function AppSidebar({ role, collapsed = false }: AppSidebarProps) {
  const pathname = usePathname();
  const router = useRouter();
  const user = useAuthStore((s) => s.user);
  const logout = useLogout();
  const { t } = useTranslation();
  const groups = useMemo(() => NAV_CONFIG[role] ?? [], [role]);
  const [open, setOpen] = useState(() => groups.map(() => true));

  const initial = useMemo(() => {
    const source = user?.name ?? user?.email ?? user?.username ?? "A";
    return source.trim().charAt(0).toUpperCase();
  }, [user]);

  const roleKey = roleLabelKey(role ?? user?.role);
  const roleLabel = roleKey ? t(roleKey) : t("account");

  function handleLogout() {
    // Without an explicit destination this left the route guard to bounce the
    // tab, and that guard sent admins to the student login.
    const loginPath = loginPathForRole(role);
    logout.mutate(undefined, {
      onSettled: () => {
        router.replace(loginPath);
      },
    });
  }

  function toggleGroup(index: number) {
    setOpen((prev) => {
      const next = [...prev];
      next[index] = !next[index];
      return next;
    });
  }

  const flatItems = useMemo(() => groups.flatMap((g) => g.items), [groups]);

  return (
    <>
      <div className="flex h-[72px] shrink-0 items-center px-4 lg:px-5">
        <Link href="/" className="flex items-center gap-3">
          <div className="flex size-10 shrink-0 items-center justify-center rounded-[11px] bg-brand-50">
            <AbakMark size={28} />
          </div>
          <div className="hidden flex-col justify-center lg:flex">
            <div className="flex items-baseline gap-1.5 leading-none">
              <span className="font-serif text-[18px] font-extrabold tracking-[-0.01em] text-ink-900">
                abak
              </span>
              <span className="text-[9.5px] font-semibold uppercase tracking-[0.22em] text-[#C6881F]">
                academy
              </span>
            </div>
            <span className="mt-[3px] text-[11px] font-medium text-ink-400">
              {ADMIN_ROLES.includes(role) ? t("admin_panel") : t("app_tag")}
            </span>
          </div>
        </Link>
      </div>

      <Separator className="shrink-0" />

      {collapsed ? (
        <nav className="flex flex-1 flex-col items-center gap-2 overflow-y-auto px-2 py-3">
          {flatItems.map((item) => (
            <RailItemLink
              key={item.href + item.labelKey}
              item={item}
              active={isNavActive(pathname, item)}
            />
          ))}
        </nav>
      ) : (
        <nav className="flex flex-1 flex-col gap-1 overflow-y-auto px-3 py-3">
          {groups.map((group, index) => (
            <div key={group.titleKey || `group-${index}`}>
              {group.titleKey && (
                <button
                  type="button"
                  onClick={() => toggleGroup(index)}
                  className="flex w-full items-center justify-between px-3 py-2 text-left"
                >
                  <span className="text-[10px] font-bold uppercase tracking-wider text-ink-400">
                    {t(group.titleKey)}
                  </span>
                  <ChevronDown
                    className={cn(
                      "size-4 text-ink-400 transition-transform",
                      open[index] && "rotate-180"
                    )}
                  />
                </button>
              )}
              <div
                className={cn(
                  "flex flex-col gap-1",
                  group.titleKey && !open[index] && "hidden"
                )}
              >
                {group.items.map((item) => (
                  <NavItemLink
                    key={item.href + item.labelKey}
                    item={item}
                    active={isNavActive(pathname, item)}
                  />
                ))}
              </div>
            </div>
          ))}
        </nav>
      )}

      <div className="mt-auto border-t border-line">
        {collapsed ? (
          <div className="flex flex-col items-center gap-2 p-2">
            <Avatar className="size-9">
              <AvatarImage src={fileUrl(user?.photo_url)} />
              <AvatarFallback className="bg-brand-600 text-[13px] font-semibold text-white">
                {initial}
              </AvatarFallback>
            </Avatar>
            <Button
              variant="ghost"
              size="icon-xs"
              onClick={handleLogout}
              aria-label={t("logout")}
            >
              <LogOut className="size-4 text-ink-500" />
            </Button>
          </div>
        ) : (
          <div className="flex items-center gap-3 p-3">
            <Avatar className="size-9">
              <AvatarImage src={fileUrl(user?.photo_url)} />
              <AvatarFallback className="bg-brand-600 text-[13px] font-semibold text-white">
                {initial}
              </AvatarFallback>
            </Avatar>
            <div className="min-w-0 flex-1">
              <p className="truncate text-[13px] font-semibold text-ink-900">
                {user?.name || user?.email || user?.username || t("account")}
              </p>
              <p className="truncate text-[11px] font-medium text-ink-400">
                {roleLabel}
              </p>
            </div>
            <Button
              variant="ghost"
              size="icon-xs"
              onClick={handleLogout}
              aria-label={t("logout")}
            >
              <LogOut className="size-4 text-ink-500" />
            </Button>
          </div>
        )}
      </div>
    </>
  );
}
