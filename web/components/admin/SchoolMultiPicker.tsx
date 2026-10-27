"use client";

import { X } from "lucide-react";
import { SchoolPicker } from "@/components/SchoolPicker";
import { Button } from "@/components/ui/button";
import { useTranslation } from "@/lib/i18n";

export interface PickedSchool {
  id: string;
  name: string;
}

interface Props {
  value: PickedSchool[];
  onChange: (schools: PickedSchool[]) => void;
  disabled?: boolean;
}

// Builds a list of schools by picking them one at a time with the school search.
export function SchoolMultiPicker({ value, onChange, disabled }: Props) {
  const { t } = useTranslation();

  return (
    <div className="grid gap-2">
      {value.length > 0 && (
        <ul className="flex flex-wrap gap-2" aria-label={t("aff_selected_schools")}>
          {value.map((s) => (
            <li key={s.id} className="md-chip md-chip-primary gap-1">
              {s.name}
              <Button
                type="button"
                variant="ghost"
                size="icon"
                className="size-8"
                aria-label={`${t("aff_remove_school")}: ${s.name}`}
                disabled={disabled}
                onClick={() => onChange(value.filter((v) => v.id !== s.id))}
              >
                <X className="size-4" aria-hidden />
              </Button>
            </li>
          ))}
        </ul>
      )}
      <SchoolPicker
        key={value.length}
        id="aff-school-picker"
        disabled={disabled}
        onChange={(school) => {
          if (school && !value.some((v) => v.id === school.id)) onChange([...value, { id: school.id, name: school.name }]);
        }}
      />
    </div>
  );
}
