"use client";

import { useState } from "react";
import { TriangleAlert } from "lucide-react";
import { toast } from "sonner";
import { SchoolMultiPicker, type PickedSchool } from "@/components/admin/SchoolMultiPicker";
import { Button } from "@/components/ui/button";
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from "@/components/ui/dialog";
import { useAdminAffiliator, usePromoteAffiliator, useSetAffiliatorSchools } from "@/lib/hooks/admin-affiliators";
import { affiliateOrderStatusLabel } from "@/lib/affiliate-order-status";
import { formatRupiah } from "@/lib/format";
import { useTranslation } from "@/lib/i18n";
import type { PromotableAdmin } from "@/lib/types";

function errorMessage(err: unknown, fallback: string): string {
  return err instanceof Error && err.message ? err.message : fallback;
}

// Promoting is one-way: the account stops being a school admin and the server
// has no way back, so the dialog says so before the button is pressed.
export function AffiliatorPromoteDialog({ admin, onClose }: { admin: PromotableAdmin | null; onClose: () => void }) {
  const { t } = useTranslation();
  const promote = usePromoteAffiliator();
  const [picked, setPicked] = useState<PickedSchool[] | null>(null);
  const own: PickedSchool[] = admin?.school_id ? [{ id: admin.school_id, name: admin.school_name ?? admin.school_id }] : [];
  const schools = picked ?? own;

  function close() {
    setPicked(null);
    onClose();
  }

  async function submit(e: React.FormEvent) {
    e.preventDefault();
    if (!admin || schools.length === 0) return;
    try {
      await promote.mutateAsync({ id: admin.id, school_ids: schools.map((s) => s.id) });
      toast.success(t("aff_admin_promoted"));
      close();
    } catch (err) {
      toast.error(errorMessage(err, t("error_generic")));
    }
  }

  return (
    <Dialog open={admin !== null} onOpenChange={(o) => !o && close()}>
      <DialogContent className="sm:max-w-lg">
        <form onSubmit={submit}>
          <DialogHeader>
            <DialogTitle>{t("aff_admin_promote")}</DialogTitle>
            <DialogDescription>{admin ? `${admin.name} (${admin.email ?? "-"})` : null}</DialogDescription>
          </DialogHeader>
          <div className="grid gap-4 py-4">
            <div role="alert" className="flex items-start gap-3 rounded-[12px] border border-line bg-surface-2 px-4 py-3 text-sm">
              <TriangleAlert size={18} className="mt-0.5 shrink-0 text-danger" aria-hidden />
              <div className="space-y-1">
                <p className="font-semibold">{t("aff_promote_warning_title")}</p>
                <p>{t("aff_promote_warning_irreversible")}</p>
                <p>{t("aff_promote_warning_access")}</p>
              </div>
            </div>
            <div className="grid gap-2">
              <span className="text-sm font-medium">{t("aff_admin_schools")}</span>
              <SchoolMultiPicker value={schools} onChange={setPicked} />
              {own.length > 0 && <p className="text-xs text-ink-600">{t("aff_promote_prefilled")}</p>}
            </div>
          </div>
          <DialogFooter>
            <Button type="button" variant="outline" onClick={close}>
              {t("aff_cancel")}
            </Button>
            <Button type="submit" variant="destructive" disabled={schools.length === 0 || promote.isPending}>
              {promote.isPending ? t("aff_saving") : t("aff_admin_promote")}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  );
}

