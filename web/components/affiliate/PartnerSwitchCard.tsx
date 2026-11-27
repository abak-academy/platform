"use client";

import { useState } from "react";
import { toast } from "sonner";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Switch } from "@/components/ui/switch";
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
    <section className="md-card-outlined flex items-center gap-3 p-6">
      <Switch checked={enabled} disabled={status.isLoading} onCheckedChange={() => setConfirming(true)} aria-label={t("admin_partner_prog_title")} />
      <div className="min-w-0 flex-1 space-y-1">
        <div className="flex items-center gap-2">
          <h2 className="text-sm font-bold text-ink-900">{t("admin_partner_prog_title")}</h2>
          <Badge variant={enabled ? "teal" : "danger"}>
            {t(enabled ? "admin_aff_prog_on" : "admin_aff_prog_off")}
          </Badge>
        </div>
        <p className="text-sm text-ink-600">{t(enabled ? "admin_partner_prog_desc_on" : "admin_partner_prog_desc_off")}</p>
      </div>
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
