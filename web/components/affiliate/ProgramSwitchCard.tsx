"use client";

import { useState } from "react";
import { toast } from "sonner";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Switch } from "@/components/ui/switch";
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
    <section className="md-card-outlined flex items-center gap-3 p-6">
      <Switch checked={enabled} disabled={blocked} onCheckedChange={() => setConfirming(true)} aria-label={t("admin_aff_prog_title")} />
      <div className="min-w-0 flex-1 space-y-1">
        <div className="flex items-center gap-2">
          <h2 className="text-sm font-bold text-ink-900">{t("admin_aff_prog_title")}</h2>
          <Badge variant={enabled ? "teal" : "danger"}>
            {t(enabled ? "admin_aff_prog_on" : "admin_aff_prog_off")}
          </Badge>
        </div>
        <p className="text-sm text-ink-600">{t(enabled ? "admin_aff_prog_desc_on" : "admin_aff_prog_desc_off")}</p>
        {blocked && <p className="text-sm text-warn">{t("admin_aff_prog_missing").replace("{fields}", missingLabels)}</p>}
        {!blocked && commissionUnset && <p className="text-sm text-warn">{t("admin_aff_prog_commission_unset")}</p>}
      </div>

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
