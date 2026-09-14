"use client";

import { useEffect, useMemo, useState } from "react";
import { Input } from "@/components/ui/input";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { useAdminSchools } from "@/lib/hooks/admin-schools";
import type { School } from "@/lib/types";

export interface SchoolFilterPickerProps {
  value: string;
  onChange: (value: string) => void;
  label: string;
  allLabel: string;
  noneLabel?: string;
  className?: string;
}

const ALL_VALUE = "__all_schools__";
const NONE_VALUE = "__no_school__";
const SCHOOL_FILTER_DEBOUNCE_MS = 300;

export function SchoolFilterPicker({
  value,
  onChange,
  label,
  allLabel,
  noneLabel,
  className,
}: SchoolFilterPickerProps) {
  const [searchInput, setSearchInput] = useState("");
  const [debouncedSearch, setDebouncedSearch] = useState("");
  const [selectedSchool, setSelectedSchool] = useState<School | null>(null);
  const isNone = value === "none";

  useEffect(() => {
    const id = setTimeout(() => setDebouncedSearch(searchInput.trim()), SCHOOL_FILTER_DEBOUNCE_MS);
    return () => clearTimeout(id);
  }, [searchInput]);

  const schoolsQuery = useAdminSchools({
    q: debouncedSearch || undefined,
    status: "active",
    limit: 20,
  });
  const schools = useMemo(() => schoolsQuery.data?.data ?? [], [schoolsQuery.data?.data]);

  useEffect(() => {
    if (!value || isNone) {
      setSelectedSchool(null);
      return;
    }
    const match = schools.find((school) => school.id === value);
    if (match) setSelectedSchool(match);
  }, [isNone, schools, value]);

  const selectValue = !value ? ALL_VALUE : isNone ? NONE_VALUE : value;
  const selectedSchoolMissing = Boolean(value && !isNone && !schools.some((school) => school.id === value));

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
        <Select
          value={selectValue}
          onValueChange={(next) => {
            if (next === ALL_VALUE) {
              setSelectedSchool(null);
              onChange("");
              return;
            }
            if (next === NONE_VALUE) {
              setSelectedSchool(null);
              onChange("none");
              return;
            }
            const picked = schools.find((school) => school.id === next) ?? null;
            setSelectedSchool(picked);
            onChange(next);
          }}
        >
          <SelectTrigger className="h-9 w-full text-xs sm:w-[240px]" aria-label={label}>
            <SelectValue placeholder={allLabel} />
          </SelectTrigger>
          <SelectContent>
            <SelectItem value={ALL_VALUE}>{allLabel}</SelectItem>
            {noneLabel ? <SelectItem value={NONE_VALUE}>{noneLabel}</SelectItem> : null}
            {selectedSchoolMissing ? (
              <SelectItem value={value}>{selectedSchool?.id === value ? selectedSchool.name : value}</SelectItem>
            ) : null}
            {schools.map((school) => (
              <SelectItem key={school.id} value={school.id}>
                {school.name}
              </SelectItem>
            ))}
          </SelectContent>
        </Select>
      </div>
    </div>
  );
}
