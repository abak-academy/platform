"use client";

import { useTranslation } from "@/lib/i18n";
import { useUIStore } from "@/stores/ui";
import { cn } from "@/lib/utils";

export function LanguageToggle({ className, onDark }: { className?: string; onDark?: boolean }) {
  const { lang } = useTranslation();
  const setLang = useUIStore((s) => s.setLang);
  return (
    <div className={cn("inline-flex items-center rounded-[var(--radius-pill)] border border-line p-0.5", className)}>
      {(["id", "en"] as const).map((l) => (
        <button
          key={l}
          type="button"
          aria-pressed={lang === l}
          onClick={() => setLang(l)}
          className={cn(
            "rounded-[var(--radius-pill)] px-2.5 py-1 text-xs font-semibold outline-none focus-visible:outline-2 focus-visible:outline-brand-600",
            lang === l ? "bg-brand-600 text-on-brand" : onDark ? "text-white/80 hover:text-white" : "text-ink-600 hover:text-ink-900",
          )}
        >
          {l.toUpperCase()}
        </button>
      ))}
    </div>
  );
}
