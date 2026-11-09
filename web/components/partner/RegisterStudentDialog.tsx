"use client";

import { useState } from "react";
import { toast } from "sonner";
import { SchoolPicker } from "@/components/SchoolPicker";
import { CredentialsCard } from "@/components/partner/CredentialsCard";
import { Button } from "@/components/ui/button";
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from "@/components/ui/dialog";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { ApiError } from "@/lib/api";
import { JENJANG_OPTIONS } from "@/lib/jenjang";
import { useRegisterPartnerStudent } from "@/lib/hooks/partner";
import { useTranslation } from "@/lib/i18n";
import type { PartnerStudentRegistration, SchoolOption } from "@/lib/types";

export function RegisterStudentDialog({ open, onOpenChange }: { open: boolean; onOpenChange: (open: boolean) => void }) {
  const { t } = useTranslation();
  const register = useRegisterPartnerStudent();
  const [name, setName] = useState("");
  const [jenjang, setJenjang] = useState("");
  const [school, setSchool] = useState<SchoolOption | null>(null);
  const [email, setEmail] = useState("");
  const [created, setCreated] = useState<PartnerStudentRegistration | null>(null);

  const reset = () => {
    setName("");
    setJenjang("");
    setSchool(null);
    setEmail("");
    setCreated(null);
  };
  const close = () => {
    reset();
    onOpenChange(false);
  };

  const submit = (e: React.FormEvent) => {
    e.preventDefault();
    if (!name.trim() || !jenjang) return;
    register.mutate(
      { name: name.trim(), jenjang, ...(school ? { school_id: school.id } : {}), ...(email.trim() ? { email: email.trim() } : {}) },
      {
        onSuccess: (resp) => setCreated(resp),
        onError: (err) => toast.error(err instanceof ApiError ? err.message : t("partner_reg_failed")),
      },
    );
  };

  return (
    <Dialog open={open} onOpenChange={(o) => (o ? onOpenChange(true) : close())}>
      <DialogContent className="sm:max-w-lg">
        <DialogHeader>
          <DialogTitle>{t(created ? "partner_reg_done_title" : "partner_reg_title")}</DialogTitle>
          <DialogDescription>{t(created ? "partner_reg_done_desc" : "partner_reg_desc")}</DialogDescription>
        </DialogHeader>
        {created ? (
          <>
            <CredentialsCard username={created.username} password={created.temp_password ?? ""} />
            <DialogFooter>
              <Button onClick={close}>{t("partner_reg_close")}</Button>
            </DialogFooter>
          </>
        ) : (
          <form onSubmit={submit} className="space-y-4">
            <div className="space-y-1.5">
              <Label htmlFor="partner-student-name">{t("partner_reg_name")}</Label>
              <Input id="partner-student-name" value={name} onChange={(e) => setName(e.target.value)} />
            </div>
            <div className="space-y-1.5">
              <Label htmlFor="partner-student-jenjang">{t("partner_reg_jenjang")}</Label>
              <Select value={jenjang} onValueChange={setJenjang}>
                <SelectTrigger id="partner-student-jenjang">
                  <SelectValue placeholder={t("partner_reg_jenjang_ph")} />
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
            <div className="space-y-1.5">
              <Label htmlFor="partner-student-school">{t("partner_reg_school")}</Label>
              <SchoolPicker id="partner-student-school" selectedSchool={school} value={school?.id} onChange={setSchool} />
              <p className="text-xs text-ink-500">{t("partner_reg_school_hint")}</p>
            </div>
            <div className="space-y-1.5">
              <Label htmlFor="partner-student-email">{t("partner_reg_email")}</Label>
              <Input id="partner-student-email" type="email" value={email} onChange={(e) => setEmail(e.target.value)} />
            </div>
            <DialogFooter>
              <Button type="button" variant="outline" onClick={close}>
                {t("aff_cancel_btn")}
              </Button>
              <Button type="submit" disabled={register.isPending || !name.trim() || !jenjang}>
                {t("partner_reg_submit")}
              </Button>
            </DialogFooter>
          </form>
        )}
      </DialogContent>
    </Dialog>
  );
}
