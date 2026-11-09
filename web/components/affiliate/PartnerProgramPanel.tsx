"use client";

import { useEffect, useState } from "react";
import { X } from "lucide-react";
import { toast } from "sonner";
import { SchoolPicker } from "@/components/SchoolPicker";
import { ProgramBadges, inAffiliate, inPartner } from "@/components/affiliate/ProgramBadges";
import { Button } from "@/components/ui/button";
import { ApiError } from "@/lib/api";
import { useAddAffiliatorProgram, useSetPartnerSchools } from "@/lib/hooks/admin-affiliates";
import { useSchoolById } from "@/lib/hooks/students";
import { useTranslation } from "@/lib/i18n";
import type { AffiliatorPrograms, SchoolOption } from "@/lib/types";

function SchoolChip({ id, onRemove, disabled }: { id: string; onRemove: () => void; disabled: boolean }) {
  const school = useSchoolById(id);
  return (
    <li className="flex items-center gap-1 rounded-full bg-surface-2 px-2.5 py-1 text-xs text-ink-800">
      {school.data?.name ?? id.slice(0, 8)}
      <button type="button" aria-label={`remove ${school.data?.name ?? id}`} onClick={onRemove} disabled={disabled}>
        <X className="size-3" />
      </button>
    </li>
  );
}

// PartnerProgramPanel is the admin's view of an account's programs: which it
// belongs to, adding the other one (programs are never removed, suspending the
// account is how access stops), and the schools a Mitra may buy for.
export function PartnerProgramPanel({ id, programs, status }: { id: string; programs?: AffiliatorPrograms; status: string }) {
  const { t } = useTranslation();
  const add = useAddAffiliatorProgram();
  const setSchools = useSetPartnerSchools();
  const [schoolIds, setSchoolIds] = useState<string[]>(programs?.partner?.school_ids ?? []);
  const approved = status !== "pending_approval" && status !== "rejected";
  const fail = (err: unknown) => toast.error(err instanceof ApiError ? err.message : t("aff_action_failed"));

  useEffect(() => {
    setSchoolIds(programs?.partner?.school_ids ?? []);
  }, [programs]);

  const addSchool = (school: SchoolOption | null) => {
    if (school && !schoolIds.includes(school.id)) setSchoolIds([...schoolIds, school.id]);
  };
  const saveSchools = () =>
    setSchools.mutate({ id, school_ids: schoolIds }, { onSuccess: () => toast.success(t("admin_aff_schools_saved")), onError: fail });
  const addProgram = (program: "affiliate" | "partner") =>
    add.mutate({ id, program }, { onSuccess: () => toast.success(t("admin_aff_programs_added")), onError: fail });

  return (
    <section className="space-y-3">
      <div className="flex flex-wrap items-center gap-3">
        <h2 className="font-semibold text-ink-900">{t("admin_aff_col_programs")}</h2>
        <ProgramBadges programs={programs} />
        {approved && !inAffiliate(programs) && (
          <Button size="sm" variant="outline" onClick={() => addProgram("affiliate")} disabled={add.isPending}>
            {t("admin_aff_add_program_affiliate")}
          </Button>
        )}
        {approved && !inPartner(programs) && (
          <Button size="sm" variant="outline" onClick={() => addProgram("partner")} disabled={add.isPending}>
            {t("admin_aff_add_program_partner")}
          </Button>
        )}
      </div>

      {inPartner(programs) && (
        <div className="md-card-outlined space-y-3 p-4">
          <div>
            <h3 className="text-sm font-semibold text-ink-900">{t("admin_aff_schools_title")}</h3>
            <p className="text-xs text-ink-500">{t("admin_aff_schools_desc")}</p>
          </div>
          {schoolIds.length === 0 ? (
            <p className="text-sm text-ink-600">{t("admin_aff_schools_empty")}</p>
          ) : (
            <ul className="flex flex-wrap gap-1.5">
              {schoolIds.map((schoolId) => (
                <SchoolChip key={schoolId} id={schoolId} disabled={setSchools.isPending} onRemove={() => setSchoolIds(schoolIds.filter((s) => s !== schoolId))} />
              ))}
            </ul>
          )}
          <SchoolPicker id="partner-schools-picker" onChange={addSchool} />
          <div className="flex justify-end">
            <Button size="sm" onClick={saveSchools} disabled={setSchools.isPending}>
              {t("admin_aff_schools_save")}
            </Button>
          </div>
        </div>
      )}
    </section>
  );
}
