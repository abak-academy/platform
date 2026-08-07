"use client";

import Link from "next/link";
import { useRouter } from "next/navigation";
import {
  Menu,
  Search,
  Moon,
  Sun,
  Bell,
  LogOut,
  UserCircle,
  ShoppingCart,
} from "lucide-react";
import { useAuthStore } from "@/stores/auth";
import { useCartStore } from "@/stores/cart";
import { useUIStore, type Lang } from "@/stores/ui";
import { useLogout } from "@/lib/hooks/auth";
import { useResolvedRole } from "@/lib/hooks/use-capability";
import { fileUrl } from "@/lib/api";
import { useTranslation } from "@/lib/i18n";
import { roleLabelKey, ADMIN_ROLES } from "@/lib/nav-config";
import type { UserRole } from "@/lib/nav-config";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Avatar, AvatarFallback, AvatarImage } from "@/components/ui/avatar";
import { Badge } from "@/components/ui/badge";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuLabel,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import { cn } from "@/lib/utils";

interface AppHeaderProps {
  onMenuClick?: () => void;
}

export function AppHeader({ onMenuClick }: AppHeaderProps) {
  const router = useRouter();
  const user = useAuthStore((s) => s.user);
  const logout = useLogout();
  const { t } = useTranslation();

  const theme = useUIStore((s) => s.theme);
  const lang = useUIStore((s) => s.lang);
  const toggleTheme = useUIStore((s) => s.toggleTheme);
  const setLang = useUIStore((s) => s.setLang);

  const cartCount = useCartStore((s) => s.count);
  const isStudent = !ADMIN_ROLES.includes(user?.role as UserRole);
  const { role: resolvedRole } = useResolvedRole();
  const profileHref = ADMIN_ROLES.includes(resolvedRole as UserRole)
    ? "/admin/profile"
    : "/profile";

  const initial = (user?.name ?? user?.email ?? user?.username ?? "A")
    .trim()
    .charAt(0)
    .toUpperCase();

  const roleKey = roleLabelKey(user?.role);
  const roleLabel = roleKey ? t(roleKey) : t("account");

  function handleLogout() {
    logout.mutate(undefined, {
      onSettled: () => {
        router.replace("/login");
      },
    });
  }

  return (
    <header className="sticky top-0 z-20 flex h-16 shrink-0 items-center gap-3 border-b border-line bg-surface/90 px-4 backdrop-blur lg:px-8">
      <Button
        variant="ghost"
        size="icon-sm"
        onClick={onMenuClick}
        aria-label="Toggle sidebar"
        className="lg:hidden"
      >
        <Menu className="size-5 text-ink-600" />
      </Button>

      <div className="flex flex-1 items-center">
        <div className="flex h-[38px] w-full max-w-md items-center gap-2 rounded-lg border border-line bg-surface px-3">
          <Search className="size-4 shrink-0 text-ink-400" />
          <Input
            type="search"
            placeholder={t("search")}
            className="h-full border-0 bg-transparent px-0 shadow-none focus-visible:ring-0"
          />
        </div>
      </div>

      <div className="flex items-center gap-1">
        {/* ID / EN segmented language toggle */}
        <div className="hidden overflow-hidden rounded-lg border border-line bg-surface sm:flex">
          {(["id", "en"] as Lang[]).map((l) => (
            <button
              key={l}
              onClick={() => setLang(l)}
              className={cn(
                "cursor-pointer border-0 px-3 py-[7px] text-[12.5px] font-bold uppercase transition-colors",
                lang === l
                  ? "bg-brand-600 text-white"
                  : "bg-transparent text-ink-500 hover:text-ink-700"
              )}
            >
              {l.toUpperCase()}
            </button>
          ))}
        </div>

        {/* Dark / light mode toggle */}
        <Button
          variant="ghost"
          size="icon"
          onClick={toggleTheme}
          aria-label={theme === "dark" ? t("light_mode") : t("dark_mode")}
          className="hidden sm:flex"
        >
          {theme === "dark" ? (
            <Sun className="size-[18px] text-ink-600" />
          ) : (
            <Moon className="size-[18px] text-ink-600" />
          )}
        </Button>

        {isStudent && (
          <Button
            variant="ghost"
            size="icon"
            className="relative"
            aria-label="Keranjang"
            asChild
          >
            <Link href="/cart">
              <ShoppingCart className="size-[18px] text-ink-600" />
              {cartCount > 0 && (
                <span className="absolute right-1 top-1 flex size-[18px] items-center justify-center rounded-full bg-danger text-[10px] font-bold text-white">
                  {cartCount > 9 ? "9+" : cartCount}
                </span>
              )}
            </Link>
          </Button>
        )}

        <Button
          variant="ghost"
          size="icon"
          className="relative"
          aria-label="Notifikasi"
        >
          <Bell className="size-[18px] text-ink-600" />
          <span className="absolute right-1.5 top-1.5 size-2 rounded-full bg-danger" />
        </Button>

        <DropdownMenu>
          <DropdownMenuTrigger asChild>
            <Button
              variant="ghost"
              className="hidden items-center gap-2 px-2 md:flex"
            >
              <Avatar size="sm">
                <AvatarImage
                  src={fileUrl(user?.photo_url)}
                  alt={user?.name ?? "User"}
                />
                <AvatarFallback className="bg-brand-600 text-xs font-semibold text-white">
                  {initial}
                </AvatarFallback>
              </Avatar>
              <span className="max-w-[120px] truncate text-sm font-medium text-ink-900">
                {user?.name ?? t("account")}
              </span>
              <Badge variant="secondary" className="text-[10px] font-normal">
                {roleLabel}
              </Badge>
            </Button>
          </DropdownMenuTrigger>
          <DropdownMenuContent align="end" className="w-52">
            <DropdownMenuLabel className="text-ink-900">
              <div className="truncate font-medium">
                {user?.name ?? t("account")}
              </div>
              <div className="text-[11px] font-normal text-ink-500">
                {roleLabel}
              </div>
            </DropdownMenuLabel>
            <DropdownMenuSeparator />
            <DropdownMenuItem asChild>
              <Link href={profileHref} className="flex items-center">
                <UserCircle className="size-4" />
                <span className="ml-2">{t("nav_profile")}</span>
              </Link>
            </DropdownMenuItem>
            <DropdownMenuSeparator />
            <DropdownMenuItem onClick={handleLogout}>
              <LogOut className="size-4" />
              <span className="ml-2">{t("logout")}</span>
            </DropdownMenuItem>
          </DropdownMenuContent>
        </DropdownMenu>
      </div>
    </header>
  );
}
