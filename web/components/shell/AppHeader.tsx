"use client";

import Link from "next/link";
import { Menu, Moon, Sun, Bell } from "lucide-react";
import { useUIStore } from "@/stores/ui";
import { useTranslation } from "@/lib/i18n";
import { ADMIN_ROLES, NAV_CONFIG, type UserRole } from "@/lib/nav-config";
import { cn } from "@/lib/utils";

const ICON_BTN =
  "relative inline-grid size-[var(--spacing-control-md)] shrink-0 place-items-center rounded-full border-[1.5px] border-line bg-surface text-ink-700 transition-[transform,border-color] duration-300 ease-spring hover:-translate-y-px hover:border-ink-900";

export function hasShellActions(role: UserRole) {
  return ADMIN_ROLES.includes(role) || role === "affiliator";
}

export function MenuButton({
  onClick,
  buttonRef,
  className,
}: {
  onClick?: () => void;
  buttonRef?: React.Ref<HTMLButtonElement>;
  className?: string;
}) {
  const { t } = useTranslation();
  return (
    <button
      ref={buttonRef}
      type="button"
      onClick={onClick}
      aria-label={t("toggle_sidebar")}
      className={cn(ICON_BTN, "lg:hidden", className)}
    >
      <Menu className="size-5" />
    </button>
  );
}

// The bell only renders when the role can open the announcements page, so it is never a dead button.
export function HeaderActions({ role }: { role: UserRole }) {
  const { t } = useTranslation();
  const theme = useUIStore((s) => s.theme);
  const toggleTheme = useUIStore((s) => s.toggleTheme);
  const bellHref = NAV_CONFIG[role]
    ?.flatMap((g) => g.items)
    .find((i) => i.href === "/admin/notifications")?.href;

  return (
    <>
      <button
        type="button"
        onClick={toggleTheme}
        aria-label={theme === "dark" ? t("light_mode") : t("dark_mode")}
        className={ICON_BTN}
      >
        {theme === "dark" ? <Sun className="size-5" /> : <Moon className="size-5" />}
      </button>
      {bellHref && (
        <Link href={bellHref} aria-label={t("notifications")} className={ICON_BTN}>
          <Bell className="size-5" />
        </Link>
      )}
    </>
  );
}

interface AppHeaderProps {
  role: UserRole;
  onMenuClick?: () => void;
  menuButtonRef?: React.Ref<HTMLButtonElement>;
}

// Fallback chrome for pages that do not render a PageHead: a slim row, not a bar.
export function AppHeader({ role, onMenuClick, menuButtonRef }: AppHeaderProps) {
  const showActions = hasShellActions(role);

  return (
    <header className={cn("flex items-center gap-3", !showActions && "lg:hidden")}>
      <MenuButton onClick={onMenuClick} buttonRef={menuButtonRef} />
      <div className="flex-1" />
      {showActions && <HeaderActions role={role} />}
    </header>
  );
}
