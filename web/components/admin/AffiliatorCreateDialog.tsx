"use client";

import { useState } from "react";
import { toast } from "sonner";
import { AffiliatorBulkPanel } from "@/components/admin/AffiliatorBulkPanel";
import { SchoolMultiPicker, type PickedSchool } from "@/components/admin/SchoolMultiPicker";
import { Button } from "@/components/ui/button";
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from "@/components/ui/dialog";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { PasswordInput } from "@/components/ui/password-input";
import { useCreateAffiliator } from "@/lib/hooks/admin-affiliators";
import { useTranslation } from "@/lib/i18n";
import type { Affiliator } from "@/lib/types";

type Mode = "single" | "bulk";

interface Props {
  open: boolean;
  onClose: () => void;
}

// One entry point for new accounts: a single form, or a CSV upload for many.
export function AffiliatorCreateDialog({ open, onClose }: Props) {
  const { t } = useTranslation();
  const create = useCreateAffiliator();
  const [mode, setMode] = useState<Mode>("single");
  const [name, setName] = useState("");
  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");
  const [schools, setSchools] = useState<PickedSchool[]>([]);
  const [created, setCreated] = useState<Affiliator | null>(null);
  const ready = name.trim() !== "" && email.trim() !== "" && (password === "" || password.length >= 8) && schools.length > 0;

  function close() {
    setMode("single");
    setName("");
    setEmail("");
    setPassword("");
    setSchools([]);
    setCreated(null);
    onClose();
  }

  async function submit(e: React.FormEvent) {
    e.preventDefault();
    if (!ready) return;
    try {
      const acc = await create.mutateAsync({
        name: name.trim(),
        email: email.trim(),
        school_ids: schools.map((s) => s.id),
        ...(password ? { password } : {}),
      });
      if (acc.temp_password) {
        setCreated(acc);
      } else {
        toast.success(t("aff_admin_created"));
        close();
      }
    } catch (err) {
      toast.error(err instanceof Error && err.message ? err.message : t("error_generic"));
    }
  }

  return (
    <Dialog open={open} onOpenChange={(o) => !o && close()}>
      <DialogContent className="max-h-[90vh] overflow-y-auto sm:max-w-2xl">
        {created ? (
          <>
            <DialogHeader>
              <DialogTitle>{t("aff_admin_created")}</DialogTitle>
              <DialogDescription>{t("aff_admin_created_hint")}</DialogDescription>
            </DialogHeader>
            <dl className="grid gap-2 py-4 text-sm">
              <div className="flex justify-between gap-4">
                <dt className="text-ink-600">{t("aff_admin_email")}</dt>
                <dd className="font-mono">{created.email}</dd>
              </div>
              <div className="flex justify-between gap-4">
                <dt className="text-ink-600">{t("aff_register_temp_password")}</dt>
                <dd className="font-mono">{created.temp_password}</dd>
              </div>
            </dl>
            <DialogFooter>
              <Button type="button" onClick={close}>
                {t("aff_close")}
              </Button>
            </DialogFooter>
          </>
        ) : (
          <>
            <DialogHeader>
              <DialogTitle>{t("aff_admin_create")}</DialogTitle>
              <DialogDescription>{t("aff_admin_create_hint")}</DialogDescription>
            </DialogHeader>

            <div role="radiogroup" aria-label={t("aff_admin_create_mode")} className="flex w-fit overflow-hidden rounded-[10px] border border-line">
              {(["single", "bulk"] as const).map((m) => (
                <button
                  key={m}
                  type="button"
                  role="radio"
                  aria-checked={mode === m}
                  onClick={() => setMode(m)}
                  className="border-r border-line px-4 py-1.5 text-sm last:border-r-0"
                  style={mode === m ? { backgroundColor: "var(--md-sys-color-primary)", color: "var(--md-sys-color-on-primary)", fontWeight: 600 } : undefined}
                >
                  {t(m === "single" ? "aff_admin_mode_single" : "aff_admin_mode_bulk")}
                </button>
              ))}
            </div>

            {mode === "single" ? (
              <form onSubmit={submit}>
                <div className="grid gap-4 py-4">
                  <div className="grid gap-2">
                    <Label htmlFor="aff-new-name">{t("aff_register_name")}</Label>
                    <Input id="aff-new-name" value={name} onChange={(e) => setName(e.target.value)} />
                  </div>
                  <div className="grid gap-2">
                    <Label htmlFor="aff-new-email">{t("aff_admin_email")}</Label>
                    <Input id="aff-new-email" type="email" value={email} onChange={(e) => setEmail(e.target.value)} />
                  </div>
                  <div className="grid gap-2">
                    <Label htmlFor="aff-new-password">{t("aff_admin_password_optional")}</Label>
                    <PasswordInput id="aff-new-password" value={password} onChange={(e) => setPassword(e.target.value)} />
                    <p className="text-xs text-ink-600">{t("aff_admin_password_hint")}</p>
                  </div>
                  <div className="grid gap-2">
                    <Label>{t("aff_admin_schools")}</Label>
                    <SchoolMultiPicker value={schools} onChange={setSchools} />
                  </div>
                </div>
                <DialogFooter>
                  <Button type="button" variant="outline" onClick={close}>
                    {t("aff_cancel")}
                  </Button>
                  <Button type="submit" disabled={!ready || create.isPending}>
                    {create.isPending ? t("aff_saving") : t("aff_admin_create")}
                  </Button>
                </DialogFooter>
              </form>
            ) : (
              <div className="py-4">
                <AffiliatorBulkPanel />
                <DialogFooter className="mt-4">
                  <Button type="button" variant="outline" onClick={close}>
                    {t("aff_close")}
                  </Button>
                </DialogFooter>
              </div>
            )}
          </>
        )}
      </DialogContent>
    </Dialog>
  );
}
