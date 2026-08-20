"use client";

import { useEffect, useMemo, useRef, useState } from "react";
import { Search, Check, Loader2 } from "lucide-react";
import { useTranslation } from "@/lib/i18n";
import { Input } from "@/components/ui/input";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { useAdminStudents } from "@/lib/hooks/admin-students";
import { useSchoolOptions } from "@/lib/hooks/admin-schools";
import { useSearchStudentsAcrossSchools } from "@/lib/hooks/admin-exam-grants";
import { cn } from "@/lib/utils";
import type { AdminStudent } from "@/lib/types";
import { JENJANG_OPTIONS } from "@/lib/jenjang";

const ALL_FILTER_VALUE = "__all__";

// ── Debounce helper ───────────────────────────────────────────────────────

function useDebouncedValue(value: string, delay: number): string {
  const [debounced, setDebounced] = useState(value);
  useEffect(() => {
    const id = setTimeout(() => setDebounced(value), delay);
    return () => clearTimeout(id);
  }, [value, delay]);
  return debounced;
}

// ── ParticipantPicker ─────────────────────────────────────────────────────

export interface ParticipantPickerProps {
  examId: string;
  schoolId?: string;
  selected: string[];
  onChange: (ids: string[]) => void;
}

export function ParticipantPicker({
  examId,
  schoolId,
  selected,
  onChange,
}: ParticipantPickerProps) {
  const { t } = useTranslation();
  const [search, setSearch] = useState("");
  const [jenjangFilter, setJenjangFilter] = useState("");
  const [gradeFilter, setGradeFilter] = useState("");
  const [schoolFilter, setSchoolFilter] = useState("");

  const debouncedSearch = useDebouncedValue(search, 300);

  // Track selected IDs across search/filter changes via a ref
  const selectedRef = useRef(selected);
  selectedRef.current = selected;

  const filterKey = [examId, schoolId, debouncedSearch, jenjangFilter, gradeFilter, schoolFilter].join("|");
  const [page, setPage] = useState<{ key: string; cursor?: string }>({ key: filterKey });
  const cursor = page.key === filterKey ? page.cursor : undefined;
  const [accumulated, setAccumulated] = useState<{ key: string; data: AdminStudent[] }>({
    key: filterKey,
    data: [],
  });

  const queryOpts = {
    q: debouncedSearch || undefined,
    jenjang: jenjangFilter || undefined,
    grade: gradeFilter || undefined,
    schoolId: schoolFilter || undefined,
    examId,
    cursor,
    limit: 20,
    enabled: !schoolId,
  };

  const scopedQuery = useAdminStudents({
    q: debouncedSearch || undefined,
    schoolId,
    examId,
    jenjang: jenjangFilter || undefined,
    grade: gradeFilter || undefined,
    cursor,
    limit: 20,
    enabled: Boolean(schoolId),
  });
  const crossQuery = useSearchStudentsAcrossSchools(queryOpts);
  const activeQuery = schoolId ? scopedQuery : crossQuery;
  const students = accumulated.key === filterKey ? accumulated.data : [];
  const nextCursor = activeQuery.data?.next_cursor;
  const { isLoading, isError } = activeQuery;

  useEffect(() => {
    if (!activeQuery.data) return;
    setAccumulated((current) => {
      const existing = current.key === filterKey && cursor ? current.data : [];
      const rows = [...existing, ...activeQuery.data.data];
      return {
        key: filterKey,
        data: [...new Map(rows.map((student) => [student.id, student])).values()],
      };
    });
  }, [activeQuery.data, cursor, filterKey]);

  // Unique jenjang/grade values from fetched students
  const facetedJenjang = useMemo(() => {
    const set = new Set([
      ...students.map((s) => s.jenjang).filter(Boolean),
      ...JENJANG_OPTIONS,
    ]);
    return [...set].sort();
  }, [students]);

  const facetedGrade = useMemo(() => {
    const set = new Set(
      students
        .map((s) => (s.grade != null ? String(s.grade) : ""))
        .filter(Boolean),
    );
    return [...set].sort((a, b) => Number(a) - Number(b));
  }, [students]);

  function toggle(id: string) {
    const next = selectedRef.current.includes(id)
      ? selectedRef.current.filter((sid) => sid !== id)
      : [...selectedRef.current, id];
    onChange(next);
  }

  function selectAll() {
    const allIds = students.map((s) => s.id);
    const current = selectedRef.current;
    const merged = [...new Set([...current, ...allIds])];
    onChange(merged);
  }

  function deselectAll() {
    const allIds = new Set(students.map((s) => s.id));
    const remaining = selectedRef.current.filter(
      (id) => !allIds.has(id),
    );
    onChange(remaining);
  }

  const selectableCount = selected.filter((id) =>
    students.some((s) => s.id === id),
  ).length;

  return (
    <div className="space-y-4">
      {/* Search + Filter row */}
      <div className="flex flex-wrap items-center gap-3">
        <div className="relative min-w-[200px] flex-1">
          <Search className="pointer-events-none absolute left-3 top-1/2 size-4 -translate-y-1/2 text-ink-400" />
          <Input
            value={search}
            onChange={(e) => setSearch(e.target.value)}
            placeholder={t("students_search_placeholder")}
            className="h-9 pl-9 text-xs"
          />
        </div>

        <Select
          value={jenjangFilter || ALL_FILTER_VALUE}
          onValueChange={(value) => setJenjangFilter(value === ALL_FILTER_VALUE ? "" : value)}
        >
          <SelectTrigger className="h-9 w-[130px] text-xs">
            <SelectValue>
              {jenjangFilter || t("students_field_jenjang")}
            </SelectValue>
          </SelectTrigger>
          <SelectContent>
            <SelectItem value={ALL_FILTER_VALUE}>
              <span className="text-ink-500">{t("students_field_jenjang")}</span>
            </SelectItem>
            {facetedJenjang.map((j) => (
              <SelectItem key={j} value={j}>
                {j}
              </SelectItem>
            ))}
          </SelectContent>
        </Select>

        <Select
          value={gradeFilter || ALL_FILTER_VALUE}
          onValueChange={(value) => setGradeFilter(value === ALL_FILTER_VALUE ? "" : value)}
        >
          <SelectTrigger className="h-9 w-[110px] text-xs">
            <SelectValue>
              {gradeFilter || t("students_field_grade")}
            </SelectValue>
          </SelectTrigger>
          <SelectContent>
            <SelectItem value={ALL_FILTER_VALUE}>
              <span className="text-ink-500">{t("students_field_grade")}</span>
            </SelectItem>
            {facetedGrade.map((g) => (
              <SelectItem key={g} value={g}>
                {t("school")} {g}
              </SelectItem>
            ))}
          </SelectContent>
        </Select>

        {/* School facet (only when schoolId absent) */}
        {!schoolId && (
          <SchoolFacetSelect value={schoolFilter} onValueChange={setSchoolFilter} />
        )}
      </div>

      {/* Selection count + bulk actions */}
      <div className="flex items-center justify-between text-xs text-ink-500">
        <span>
          {t("participant_picker_selected_count")
            .replace("{selected}", String(selectableCount))
            .replace("{total}", String(students.length))}
        </span>
        <div className="flex gap-2">
          <button
            onClick={selectAll}
            className="font-medium text-brand-600 hover:underline"
          >
            {t("participant_picker_select_all")}
          </button>
          <button
            onClick={deselectAll}
            className="font-medium text-ink-500 hover:underline"
          >
            {t("participant_picker_deselect_all")}
          </button>
        </div>
      </div>

      {/* Student list */}
      <div className="max-h-[320px] space-y-1 overflow-y-auto rounded-lg border border-line p-2">
        {isLoading && (
          <div className="flex items-center justify-center py-8">
            <Loader2 className="size-5 animate-spin text-ink-400" />
          </div>
        )}
        {isError && !isLoading && (
          <div className="py-8 text-center text-sm text-danger">
            {t("sys_error_load")}
          </div>
        )}
        {!isLoading && !isError && students.length === 0 && (
          <div className="py-8 text-center text-sm text-ink-500">
            {t("students_empty")}
          </div>
        )}
        {students.map((s) => {
          const isSelected = selected.includes(s.id);
          const schoolLabel = s.school_name
            ? s.school_name
            : s.unlisted_school_name
              ? `${t("students_school_unlisted_tag")}: ${s.unlisted_school_name}`
              : "";
          return (
            <button
              key={s.id}
              type="button"
              role="checkbox"
              aria-checked={isSelected}
              aria-label={s.name}
              onClick={() => toggle(s.id)}
              className={cn(
                "flex w-full items-center gap-3 rounded-md border border-transparent px-3 py-2 text-left text-sm transition-colors hover:bg-surface-2",
                isSelected && "border-brand-600 bg-brand-50",
              )}
            >
              <div
                className={cn(
                  "flex size-5 shrink-0 items-center justify-center rounded border",
                  isSelected
                    ? "border-brand-600 bg-brand-600 text-white"
                    : "border-line",
                )}
              >
                {isSelected && <Check className="size-3.5" />}
              </div>
              <div className="min-w-0 flex-1">
                <div className="truncate font-medium text-ink-900">
                  {s.name}
                </div>
                <div className="truncate text-[11px] text-ink-500">
                  @{s.username}
                  {s.grade ? ` · ${t("school")} ${s.grade}` : ""}
                  {s.jenjang ? ` · ${s.jenjang}` : ""}
                  {schoolLabel ? ` · ${schoolLabel}` : ""}
                </div>
              </div>
            </button>
          );
        })}
        {nextCursor && !isLoading && (
          <div className="flex justify-center pt-2">
            <Button
              type="button"
              size="sm"
              variant="outline"
              onClick={() => setPage({ key: filterKey, cursor: nextCursor })}
            >
              {t("sys_load_more")}
            </Button>
          </div>
        )}
      </div>
    </div>
  );
}

// ── School facet (placeholder for cross-school mode) ──────────────────────

function SchoolFacetSelect({
  value,
  onValueChange,
}: {
  value: string;
  onValueChange: (v: string) => void;
}) {
  const { t } = useTranslation();
  const { data: schoolsData } = useSchoolOptions();
  const schools = useMemo(() => schoolsData?.data ?? [], [schoolsData]);

  return (
    <Select
      value={value || ALL_FILTER_VALUE}
      onValueChange={(next) => onValueChange(next === ALL_FILTER_VALUE ? "" : next)}
    >
      <SelectTrigger className="h-9 w-[180px] text-xs">
        <SelectValue placeholder={t("select_school")} />
      </SelectTrigger>
      <SelectContent>
        <SelectItem value={ALL_FILTER_VALUE}>
          <span className="text-ink-500">{t("select_school")}</span>
        </SelectItem>
        <SelectItem value="none">{t("students_school_facet_none")}</SelectItem>
        {schools.map((s) => (
          <SelectItem key={s.id} value={s.id}>
            {s.name}
          </SelectItem>
        ))}
      </SelectContent>
    </Select>
  );
}
