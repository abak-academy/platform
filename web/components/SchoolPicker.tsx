"use client";

import { useEffect, useRef, useState } from "react";
import { Loader2, Search, X } from "lucide-react";
import { useSchoolCities, useProvinces } from "@/lib/hooks/regions";
import { useTranslation } from "@/lib/i18n";
import { useSchoolById, useSchoolAutocomplete } from "@/lib/hooks/students";
import { SCHOOL_TYPE_OPTIONS } from "@/lib/school-types";
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
}: SchoolPickerProps) {
  const [mode, setMode] = useState<"name" | "npsn">("name");
  const [provinceId, setProvinceId] = useState("");
  const [cityId, setCityId] = useState("");
  const [schoolType, setSchoolType] = useState("");
  const [qInput, setQInput] = useState("");
  const [npsnInput, setNpsnInput] = useState("");
  const [unlistedActive, setUnlistedActive] = useState(Boolean(unlistedName));
  const [open, setOpen] = useState(false);
  const [activeIndex, setActiveIndex] = useState(-1);
  const [debouncedQ, setDebouncedQ] = useState("");
  const inputRef = useRef<HTMLInputElement>(null);
  const clearRef = useRef<HTMLButtonElement>(null);
  const focusAfterSelection = useRef(false);
  const { t } = useTranslation();

  const { data: provinces } = useProvinces();
  const { data: cities } = useSchoolCities(provinceId);
  const hydrate = useSchoolById(selectedSchool ? "" : value);
  const selected = selectedSchool ?? hydrate.data ?? null;

  useEffect(() => {
    if (!focusAfterSelection.current) return;
    focusAfterSelection.current = false;
    if (selected) clearRef.current?.focus();
    else inputRef.current?.focus();
  }, [selected]);

  useEffect(() => {
    if (unlistedName) setUnlistedActive(true);
  }, [unlistedName]);

  const nameValue = selected?.name ?? qInput;
  const npsnValue = selected?.npsn ?? npsnInput;
  useEffect(() => {
    const timer = setTimeout(() => setDebouncedQ(nameValue.trim()), 500);
    return () => clearTimeout(timer);
  }, [nameValue]);

  const normalizedNPSN = npsnValue.trim().toUpperCase();
  const pendingName = nameValue.trim() !== debouncedQ;
  const validSearch = mode === "npsn"
    ? /^[A-Z0-9]{8}$/.test(normalizedNPSN)
    : !pendingName && (debouncedQ.length >= 2 || (!debouncedQ && Boolean(provinceId || cityId || schoolType)));
  const params = mode === "npsn"
    ? { npsn: normalizedNPSN, limit: 20 }
    : { province_id: provinceId, city_id: cityId, school_type: schoolType, q: debouncedQ, limit: 20 };
  const enabled = !disabled && !unlistedActive && open && validSearch;
  const search = useSchoolAutocomplete(params, enabled);
  const results = enabled && !search.isError ? search.data?.pages.flatMap((page) => page.data) ?? [] : [];
  const showDropdown = open && !disabled;

  function pickSchool(school: SchoolOption) {
    setQInput(school.name);
    setNpsnInput(school.npsn ?? "");
    setProvinceId(school.provinsi_id ?? "");
    setCityId(school.kota_id ?? "");
    const types = (school.school_types ?? []).map((type) => type.trim().toUpperCase()).filter((type) => SCHOOL_TYPE_OPTIONS.some((option) => option === type));
    setSchoolType(types.includes(schoolType) ? schoolType : types[0] ?? "");
    setOpen(false);
    setActiveIndex(-1);
    setUnlistedActive(false);
    onUnlistedNameChange?.("");
    focusAfterSelection.current = true;
    onChange(school);
  }

  function clearSchool() {
    setQInput("");
    setNpsnInput("");
    setDebouncedQ("");
    setProvinceId("");
    setCityId("");
    setSchoolType("");
    setActiveIndex(-1);
    focusAfterSelection.current = true;
    onChange(null);
  }

  function changeFilter() {
    if (selected) {
      setQInput(selected.name);
      onChange(null);
    }
    setActiveIndex(-1);
    setOpen(true);
  }

  function activateSearchMode(nextMode: "name" | "npsn") {
    setMode(nextMode);
    setOpen(false);
    setActiveIndex(-1);
    setUnlistedActive(false);
    onUnlistedNameChange?.("");
  }

  return (
    <div className={className}>
      <div className="mb-2 flex flex-wrap gap-2">
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
              setOpen(false);
              setQInput("");
              setNpsnInput("");
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
            <>
              <div
                data-testid="school-picker-location-filters"
                className="grid gap-2 [grid-template-columns:repeat(auto-fit,minmax(9rem,1fr))]"
              >
              <Select value={provinceId || "_empty_"} onValueChange={(v) => { setProvinceId(v === "_empty_" ? "" : v); setCityId(""); changeFilter(); }} disabled={disabled}>
                <SelectTrigger id={`${id}-province`} aria-label={t("school_picker_province")} className="w-full min-w-0">
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
              <Select value={cityId || "_empty_"} onValueChange={(v) => { setCityId(v === "_empty_" ? "" : v); changeFilter(); }} disabled={disabled}>
                <SelectTrigger id={`${id}-city`} aria-label={t("school_picker_city")} className="w-full min-w-0">
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
              <Select value={schoolType || "_empty_"} onValueChange={(v) => { setSchoolType(v === "_empty_" ? "" : v); changeFilter(); }} disabled={disabled}>
                <SelectTrigger id={`${id}-school-type`} aria-label={t("schools_field_school_types")} className="w-full min-w-0">
                  <SelectValue placeholder={t("schools_field_school_types")} />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="_empty_">{t("schools_field_school_types")}</SelectItem>
                  {SCHOOL_TYPE_OPTIONS.map((c) => (
                    <SelectItem key={c} value={c}>
                      {c}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
              </div>
            </>
          ) : null}

          {selected ? (
            <div id={id} role="group" aria-label={t("school_picker_selected")} className="mt-2 flex items-center gap-3 rounded-md border border-line bg-surface px-3 py-2">
              <div className="min-w-0 flex-1">
                <div className="break-words font-medium text-ink-900">{selected.name}</div>
                <div className="break-words text-xs text-ink-500">{[selected.npsn, selected.school_types?.join(" / "), selected.kota_name, selected.provinsi_name].filter(Boolean).join(" · ")}</div>
              </div>
              <Button ref={clearRef} type="button" variant="ghost" size="icon" className="shrink-0" aria-label={t("school_picker_change")} onClick={clearSchool} disabled={disabled}>
                <X className="size-4" aria-hidden="true" />
              </Button>
            </div>
          ) : (
          <div
            className="relative mt-2"
            onBlur={(event) => {
              if (!event.currentTarget.contains(event.relatedTarget)) setOpen(false);
            }}
          >
            <Search className="pointer-events-none absolute top-3 left-3 size-4 text-ink-400" />
            <Input
              ref={inputRef}
              id={id}
              role="combobox"
              aria-label={t(mode === "name" ? "school_picker_search_name_placeholder" : "school_picker_npsn_placeholder")}
              aria-autocomplete="list"
              aria-expanded={showDropdown}
              aria-controls={showDropdown ? `${id}-results` : undefined}
              aria-activedescendant={showDropdown && results[activeIndex] ? `${id}-option-${results[activeIndex].id}` : undefined}
              autoComplete="off"
              value={mode === "name" ? nameValue : npsnValue}
              onFocus={() => { setOpen(true); setActiveIndex(-1); }}
              onChange={(event) => {
                if (selected) onChange(null);
                if (mode === "name") setQInput(event.target.value);
                else setNpsnInput(event.target.value);
                setOpen(true);
                setActiveIndex(-1);
              }}
              onKeyDown={(event) => {
                if (event.key === "Escape") { setOpen(false); setActiveIndex(-1); }
                if (event.key === "ArrowDown" || event.key === "ArrowUp") {
                  event.preventDefault();
                  setOpen(true);
                  const next = event.key === "ArrowDown" ? Math.min(activeIndex + 1, results.length - 1) : Math.max(activeIndex - 1, 0);
                  setActiveIndex(next);
                  if (results[next]) document.getElementById(`${id}-option-${results[next].id}`)?.scrollIntoView({ block: "nearest" });
                }
                if (event.key === "Enter" && showDropdown) {
                  event.preventDefault();
                  if (results[activeIndex]) pickSchool(results[activeIndex]);
                }
              }}
              placeholder={t(mode === "name" ? "school_picker_search_name_placeholder" : "school_picker_npsn_placeholder")}
              disabled={disabled}
              className="pl-9"
            />
            {showDropdown ? (
              <div className="absolute z-50 mt-1 max-h-72 w-full overflow-y-auto rounded-md border border-line bg-surface p-1 shadow-lg">
                <div id={`${id}-results`} role="listbox" aria-label={t("school_picker_search_results")}>
                  {results.map((school, index) => (
                    <div
                      key={school.id}
                      id={`${id}-option-${school.id}`}
                      role="option"
                      aria-selected={index === activeIndex}
                      className={`cursor-pointer rounded-md px-3 py-2 text-sm hover:bg-surface-2 ${index === activeIndex ? "bg-surface-2" : ""}`}
                      onMouseDown={(event) => event.preventDefault()}
                      onClick={() => pickSchool(school)}
                    >
                      <span className="block font-medium text-ink-900">{school.name}</span>
                      <span className="block text-xs text-ink-500">{[school.npsn, school.school_types?.join(" / "), school.kota_name, school.provinsi_name].filter(Boolean).join(" · ")}</span>
                    </div>
                  ))}
                </div>
                {search.isError && enabled ? (
                  <div role="alert" className="px-3 py-2 text-xs text-ink-700">
                    {t("school_picker_search_error")}
                    <Button type="button" variant="ghost" size="sm" onClick={() => search.refetch()}>{t("school_picker_retry")}</Button>
                  </div>
                ) : search.isFetching || (mode === "name" && pendingName) ? (
                  <div role="status" className="flex items-center gap-2 px-3 py-2 text-xs text-ink-500"><Loader2 className="size-3 animate-spin" />{t("school_picker_searching")}</div>
                ) : !validSearch ? (
                  <div className="px-3 py-2 text-xs text-ink-500">{t(mode === "name" ? "school_picker_search_hint" : "school_picker_npsn_placeholder")}</div>
                ) : results.length === 0 ? (
                  <div role="status" className="px-3 py-2 text-xs text-ink-500">{t("school_picker_no_results")}</div>
                ) : null}
                {enabled && search.hasNextPage ? (
                  <Button type="button" variant="ghost" size="sm" className="w-full" disabled={search.isFetching} onClick={() => search.fetchNextPage()}>{t("school_picker_load_more")}</Button>
                ) : null}
              </div>
            ) : null}
          </div>
          )}
          {!selected && mode === "name" ? <p className="mt-1 text-xs text-ink-500">{t("school_picker_optional_filters")}</p> : null}
        </>
      )}
    </div>
  );
}
