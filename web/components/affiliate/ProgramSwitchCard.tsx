"use client";

import { useState } from "react";
import { toast } from "sonner";
import { Button } from "@/components/ui/button";
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from "@/components/ui/dialog";
import { ApiError } from "@/lib/api";
import { useSetAffiliateProgram } from "@/lib/hooks/admin-affiliates";
import { useTranslation, type I18nKey } from "@/lib/i18n";

interface Props {
  enabled: boolean;
  /** Required settings still empty: the program cannot be switched on while any is left. */
  missing: string[];
  /** No level has a commission rule: switching on creates no commission. */
  commissionUnset: boolean;
}

// Master switch of the affiliate program; asks for confirmation because it
// closes sign-up and freezes new commissions for everyone.
export function ProgramSwitchCard({ enabled, missing, commissionUnset }: Props) {
  const { t } = useTranslation();
  const [confirming, setConfirming] = useState(false);
  const setProgram = useSetAffiliateProgram();

  const blocked = !enabled && missing.length > 0;
  const missingLabels = missing.map((key) => t(`admin_aff_f_${key}` as I18nKey)).join(", ");

  const confirm = () =>
    setProgram.mutate(!enabled, {
      onSuccess: () => {
        toast.success(t(enabled ? "admin_aff_prog_turned_off" : "admin_aff_prog_turned_on"));
        setConfirming(false);
      },
      onError: (err) => toast.error(err instanceof ApiError ? err.message : t("aff_action_failed")),
    });

  return (
    <section className="md-card-outlined flex flex-wrap items-center justify-between gap-4 p-5">
      <div className="space-y-1">
        <div className="flex items-center gap-2">
          <h2 className="font-semibold text-ink-900">{t("admin_aff_prog_title")}</h2>
          <span
            className={`rounded-full px-2.5 py-0.5 text-xs font-semibold ${enabled ? "bg-success/15 text-success" : "bg-danger/15 text-danger"}`}
          >
            {t(enabled ? "admin_aff_prog_on" : "admin_aff_prog_off")}
          </span>
        </div>
        <p className="text-sm text-ink-600">{t(enabled ? "admin_aff_prog_desc_on" : "admin_aff_prog_desc_off")}</p>
        {blocked && <p className="text-sm text-warn">{t("admin_aff_prog_missing").replace("{fields}", missingLabels)}</p>}
        {!blocked && commissionUnset && <p className="text-sm text-warn">{t("admin_aff_prog_commission_unset")}</p>}
      </div>
      <Button variant={enabled ? "outline" : "default"} disabled={blocked} onClick={() => setConfirming(true)}>
        {t(enabled ? "admin_aff_prog_turn_off" : "admin_aff_prog_turn_on")}
      </Button>

      <Dialog open={confirming} onOpenChange={setConfirming}>
        <DialogContent className="sm:max-w-md">
          <DialogHeader>
            <DialogTitle>{t(enabled ? "admin_aff_prog_confirm_off_title" : "admin_aff_prog_confirm_on_title")}</DialogTitle>
            <DialogDescription>{t(enabled ? "admin_aff_prog_confirm_off" : "admin_aff_prog_confirm_on")}</DialogDescription>
          </DialogHeader>
          {!enabled && commissionUnset && <p className="text-sm text-warn">{t("admin_aff_prog_commission_unset")}</p>}
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
