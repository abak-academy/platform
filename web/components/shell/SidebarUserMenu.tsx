"use client";

import Link from "next/link";
import { useRouter } from "next/navigation";
import { ChevronUp, LogOut, Moon, Sun } from "lucide-react";
import { useAuthStore } from "@/stores/auth";
import { useUIStore, type Lang } from "@/stores/ui";
import { useLogout, useMe } from "@/lib/hooks/auth";
import { fileUrl } from "@/lib/api";
import { Avatar, AvatarImage, AvatarFallback } from "@/components/ui/avatar";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import {
  USER_MENU,
  roleLabelKey,
  affiliatorRoleLabelKey,
  type UserRole,
  type UserMenuItem,
} from "@/lib/nav-config";
import { useTranslation } from "@/lib/i18n";
import { loginPathForRole } from "@/lib/auth-redirect";
import { cn } from "@/lib/utils";

interface SidebarUserMenuProps {
  role: UserRole;
  counts?: Record<string, number>;
  dotCountKey?: string;
}

const LANGS: Lang[] = ["id", "en"];
const ROW = "gap-2.5 rounded-md px-3 py-2.5 type-label text-ink-900 focus:bg-brand-50 focus:text-brand-ink";

function CountBadge({ n }: { n?: number }) {
  if (!n || n <= 0) return null;
  return (
    <span className="ml-auto min-w-5 rounded-full bg-ember-600 px-1.5 text-center type-label-xs text-on-ember">
      {n}
    </span>
  );
}

function LanguageRow({ item }: { item: UserMenuItem }) {
  const { t } = useTranslation();
  const lang = useUIStore((s) => s.lang);
  const setLang = useUIStore((s) => s.setLang);
  return (
    <DropdownMenuItem
      className={cn(ROW, "pr-1.5")}
      onSelect={(e) => {
        e.preventDefault();
        setLang(lang === "id" ? "en" : "id");
      }}
    >
      <item.icon className="size-5 text-ink-600" />
      <span>{t(item.labelKey)}</span>
      <span className="ml-auto flex overflow-hidden rounded-md border border-line">
        {LANGS.map((l) => (
          <button
            key={l}
            type="button"
            tabIndex={-1}
            aria-pressed={lang === l}
            onClick={(e) => {
              e.stopPropagation();
              setLang(l);
            }}
            className={cn(
              "h-8 px-2.5 type-label-xs uppercase",
              lang === l ? "bg-brand-600 text-on-brand" : "text-ink-600 hover:text-ink-700"
            )}
          >
            {l.toUpperCase()}
          </button>
        ))}
      </span>
    </DropdownMenuItem>
  );
}

function ThemeRow({ item }: { item: UserMenuItem }) {
  const { t } = useTranslation();
  const theme = useUIStore((s) => s.theme);
  const toggleTheme = useUIStore((s) => s.toggleTheme);
  const Icon = theme === "dark" ? Sun : Moon;
  return (
    <DropdownMenuItem className={ROW} onSelect={toggleTheme}>
      <Icon className="size-5 text-ink-600" />
      <span>{t(item.labelKey)}</span>
    </DropdownMenuItem>
  );
}

function LinkRow({ item, count }: { item: UserMenuItem; count?: number }) {
  const { t } = useTranslation();
  const body = (
    <>
      <item.icon className="size-5 text-ink-600" />
      <span className="flex min-w-0 flex-col">
        <span>{t(item.labelKey)}</span>
        {item.hintKey && (
          <span className="type-caption font-normal text-ink-600">{t(item.hintKey)}</span>
        )}
      </span>
      <CountBadge n={count} />
    </>
  );
  return (
    <DropdownMenuItem asChild className={ROW}>
      {item.external ? (
        <a href={item.href} target="_blank" rel="noopener noreferrer">
          {body}
        </a>
      ) : (
        <Link href={item.href!}>{body}</Link>
      )}
    </DropdownMenuItem>
  );
}

export function SidebarUserMenu({ role, counts, dotCountKey }: SidebarUserMenuProps) {
  const router = useRouter();
  const user = useAuthStore((s) => s.user);
  const logout = useLogout();
  const { t } = useTranslation();
  const me = useMe({ enabled: role === "affiliator" });

  const name = user?.name || user?.email || user?.username || t("account");
  const initial = (user?.name ?? user?.email ?? user?.username ?? "A").trim().charAt(0).toUpperCase();
  const roleKey =
    role === "affiliator"
      ? affiliatorRoleLabelKey(me.data?.affiliator_programs)
      : roleLabelKey(role ?? user?.role);
  const roleLabel = roleKey ? t(roleKey) : t("account");
  const dot = dotCountKey ? counts?.[dotCountKey] : undefined;

  function handleLogout() {
    const loginPath = loginPathForRole(role);
    logout.mutate(undefined, {
      onSettled: () => {
        router.replace(loginPath);
      },
    });
  }

  const avatar = (
    <Avatar className="size-9">
      <AvatarImage src={fileUrl(user?.photo_url)} />
      <AvatarFallback className="bg-brand-600 type-label-sm text-on-brand">{initial}</AvatarFallback>
    </Avatar>
  );

  return (
    <DropdownMenu>
      <DropdownMenuTrigger asChild>
        <button
          type="button"
          aria-label={t("user_menu_label")}
          className="group relative flex w-full items-center gap-3 rounded-md border border-line bg-surface p-2 text-left"
        >
          {avatar}
          <span className="min-w-0 flex-1">
            <span className="block truncate type-label text-ink-900">{name}</span>
            <span className="block truncate type-caption text-ink-600">{roleLabel}</span>
          </span>
          {dot !== undefined && dot > 0 && (
            <span
              data-testid="user-menu-dot"
              className="absolute -top-1 left-7 min-w-5 rounded-full bg-ember-600 px-1 text-center type-label-xs text-on-ember"
            >
              {dot}
            </span>
          )}
          <ChevronUp className="size-4 shrink-0 text-ink-600 transition-transform group-data-[state=open]:rotate-180" />
        </button>
      </DropdownMenuTrigger>
      <DropdownMenuContent
        side="top"
        align="start"
        sideOffset={8}
        className="w-(--radix-dropdown-menu-trigger-width) min-w-menu rounded-lg p-2"
      >
        <div className="flex items-center gap-2.5 border-b border-dashed border-line px-2.5 pb-3 pt-2">
          {avatar}
          <div className="min-w-0">
            <p className="truncate type-label text-ink-900">{name}</p>
            <p className="truncate type-caption text-ink-600">
              {user?.email ? `${user.email} · ${roleLabel}` : roleLabel}
            </p>
          </div>
        </div>
        {USER_MENU[role].map((item) =>
          item.action === "language" ? (
            <LanguageRow key={item.labelKey} item={item} />
          ) : item.action === "theme" ? (
            <ThemeRow key={item.labelKey} item={item} />
          ) : (
            <LinkRow key={item.labelKey} item={item} count={item.countKey ? counts?.[item.countKey] : undefined} />
          )
        )}
        <DropdownMenuSeparator className="border-t border-dashed border-line bg-transparent" />
        <DropdownMenuItem className={ROW} onSelect={handleLogout}>
          <LogOut className="size-5 text-ink-600" />
          <span>{t("logout")}</span>
        </DropdownMenuItem>
      </DropdownMenuContent>
    </DropdownMenu>
  );
}
