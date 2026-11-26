"use client";

import { createContext, useContext, useEffect } from "react";
import Link from "next/link";
import { ChevronLeft } from "lucide-react";
import { cn } from "@/lib/utils";
import type { UserRole } from "@/lib/nav-config";
import { HeaderActions, MenuButton } from "./AppHeader";

export interface ShellChrome {
  role: UserRole;
  showActions: boolean;
  openMenu: () => void;
  menuButtonRef: React.Ref<HTMLButtonElement>;
  registerHead: () => () => void;
}

export const ShellChromeContext = createContext<ShellChrome | null>(null);

export interface PageHeadProps {
  title: React.ReactNode;
  sub?: React.ReactNode;
  eyebrow?: React.ReactNode;
  actions?: React.ReactNode;
  back?: { href: string; label: string };
  className?: string;
}

export function PageHead({ title, sub, eyebrow, actions, back, className }: PageHeadProps) {
  const chrome = useContext(ShellChromeContext);
  const registerHead = chrome?.registerHead;
  useEffect(() => registerHead?.(), [registerHead]);

  const showActions = chrome?.showActions ?? false;

  return (
    <header className={cn("flex flex-col gap-1", className)}>
      {back && (
        <Link href={back.href} className="inline-flex w-fit items-center gap-1 type-label-sm text-ink-600 hover:text-ink-900">
          <ChevronLeft className="size-4" aria-hidden />
          {back.label}
        </Link>
      )}
      <div className="flex flex-wrap items-center gap-3">
        {chrome && <MenuButton onClick={chrome.openMenu} buttonRef={chrome.menuButtonRef} />}
        <div className="min-w-0 flex-1">
          {eyebrow && <p className="mb-1 type-overline text-ink-600">{eyebrow}</p>}
          <h1 className="type-h1 text-ink-900">{title}</h1>
          {sub && <p className="mt-1 text-sm text-ink-600">{sub}</p>}
        </div>
        {(actions || (chrome && showActions)) && (
          <div className="flex flex-wrap items-center gap-2 max-narrow:w-full">
            {actions}
            {chrome && showActions && <HeaderActions role={chrome.role} />}
          </div>
        )}
      </div>
    </header>
  );
}
