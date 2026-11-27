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

function promoStatus(promo: PromoCode): "active" | "inactive" | "expired" | "exhausted" {
  if (promo.is_active === false) return "inactive";
  if (promo.expires_at && new Date(promo.expires_at).getTime() <= Date.now()) return "expired";
  if (promo.max_uses != null && promo.used_count >= promo.max_uses) return "exhausted";
  return "active";
}

function expiryDate(iso: string): string {
  const d = new Date(iso);
  if (Number.isNaN(d.getTime())) return iso;
  return new Intl.DateTimeFormat("id-ID", { day: "numeric", month: "short", year: "numeric", timeZone: "Asia/Jakarta" }).format(d);
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

  // PUT replaces max_uses, expires_at and the two flags, so the toggle resends them as they are.
  async function handleToggleActive(promo: PromoCode) {
    try {
      await update.mutateAsync({
        id: promo.id,
        input: {
          max_uses: promo.max_uses ?? undefined,
          expires_at: promo.expires_at ?? undefined,
          is_public: promo.is_public ?? false,
          affiliator_only: promo.affiliator_only ?? false,
          is_active: promo.is_active === false,
        },
      });
      toast.success(t("changes_saved"));
    } catch (e) {
      toast.error(errorMessage(e));
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
            options: (["active", "inactive", "expired", "exhausted"] as const).map((v) => ({ value: v, label: t(`promos_status_${v}`) })),
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
            const toggleLabel = t(promo.is_active === false ? "promo_activate" : "promo_deactivate");
            const pct = promo.max_uses ? Math.min(100, Math.round((promo.used_count / promo.max_uses) * 100)) : null;
            return (
              <article key={promo.id} className={cn("md-card-outlined flex flex-col gap-2.5", status !== "active" && "border-dashed bg-surface-sunken")}>
                <div className="flex items-center gap-2">
                  <code className="min-w-0 truncate rounded-[9px] bg-deep px-2.5 py-1.5 font-mono text-[13px] leading-4 font-bold tracking-[.06em] text-white">{promo.code}</code>
                  <span className="flex-1" />
                  {promo.affiliator_only ? (
                    <Badge variant="brand">{t("th_affiliator_only")}</Badge>
                  ) : promo.is_public ? (
                    <Badge variant="teal">{t("th_public")}</Badge>
                  ) : null}
                </div>
                <div className="font-[family-name:var(--font-display)] text-[46px] leading-none font-extrabold tracking-[-0.04em] text-ember-700">
                  {promo.discount_percent != null ? (
                    <>{promo.discount_percent}<small className="text-sm font-semibold tracking-normal text-ink-600">{t("promos_pct_off")}</small></>
                  ) : discountLabel(promo)}
                </div>
                <p className="text-[13px] leading-[1.55] text-ink-600">{promo.expires_at ? `${t("promos_until")} ${expiryDate(promo.expires_at)}` : t("promos_no_expiry")}</p>
                <div className="flex flex-col gap-1.5">
                  <div className="flex justify-between text-[13px] text-ink-600">
                    <span>{t("promos_used")}</span>
                    <span className="tabular-nums"><b className="font-bold text-ink-900">{promo.used_count}</b> / {promo.max_uses ?? "∞"}</span>
                  </div>
                  {pct !== null && <Progress value={pct} variant={pct > 80 ? "hatch" : "ember"} aria-label={t("promos_used")} />}
                </div>
                <Badge variant={status === "active" ? "teal" : status === "expired" ? "danger" : "mute"} className="w-full justify-start">{t(`promos_status_${status}`)}</Badge>
                <div className="mt-auto flex items-center justify-end gap-1.5 border-t border-line pt-3">
                  <Button
                    size="sm"
                    variant="outline"
                    onClick={() => handleToggleActive(promo)}
                    disabled={update.isPending}
                    aria-label={`${toggleLabel} ${promo.code}`}
                  >
                    {toggleLabel}
                  </Button>
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
