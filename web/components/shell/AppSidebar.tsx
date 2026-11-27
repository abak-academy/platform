"use client";

import { useEffect, useId, useMemo, useState } from "react";
import Link from "next/link";
import { usePathname } from "next/navigation";
import { ChevronDown } from "lucide-react";
import { useMe } from "@/lib/hooks/auth";
import { Badge } from "@/components/ui/badge";
import { AbakWordmark } from "./AbakWordmark";
import { SidebarUserMenu } from "./SidebarUserMenu";
import {
  NAV_CONFIG,
  type UserRole,
  type NavItem,
  type NavGroup,
} from "@/lib/nav-config";
import { useTranslation } from "@/lib/i18n";
import { cn } from "@/lib/utils";

interface AppSidebarProps {
  role: UserRole;
  counts?: Record<string, number>;
  subtitle?: string;
}

function isNavActive(pathname: string, item: NavItem): boolean {
  if (item.exact) return pathname === item.href;
  return pathname === item.href || pathname.startsWith(item.href + "/");
}

function activeGroupKey(groups: NavGroup[], pathname: string): string | null {
  const g = groups.find((g) => g.titleKey && g.items.some((i) => isNavActive(pathname, i)));
  return g?.titleKey ?? null;
}

function useReducedMotion() {
  const [reduced, setReduced] = useState(false);
  useEffect(() => {
    if (typeof window.matchMedia !== "function") return;
    const mq = window.matchMedia("(prefers-reduced-motion: reduce)");
    const sync = () => setReduced(mq.matches);
    sync();
    mq.addEventListener?.("change", sync);
    return () => mq.removeEventListener?.("change", sync);
  }, []);
  return reduced;
}

function CountPill({ n, group }: { n?: number; group?: boolean }) {
  if (!n || n <= 0) return null;
  return (
    <span
      className={cn(
        "ml-auto rounded-full bg-ember-600 text-center font-extrabold text-on-ember",
        group ? "h-5 min-w-5 px-1.5 text-[10.5px] leading-5" : "h-[22px] min-w-[22px] px-[7px] text-[11px] leading-[22px]"
      )}
    >
      {n}
    </span>
  );
}

function IconTile({
  active,
  nested,
  group,
  animate,
  children,
}: {
  active: boolean;
  animate?: boolean;
  nested?: boolean;
  group?: "idle" | "has-active";
  children: React.ReactNode;
}) {
  return (
    <span
      className={cn(
        "flex shrink-0 items-center justify-center",
        animate && "transition-transform duration-300 ease-spring",
        nested ? "size-[26px] rounded-[9px] [&_svg]:size-[17px]" : "size-8 rounded-[11px]",
        active && "-rotate-6 bg-brand-600 text-on-brand shadow-[0_6px_14px_-6px_var(--color-brand-600)]",
        group === "idle" && "bg-surface-sunken text-ink-700",
        group === "has-active" && "bg-brand-50 text-brand-ink"
      )}
    >
      {children}
    </span>
  );
}

function NavItemLink({ item, active, count, nested, animate }: { item: NavItem; active: boolean; count?: number; nested?: boolean; animate: boolean }) {
  const { t } = useTranslation();
  return (
    <Link
      href={item.href}
      title={t(item.labelKey)}
      aria-current={active ? "page" : undefined}
      className={cn(
        "flex items-center gap-[11px] rounded-md px-2.5 type-label outline-none focus-visible:ring-2 focus-visible:ring-brand-600",
        animate && "transition-[background-color,transform] duration-300 ease-spring",
        nested ? "h-[38px] text-[13.5px]" : "h-11",
        active
          ? "bg-brand-50 text-brand-ink"
          : "text-ink-700 hover:translate-x-0.5 hover:bg-surface-sunken"
      )}
    >
      <IconTile active={active} nested={nested} animate={animate}>
        <item.icon className="size-5" />
      </IconTile>
      <span className="truncate">{t(item.labelKey)}</span>
      {item.comingSoon && (
        <Badge variant="outline" className="ml-auto px-1.5 py-0 type-caption text-ink-600">
          Akan datang
        </Badge>
      )}
      <CountPill n={count} />
    </Link>
  );
}

