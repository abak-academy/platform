"use client";

import { useMemo, useState } from "react";
import { Input } from "@/components/ui/input";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { useAdminSchools } from "@/lib/hooks/admin-schools";
import { useSchoolById } from "@/lib/hooks/students";
import { useDebouncedValue } from "@/lib/hooks/use-debounced-value";
import { cn } from "@/lib/utils";

export interface SchoolFilterPickerProps {
  value: string;
  onChange: (value: string) => void;
  label: string;
  allLabel: string;
  noneLabel?: string;
  className?: string;
  // compact renders a single select-level control (no label above, no inline
  // search input) with the school search moved into the dropdown, so it sits
  // level with sibling selects in a filter row.
  compact?: boolean;
}

const ALL_VALUE = "__all_schools__";
const NONE_VALUE = "__no_school__";

export function SchoolFilterPicker({
  value,
  onChange,
  label,
  allLabel,
  noneLabel,
  className,
  compact,
}: SchoolFilterPickerProps) {
  const [searchInput, setSearchInput] = useState("");
  const debouncedSearch = useDebouncedValue(searchInput).trim();
  const isNone = value === "none";

  const schoolsQuery = useAdminSchools({
    q: debouncedSearch || undefined,
    status: "active",
    limit: 20,
  });
  const schools = useMemo(() => schoolsQuery.data?.data ?? [], [schoolsQuery.data?.data]);
  const selectedSchoolOnPage = schools.find((school) => school.id === value) ?? null;
  const hydratedSchool = useSchoolById(value && !isNone && !selectedSchoolOnPage ? value : "");
  const selectedSchool: { id: string; name: string } | null =
    !value || isNone ? null : selectedSchoolOnPage ?? hydratedSchool.data ?? null;

  const selectValue = !value ? ALL_VALUE : isNone ? NONE_VALUE : value;
  const selectedSchoolMissing = Boolean(value && !isNone && !schools.some((school) => school.id === value));

  const handleSelect = (next: string) => {
    if (next === ALL_VALUE) {
      onChange("");
      return;
    }
    if (next === NONE_VALUE) {
      onChange("none");
      return;
    }
    onChange(next);
  };

  const selectItems = (
    <>
      <SelectItem value={ALL_VALUE}>{allLabel}</SelectItem>
      {noneLabel ? <SelectItem value={NONE_VALUE}>{noneLabel}</SelectItem> : null}
      {selectedSchoolMissing ? (
        <SelectItem value={value}>{selectedSchool?.id === value ? selectedSchool.name : allLabel}</SelectItem>
      ) : null}
      {schools.map((school) => (
        <SelectItem key={school.id} value={school.id}>
          {school.name}
        </SelectItem>
      ))}
    </>
  );

  if (compact) {
    return (
      <Select value={selectValue} onValueChange={handleSelect}>
        <SelectTrigger className={cn("h-9 text-xs", className)} aria-label={label}>
          <SelectValue placeholder={allLabel} />
        </SelectTrigger>
        <SelectContent>
          <div
            className="border-b border-line p-1"
            onClick={(event) => event.stopPropagation()}
            onKeyDown={(event) => event.stopPropagation()}
          >
            <Input
              autoFocus
              aria-label={`${label} search`}
              className="h-8 text-xs"
              placeholder={label}
              value={searchInput}
              onChange={(event) => setSearchInput(event.target.value)}
            />
          </div>
          {selectItems}
        </SelectContent>
      </Select>
    );
  }

  return (
    <div className={className}>
      <p className="text-xs text-ink-500">{label}</p>
      <div className="mt-1 flex flex-col gap-2 sm:flex-row sm:items-center">
        <Input
          aria-label={`${label} search`}
          className="h-9 w-full text-xs sm:w-[220px]"
          placeholder={label}
          value={searchInput}
          onChange={(event) => setSearchInput(event.target.value)}
        />
        <Select value={selectValue} onValueChange={handleSelect}>
          <SelectTrigger className="h-9 w-full text-xs sm:w-[240px]" aria-label={label}>
            <SelectValue placeholder={allLabel} />
          </SelectTrigger>
          <SelectContent>{selectItems}</SelectContent>
        </Select>
      </div>
    </div>
  );
}
