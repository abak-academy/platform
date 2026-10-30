"use client";

import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { cn } from "@/lib/utils";

export interface Option {
  value: string;
  label: string;
  /** Classes for a colored label chip; plain text when absent. */
  tone?: string;
}

interface OptionSelectProps {
  id: string;
  label: string;
  value: string;
  onChange: (value: string) => void;
  options: Option[];
}

function OptionLabel({ option }: { option: Option }) {
  if (!option.tone) return <>{option.label}</>;
  return <span className={cn("rounded-full px-2.5 py-0.5 text-xs font-semibold", option.tone)}>{option.label}</span>;
}

// Dropdown with the same look as the list page-size picker; options can carry a colored chip.
export function OptionSelect({ id, label, value, onChange, options }: OptionSelectProps) {
  return (
    <Select value={value} onValueChange={onChange}>
      <SelectTrigger id={id} className="w-full" aria-label={label}>
        <SelectValue />
      </SelectTrigger>
      <SelectContent>
        {options.map((o) => (
          <SelectItem key={o.value} value={o.value} textValue={o.label}>
            <OptionLabel option={o} />
          </SelectItem>
        ))}
      </SelectContent>
    </Select>
  );
}
