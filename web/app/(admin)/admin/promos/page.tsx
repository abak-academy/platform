"use client";

import { useState } from "react";
import { toast } from "sonner";
import { Tag } from "lucide-react";
import { AdminPageHeader } from "@/components/admin/AdminPageHeader";
import {
  useAdminPromoCodes,
  useCreatePromoCode,
  useUpdatePromoCode,
  useDeletePromoCode,
} from "@/lib/hooks/admin-promos";
import { useTranslation } from "@/lib/i18n";
import { PromoModal } from "@/components/admin/PromoModal";
import { Button } from "@/components/ui/button";
import { Skeleton } from "@/components/ui/skeleton";
import { Badge } from "@/components/ui/badge";
import { formatRupiah } from "@/lib/format";
import type { PromoCode, AdminCreatePromoCodeInput, AdminUpdatePromoCodeInput } from "@/lib/types";

function discountLabel(promo: PromoCode): string {
  if (promo.discount_percent != null) return `${promo.discount_percent}%`;
  if (promo.discount_amount != null) return formatRupiah(promo.discount_amount);
  return "-";
}

function usageText(promo: PromoCode): string {
  const max = promo.max_uses != null ? String(promo.max_uses) : "∞";
  return `${promo.used_count} / ${max}`;
}

function expiryText(iso?: string | null): string {
  if (!iso) return "No expiry";
  const d = new Date(iso);
  if (Number.isNaN(d.getTime())) return iso;
  return d.toLocaleDateString("id-ID");
}

export default function PromosPage() {
  const { t } = useTranslation();
  const [modalOpen, setModalOpen] = useState(false);
  const [editingPromo, setEditingPromo] = useState<PromoCode | null>(null);

  const { data: promos, isLoading, isError, error } = useAdminPromoCodes();
  const create = useCreatePromoCode();
  const update = useUpdatePromoCode();
  const remove = useDeletePromoCode();

  function openCreate() {
    setEditingPromo(null);
    setModalOpen(true);
  }

  function openEdit(promo: PromoCode) {
    setEditingPromo(promo);
    setModalOpen(true);
  }

  function errorMessage(error: unknown): string {
    if (error instanceof Error) return error.message;
    return t("error_generic");
  }

  async function handleCreate(input: AdminCreatePromoCodeInput) {
    try {
      await create.mutateAsync(input);
      toast.success(t("promo_created"));
      setModalOpen(false);
    } catch (e) {
      toast.error(errorMessage(e));
    }
  }

  async function handleUpdate(input: AdminUpdatePromoCodeInput) {
    if (!editingPromo) return;
    try {
      await update.mutateAsync({ id: editingPromo.id, input });
      toast.success(t("changes_saved"));
      setModalOpen(false);
      setEditingPromo(null);
    } catch (e) {
      toast.error(errorMessage(e));
    }
  }

  async function handleSubmit(input: AdminCreatePromoCodeInput | AdminUpdatePromoCodeInput) {
    if (editingPromo) {
      await handleUpdate(input as AdminUpdatePromoCodeInput);
    } else {
      await handleCreate(input as AdminCreatePromoCodeInput);
    }
  }

  async function handleDelete(id: string) {
    if (!confirm(t("promo_confirm_delete"))) return;
    try {
      await remove.mutateAsync(id);
      toast.success(t("promo_deleted"));
    } catch (e) {
      toast.error(errorMessage(e));
    }
  }

  return (
    <div className="space-y-6 fade-in">
      <AdminPageHeader
        icon={Tag}
        title={t("promos_page_title")}
        description={t("promos_page_description")}
        actions={<Button onClick={openCreate}>{t("promo_create")}</Button>}
      />

      {isLoading && (
        <div className="space-y-2">
          {Array.from({ length: 5 }).map((_, i) => (
            <Skeleton key={i} className="h-12 w-full" />
          ))}
        </div>
      )}

      {isError && (
        <div className="rounded-lg border border-destructive/20 bg-destructive/10 p-4 text-destructive">
          {t("promo_load_failed")}: {errorMessage(error)}
        </div>
      )}

      {!isLoading && !isError && (
        <div className="overflow-x-auto md-card-outlined">
          <table className="w-full text-sm">
            <thead className="bg-muted">
              <tr>
                <th className="px-4 py-3 text-left font-medium">{t("th_code")}</th>
                <th className="px-4 py-3 text-left font-medium">{t("th_discount")}</th>
                <th className="px-4 py-3 text-left font-medium">{t("th_usage")}</th>
                <th className="px-4 py-3 text-left font-medium">{t("th_expiry")}</th>
                <th className="px-4 py-3 text-left font-medium">{t("th_public")}</th>
                <th className="px-4 py-3 text-right font-medium">{t("th_actions")}</th>
              </tr>
            </thead>
            <tbody>
              {promos?.map((promo) => (
                <tr key={promo.id} className="border-t hover:bg-muted/40">
                  <td className="px-4 py-3 font-medium">{promo.code}</td>
                  <td className="px-4 py-3">{discountLabel(promo)}</td>
                  <td className="px-4 py-3">{usageText(promo)}</td>
                  <td className="px-4 py-3">{expiryText(promo.expires_at)}</td>
                  <td className="px-4 py-3">
                    {promo.is_public ? (
                      <Badge variant="outline" className="border-transparent bg-success-bg text-success">
                        {t("th_public")}
                      </Badge>
                    ) : (
                      "-"
                    )}
                  </td>
                  <td className="px-4 py-3 text-right">
                    <div className="flex items-center justify-end gap-2">
                      <Button size="sm" variant="outline" onClick={() => openEdit(promo)}>
                        {t("action_edit")}
                      </Button>
                      <Button
                        size="sm"
                        variant="destructive"
                        onClick={() => handleDelete(promo.id)}
                        disabled={remove.isPending}
                      >
                        {t("action_delete")}
                      </Button>
                    </div>
                  </td>
                </tr>
              ))}
              {(promos?.length ?? 0) === 0 && (
                <tr>
                  <td colSpan={6} className="px-4 py-8 text-center text-muted-foreground">
                    {t("empty_promos")}
                  </td>
                </tr>
              )}
            </tbody>
          </table>
        </div>
      )}

      <PromoModal
        open={modalOpen}
        onOpenChange={setModalOpen}
        promo={editingPromo}
        onSubmit={handleSubmit}
        isPending={create.isPending || update.isPending}
      />
    </div>
  );
}
