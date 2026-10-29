"use client";

import { cn } from "@/lib/utils";

interface Option<T extends string> {
  value: T;
  label: string;
}

interface Props<T extends string> {
  value: T;
  onChange: (value: T) => void;
  options: Option<T>[];
  label: string;
}

// Choose one of a few modes: a joined bar where the chosen option is filled with
// the primary colour. Colours come from Tailwind tokens, not the admin theme
// variables, so they also show inside dialogs (which render in a portal). The
// chosen option darkens on hover, the others tint, so the two hovers differ.
export function SegmentedControl<T extends string>({ value, onChange, options, label }: Props<T>) {
  return (
    <div role="radiogroup" aria-label={label} className="flex w-fit overflow-hidden rounded-[10px] border border-line">
      {options.map((o) => {
        const active = o.value === value;
        return (
          <button
            key={o.value}
            type="button"
            role="radio"
            aria-checked={active}
            onClick={() => onChange(o.value)}
            className={cn(
              "border-r border-line px-4 py-1.5 text-sm transition-colors outline-none last:border-r-0",
              "focus-visible:ring-[3px] focus-visible:ring-inset focus-visible:ring-ring/50",
              active
                ? "bg-primary font-semibold text-primary-foreground hover:bg-brand-700"
                : "text-ink-700 hover:bg-brand-50 hover:text-brand-700",
            )}
          >
            {o.label}
          </button>
        );
      })}
    </div>
  );
}
