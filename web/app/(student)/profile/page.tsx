"use client";

import { useEffect, useMemo, useRef, useState } from "react";
import { ImageIcon, Loader2, Lock, Pencil } from "lucide-react";
import { cn } from "@/lib/utils";
import {
  useChangePassword,
  usePresignUpload,
  useProfile,
  useSchoolById,
  useUpdatePhoto,
  useUpdateProfile,
} from "@/lib/hooks/students";
import {
  useCitiesByProvince,
  useDistrictsByCity,
  useProvinces,
} from "@/lib/hooks/regions";
import { fileUrl } from "@/lib/api";
import { useTranslation } from "@/lib/i18n";
import { JENJANG_OPTIONS } from "@/lib/jenjang";
import { useAuthStore } from "@/stores/auth";
import { AbakAvatar } from "@/components/ui/abak-avatar";
import { AvatarPicker } from "@/components/profile/AvatarPicker";
import { Button } from "@/components/ui/button";
import { Card } from "@/components/ui/card";
import {
  Dialog,
  DialogContent,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { PasswordInput } from "@/components/ui/password-input";
import {
  Select,
  SelectContent,
  SelectGroup,
  SelectItem,
  SelectLabel,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { Switch } from "@/components/ui/switch";
import { Skeleton } from "@/components/ui/skeleton";
import { PageHead } from "@/components/shell/PageHead";
import { SchoolPicker } from "@/components/SchoolPicker";
import type { SchoolOption } from "@/lib/types";
import { formatGrade, GRADE_GROUPS, gradeOptionsForJenjang } from "@/lib/grade";
import { toast } from "sonner";

// Read mode keeps the prototype .ab-input look: fields are locked, not greyed.
const PROFILE_INPUT_CLASS =
  "h-[var(--spacing-input)] w-full rounded-[14px] border-[1.5px] border-line bg-surface px-[15px] py-0 text-[15px] text-ink-900 shadow-none transition-[border-color,box-shadow] outline-none placeholder:text-ink-600 hover:border-line-strong focus-visible:border-brand-600 focus-visible:ring-4 focus-visible:ring-brand-50 disabled:cursor-default disabled:opacity-100";
const FIELD_CLASS = "flex min-w-0 flex-col gap-[7px]";
const LABEL_CLASS = "type-label-sm font-bold text-ink-900";

function formatJoined(iso?: string, lang: "id" | "en" = "id"): string {
  if (!iso) return "—";
  try {
    const d = new Date(iso);
    return d.toLocaleDateString(lang === "id" ? "id-ID" : "en-US", {
      month: "long",
      year: "numeric",
    });
  } catch {
    return iso;
  }
}

function Field({
  id,
  label,
  value,
  onChange,
  locked,
  hint,
  isLoading,
  type = "text",
}: {
  id: string;
  label: string;
  value?: string;
  onChange?: (v: string) => void;
  locked?: boolean;
  hint?: string;
  isLoading?: boolean;
  type?: string;
}) {
  return (
    <div className={FIELD_CLASS}>
      <Label htmlFor={id} className={LABEL_CLASS}>
        {label}
      </Label>
      {isLoading ? (
        <Skeleton className="h-[var(--spacing-input)] w-full rounded-[14px]" />
      ) : (
        <Input
          id={id}
          type={type}
          value={value ?? ""}
          placeholder="—"
          disabled={locked || !onChange}
          readOnly={locked || !onChange}
          onChange={onChange ? (e) => onChange(e.target.value) : undefined}
          className={PROFILE_INPUT_CLASS}
        />
      )}
      {hint && <div className="text-[12.5px] text-ink-600">{hint}</div>}
    </div>
  );
}

function ChangePasswordDialog({
  open,
  onOpenChange,
}: {
  open: boolean;
  onOpenChange: (v: boolean) => void;
}) {
  const { t } = useTranslation();
  const change = useChangePassword();
  const [current, setCurrent] = useState("");
  const [next, setNext] = useState("");
  const [confirm, setConfirm] = useState("");

  useEffect(() => {
    if (!open) {
      setCurrent("");
      setNext("");
      setConfirm("");
    }
  }, [open]);

  function submit(e: React.FormEvent) {
    e.preventDefault();
    if (next.length < 8) {
      toast.error(t("weak_password"));
      return;
    }
    if (next !== confirm) {
      toast.error(t("passwords_do_not_match"));
      return;
    }
    change.mutate(
      { current_password: current, new_password: next },
      {
        onSuccess: () => {
          toast.success(t("password_changed"));
          onOpenChange(false);
        },
        onError: (err) => {
          toast.error(
            err instanceof Error ? err.message : t("change_password_failed")
          );
        },
      }
    );
  }

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="sm:max-w-md">
        <DialogHeader>
          <DialogTitle>{t("change_password")}</DialogTitle>
        </DialogHeader>
        <form onSubmit={submit} className="flex flex-col gap-4 py-2">
          <div className="flex flex-col gap-1.5">
            <Label htmlFor="current-password">{t("current_password")}</Label>
            <PasswordInput
              id="current-password"
              value={current}
              onChange={(e) => setCurrent(e.target.value)}
            />
          </div>
          <div className="flex flex-col gap-1.5">
            <Label htmlFor="new-password">{t("new_password")}</Label>
            <PasswordInput
              id="new-password"
              value={next}
              onChange={(e) => setNext(e.target.value)}
            />
          </div>
          <div className="flex flex-col gap-1.5">
            <Label htmlFor="confirm-password">{t("confirm_password")}</Label>
            <PasswordInput
              id="confirm-password"
              value={confirm}
              onChange={(e) => setConfirm(e.target.value)}
            />
          </div>
          <DialogFooter>
            <Button
              type="button"
              variant="outline"
              onClick={() => onOpenChange(false)}
            >
              {t("cancel")}
            </Button>
            <Button type="submit" disabled={change.isPending}>
              {change.isPending ? (
                <Loader2 className="mr-2 size-4 animate-spin" />
              ) : null}
              {t("update")}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  );
}

export default function ProfilePage() {
  const { t, lang } = useTranslation();
  const { data: profile, isLoading, isError, error, refetch } = useProfile();
  const updateProfile = useUpdateProfile();
  const presign = usePresignUpload();
  const updatePhoto = useUpdatePhoto();
  const token = useAuthStore((s) => s.token);
  const setSession = useAuthStore((s) => s.setSession);
  const fileInputRef = useRef<HTMLInputElement>(null);

  const [editMode, setEditMode] = useState(false);
  const [name, setName] = useState("");
  const [phone, setPhone] = useState("");
  const [address, setAddress] = useState("");
  const [targetExam, setTargetExam] = useState("");
  const [grade, setGrade] = useState<string>("");
  const [dob, setDob] = useState("");
  const [schoolId, setSchoolId] = useState<string>("");
  const [unlistedSchoolName, setUnlistedSchoolName] = useState("");
  const [selectedSchool, setSelectedSchool] = useState<SchoolOption | null>(null);
  const [jenjang, setJenjang] = useState("");
  const [provinsiId, setProvinsiId] = useState("");
  const [kotaId, setKotaId] = useState("");
  const [kecamatanId, setKecamatanId] = useState("");
  const [kodePos, setKodePos] = useState("");
  const [emailNotif, setEmailNotif] = useState(true);
  const [waNotif, setWaNotif] = useState(true);
  const [pushNotif, setPushNotif] = useState(false);
  const [passwordOpen, setPasswordOpen] = useState(false);
  const [photoUploading, setPhotoUploading] = useState(false);

  // Region hooks (Task 17). Reused, not reimplemented.
  const { data: provinces } = useProvinces();
  const { data: cities } = useCitiesByProvince(provinsiId);
  const { data: districts } = useDistrictsByCity(kotaId);

  // The displayed school value: when in edit mode, use the user-editable
  // schoolId; when in read-only mode, fall back to the profile's stored
  // school_id or unlisted_school_name. This makes the read-only display
  // correct on the first render without depending on the sync useEffect.
  const displayedSchoolId = editMode ? schoolId : schoolId || profile?.school_id || "";
  const displayedIsUnlisted = !editMode && Boolean(profile?.unlisted_school_name);
  // Same pattern for the cascade of optional biodata fields.
  const displayedJenjang = editMode ? jenjang : jenjang || (profile?.jenjang ?? "");
  const displayedProvinsiId = editMode ? provinsiId : provinsiId || (profile?.provinsi_id ?? "");
  const displayedKotaId = editMode ? kotaId : kotaId || (profile?.kota_id ?? "");
  const displayedKecamatanId = editMode ? kecamatanId : kecamatanId || (profile?.kecamatan_id ?? "");
  const displayedKodePos = editMode ? kodePos : kodePos || (profile?.kode_pos ?? "");
  const { data: hydratedSchool } = useSchoolById(displayedIsUnlisted ? "" : displayedSchoolId);
  const ownSchool = selectedSchool?.id === displayedSchoolId ? selectedSchool : hydratedSchool;
  const jenjangOptions = ownSchool?.school_types?.length
    ? ownSchool.school_types
    : JENJANG_OPTIONS;
  const gradeOptions = gradeOptionsForJenjang(displayedJenjang);
  const gradeInOptions =
    grade === "" || (gradeOptions.length > 0 && gradeOptions.includes(Number(grade)));

  const syncedVersion = useRef<string | null>(null);
  useEffect(() => {
    if (!profile || editMode) return;
    const version = `${profile.id}:${profile.updated_at}`;
    if (syncedVersion.current === version) return;
    syncedVersion.current = version;
    setName(profile.name ?? "");
    setPhone(profile.phone ?? "");
    setAddress(profile.alamat_domisili ?? "");
    setTargetExam(profile.target_exam ?? "");
    setDob(profile.dob ? profile.dob.slice(0, 10) : "");
    setGrade(profile.grade != null ? String(profile.grade) : "");
    setSchoolId(profile.school_id ?? "");
    setSelectedSchool(null);
    setUnlistedSchoolName(profile.unlisted_school_name ?? "");
    setJenjang(profile.jenjang ?? "");
    setProvinsiId(profile.provinsi_id ?? "");
    setKotaId(profile.kota_id ?? "");
    setKecamatanId(profile.kecamatan_id ?? "");
    setKodePos(profile.kode_pos ?? "");
    setWaNotif(!!profile.phone);
  }, [profile, editMode]);

  const displayName = profile?.name ?? "";
  const metaLine = useMemo(() => {
    if (!profile) return "";
    const joined = formatJoined(profile.created_at, lang);
    return profile.username
      ? `@${profile.username} · ${t("joined")} ${joined}`
      : `${t("joined")} ${joined}`;
  }, [profile, lang, t]);

  function cancelEdit() {
    if (profile) {
      setName(profile.name ?? "");
      setPhone(profile.phone ?? "");
      setAddress(profile.alamat_domisili ?? "");
      setTargetExam(profile.target_exam ?? "");
      setDob(profile.dob ? profile.dob.slice(0, 10) : "");
      setGrade(profile.grade != null ? String(profile.grade) : "");
      setSchoolId(profile.school_id ?? "");
      setSelectedSchool(null);
      setUnlistedSchoolName(profile.unlisted_school_name ?? "");
      setJenjang(profile.jenjang ?? "");
      setProvinsiId(profile.provinsi_id ?? "");
      setKotaId(profile.kota_id ?? "");
      setKecamatanId(profile.kecamatan_id ?? "");
      setKodePos(profile.kode_pos ?? "");
    }
    setEditMode(false);
  }

  function handleSave(e: React.FormEvent) {
    e.preventDefault();
    if (!editMode) return;
    const gradeNum = grade ? parseInt(grade, 10) : undefined;
    if (gradeNum !== undefined && Number.isNaN(gradeNum)) {
      toast.error("Kelas tidak valid.");
      return;
    }
    // Build the payload incrementally so we can strip blank optional fields
    // before sending -- the backend treats absent vs. empty-string differently
    // for the all-or-nothing address validation (FR-FE-27).
    const payload: Record<string, unknown> = {
      name: name || undefined,
      phone: phone || undefined,
      address: address || undefined,
      target_exam: targetExam || undefined,
      grade: gradeNum,
    };
    const trimmedUnlistedSchoolName = unlistedSchoolName.trim();
    const initialSchoolId = profile?.school_id ?? "";
    const initialUnlistedSchoolName = (profile?.unlisted_school_name ?? "").trim();
    const schoolChanged = schoolId !== initialSchoolId || trimmedUnlistedSchoolName !== initialUnlistedSchoolName;
    if (schoolChanged) {
      if (!schoolId && !trimmedUnlistedSchoolName) {
        toast.error("Sekolah belum dipilih.");
        return;
      }
      payload.school_id = schoolId || "";
      payload.unlisted_school_name = schoolId ? "" : trimmedUnlistedSchoolName;
    }
    if (dob) payload.dob = dob;
    if (jenjang) payload.jenjang = jenjang;
    if (provinsiId) payload.provinsi_id = provinsiId;
    if (kotaId) payload.kota_id = kotaId;
    if (kecamatanId) payload.kecamatan_id = kecamatanId;
    if (kodePos) payload.kode_pos = kodePos;

    updateProfile.mutate(
      payload as Parameters<typeof updateProfile.mutate>[0],
      {
        onSuccess: (updated) => {
          toast.success(t("saved"));
          setEditMode(false);
          if (updated && token) {
            const rt = useAuthStore.getState().refreshToken ?? "";
            setSession(token, rt, updated);
          }
        },
        onError: (err) => {
          toast.error(err instanceof Error ? err.message : t("save_failed"));
        },
      }
    );
  }

  async function handlePhotoSelect(e: React.ChangeEvent<HTMLInputElement>) {
    const file = e.target.files?.[0];
    if (!file) return;
    setPhotoUploading(true);
    try {
      const presigned = await presign.mutateAsync({
        filename: file.name,
        content_type: file.type,
      });
      const uploadRes = await fetch(presigned.url, {
        method: "PUT",
        body: file,
        headers: { "Content-Type": file.type },
      });
      if (!uploadRes.ok) {
        throw new Error(`Upload failed: ${uploadRes.status}`);
      }
      await updatePhoto.mutateAsync(presigned.key);
      toast.success(t("photo_uploaded"));
    } catch (err) {
      toast.error(
        err instanceof Error ? err.message : t("photo_upload_failed")
      );
    } finally {
      setPhotoUploading(false);
      if (fileInputRef.current) {
        fileInputRef.current.value = "";
      }
    }
  }

  if (isError) {
    return (
      <>
        <PageHead title={t("profile_title")} />
        <Card className="gap-0 border-danger/30 bg-danger-bg p-4">
          <div className="flex items-center gap-3">
            <div className="flex-1 text-sm text-ink-700">
              {t("load_failed")}
              {error instanceof Error && error.message
                ? ` ${error.message}`
                : ""}
            </div>
            <Button variant="outline" size="sm" onClick={() => refetch()}>
              {t("retry")}
            </Button>
          </div>
        </Card>
      </>
    );
  }

  const avatarEl = (
    <AbakAvatar
      name={displayName ?? ""}
      index={profile?.avatar_index}
      size="xl"
      ring="0 0 0 4px var(--color-surface), 0 0 0 6px var(--color-brand-50)"
      src={profile?.photo_url ? fileUrl(profile.photo_url) : undefined}
      className={photoUploading ? "animate-pulse" : undefined}
    />
  );
  const meta = [profile?.grade != null ? formatGrade(profile.grade) : "", profile?.unlisted_school_name || hydratedSchool?.name || ""]
    .filter(Boolean)
    .join(" · ");

  return (
    <>
      <PageHead title={t("profile_title")} sub={t("profile_sub")} />

      <div className="grid grid-cols-1 items-start gap-4 lg:grid-cols-[300px_minmax(0,1fr)]">
        <div className="flex min-w-0 flex-col gap-4">
          <Card className="items-center gap-2.5 bg-[radial-gradient(100%_70%_at_50%_0%,var(--color-brand-50),var(--color-surface)_70%)] p-6 text-center">
            {isLoading ? (
              <Skeleton className="size-[104px] rounded-full" />
            ) : (
              <button
                type="button"
                onClick={() => fileInputRef.current?.click()}
                disabled={photoUploading}
                aria-label={t("upload_photo")}
                className="relative cursor-pointer rounded-full border-0 bg-transparent p-0 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-brand-300 disabled:cursor-not-allowed"
              >
                {avatarEl}
                <span className="absolute right-0.5 bottom-1 grid size-8 place-items-center rounded-full bg-ember-600 text-on-ember shadow-[0_0_0_3px_var(--color-surface)]">
                  {photoUploading ? <Loader2 className="size-4 animate-spin" /> : <Pencil className="size-4" />}
                </span>
              </button>
            )}
            <input
              ref={fileInputRef}
              type="file"
              accept="image/*"
              className="hidden"
              onChange={handlePhotoSelect}
            />

            {isLoading ? (
              <>
                <Skeleton className="h-5 w-40" />
                <Skeleton className="h-4 w-48" />
              </>
            ) : (
              <>
                <h2 className="type-h2 text-ink-900 [overflow-wrap:anywhere]">{displayName || t("unnamed")}</h2>
                <p className="mb-2.5 type-body text-ink-600">{meta || metaLine}</p>
              </>
            )}

            {!isLoading && (
              <div className="mt-2 flex w-full flex-col gap-2 border-t border-dashed border-line pt-3.5">
                <Button
                  type="button"
                  variant="outline"
                  size="sm"
                  className="w-full gap-2 border has-[>svg]:px-[13px]"
                  onClick={() => fileInputRef.current?.click()}
                  disabled={photoUploading}
                >
                  <ImageIcon />
                  {photoUploading ? t("saving") : t("profile_change_photo")}
                </Button>
                <AvatarPicker name={displayName ?? ""} current={profile?.avatar_index} />
                <Button type="button" variant="outline" size="sm" className="w-full gap-2 border has-[>svg]:px-[13px]" onClick={() => setPasswordOpen(true)}>
                  <Lock />
                  {t("change_password")}
                </Button>
              </div>
            )}
          </Card>

          <Card className="gap-0 p-6">
            <h2 className="mb-3 type-h2 text-ink-900">{t("notif_prefs")}</h2>
            <div className="flex flex-col text-sm">
              <label className="flex cursor-pointer items-center gap-3 border-b border-line py-3">
                <Switch checked={emailNotif} onCheckedChange={setEmailNotif} aria-label={t("email_notif")} />
                <span className="min-w-0 flex-1">
                  <b className="block text-ink-900">{t("email_notif")}</b>
                  <small className="block truncate text-xs text-ink-600">{profile?.email ?? "—"}</small>
                </span>
              </label>
              <label className="flex cursor-pointer items-center gap-3 border-b border-line py-3">
                <Switch checked={waNotif} onCheckedChange={setWaNotif} aria-label={t("wa_notif")} />
                <span className="min-w-0 flex-1">
                  <b className="block text-ink-900">{t("wa_notif")}</b>
                  <small className="block truncate text-xs text-ink-600">{profile?.phone ?? "—"}</small>
                </span>
              </label>
              <label className="flex cursor-pointer items-center gap-3 border-b border-line py-3">
                <Switch checked={pushNotif} onCheckedChange={setPushNotif} aria-label={t("push_notif")} />
                <b className="min-w-0 flex-1 text-ink-900">{t("push_notif")}</b>
              </label>
            </div>
          </Card>
        </div>

        <form onSubmit={handleSave} className="flex min-w-0 flex-col gap-4" noValidate>
          <Card className="gap-0 p-6">
            <div className="mb-3.5 flex flex-wrap items-center justify-between gap-3">
              <h2 className="type-h2 text-ink-900">{t("profile_personal_data")}</h2>
              {!editMode && !isLoading && (
                <Button type="button" variant="ghost" size="sm" onClick={() => setEditMode(true)}>
                  <Pencil />
                  {t("edit_profile")}
                </Button>
              )}
            </div>
            <div className="grid grid-cols-1 gap-3.5 sm:grid-cols-2">
              <div className="sm:col-span-2">
                <Field id="name" label={t("full_name")} value={name} onChange={editMode ? setName : undefined} isLoading={isLoading} />
              </div>
              <Field id="phone" label={t("phone")} value={phone} onChange={editMode ? setPhone : undefined} isLoading={isLoading} />
              <Field id="email" label={t("email")} value={profile?.email} locked hint={t("email_locked")} isLoading={isLoading} />
              <Field id="dob" type="date" label={t("students_field_dob")} value={dob} onChange={editMode ? setDob : undefined} isLoading={isLoading} />
              <Field
                id="target_exam"
                label={t("target_exam")}
                value={targetExam}
                onChange={editMode ? setTargetExam : undefined}
                hint={editMode ? t("target_exam_examples") : undefined}
                isLoading={isLoading}
              />
              <div className={cn(FIELD_CLASS, "sm:col-span-2")}>
                <Label htmlFor="school" className={LABEL_CLASS}>
                  {t("school")}
                </Label>
                {isLoading ? (
                  <Skeleton className="h-[var(--spacing-input)] w-full rounded-[14px]" />
                ) : editMode ? (
                  <SchoolPicker
                    id="school"
                    value={schoolId}
                    selectedSchool={selectedSchool}
                    onChange={(school) => {
                      setSelectedSchool(school);
                      setSchoolId(school?.id ?? "");
                    }}
                    allowUnlisted
                    unlistedName={unlistedSchoolName}
                    onUnlistedNameChange={setUnlistedSchoolName}
                  />
                ) : (
                  <Input
                    id="school"
                    value={displayedIsUnlisted ? profile?.unlisted_school_name ?? "" : ownSchool?.name ?? ""}
                    placeholder={t("select_school")}
                    className={PROFILE_INPUT_CLASS}
                    readOnly
                    disabled
                  />
                )}
              </div>

              {/* Optional biodata (FR-FE-24..27) -- no required markers. Jenjang options
                  come from the user's own school when resolvable, else the generic
                  fallback list. Submitting any subset is fine; blank fields are
                  stripped server-side. */}
              <div className={FIELD_CLASS}>
                <Label htmlFor="jenjang" className={LABEL_CLASS}>
                  {t("students_field_jenjang")}
                </Label>
                {isLoading ? (
                  <Skeleton className="h-[var(--spacing-input)] w-full rounded-[14px]" />
                ) : (
                  <Select
                    value={displayedJenjang || "_empty_"}
                    onValueChange={(v) => {
                      const next = v === "_empty_" ? "" : v;
                      setJenjang(next);
                      if (grade && !gradeOptionsForJenjang(next).includes(Number(grade))) {
                        setGrade("");
                      }
                    }}
                    disabled={!editMode}
                  >
                    <SelectTrigger id="jenjang" className={cn(PROFILE_INPUT_CLASS, !displayedJenjang && "text-ink-600")}>
                      <SelectValue placeholder={t("students_field_jenjang")} />
                    </SelectTrigger>
                    <SelectContent>
                      <SelectItem value="_empty_">
                        {t("students_field_jenjang")}
                      </SelectItem>
                      {jenjangOptions.map((opt) => (
                        <SelectItem key={opt} value={opt}>
                          {opt}
                        </SelectItem>
                      ))}
                    </SelectContent>
                  </Select>
                )}
              </div>
              <div className={FIELD_CLASS}>
                <Label htmlFor="grade" className={LABEL_CLASS}>
                  {t("grade")}
                </Label>
                {isLoading ? (
                  <Skeleton className="h-[var(--spacing-input)] w-full rounded-[14px]" />
                ) : (
                  <Select
                    value={grade || "_empty_"}
                    onValueChange={(v) => setGrade(v === "_empty_" ? "" : v)}
                    disabled={!editMode}
                  >
                    <SelectTrigger id="grade" className={cn(PROFILE_INPUT_CLASS, !grade && "text-ink-600")}>
                      <SelectValue placeholder={t("select_grade")} />
                    </SelectTrigger>
                    <SelectContent>
                      <SelectItem value="_empty_">
                        {t("select_grade")}
                      </SelectItem>
                      {gradeOptions.length > 0 && gradeInOptions
                        ? gradeOptions.map((g) => (
                            <SelectItem key={g} value={String(g)}>
                              {formatGrade(g)}
                            </SelectItem>
                          ))
                        : GRADE_GROUPS.map((group) => (
                            <SelectGroup key={group.label}>
                              <SelectLabel>{group.label}</SelectLabel>
                              {group.values.map((g) => (
                                <SelectItem key={g} value={String(g)}>
                                  {formatGrade(g)}
                                </SelectItem>
                              ))}
                            </SelectGroup>
                          ))}
                    </SelectContent>
                  </Select>
                )}
              </div>
            </div>
          </Card>

          <Card className="gap-0 p-6">
            <h2 className="mb-3.5 type-h2 text-ink-900">{t("students_register_section_address")}</h2>
            <div className="grid grid-cols-1 gap-3.5 sm:grid-cols-2">
              <div className="sm:col-span-2">
                <Field id="address" label={t("address")} value={address} onChange={editMode ? setAddress : undefined} isLoading={isLoading} />
              </div>
              <div className={FIELD_CLASS}>
                <Label htmlFor="provinsi" className={LABEL_CLASS}>
                  {t("students_field_provinsi")}
                </Label>
                {isLoading ? (
                  <Skeleton className="h-[var(--spacing-input)] w-full rounded-[14px]" />
                ) : (
                  <Select
                    value={displayedProvinsiId || "_empty_"}
                    onValueChange={(v) => {
                      setProvinsiId(v === "_empty_" ? "" : v);
                      setKotaId("");
                      setKecamatanId("");
                    }}
                    disabled={!editMode}
                  >
                    <SelectTrigger id="provinsi" className={cn(PROFILE_INPUT_CLASS, !displayedProvinsiId && "text-ink-600")}>
                      <SelectValue placeholder={t("students_field_provinsi")} />
                    </SelectTrigger>
                    <SelectContent>
                      <SelectItem value="_empty_">
                        {t("students_field_provinsi")}
                      </SelectItem>
                      {(provinces ?? []).map((p) => (
                        <SelectItem key={p.id} value={p.id}>
                          {p.name}
                        </SelectItem>
                      ))}
                    </SelectContent>
                  </Select>
                )}
              </div>
              <div className={FIELD_CLASS}>
                <Label htmlFor="kota" className={LABEL_CLASS}>
                  {t("students_field_kota")}
                </Label>
                {isLoading ? (
                  <Skeleton className="h-[var(--spacing-input)] w-full rounded-[14px]" />
                ) : (
                  <Select
                    value={displayedKotaId || "_empty_"}
                    onValueChange={(v) => {
                      setKotaId(v === "_empty_" ? "" : v);
                      setKecamatanId("");
                    }}
                    disabled={!editMode || !displayedProvinsiId}
                  >
                    <SelectTrigger id="kota" className={cn(PROFILE_INPUT_CLASS, !displayedKotaId && "text-ink-600")}>
                      <SelectValue placeholder={t("students_field_kota")} />
                    </SelectTrigger>
                    <SelectContent>
                      <SelectItem value="_empty_">
                        {t("students_field_kota")}
                      </SelectItem>
                      {(cities ?? []).map((c) => (
                        <SelectItem key={c.id} value={c.id}>
                          {c.name}
                        </SelectItem>
                      ))}
                    </SelectContent>
                  </Select>
                )}
              </div>
              <div className={FIELD_CLASS}>
                <Label htmlFor="kecamatan" className={LABEL_CLASS}>
                  {t("students_field_kecamatan")}
                </Label>
                {isLoading ? (
                  <Skeleton className="h-[var(--spacing-input)] w-full rounded-[14px]" />
                ) : (
                  <Select
                    value={displayedKecamatanId || "_empty_"}
                    onValueChange={(v) => setKecamatanId(v === "_empty_" ? "" : v)}
                    disabled={!editMode || !displayedKotaId}
                  >
                    <SelectTrigger id="kecamatan" className={cn(PROFILE_INPUT_CLASS, !displayedKecamatanId && "text-ink-600")}>
                      <SelectValue placeholder={t("students_field_kecamatan")} />
                    </SelectTrigger>
                    <SelectContent>
                      <SelectItem value="_empty_">
                        {t("students_field_kecamatan")}
                      </SelectItem>
                      {(districts ?? []).map((d) => (
                        <SelectItem key={d.id} value={d.id}>
                          {d.name}
                        </SelectItem>
                      ))}
                    </SelectContent>
                  </Select>
                )}
              </div>
              <div className={FIELD_CLASS}>
                <Label htmlFor="kode_pos" className={LABEL_CLASS}>
                  {t("students_field_kode_pos")}
                </Label>
                {isLoading ? (
                  <Skeleton className="h-[var(--spacing-input)] w-full rounded-[14px]" />
                ) : (
                  <Input
                    id="kode_pos"
                    value={displayedKodePos}
                    onChange={editMode ? (e) => setKodePos(e.target.value) : undefined}
                    placeholder={t("students_field_kode_pos")}
                    className={PROFILE_INPUT_CLASS}
                    readOnly={!editMode}
                    disabled={!editMode}
                  />
                )}
              </div>
            </div>

            {editMode && (
              <div className="mt-3.5 flex flex-wrap items-center justify-end gap-2">
                <Button
                  type="button"
                  variant="ghost"
                  onClick={cancelEdit}
                  disabled={updateProfile.isPending}
                >
                  {t("cancel")}
                </Button>
                <Button type="submit" disabled={updateProfile.isPending}>
                  {updateProfile.isPending ? (
                    <Loader2 className="mr-2 size-4 animate-spin" />
                  ) : null}
                  {updateProfile.isPending ? t("saving") : t("save_changes")}
                </Button>
              </div>
            )}
          </Card>
        </form>
      </div>

      <ChangePasswordDialog
        open={passwordOpen}
        onOpenChange={setPasswordOpen}
      />
    </>
  );
}
