"use client";

import { useEffect, useRef, useState } from "react";
import { AlertTriangle, Loader2 } from "lucide-react";
import { toast } from "sonner";
import { useTranslation } from "@/lib/i18n";
import { useUpdateProfile } from "@/lib/hooks/students";
import { missingProfileFields, type MissingProfileField } from "@/lib/profile-completeness";
import { GRADE_GROUPS } from "@/lib/grade";
import { SchoolPicker } from "@/components/SchoolPicker";
import { Dialog, DialogContent } from "@/components/ui/dialog";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Select, SelectContent, SelectGroup, SelectItem, SelectLabel, SelectTrigger, SelectValue } from "@/components/ui/select";
import type { User } from "@/lib/types";

export interface CompleteProfileModalProps {
  open: boolean;
  profile: User | undefined;
}

const WARN_FIELD_CLASS = "border-warn/40 bg-warn-bg/40 focus-visible:border-warn";

export function CompleteProfileModal({ open, profile }: CompleteProfileModalProps) {
  const { t } = useTranslation();
  const updateProfile = useUpdateProfile();

  const [name, setName] = useState("");
  const [phone, setPhone] = useState("");
  const [email, setEmail] = useState("");
  const [dob, setDob] = useState("");
  const [schoolId, setSchoolId] = useState("");
  const [unlistedSchoolName, setUnlistedSchoolName] = useState("");
  const [grade, setGrade] = useState("");

  // Seed once per profile version, same pattern as the /profile page — an
  // edit in progress must not be clobbered by a background refetch.
  const syncedVersion = useRef<string | null>(null);
  useEffect(() => {
    if (!profile) return;
    const version = `${profile.id}:${profile.updated_at ?? ""}`;
    if (syncedVersion.current === version) return;
    syncedVersion.current = version;
    setName(profile.name ?? "");
    setPhone(profile.phone ?? "");
    setEmail(profile.email ?? "");
    setDob(profile.dob ? profile.dob.slice(0, 10) : "");
    setSchoolId(profile.school_id ?? "");
    setUnlistedSchoolName(profile.unlisted_school_name ?? "");
    setGrade(profile.grade != null ? String(profile.grade) : "");
  }, [profile]);

  const missing = missingProfileFields(profile);
  const isMissing = (field: MissingProfileField) => missing.includes(field);
  // Email is only ever editable here because it was never set — once saved it
  // locks, same rule as the /profile page (no backend change needed: that page
  // only ever renders it read-only, it never enforces the lock server-side).
  const emailLocked = Boolean(profile?.email?.trim());

  const canSubmit =
    name.trim() !== "" &&
    (emailLocked || email.trim() !== "") &&
    phone.trim() !== "" &&
    dob !== "" &&
    grade !== "" &&
    (schoolId !== "" || unlistedSchoolName.trim() !== "");

  const handleSubmit = () => {
    updateProfile.mutate(
      {
        name: name.trim(),
        phone: phone.trim(),
        ...(emailLocked ? {} : { email: email.trim() }),
        dob,
        grade: grade ? parseInt(grade, 10) : undefined,
        school_id: schoolId || "",
        unlisted_school_name: schoolId ? "" : unlistedSchoolName.trim(),
      },
      { onError: () => toast.error(t("cart_complete_profile_error")) }
    );
  };

  return (
    <Dialog open={open}>
      <DialogContent
        showCloseButton={false}
        onEscapeKeyDown={(e) => e.preventDefault()}
        onPointerDownOutside={(e) => e.preventDefault()}
        onInteractOutside={(e) => e.preventDefault()}
        className="max-w-xl"
      >
        <div className="flex items-start gap-3">
          <div className="flex size-10 shrink-0 items-center justify-center rounded-xl bg-warn-bg text-warn">
            <AlertTriangle className="size-5" aria-hidden="true" />
          </div>
          <div>
            <h2 className="font-serif text-lg font-bold text-ink-900">{t("cart_complete_profile_title")}</h2>
            <p className="mt-0.5 text-xs text-ink-500">{t("cart_complete_profile_subtitle")}</p>
          </div>
        </div>

        <div className="mt-2 flex items-start gap-2 rounded-lg border border-warn/30 bg-warn-bg px-3 py-2.5">
          <AlertTriangle className="mt-0.5 size-4 shrink-0 text-warn" aria-hidden="true" />
          <p className="text-xs font-semibold text-warn">{t("cart_complete_profile_warning")}</p>
        </div>

        <div className="mt-4 grid gap-3.5 sm:grid-cols-2">
          <div className="flex flex-col gap-1.5">
            <Label htmlFor="cpm-name" className="text-xs font-semibold text-ink-600">
              {t("full_name")}
            </Label>
            <Input
              id="cpm-name"
              value={name}
              onChange={(e) => setName(e.target.value)}
              className={isMissing("name") ? WARN_FIELD_CLASS : undefined}
            />
          </div>

          <div className="flex flex-col gap-1.5">
            <Label htmlFor="cpm-phone" className="text-xs font-semibold text-ink-600">
              {t("phone")}
            </Label>
            <Input
              id="cpm-phone"
              value={phone}
              onChange={(e) => setPhone(e.target.value)}
              className={isMissing("phone") ? WARN_FIELD_CLASS : undefined}
            />
          </div>

          <div className="sm:col-span-2">
            <Label htmlFor="cpm-email" className="text-xs font-semibold text-ink-600">
              {t("email")}
            </Label>
            <Input
              id="cpm-email"
              type="email"
              value={email}
              onChange={(e) => setEmail(e.target.value)}
              readOnly={emailLocked}
              disabled={emailLocked}
              className={!emailLocked && isMissing("email") ? WARN_FIELD_CLASS : undefined}
            />
            <p className="mt-1 text-[11px] text-ink-400">
              {emailLocked ? t("email_locked") : t("cart_complete_profile_email_hint")}
            </p>
          </div>

          <div className="flex flex-col gap-1.5">
            <Label htmlFor="cpm-dob" className="text-xs font-semibold text-ink-600">
              {t("students_field_dob")}
            </Label>
            <Input
              id="cpm-dob"
              type="date"
              value={dob}
              onChange={(e) => setDob(e.target.value)}
              className={isMissing("dob") ? WARN_FIELD_CLASS : undefined}
            />
          </div>

          <div className="flex flex-col gap-1.5">
            <Label htmlFor="cpm-grade" className="text-xs font-semibold text-ink-600">
              {t("grade")}
            </Label>
            <Select value={grade || "_empty_"} onValueChange={(v) => setGrade(v === "_empty_" ? "" : v)}>
              <SelectTrigger id="cpm-grade" className={isMissing("grade") ? WARN_FIELD_CLASS : undefined}>
                <SelectValue placeholder={t("select_grade")} />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="_empty_">{t("select_grade")}</SelectItem>
                {GRADE_GROUPS.map((group) => (
                  <SelectGroup key={group.label}>
                    <SelectLabel>{group.label}</SelectLabel>
                    {group.values.map((v) => (
                      <SelectItem key={v} value={String(v)}>
                        {t("grade")} {v}
                      </SelectItem>
                    ))}
                  </SelectGroup>
                ))}
              </SelectContent>
            </Select>
          </div>

          <div className="sm:col-span-2">
            <Label htmlFor="cpm-school" className="text-xs font-semibold text-ink-600">
              {t("school")}
            </Label>
            <div className={isMissing("school") ? `rounded-lg p-1.5 ${WARN_FIELD_CLASS}` : undefined}>
              <SchoolPicker
                id="cpm-school"
                value={schoolId}
                onChange={(school) => setSchoolId(school?.id ?? "")}
                allowUnlisted
                unlistedName={unlistedSchoolName}
                onUnlistedNameChange={setUnlistedSchoolName}
              />
            </div>
          </div>
        </div>

        <p className="mt-2.5 text-[11px] text-warn">{t("cart_complete_profile_required_note")}</p>

        <Button
          type="button"
          onClick={handleSubmit}
          disabled={!canSubmit || updateProfile.isPending}
          className="mt-3 w-full rounded-full bg-warn text-white hover:bg-warn/90"
        >
          {updateProfile.isPending && <Loader2 className="size-4 animate-spin" aria-hidden="true" />}
          {t("cart_complete_profile_submit")}
        </Button>
      </DialogContent>
    </Dialog>
  );
}
