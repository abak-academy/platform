"use client";

import { useMemo, useState } from "react";
import { Input } from "@/components/ui/input";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { useAdminAffiliator, useAdminAffiliators } from "@/lib/hooks/admin-affiliators";
import { useDebouncedValue } from "@/lib/hooks/use-debounced-value";

const ALL = "__all_affiliators__";

interface Props {
  value: string;
  onChange: (value: string) => void;
  label: string;
  allLabel: string;
}

// Pick one affiliator from a dropdown that has its own search, like
// SchoolFilterPicker does for schools. The chosen one is still offered when a
// later search no longer lists it, so the selection never looks empty.
export function AffiliatorFilterPicker({ value, onChange, label, allLabel }: Props) {
  const [search, setSearch] = useState("");
  const q = useDebouncedValue(search).trim();
  const list = useAdminAffiliators({ q: q || undefined, limit: 20 });
  const rows = useMemo(() => list.data?.data ?? [], [list.data?.data]);
  const onPage = rows.find((a) => a.id === value);
  const hydrated = useAdminAffiliator(value && !onPage ? value : null);
  const missingName = hydrated.data?.affiliator.name ?? allLabel;

  return (
    <Select value={value || ALL} onValueChange={(v) => onChange(v === ALL ? "" : v)}>
      <SelectTrigger className="h-9 w-full text-xs" aria-label={label}>
        <SelectValue placeholder={allLabel} />
      </SelectTrigger>
      <SelectContent>
        <div className="border-b border-line p-1" onClick={(e) => e.stopPropagation()} onKeyDown={(e) => e.stopPropagation()}>
          <Input autoFocus aria-label={`${label} search`} className="h-8 text-xs" placeholder={label} value={search} onChange={(e) => setSearch(e.target.value)} />
        </div>
        <SelectItem value={ALL}>{allLabel}</SelectItem>
        {value && !onPage ? <SelectItem value={value}>{missingName}</SelectItem> : null}
        {rows.map((a) => (
          <SelectItem key={a.id} value={a.id}>
            {a.name}
          </SelectItem>
        ))}
      </SelectContent>
    </Select>
  );
}
