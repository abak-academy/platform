"use client";

import { useEffect, useState } from "react";
import { Button } from "@/components/ui/button";
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from "@/components/ui/dialog";
import { inAffiliate, inPartner } from "@/components/affiliate/ProgramBadges";
import { useTranslation } from "@/lib/i18n";
import type { AffiliateAdminRow } from "@/lib/types";

type Program = "affiliate" | "partner";

// ApproveProgramDialog approves an application and lets the admin change which
// program the account gets: the one it asked for, the other, or both.
export function ApproveProgramDialog({
  row,
  isPending,
  onConfirm,
  onClose,
}: {
  row: AffiliateAdminRow | null;
  isPending: boolean;
  onConfirm: (programs: Program[]) => void;
  onClose: () => void;
}) {
  const { t } = useTranslation();
  const [chosen, setChosen] = useState<Program[]>([]);

  useEffect(() => {
    if (!row) return;
    const requested: Program[] = [];
    if (inAffiliate(row.programs)) requested.push("affiliate");
    if (inPartner(row.programs)) requested.push("partner");
    setChosen(requested);
  }, [row]);

  const toggle = (program: Program) =>
    setChosen((prev) => (prev.includes(program) ? prev.filter((p) => p !== program) : [...prev, program]));

  return (
    <Dialog open={row !== null} onOpenChange={(open) => !open && onClose()}>
      <DialogContent className="sm:max-w-md">
        <DialogHeader>
          <DialogTitle>{t("admin_aff_approve_title")}</DialogTitle>
          <DialogDescription>{row ? t("admin_aff_approve_desc").replace("{name}", row.name) : ""}</DialogDescription>
        </DialogHeader>
        <div className="space-y-2">
          {(["affiliate", "partner"] as const).map((program) => (
            <label key={program} className="flex items-center gap-2 text-sm text-ink-900">
              <input type="checkbox" checked={chosen.includes(program)} onChange={() => toggle(program)} />
              {t(program === "affiliate" ? "admin_aff_program_affiliate" : "admin_aff_program_partner")}
            </label>
          ))}
        </div>
        <DialogFooter>
          <Button variant="outline" onClick={onClose} disabled={isPending}>
            {t("aff_cancel_btn")}
          </Button>
          <Button onClick={() => onConfirm((["affiliate", "partner"] as const).filter((p) => chosen.includes(p)))} disabled={isPending || chosen.length === 0}>
            {t("admin_aff_approve")}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
