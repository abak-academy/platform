"use client";

import { Menu, Moon, Sun, Bell } from "lucide-react";
import { useUIStore } from "@/stores/ui";
import { useTranslation } from "@/lib/i18n";
import { ADMIN_ROLES, type UserRole } from "@/lib/nav-config";
import { Button } from "@/components/ui/button";

interface AppHeaderProps {
  role: UserRole;
  onMenuClick?: () => void;
  menuButtonRef?: React.Ref<HTMLButtonElement>;
}

export function AppHeader({ role, onMenuClick, menuButtonRef }: AppHeaderProps) {
  const { t } = useTranslation();
  const theme = useUIStore((s) => s.theme);
  const toggleTheme = useUIStore((s) => s.toggleTheme);
  const showActions = ADMIN_ROLES.includes(role) || role === "affiliator";

  return (
    <header className="sticky top-0 z-20 flex h-16 shrink-0 items-center gap-3 border-b border-line bg-surface/90 px-4 backdrop-blur lg:px-8">
      <Button
        ref={menuButtonRef}
        variant="ghost"
        size="icon-sm"
        onClick={onMenuClick}
        aria-label="Toggle sidebar"
        className="lg:hidden"
      >
        <Menu className="size-5 text-ink-600" />
      </Button>

      <div className="flex-1" />

      {showActions && (
        <div className="flex items-center gap-1">
          <Button
            variant="ghost"
            size="icon"
            onClick={toggleTheme}
            aria-label={theme === "dark" ? t("light_mode") : t("dark_mode")}
            className="flex"
          >
            {theme === "dark" ? (
              <Sun className="size-5 text-ink-600" />
            ) : (
              <Moon className="size-5 text-ink-600" />
            )}
          </Button>

          <Button
            variant="ghost"
            size="icon"
            className="relative"
            aria-label="Notifikasi"
          >
            <Bell className="size-5 text-ink-600" />
            <span className="absolute right-1.5 top-1.5 size-2 rounded-full bg-danger" />
          </Button>
        </div>
      )}
    </header>
  );
}
