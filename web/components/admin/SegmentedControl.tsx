"use client";

import { Button } from "@/components/ui/button";

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

// Choose one of a few modes. The chosen one is the filled primary button and the
// rest are outlined, built from Button so it keeps its colour inside dialogs too
// (they render in a portal, outside the admin theme variables).
export function SegmentedControl<T extends string>({ value, onChange, options, label }: Props<T>) {
  return (
    <div role="radiogroup" aria-label={label} className="flex w-fit gap-2">
      {options.map((o) => {
        const active = o.value === value;
        return (
          <Button
            key={o.value}
            type="button"
            role="radio"
            aria-checked={active}
            variant={active ? "default" : "outline"}
            size="sm"
            onClick={() => onChange(o.value)}
          >
            {o.label}
          </Button>
        );
      })}
    </div>
  );
}
