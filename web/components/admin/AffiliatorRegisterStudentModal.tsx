"use client";

import { useEffect, useState } from "react";
import { toast } from "sonner";
import { Button } from "@/components/ui/button";
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from "@/components/ui/dialog";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { useRegisterStudent } from "@/lib/hooks/admin-students";
import { useTranslation } from "@/lib/i18n";
import { JENJANG_OPTIONS } from "@/lib/jenjang";
import type { SchoolOption, StudentRegistrationResult } from "@/lib/types";

interface Props {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  schools: Pick<SchoolOption, "id" | "name">[];
  defaultSchoolId?: string;
}

export function AffiliatorRegisterStudentModal({ open, onOpenChange, schools, defaultSchoolId }: Props) {
  const { t } = useTranslation();
  const register = useRegisterStudent();
  const [schoolId, setSchoolId] = useState("");
  const [name, setName] = useState("");
  const [jenjang, setJenjang] = useState("");
  const [email, setEmail] = useState("");
  const [result, setResult] = useState<StudentRegistrationResult | null>(null);

  useEffect(() => {
    if (!open) return;
    setSchoolId(defaultSchoolId ?? (schools.length === 1 ? schools[0].id : ""));
    setName("");
    setJenjang("");
    setEmail("");
    setResult(null);
  }, [open, defaultSchoolId, schools]);

  const canSubmit = schoolId !== "" && name.trim() !== "" && jenjang !== "" && !register.isPending;

  async function handleSubmit(e: React.FormEvent) {
    e.preventDefault();
    if (!canSubmit) return;
    try {
      const created = await register.mutateAsync({
        schoolId,
        input: { name: name.trim(), jenjang, ...(email.trim() ? { email: email.trim() } : {}) },
      });
      setResult(created);
    } catch (err) {
      toast.error(err instanceof Error ? err.message : t("error_generic"));
    }
  }

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="sm:max-w-lg">
        {result ? (
          <>
            <DialogHeader>
              <DialogTitle>{t("students_credential_display")}</DialogTitle>
              <DialogDescription>{t("aff_register_credentials_hint")}</DialogDescription>
            </DialogHeader>
            <dl className="grid gap-2 py-4 text-sm">
              <div className="flex justify-between gap-4">
                <dt className="text-ink-600">{t("aff_register_username")}</dt>
                <dd className="font-mono">{result.username}</dd>
              </div>
              <div className="flex justify-between gap-4">
                <dt className="text-ink-600">{t("aff_register_temp_password")}</dt>
                <dd className="font-mono">{result.temp_password}</dd>
              </div>
            </dl>
            <DialogFooter>
              <Button type="button" onClick={() => onOpenChange(false)}>
                {t("aff_close")}
              </Button>
            </DialogFooter>
          </>
        ) : (
          <form onSubmit={handleSubmit}>
            <DialogHeader>
              <DialogTitle>{t("aff_register")}</DialogTitle>
              <DialogDescription>{t("aff_register_hint")}</DialogDescription>
            </DialogHeader>
            <div className="grid gap-4 py-4">
              <div className="grid gap-2">
                <Label htmlFor="aff-reg-school">{t("aff_th_school")}</Label>
                <Select value={schoolId} onValueChange={setSchoolId}>
                  <SelectTrigger id="aff-reg-school" className="w-full" aria-label={t("aff_th_school")}>
                    <SelectValue placeholder={t("aff_choose_school")} />
                  </SelectTrigger>
                  <SelectContent>
                    {schools.map((s) => (
                      <SelectItem key={s.id} value={s.id}>
                        {s.name}
                      </SelectItem>
                    ))}
                  </SelectContent>
                </Select>
              </div>
              <div className="grid gap-2">
                <Label htmlFor="aff-reg-name">{t("aff_register_name")}</Label>
                <Input id="aff-reg-name" value={name} onChange={(e) => setName(e.target.value)} disabled={register.isPending} />
              </div>
              <div className="grid gap-2">
                <Label htmlFor="aff-reg-jenjang">{t("aff_th_jenjang")}</Label>
                <Select value={jenjang} onValueChange={setJenjang}>
                  <SelectTrigger id="aff-reg-jenjang" className="w-full" aria-label={t("aff_th_jenjang")}>
                    <SelectValue placeholder={t("aff_choose_jenjang")} />
                  </SelectTrigger>
                  <SelectContent>
                    {JENJANG_OPTIONS.map((j) => (
                      <SelectItem key={j} value={j}>
                        {j}
                      </SelectItem>
                    ))}
                  </SelectContent>
                </Select>
              </div>
              <div className="grid gap-2">
                <Label htmlFor="aff-reg-email">{t("aff_register_email")}</Label>
                <Input id="aff-reg-email" type="email" value={email} onChange={(e) => setEmail(e.target.value)} disabled={register.isPending} />
              </div>
            </div>
            <DialogFooter>
              <Button type="button" variant="outline" onClick={() => onOpenChange(false)} disabled={register.isPending}>
                {t("aff_cancel")}
              </Button>
              <Button type="submit" disabled={!canSubmit}>
                {register.isPending ? t("aff_saving") : t("aff_register")}
              </Button>
            </DialogFooter>
          </form>
        )}
      </DialogContent>
    </Dialog>
  );
}
