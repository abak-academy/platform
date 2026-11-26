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
  /** Substring of a string `title` drawn in ember with the gold squiggle. */
  highlight?: string;
  size?: "default" | "home";
}

function highlighted(title: React.ReactNode, highlight?: string) {
  if (typeof title !== "string" || !highlight) return title;
  const at = title.lastIndexOf(highlight);
  if (at < 0) return title;
  return (
    <>
      {title.slice(0, at)}
      <span className="relative inline-block whitespace-nowrap text-ember-700">
        {highlight}
        <svg viewBox="0 0 200 14" preserveAspectRatio="none" aria-hidden="true" className="absolute -bottom-2.5 left-[-2%] h-4 w-[104%]">
          <path d="M2 9 C 30 2, 50 14, 80 7 S 140 3, 198 8" fill="none" stroke="var(--color-gold-600)" strokeWidth="5" strokeLinecap="round" />
        </svg>
      </span>
      {title.slice(at + highlight.length)}
    </>
  );
}

export function PageHead({ title, sub, eyebrow, actions, back, className, highlight, size = "default" }: PageHeadProps) {
  const chrome = useContext(ShellChromeContext);
  const registerHead = chrome?.registerHead;
  useEffect(() => registerHead?.(), [registerHead]);

  const showActions = chrome?.showActions ?? false;
  const home = size === "home";

  return (
    <header className={cn("flex flex-col gap-1", home && "pt-6 pb-1 max-narrow:pt-3", className)}>
      {back && (
        <Link href={back.href} className="mb-5 inline-flex w-full items-center gap-1 text-[13px] leading-normal font-semibold text-ink-600 hover:text-ink-900">
          <ChevronLeft className="size-4" aria-hidden />
          {back.label}
        </Link>
      )}
      <div className={cn("flex flex-wrap", home ? "items-end gap-5" : "items-center gap-3")}>
        {chrome && <MenuButton onClick={chrome.openMenu} buttonRef={chrome.menuButtonRef} />}
        <div className="min-w-0 flex-1">
          {eyebrow && <p className={cn("type-overline text-ink-600", !home && "mb-1")}>{eyebrow}</p>}
          <h1
            className={cn(
              "text-ink-900",
              home
                ? "mt-2 mb-1.5 font-[family-name:var(--font-display)] text-[clamp(32px,4vw,48px)] leading-[1.08] font-extrabold tracking-[-0.04em]"
                : "type-h1",
            )}
          >
            {highlighted(title, highlight)}
          </h1>
          {sub && <p className={cn("text-ink-600", home ? "text-[15px]" : "mt-1 text-sm leading-[1.55]")}>{sub}</p>}
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
