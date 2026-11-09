"use client";

import { useState } from "react";
import { X } from "lucide-react";
import { Input } from "@/components/ui/input";
import { useAdminAffiliates } from "@/lib/hooks/admin-affiliates";
import { useDebouncedValue } from "@/lib/hooks/use-debounced-value";

interface PartnerPickerProps {
  /** Chosen Mitra ids. */
  value: string[];
  onChange: (ids: string[]) => void;
  disabled?: boolean;
  /** Shown while nothing is chosen. */
  emptyLabel: string;
  searchLabel: string;
}

// PartnerPicker searches the Mitra accounts by name or email and keeps the ones
// the admin ticks. Names of ids it has not seen yet fall back to a short id.
export function PartnerPicker({ value, onChange, disabled, emptyLabel, searchLabel }: PartnerPickerProps) {
  const [q, setQ] = useState("");
  const [names, setNames] = useState<Record<string, string>>({});
  const query = useAdminAffiliates({ program: "partner", q: useDebouncedValue(q, 300), per_page: 20 });
  const rows = query.data?.data ?? [];

  const toggle = (id: string, name: string) => {
    setNames((prev) => ({ ...prev, [id]: name }));
    onChange(value.includes(id) ? value.filter((v) => v !== id) : [...value, id]);
  };

  return (
    <div className="space-y-2">
      {value.length === 0 ? (
        <p className="text-xs text-ink-500">{emptyLabel}</p>
      ) : (
        <ul className="flex flex-wrap gap-1.5">
          {value.map((id) => (
            <li key={id} className="flex items-center gap-1 rounded-full bg-surface-2 px-2.5 py-1 text-xs text-ink-800">
              {names[id] ?? id.slice(0, 8)}
              <button type="button" aria-label={`remove ${names[id] ?? id}`} disabled={disabled} onClick={() => onChange(value.filter((v) => v !== id))}>
                <X className="size-3" />
              </button>
            </li>
          ))}
        </ul>
      )}
      <Input value={q} onChange={(e) => setQ(e.target.value)} placeholder={searchLabel} aria-label={searchLabel} disabled={disabled} />
      <ul className="max-h-40 divide-y divide-line overflow-y-auto rounded-md border border-line">
        {rows.map((row) => (
          <li key={row.id}>
            <label className="flex cursor-pointer items-center gap-2 px-3 py-1.5 text-sm">
              <input type="checkbox" checked={value.includes(row.id)} disabled={disabled} onChange={() => toggle(row.id, row.name)} />
              <span>{row.name}</span>
              <span className="text-ink-500">{row.email ?? ""}</span>
            </label>
          </li>
        ))}
      </ul>
    </div>
  );
}
