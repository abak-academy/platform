"use client";

import { useState } from "react";
import { toast } from "sonner";
import { AffiliateList } from "@/components/affiliate/AffiliateList";
import { ListToolbar } from "@/components/affiliate/ListToolbar";
import { ProductPicker } from "@/components/affiliate/ProductPicker";
import { RuleTypeSelect } from "@/components/affiliate/RuleTypeSelect";
import { Button } from "@/components/ui/button";
import type { DataTableColumn } from "@/components/ui/data-table";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { ApiError } from "@/lib/api";
import { affLabel, formatAffDate } from "@/lib/affiliate-labels";
import { formatCommissionRule } from "@/lib/affiliate-rules";
import { useAdminAffiliateOverrides, useDeleteAffiliateOverride, useUpsertAffiliateOverride } from "@/lib/hooks/admin-affiliates";
import type { ListParams } from "@/lib/hooks/use-list-params";
import { useTranslation } from "@/lib/i18n";
import type { AffiliateCommissionRuleType, AffiliateOverrideProduct, AffiliateProductOverride } from "@/lib/types";

const INITIAL: ListParams = { q: "", status: "", type: "", from: "", to: "", page: 1, per_page: 10 };

export function OverridesPanel() {
  const { t } = useTranslation();
  const [params, setParams] = useState<ListParams>(INITIAL);
  const [product, setProduct] = useState<AffiliateOverrideProduct | null>(null);
  const [ruleType, setRuleType] = useState<AffiliateCommissionRuleType>("percent");
  const [ruleValue, setRuleValue] = useState("");
  const query = useAdminAffiliateOverrides({ q: params.q, page: params.page, per_page: params.per_page });
  const upsert = useUpsertAffiliateOverride();
  const remove = useDeleteAffiliateOverride();

  const fail = (err: unknown) => toast.error(err instanceof ApiError ? err.message : t("aff_action_failed"));
  const update = (patch: Partial<ListParams>) => setParams((current) => ({ ...current, ...patch, page: patch.page ?? 1 }));

  const save = (e: React.FormEvent) => {
    e.preventDefault();
    const value = Number(ruleValue);
    const invalid =
      !product ||
      ruleValue.trim() === "" ||
      !Number.isFinite(value) ||
      value < 0 ||
      (ruleType === "percent" ? value > 100 || Math.abs(value * 100 - Math.round(value * 100)) > 1e-6 : !Number.isInteger(value));
    if (invalid) {
      toast.error(t("admin_aff_ovr_invalid"));
      return;
    }
    upsert.mutate(
      { productId: product.id, type: ruleType, value },
      {
        onSuccess: () => {
          toast.success(t("admin_aff_ovr_saved"));
          setProduct(null);
          setRuleValue("");
        },
        onError: fail,
      },
    );
  };

  const columns: DataTableColumn<AffiliateProductOverride>[] = [
    {
      key: "product",
      header: t("admin_aff_ovr_col_product"),
      cell: (o) => (
        <div>
          <div className="font-medium text-ink-900">{o.product_name ?? o.product_id}</div>
          {o.product_type && <div className="text-xs text-ink-500">{affLabel(t, o.product_type)}</div>}
        </div>
      ),
    },
    { key: "pct", header: t("admin_aff_ovr_col_pct"), align: "right", cell: (o) => formatCommissionRule(o.type, o.value, t("aff_per_unit")) },
    { key: "updated", header: t("admin_aff_ovr_col_updated"), cell: (o) => formatAffDate(o.updated_at) },
    {
      key: "actions",
      header: "",
      align: "right",
      cell: (o) => (
        <Button size="sm" variant="ghost" disabled={remove.isPending} onClick={() => remove.mutate(o.product_id, { onError: fail })}>
          {t("admin_aff_ovr_delete")}
        </Button>
      ),
    },
  ];

  return (
    <div className="space-y-6">
      <section className="md-card-outlined space-y-4 p-5">
        <div>
          <h2 className="font-semibold text-ink-900">{t("admin_aff_ovr_title")}</h2>
          <p className="text-sm text-ink-600">{t("admin_aff_ovr_desc")}</p>
        </div>
        <form onSubmit={save} className="grid items-start gap-4 sm:grid-cols-[minmax(0,1fr)_7.5rem_12rem_auto]">
          <div className="space-y-1.5">
            <Label htmlFor="ovr-product">{t("admin_aff_ovr_product")}</Label>
            <ProductPicker id="ovr-product" value={product} onChange={setProduct} />
          </div>
          <div className="space-y-1.5">
            <Label htmlFor="ovr-type">{t("admin_aff_rule_type")}</Label>
            <RuleTypeSelect id="ovr-type" label={t("admin_aff_rule_type")} value={ruleType} onChange={setRuleType} />
          </div>
          <div className="space-y-1.5">
            <Label htmlFor="ovr-value">{t("admin_aff_ovr_pct")}</Label>
            <Input id="ovr-value" inputMode="decimal" value={ruleValue} onChange={(e) => setRuleValue(e.target.value)} aria-describedby="ovr-value-hint" />
            <p id="ovr-value-hint" className="text-xs leading-relaxed text-ink-500">
              {t("admin_aff_rule_hint")}
            </p>
          </div>
          <div className="sm:pt-[1.625rem]">
            <Button type="submit" disabled={upsert.isPending}>
              {t("admin_aff_ovr_add")}
            </Button>
          </div>
        </form>
      </section>

      <section className="space-y-3">
        <ListToolbar
          params={params}
          onChange={update}
          onReset={() => setParams((current) => ({ ...INITIAL, per_page: current.per_page }))}
          searchPlaceholder={t("admin_aff_ovr_list_search")}
        />
        <AffiliateList
          query={query}
          columns={columns}
          rowKey={(o) => o.product_id}
          page={params.page}
          onPageChange={(page) => update({ page })}
          empty={t("admin_aff_ovr_empty")}
          filtered={Boolean(params.q)}
        />
      </section>
    </div>
  );
}
