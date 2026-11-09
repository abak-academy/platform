"use client";

import { useState } from "react";
import { toast } from "sonner";
import { Button } from "@/components/ui/button";
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from "@/components/ui/dialog";
import { ApiError } from "@/lib/api";
import { useAffiliateProgramStatus } from "@/lib/hooks/affiliate";
import { useSetPartnerProgram } from "@/lib/hooks/admin-affiliates";
import { useTranslation } from "@/lib/i18n";

// The switch of the Mitra program. It is separate from the Affiliator switch:
// either program can be on without the other.
export function PartnerSwitchCard() {
  const { t } = useTranslation();
  const status = useAffiliateProgramStatus();
  const setProgram = useSetPartnerProgram();
  const [confirming, setConfirming] = useState(false);
  const enabled = status.data?.partner_enabled === true;

  const confirm = () =>
    setProgram.mutate(!enabled, {
      onSuccess: () => {
        toast.success(t(enabled ? "admin_partner_prog_turned_off" : "admin_partner_prog_turned_on"));
        setConfirming(false);
        status.refetch();
      },
      onError: (err) => toast.error(err instanceof ApiError ? err.message : t("aff_action_failed")),
    });

  return (
    <section className="md-card-outlined flex flex-wrap items-center justify-between gap-4 p-5">
      <div className="space-y-1">
        <div className="flex items-center gap-2">
          <h2 className="font-semibold text-ink-900">{t("admin_partner_prog_title")}</h2>
          <span className={`rounded-full px-2.5 py-0.5 text-xs font-semibold ${enabled ? "bg-success/15 text-success" : "bg-danger/15 text-danger"}`}>
            {t(enabled ? "admin_aff_prog_on" : "admin_aff_prog_off")}
          </span>
        </div>
        <p className="text-sm text-ink-600">{t(enabled ? "admin_partner_prog_desc_on" : "admin_partner_prog_desc_off")}</p>
      </div>
      <Button variant={enabled ? "outline" : "default"} onClick={() => setConfirming(true)} disabled={status.isLoading}>
        {t(enabled ? "admin_aff_prog_turn_off" : "admin_aff_prog_turn_on")}
      </Button>
      <Dialog open={confirming} onOpenChange={setConfirming}>
        <DialogContent className="sm:max-w-md">
          <DialogHeader>
            <DialogTitle>{t(enabled ? "admin_partner_prog_confirm_off_title" : "admin_partner_prog_confirm_on_title")}</DialogTitle>
            <DialogDescription>{t(enabled ? "admin_partner_prog_confirm_off" : "admin_partner_prog_confirm_on")}</DialogDescription>
          </DialogHeader>
          <DialogFooter>
            <Button variant="outline" onClick={() => setConfirming(false)} disabled={setProgram.isPending}>
              {t("aff_cancel_btn")}
            </Button>
            <Button onClick={confirm} disabled={setProgram.isPending}>
              {t("aff_confirm")}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </section>
  );
}
