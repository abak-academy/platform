"use client";

import { useMemo, useState } from "react";
import { toast } from "sonner";
import { Plus, Tag } from "lucide-react";
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
import { EmptyState } from "@/components/ui/empty-state";
import { Progress } from "@/components/ui/progress";
import { ListToolbar, type ListToolbarValue } from "@/components/admin/ListToolbar";
import { cn } from "@/lib/utils";
import { formatRupiah } from "@/lib/format";
import type { PromoCode, AdminCreatePromoCodeInput, AdminUpdatePromoCodeInput } from "@/lib/types";

function discountLabel(promo: PromoCode): string {
  if (promo.discount_percent != null) return `${promo.discount_percent}%`;
  if (promo.discount_amount != null) return formatRupiah(promo.discount_amount);
  return "-";
}

function promoStatus(promo: PromoCode): "active" | "expired" | "exhausted" {
  if (promo.expires_at && new Date(promo.expires_at).getTime() <= Date.now()) return "expired";
  if (promo.max_uses != null && promo.used_count >= promo.max_uses) return "exhausted";
  return "active";
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

  const [toolbar, setToolbar] = useState<ListToolbarValue>({ q: "", filters: {}, sort: "used" });
  const shown = useMemo(() => {
    const q = toolbar.q.trim().toLowerCase();
    const rows = (promos ?? []).filter(
      (p) => p.code.toLowerCase().includes(q) && (!toolbar.filters.status || promoStatus(p) === toolbar.filters.status),
    );
    const endsAt = (p: PromoCode) => (p.expires_at ? new Date(p.expires_at).getTime() : Infinity);
    return rows.sort((a, b) => (toolbar.sort === "ending" ? endsAt(a) - endsAt(b) : b.used_count - a.used_count));
  }, [promos, toolbar]);

  return (
    <div className="flex flex-col gap-5 fade-in">
      <AdminPageHeader
        title={t("promos_page_title")}
        description={t("promos_page_description")}
        actions={<Button variant="dark" onClick={openCreate}><Plus aria-hidden />{t("promo_create")}</Button>}
      />

      {!isLoading && !isError && (
        <ListToolbar
          variant="standalone"
          value={toolbar}
          onChange={setToolbar}
          count={shown.length}
          unit={t("promos_unit")}
          searchPlaceholder={t("promos_search")}
          filters={[{
            key: "status",
            label: t("th_status"),
            options: (["active", "expired", "exhausted"] as const).map((v) => ({ value: v, label: t(`promos_status_${v}`) })),
          }]}
          sorts={[{ value: "used", label: t("promos_sort_used") }, { value: "ending", label: t("promos_sort_ending") }]}
        />
      )}

      {isLoading && (
        <div className="flex flex-col gap-2">
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

      {!isLoading && !isError && shown.length === 0 && (
        <EmptyState framed icon={Tag} title={t("empty_promos")} />
      )}

      {!isLoading && !isError && shown.length > 0 && (
        <div className="grid gap-[18px] [grid-template-columns:repeat(auto-fill,minmax(255px,1fr))]">
          {shown.map((promo) => {
            const status = promoStatus(promo);
            const pct = promo.max_uses ? Math.min(100, Math.round((promo.used_count / promo.max_uses) * 100)) : null;
            return (
              <article key={promo.id} className="md-card-outlined flex flex-col gap-2.5">
                <div className="flex items-center gap-2">
                  <code className={cn("rounded-[9px] px-2.5 py-1.5 font-mono text-[13px] font-bold tracking-[.06em] text-surface", status === "active" ? "bg-ink-900" : "bg-ink-600")}>{promo.code}</code>
                  <span className="flex-1" />
                  {promo.affiliator_only ? (
                    <Badge variant="brand">{t("th_affiliator_only")}</Badge>
                  ) : promo.is_public ? (
                    <Badge variant="teal">{t("th_public")}</Badge>
                  ) : null}
                </div>
                <div className={cn("font-[family-name:var(--font-display)] text-[46px] leading-none font-extrabold tracking-[-0.04em]", status === "active" ? "text-ember-700" : "text-ink-600")}>
                  {discountLabel(promo)}
                </div>
                <p className="text-xs text-ink-600">{expiryText(promo.expires_at)}</p>
                <div>
                  <div className="mb-1.5 flex justify-between text-xs text-ink-600">
                    <span>{t("promos_used")}</span>
                    <span className="tabular-nums">{usageText(promo)}</span>
                  </div>
                  {pct !== null && <Progress value={pct} variant={pct > 80 ? "hatch" : "ember"} aria-label={t("promos_used")} />}
                </div>
                <Badge variant={status === "active" ? "teal" : status === "expired" ? "danger" : "mute"} className="self-start">{t(`promos_status_${status}`)}</Badge>
                <div className="mt-auto flex items-center justify-end gap-1.5 border-t border-line pt-3">
                  <Button size="sm" variant="outline" onClick={() => openEdit(promo)}>
                    {t("action_edit")}
                  </Button>
                  <Button size="sm" variant="ghost" onClick={() => handleDelete(promo.id)} disabled={remove.isPending}>
                    {t("action_delete")}
                  </Button>
                </div>
              </article>
            );
          })}
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
