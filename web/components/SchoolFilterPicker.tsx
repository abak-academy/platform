"use client";

import { useState } from "react";
import { Button } from "@/components/ui/button";
import { SchoolPicker } from "@/components/SchoolPicker";
import type { SchoolOption } from "@/lib/types";

export interface SchoolFilterPickerProps {
  value: string;
  onChange: (value: string) => void;
  label: string;
  allLabel: string;
  noneLabel?: string;
  className?: string;
}

export function SchoolFilterPicker({
  value,
  onChange,
  label,
  allLabel,
  noneLabel,
  className,
}: SchoolFilterPickerProps) {
  const [selectedSchool, setSelectedSchool] = useState<SchoolOption | null>(null);
  const isNone = value === "none";

  return (
    <div className={className}>
      <p className="text-xs text-ink-500">{label}</p>
      <div className="mt-1 flex flex-wrap gap-2">
        <Button
          type="button"
          variant={!value ? "default" : "outline"}
          size="sm"
          onClick={() => {
            setSelectedSchool(null);
            onChange("");
          }}
        >
          {allLabel}
        </Button>
        {noneLabel ? (
          <Button
            type="button"
            variant={isNone ? "default" : "outline"}
            size="sm"
            onClick={() => {
              setSelectedSchool(null);
              onChange("none");
            }}
          >
            {noneLabel}
          </Button>
        ) : null}
      </div>
      <SchoolPicker
        id={`${label.toLowerCase().replace(/\s+/g, "-")}-school-filter`}
        value={isNone ? "" : value}
        selectedSchool={isNone ? null : selectedSchool}
        onChange={(school) => {
          setSelectedSchool(school);
          onChange(school?.id ?? "");
        }}
        className="mt-2"
      />
    </div>
  );
}
