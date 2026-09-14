"use client";

import { useEffect, useMemo, useState } from "react";
import { Loader2, Search } from "lucide-react";
import { useCitiesByProvince, useProvinces } from "@/lib/hooks/regions";
import { useTranslation } from "@/lib/i18n";
import { useSchoolById, useSchoolSearch } from "@/lib/hooks/students";
import type { SchoolOption } from "@/lib/types";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";

const CATEGORIES = ["SD", "MI", "SMP", "MTS", "SMA", "MA", "SMK"];

export interface SchoolPickerProps {
  id?: string;
  value?: string;
  selectedSchool?: SchoolOption | null;
  onChange: (school: SchoolOption | null) => void;
  allowUnlisted?: boolean;
  unlistedName?: string;
  onUnlistedNameChange?: (value: string) => void;
  disabled?: boolean;
  className?: string;
  tone?: "default" | "inverse";
}

export function SchoolPicker({
  id = "school-picker",
  value,
  selectedSchool,
  onChange,
  allowUnlisted,
  unlistedName = "",
  onUnlistedNameChange,
  disabled,
  className,
  tone = "default",
}: SchoolPickerProps) {
  const [mode, setMode] = useState<"name" | "npsn">("name");
  const [provinceId, setProvinceId] = useState("");
  const [cityId, setCityId] = useState("");
  const [category, setCategory] = useState("");
  const [qInput, setQInput] = useState("");
  const [npsnInput, setNpsnInput] = useState("");
  const [unlistedActive, setUnlistedActive] = useState(Boolean(unlistedName));
  const { t } = useTranslation();

  const { data: provinces } = useProvinces();
  const { data: cities } = useCitiesByProvince(provinceId);
  const hydrate = useSchoolById(selectedSchool ? "" : value);
  const selected = selectedSchool ?? hydrate.data ?? null;

  useEffect(() => {
    if (unlistedName) setUnlistedActive(true);
  }, [unlistedName]);

  useEffect(() => {
    setCityId("");
  }, [provinceId]);

  const normalizedNPSN = npsnInput.trim().toUpperCase();
  const params = useMemo(() => {
    if (mode === "npsn") {
      return { npsn: normalizedNPSN, limit: 20 };
    }
    return {
      province_id: provinceId,
      city_id: cityId,
      category,
      limit: 1000,
    };
  }, [category, cityId, mode, normalizedNPSN, provinceId]);

  const enabled =
    !disabled &&
    (mode === "npsn"
      ? /^[A-Z0-9]{8}$/.test(normalizedNPSN)
      : Boolean(provinceId && cityId && category));
  const search = useSchoolSearch(params, enabled);
  const results = search.data?.data ?? [];
  const nameFilter = qInput.trim().toLowerCase();
  const displayedResults =
    mode === "name" && nameFilter
      ? results.filter((school) => school.name.toLowerCase().includes(nameFilter))
      : results;

  function activateSearchMode(nextMode: "name" | "npsn") {
    setMode(nextMode);
    setUnlistedActive(false);
    onUnlistedNameChange?.("");
  }

  return (
    <div className={className} data-school-picker-tone={tone}>
      <div className="mb-2 flex gap-2">
        <Button
          type="button"
          variant={mode === "name" ? "default" : "outline"}
          size="sm"
          onClick={() => activateSearchMode("name")}
          disabled={disabled}
        >
          {t("school_picker_mode_name")}
        </Button>
        <Button
          type="button"
          variant={mode === "npsn" ? "default" : "outline"}
          size="sm"
          onClick={() => activateSearchMode("npsn")}
          disabled={disabled}
        >
          {t("school_picker_mode_npsn")}
        </Button>
        {allowUnlisted ? (
          <Button
            type="button"
            variant={unlistedActive ? "default" : "outline"}
            size="sm"
            onClick={() => {
              setUnlistedActive(true);
              onChange(null);
              onUnlistedNameChange?.("");
            }}
            disabled={disabled}
          >
            {t("school_picker_unlisted")}
          </Button>
        ) : null}
      </div>

      {allowUnlisted && unlistedActive ? (
        <Input
          id={id}
          value={unlistedName}
          onChange={(e) => {
            onChange(null);
            onUnlistedNameChange?.(e.target.value);
          }}
          placeholder={t("school_picker_unlisted_placeholder")}
          disabled={disabled}
        />
      ) : (
        <>
          {mode === "name" ? (
            <div className="grid gap-2 sm:grid-cols-3">
              <Select value={provinceId || "_empty_"} onValueChange={(v) => setProvinceId(v === "_empty_" ? "" : v)} disabled={disabled}>
                <SelectTrigger id={`${id}-province`}>
                  <SelectValue placeholder={t("school_picker_province")} />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="_empty_">{t("school_picker_province")}</SelectItem>
                  {(provinces ?? []).map((p) => (
                    <SelectItem key={p.id} value={p.id}>
                      {p.name}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
              <Select value={cityId || "_empty_"} onValueChange={(v) => setCityId(v === "_empty_" ? "" : v)} disabled={disabled || !provinceId}>
                <SelectTrigger id={`${id}-city`}>
                  <SelectValue placeholder={t("school_picker_city")} />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="_empty_">{t("school_picker_city")}</SelectItem>
                  {(cities ?? []).map((c) => (
                    <SelectItem key={c.id} value={c.id}>
                      {c.name}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
              <Select value={category || "_empty_"} onValueChange={(v) => setCategory(v === "_empty_" ? "" : v)} disabled={disabled}>
                <SelectTrigger id={`${id}-category`}>
                  <SelectValue placeholder={t("school_picker_category")} />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="_empty_">{t("school_picker_category")}</SelectItem>
                  {CATEGORIES.map((c) => (
                    <SelectItem key={c} value={c}>
                      {c}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
              <div className="relative sm:col-span-3">
                <Search className="pointer-events-none absolute top-1/2 left-3 size-4 -translate-y-1/2 text-ink-400" />
                <Input
                  id={id}
                  value={qInput}
                  onChange={(e) => setQInput(e.target.value)}
                  placeholder={t("school_picker_search_name_placeholder")}
                  disabled={disabled}
                  className="pl-9"
                />
              </div>
            </div>
          ) : (
            <Input
              id={id}
              value={npsnInput}
              onChange={(e) => setNpsnInput(e.target.value)}
              placeholder={t("school_picker_npsn_placeholder")}
              disabled={disabled}
            />
          )}

          {selected ? (
            <div className="mt-2 rounded-md border border-line bg-surface-2 px-3 py-2 text-xs text-ink-700">
              <div className="font-medium text-ink-900">{selected.name}</div>
              <div>{[selected.npsn, selected.category, selected.kota_name, selected.provinsi_name].filter(Boolean).join(" · ")}</div>
            </div>
          ) : null}

          <div className="mt-2 space-y-1">
            {search.isFetching ? (
              <div className="flex items-center gap-2 text-xs text-ink-500">
                <Loader2 className="size-3 animate-spin" />
                {t("school_picker_searching")}
              </div>
            ) : enabled && displayedResults.length === 0 ? (
              <div className="text-xs text-ink-500">{t("school_picker_no_results")}</div>
            ) : null}
            {displayedResults.map((school) => (
              <button
                key={school.id}
                type="button"
                className="block w-full rounded-md border border-line px-3 py-2 text-left text-sm hover:border-brand-300 disabled:opacity-60"
                disabled={disabled}
                onClick={() => {
                  setUnlistedActive(false);
                  onUnlistedNameChange?.("");
                  onChange(school);
                }}
              >
                <span className="block font-medium text-ink-900">{school.name}</span>
                <span className="block text-xs text-ink-500">
                  {[school.npsn, school.category, school.kota_name, school.provinsi_name].filter(Boolean).join(" · ")}
                </span>
              </button>
            ))}
          </div>
        </>
      )}
    </div>
  );
}
