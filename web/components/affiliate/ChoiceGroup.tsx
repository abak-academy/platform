"use client";

import { cn } from "@/lib/utils";

export interface Choice {
  value: string;
  label: string;
  /** Classes for the selected state; defaults to the primary fill. */
  activeClass?: string;
}

interface ChoiceGroupProps {
  label: string;
  value: string;
  onChange: (value: string) => void;
  choices: Choice[];
}

// Pill-style single choice, used where a native <select> would hide the options.
export function ChoiceGroup({ label, value, onChange, choices }: ChoiceGroupProps) {
  return (
    <div role="radiogroup" aria-label={label} className="flex flex-wrap gap-2">
      {choices.map((c) => {
        const selected = c.value === value;
        return (
          <button
            key={c.value}
            type="button"
            role="radio"
            aria-checked={selected}
            onClick={() => onChange(c.value)}
            className={cn(
              "rounded-full border px-4 py-1.5 text-sm transition-colors",
              selected
                ? cn("border-transparent font-semibold ring-2 ring-offset-1 ring-current", c.activeClass ?? "bg-primary text-primary-foreground")
                : "border-line bg-surface text-ink-700 hover:bg-brand-50 hover:text-brand-700",
            )}
          >
            {c.label}
          </button>
        );
      })}
    </div>
  );
}
