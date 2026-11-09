"use client";

import { toast } from "sonner";
import { CredentialsCard } from "@/components/partner/CredentialsCard";
import { Button } from "@/components/ui/button";
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from "@/components/ui/dialog";
import { ApiError } from "@/lib/api";
import { useReissuePartnerCredentials } from "@/lib/hooks/partner";
import { useTranslation } from "@/lib/i18n";
import type { PartnerStudent } from "@/lib/types";

// Asks first, because the old temporary password stops working. Once the
// student has chosen their own password the server refuses, and the dialog says so.
export function ReissueCredentialsDialog({ student, onClose }: { student: PartnerStudent | null; onClose: () => void }) {
  const { t } = useTranslation();
  const reissue = useReissuePartnerCredentials();
  const result = reissue.data;

  const close = () => {
    reissue.reset();
    onClose();
  };
  const confirm = () => {
    if (!student) return;
    reissue.mutate(student.id, {
      onError: (err) => {
        const activated = err instanceof ApiError && err.code === "student_already_activated";
        toast.error(activated ? t("partner_reissue_activated") : t("partner_reissue_failed"));
      },
    });
  };

  return (
    <Dialog open={Boolean(student)} onOpenChange={(o) => !o && close()}>
      <DialogContent className="sm:max-w-md">
        <DialogHeader>
          <DialogTitle>{t("partner_reissue_title")}</DialogTitle>
          <DialogDescription>{student ? t(result ? "partner_reissue_done" : "partner_reissue_confirm").replace("{name}", student.name) : ""}</DialogDescription>
        </DialogHeader>
        {result && <CredentialsCard username={result.username} password={result.temp_password} />}
        <DialogFooter>
          {result ? (
            <Button onClick={close}>{t("partner_reg_close")}</Button>
          ) : (
            <>
              <Button variant="outline" onClick={close} disabled={reissue.isPending}>
                {t("aff_cancel_btn")}
              </Button>
              <Button onClick={confirm} disabled={reissue.isPending}>
                {t("partner_reissue_submit")}
              </Button>
            </>
          )}
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