export function AffiliatorSchoolsDialog({ id, onClose }: { id: string | null; onClose: () => void }) {
  const { t } = useTranslation();
  const detail = useAdminAffiliator(id);
  const save = useSetAffiliatorSchools();
  const [picked, setPicked] = useState<PickedSchool[] | null>(null);
  const current: PickedSchool[] = (detail.data?.affiliator.schools ?? []).map((s) => ({ id: s.id, name: s.name }));
  const schools = picked ?? current;

  function close() {
    setPicked(null);
    onClose();
  }

  async function submit(e: React.FormEvent) {
    e.preventDefault();
    if (!id || schools.length === 0) return;
    try {
      await save.mutateAsync({ id, school_ids: schools.map((s) => s.id) });
      toast.success(t("changes_saved"));
      close();
    } catch (err) {
      toast.error(errorMessage(err, t("error_generic")));
    }
  }

  return (
    <Dialog open={id !== null} onOpenChange={(o) => !o && close()}>
      <DialogContent className="sm:max-w-lg">
        <form onSubmit={submit}>
          <DialogHeader>
            <DialogTitle>{t("aff_admin_edit_schools")}</DialogTitle>
            <DialogDescription>{detail.data?.affiliator.name}</DialogDescription>
          </DialogHeader>
          <div className="py-4">
            <SchoolMultiPicker value={schools} onChange={setPicked} disabled={detail.isLoading} />
          </div>
          <DialogFooter>
            <Button type="button" variant="outline" onClick={close}>
              {t("aff_cancel")}
            </Button>
            <Button type="submit" disabled={schools.length === 0 || save.isPending || detail.isLoading}>
              {save.isPending ? t("aff_saving") : t("aff_save")}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  );
}

export function AffiliatorDetailDialog({ id, onClose }: { id: string | null; onClose: () => void }) {
  const { t } = useTranslation();
  const detail = useAdminAffiliator(id);
  const d = detail.data;

  return (
    <Dialog open={id !== null} onOpenChange={(o) => !o && onClose()}>
      <DialogContent className="sm:max-w-2xl">
        <DialogHeader>
          <DialogTitle>{d?.affiliator.name ?? t("aff_admin_detail")}</DialogTitle>
          <DialogDescription>{d?.affiliator.email}</DialogDescription>
        </DialogHeader>
        {detail.isLoading || !d ? (
          <p className="py-6 text-sm text-ink-500">{t("aff_loading")}</p>
        ) : (
          <div className="space-y-4 py-2 text-sm">
            <dl className="grid grid-cols-2 gap-3 sm:grid-cols-3">
              <div><dt className="text-ink-600">{t("aff_stat_paid_orders")}</dt><dd className="font-semibold">{d.totals.paid_orders}</dd></div>
              <div><dt className="text-ink-600">{t("aff_stat_units")}</dt><dd className="font-semibold">{d.totals.paid_units}</dd></div>
              <div><dt className="text-ink-600">{t("aff_stat_paid_amount")}</dt><dd className="font-semibold">{formatRupiah(d.totals.paid_amount)}</dd></div>
              <div><dt className="text-ink-600">{t("aff_stat_discount")}</dt><dd className="font-semibold">{formatRupiah(d.totals.discount_total)}</dd></div>
              <div><dt className="text-ink-600">{t("aff_stat_pending")}</dt><dd className="font-semibold">{d.totals.pending_orders}</dd></div>
              <div><dt className="text-ink-600">{t("aff_th_students")}</dt><dd className="font-semibold">{d.affiliator.student_count}</dd></div>
            </dl>
            <div>
              <h3 className="font-semibold">{t("aff_section_schools")}</h3>
              <p>{(d.affiliator.schools ?? []).map((s) => s.name).join(", ") || "-"}</p>
            </div>
            <div>
              <h3 className="font-semibold">{t("aff_admin_recent_orders")}</h3>
              <ul className="space-y-1">
                {d.recent_orders.map((o) => (
                  <li key={o.id} className="flex justify-between gap-3">
                    <span>{`${o.product_name} · ${o.participant_count}`}</span>
                    <span>{`${formatRupiah(o.total)} · ${affiliateOrderStatusLabel(o.status, t)}`}</span>
                  </li>
                ))}
                {d.recent_orders.length === 0 && <li className="text-ink-500">{t("aff_orders_empty")}</li>}
              </ul>
            </div>
          </div>
        )}
        <DialogFooter>
          <Button type="button" onClick={onClose}>
            {t("aff_close")}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