function GroupSection({
  group,
  open,
  onToggle,
  pathname,
  counts,
  animate,
}: {
  group: NavGroup;
  open: boolean;
  onToggle: () => void;
  pathname: string;
  counts?: Record<string, number>;
  animate: boolean;
}) {
  const { t } = useTranslation();
  const key = group.titleKey!;
  const panelId = `${useId()}-nav-group-${key}`;
  const sum = group.items.reduce((acc, i) => acc + (i.countKey ? (counts?.[i.countKey] ?? 0) : 0), 0);
  const Icon = group.icon;
  const hasActive = group.items.some((i) => isNavActive(pathname, i));
  return (
    <div>
      <button
        type="button"
        aria-expanded={open}
        aria-controls={panelId}
        onClick={onToggle}
        className="flex h-11 w-full items-center gap-[11px] rounded-md px-2.5 text-left type-label text-[13.5px] font-extrabold text-ink-900 outline-none hover:bg-surface-sunken focus-visible:ring-2 focus-visible:ring-brand-600"
      >
        {Icon && (
          <IconTile active={false} animate={animate} group={hasActive ? "has-active" : "idle"}>
            <Icon className="size-5" />
          </IconTile>
        )}
        <span className="truncate">{t(key)}</span>
        {!open && <CountPill n={sum} group />}
        <ChevronDown
          className={cn(
            "size-4 shrink-0 text-ink-600",
            sum > 0 && !open ? "ml-1" : "ml-auto",
            animate && "transition-transform duration-[350ms]",
            open && "rotate-180"
          )}
        />
      </button>
      <div
        id={panelId}
        className={cn(
          "grid",
          animate && "transition-[grid-template-rows] duration-[350ms] ease-standard",
          open ? "grid-rows-[1fr]" : "grid-rows-[0fr]"
        )}
      >
        <div className="overflow-hidden" inert={!open}>
          <div className="ml-[15px] flex flex-col gap-0.5 border-l-[1.5px] border-line pt-1 pb-1.5 pl-3.5">
            {group.items.map((item) => (
              <NavItemLink
                key={item.href + item.labelKey}
                item={item}
                active={isNavActive(pathname, item)}
                count={item.countKey ? counts?.[item.countKey] : undefined}
                nested
                animate={animate}
              />
            ))}
          </div>
        </div>
      </div>
    </div>
  );
}

export function AppSidebar({ role, counts, subtitle }: AppSidebarProps) {
  const pathname = usePathname();
  const me = useMe({ enabled: role === "affiliator" });
  const programStates = me.data?.affiliator_programs;
  const groups = useMemo(
    () => (NAV_CONFIG[role] ?? []).filter((g) => !g.program || !programStates || programStates[g.program] !== "not_enrolled"),
    [role, programStates],
  );
  const [openKey, setOpenKey] = useState<string | null>(() => activeGroupKey(groups, pathname));
  const animate = !useReducedMotion();

  useEffect(() => {
    setOpenKey(activeGroupKey(groups, pathname));
  }, [pathname, groups]);

  return (
    <div className="flex min-h-0 flex-1 flex-col px-3.5 py-[18px]">
      <div className="flex shrink-0 flex-col px-1.5 pb-4">
        <Link href="/" aria-label="Abak Academy">
          <AbakWordmark size={36} />
        </Link>
      </div>

      <nav className="flex flex-1 flex-col gap-0.5 overflow-y-auto">
        {groups.map((group, index) =>
          group.titleKey ? (
            <GroupSection
              key={group.titleKey}
              group={group}
              open={openKey === group.titleKey}
              onToggle={() => setOpenKey((k) => (k === group.titleKey ? null : group.titleKey!))}
              pathname={pathname}
              counts={counts}
              animate={animate}
            />
          ) : (
            <div key={`group-${index}`} className="flex flex-col gap-0.5">
              {group.items.map((item) => (
                <NavItemLink
                  key={item.href + item.labelKey}
                  item={item}
                  active={isNavActive(pathname, item)}
                  count={item.countKey ? counts?.[item.countKey] : undefined}
                  animate={animate}
                />
              ))}
            </div>
          )
        )}
      </nav>

      <div className="mt-auto border-t border-line pt-4">
        <SidebarUserMenu role={role} counts={counts} dotCountKey={role === "student" ? "cart" : undefined} subtitle={subtitle} />
      </div>
    </div>
  );
}
