"use client";

import { useEffect, useMemo, useState } from "react";
import Link from "next/link";
import { usePathname } from "next/navigation";
import { ChevronDown } from "lucide-react";
import { useMe } from "@/lib/hooks/auth";
import { Badge } from "@/components/ui/badge";
import { Separator } from "@/components/ui/separator";
import { AbakMark } from "./AbakMark";
import { SidebarUserMenu } from "./SidebarUserMenu";
import {
  NAV_CONFIG,
  ADMIN_ROLES,
  type UserRole,
  type NavItem,
  type NavGroup,
} from "@/lib/nav-config";
import { useTranslation } from "@/lib/i18n";
import { cn } from "@/lib/utils";

interface AppSidebarProps {
  role: UserRole;
  counts?: Record<string, number>;
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

function CountPill({ n }: { n?: number }) {
  if (!n || n <= 0) return null;
  return (
    <span className="ml-auto min-w-5 rounded-full bg-ember-600 px-1.5 text-center type-label-xs text-on-ember">
      {n}
    </span>
  );
}

function IconTile({ active, children }: { active: boolean; children: React.ReactNode }) {
  return (
    <span
      className={cn(
        "flex size-8 shrink-0 items-center justify-center rounded-sm",
        active ? "bg-brand-600 text-on-brand" : "text-ink-600"
      )}
    >
      {children}
    </span>
  );
}

function NavItemLink({ item, active, count, nested }: { item: NavItem; active: boolean; count?: number; nested?: boolean }) {
  const { t } = useTranslation();
  return (
    <Link
      href={item.href}
      title={t(item.labelKey)}
      aria-current={active ? "page" : undefined}
      className={cn(
        "flex items-center gap-3 rounded-md px-2.5 type-label",
        nested ? "min-h-9.5" : "min-h-nav-row",
        active
          ? "bg-brand-50 text-brand-700"
          : "text-ink-600 hover:bg-brand-50/60 hover:text-brand-700"
      )}
    >
      <IconTile active={active}>
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
  const panelId = `nav-group-${key}`;
  const sum = group.items.reduce((acc, i) => acc + (i.countKey ? (counts?.[i.countKey] ?? 0) : 0), 0);
  const Icon = group.icon;
  return (
    <div>
      <button
        type="button"
        aria-expanded={open}
        aria-controls={panelId}
        onClick={onToggle}
        className="flex min-h-nav-row w-full items-center gap-3 rounded-md px-2.5 text-left type-label text-ink-600 hover:bg-brand-50/60"
      >
        {Icon && (
          <IconTile active={false}>
            <Icon className="size-5" />
          </IconTile>
        )}
        <span className="truncate">{t(key)}</span>
        {!open && <CountPill n={sum} />}
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
          <div className="flex flex-col gap-1 pt-1">
            {group.items.map((item) => (
              <NavItemLink
                key={item.href + item.labelKey}
                item={item}
                active={isNavActive(pathname, item)}
                count={item.countKey ? counts?.[item.countKey] : undefined}
                nested
              />
            ))}
          </div>
        </div>
      </div>
    </div>
  );
}

export function AppSidebar({ role, counts }: AppSidebarProps) {
  const pathname = usePathname();
  const { t } = useTranslation();
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
    <>
      <div className="flex h-[72px] shrink-0 items-center px-5">
        <Link href="/" className="flex items-center gap-3">
          <div className="flex size-10 shrink-0 items-center justify-center rounded-[11px] bg-brand-50">
            <AbakMark size={28} />
          </div>
          <div className="flex flex-col justify-center">
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

      <nav className="flex flex-1 flex-col gap-1 overflow-y-auto px-3 py-3">
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
            <div key={`group-${index}`} className="flex flex-col gap-1">
              {group.items.map((item) => (
                <NavItemLink
                  key={item.href + item.labelKey}
                  item={item}
                  active={isNavActive(pathname, item)}
                  count={item.countKey ? counts?.[item.countKey] : undefined}
                />
              ))}
            </div>
          )
        )}
      </nav>

      <div className="mt-auto border-t border-line p-3">
        <SidebarUserMenu role={role} counts={counts} />
      </div>
    </>
  );
}
