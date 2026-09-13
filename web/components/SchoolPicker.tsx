"use client";

import { useEffect, useMemo, useState } from "react";
import { Loader2, Search } from "lucide-react";
import { useProvinces } from "@/lib/hooks/regions";
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
const ALL_CATEGORY = "_all_";

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
  const [category, setCategory] = useState("");
  const [qInput, setQInput] = useState("");
  const [q, setQ] = useState("");
  const [npsnInput, setNpsnInput] = useState("");
  const [cursor, setCursor] = useState("");
  const [unlistedActive, setUnlistedActive] = useState(Boolean(unlistedName));

  const { data: provinces } = useProvinces();
  const hydrate = useSchoolById(value);
  const selected = selectedSchool ?? hydrate.data ?? null;

  useEffect(() => {
    if (unlistedName) setUnlistedActive(true);
  }, [unlistedName]);

  useEffect(() => {
    const timer = setTimeout(() => setQ(qInput.trim()), 300);
    return () => clearTimeout(timer);
  }, [qInput]);

  useEffect(() => {
    setCursor("");
  }, [mode, provinceId, category, q, npsnInput]);

  const normalizedNPSN = npsnInput.trim().toUpperCase();
  const params = useMemo(() => {
    if (mode === "npsn") {
      return { npsn: normalizedNPSN, limit: 20 };
    }
    return {
      q,
      province_id: provinceId,
      ...(category ? { category } : {}),
      ...(cursor ? { cursor } : {}),
      limit: 20,
    };
  }, [category, cursor, mode, normalizedNPSN, provinceId, q]);

  const enabled =
    !disabled &&
    (mode === "npsn"
      ? /^[A-Z0-9]{8}$/.test(normalizedNPSN)
      : Boolean(provinceId && q.length >= 3));
  const search = useSchoolSearch(params, enabled);
  const results = search.data?.data ?? [];

  return (
    <div className={className}>
      <div className="mb-2 flex gap-2">
        <Button
          type="button"
          variant={mode === "name" ? "default" : "outline"}
          size="sm"
          onClick={() => setMode("name")}
          disabled={disabled}
        >
          Nama
        </Button>
        <Button
          type="button"
          variant={mode === "npsn" ? "default" : "outline"}
          size="sm"
          onClick={() => setMode("npsn")}
          disabled={disabled}
        >
          NPSN
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
            Sekolah tidak ditemukan
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
          placeholder="Tulis nama sekolah"
          disabled={disabled}
        />
      ) : (
        <>
          {mode === "name" ? (
            <div className="grid gap-2 sm:grid-cols-[1fr_120px]">
              <Select value={provinceId || "_empty_"} onValueChange={(v) => setProvinceId(v === "_empty_" ? "" : v)} disabled={disabled}>
                <SelectTrigger id={`${id}-province`}>
                  <SelectValue placeholder="Provinsi" />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="_empty_">Provinsi</SelectItem>
                  {(provinces ?? []).map((p) => (
                    <SelectItem key={p.id} value={p.id}>
                      {p.name}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
              <Select value={category || ALL_CATEGORY} onValueChange={(v) => setCategory(v === ALL_CATEGORY ? "" : v)} disabled={disabled}>
                <SelectTrigger id={`${id}-category`}>
                  <SelectValue placeholder="Kategori" />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value={ALL_CATEGORY}>Semua</SelectItem>
                  {CATEGORIES.map((c) => (
                    <SelectItem key={c} value={c}>
                      {c}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
              <div className="relative sm:col-span-2">
                <Search className="pointer-events-none absolute top-1/2 left-3 size-4 -translate-y-1/2 text-ink-400" />
                <Input
                  id={id}
                  value={qInput}
                  onChange={(e) => setQInput(e.target.value)}
                  placeholder="Cari nama sekolah"
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
              placeholder="Masukkan NPSN"
              disabled={disabled}
            />
          )}

          {selected ? (
            <div className="mt-2 rounded-md border border-line bg-surface-2 px-3 py-2 text-xs text-ink-700">
              <div className="font-medium text-ink-900">{selected.name}</div>
              <div>{[selected.npsn, selected.category, selected.city_name, selected.province_name].filter(Boolean).join(" · ")}</div>
            </div>
          ) : null}

          <div className="mt-2 space-y-1">
            {search.isFetching ? (
              <div className="flex items-center gap-2 text-xs text-ink-500">
                <Loader2 className="size-3 animate-spin" />
                Mencari sekolah…
              </div>
            ) : enabled && results.length === 0 ? (
              <div className="text-xs text-ink-500">Sekolah tidak ditemukan.</div>
            ) : null}
            {results.map((school) => (
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
                  {[school.npsn, school.category, school.city_name, school.province_name].filter(Boolean).join(" · ")}
                </span>
              </button>
            ))}
            {search.data?.next_cursor ? (
              <Button
                type="button"
                variant="outline"
                size="sm"
                onClick={() => setCursor(search.data?.next_cursor ?? "")}
                disabled={disabled || search.isFetching}
              >
                Halaman berikutnya
              </Button>
            ) : null}
          </div>
        </>
      )}
    </div>
  );
}
