"use client";

import { useEffect, useState } from "react";
import { Settings } from "lucide-react";
import { toast } from "sonner";
import { AdminPageHeader } from "@/components/admin/AdminPageHeader";
import { NoAccess } from "@/components/admin/NoAccess";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { ApiError } from "@/lib/api";
import { affLabel } from "@/lib/affiliate-labels";
import {
  AFFILIATE_LEVELS,
  COMMISSION_CATEGORIES,
  GLOBAL_NUMBER_FIELDS,
  GLOBAL_OPTIONAL_FIELDS,
  HIDDEN_NUMBER_FIELDS,
  LEVEL_FIELDS,
  formToInput,
  settingsToForm,
  type SettingsForm,
} from "@/lib/affiliate-settings-form";
import {
  useAdminAffiliateOverrides,
  useAdminAffiliateSettings,
  useDeleteAffiliateOverride,
  useRunAffiliateJob,
  useUpdateAffiliateSettings,
  useUpsertAffiliateOverride,
} from "@/lib/hooks/admin-affiliates";
import { useHasCapability } from "@/lib/hooks/use-capability";
import { useTranslation, type I18nKey } from "@/lib/i18n";

const JOB_SCOPES = ["bindings", "commissions", "flags", "levels"] as const;

function NumberField({ id, label, value, onChange }: { id: string; label: string; value: string; onChange: (v: string) => void }) {
  return (
    <div className="space-y-1">
      <Label htmlFor={id}>{label}</Label>
      <Input id={id} inputMode="decimal" value={value} onChange={(e) => onChange(e.target.value)} />
    </div>
  );
}

export default function AdminAffiliateSettingsPage() {
  const { t } = useTranslation();
  const canManage = useHasCapability("affiliates:*");
  const settings = useAdminAffiliateSettings();
  const update = useUpdateAffiliateSettings();
  const overrides = useAdminAffiliateOverrides();
  const upsertOverride = useUpsertAffiliateOverride();
  const deleteOverride = useDeleteAffiliateOverride();
  const runJob = useRunAffiliateJob();

  const [form, setForm] = useState<SettingsForm | null>(null);
  const [ovrProduct, setOvrProduct] = useState("");
  const [ovrPct, setOvrPct] = useState("");
  const [jobScope, setJobScope] = useState<string>(JOB_SCOPES[0]);
  const [jobQuarter, setJobQuarter] = useState("");

  const loaded = settings.data;
  useEffect(() => {
    if (loaded) setForm(settingsToForm(loaded.settings, loaded.levels));
  }, [loaded]);

  if (!canManage) return <NoAccess />;
  if (settings.isLoading || (!form && !settings.isError)) return <div className="text-sm text-ink-500">{t("aff_loading")}</div>;
  if (settings.isError || !form) {
    return (
      <div role="alert" className="md-card-outlined space-y-3 p-6 text-sm text-ink-600">
        <p>{t("aff_error")}</p>
        <Button variant="outline" size="sm" onClick={() => settings.refetch()}>
          {t("aff_retry")}
        </Button>
      </div>
    );
  }

  const fail = (err: unknown) => toast.error(err instanceof ApiError ? err.message : t("aff_action_failed"));

  const save = (e: React.FormEvent) => {
    e.preventDefault();
    const input = formToInput(form);
    if (!input) {
      toast.error(t("admin_aff_set_invalid"));
      return;
    }
    update.mutate(input, { onSuccess: () => toast.success(t("admin_aff_set_saved")), onError: fail });
  };

  const saveOverride = (e: React.FormEvent) => {
    e.preventDefault();
    const pct = Number(ovrPct);
    if (!ovrProduct.trim() || ovrPct.trim() === "" || !Number.isFinite(pct) || pct < 0 || pct > 100) {
      toast.error(t("admin_aff_ovr_invalid"));
      return;
    }
    upsertOverride.mutate(
      { productId: ovrProduct.trim(), percentage: pct },
      {
        onSuccess: () => {
          toast.success(t("admin_aff_ovr_saved"));
          setOvrProduct("");
          setOvrPct("");
        },
        onError: fail,
      },
    );
  };

  const run = (e: React.FormEvent) => {
    e.preventDefault();
    runJob.mutate(
      { scope: jobScope, quarter: jobScope === "levels" && jobQuarter.trim() ? jobQuarter.trim() : undefined },
      { onSuccess: () => toast.success(t("admin_aff_job_done")), onError: fail },
    );
  };

  const setNumber = (key: keyof SettingsForm["numbers"], v: string) =>
    setForm({ ...form, numbers: { ...form.numbers, [key]: v } });
  const setOptional = (key: keyof SettingsForm["optional"], v: string) =>
    setForm({ ...form, optional: { ...form.optional, [key]: v } });
  const setLevelField = (level: string, key: keyof SettingsForm["levels"][string]["fields"], v: string) =>
    setForm({
      ...form,
      levels: { ...form.levels, [level]: { ...form.levels[level], fields: { ...form.levels[level].fields, [key]: v } } },
    });
  const setLevelPct = (level: string, category: string, v: string) =>
    setForm({
      ...form,
      levels: { ...form.levels, [level]: { ...form.levels[level], pct: { ...form.levels[level].pct, [category]: v } } },
    });

  const fieldLabel = (key: string) => t(`admin_aff_f_${key}` as I18nKey);

  return (
    <div className="space-y-8">
      <AdminPageHeader icon={Settings} title={t("admin_aff_set_title")} description={t("admin_aff_set_desc")} />

      <form onSubmit={save} className="space-y-8">
        <section className="md-card-outlined space-y-4 p-5">
          <h2 className="font-semibold text-ink-900">{t("admin_aff_set_global")}</h2>
          <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
            {GLOBAL_NUMBER_FIELDS.filter((key) => !HIDDEN_NUMBER_FIELDS.includes(key)).map((key) => (
              <NumberField key={key} id={`g-${key}`} label={fieldLabel(key)} value={form.numbers[key]} onChange={(v) => setNumber(key, v)} />
            ))}
            {GLOBAL_OPTIONAL_FIELDS.map((key) => (
              <NumberField key={key} id={`g-${key}`} label={fieldLabel(key)} value={form.optional[key]} onChange={(v) => setOptional(key, v)} />
            ))}
            <div className="space-y-1">
              <Label htmlFor="g-launch_date">{fieldLabel("launch_date")}</Label>
              <Input id="g-launch_date" type="date" value={form.launchDate} onChange={(e) => setForm({ ...form, launchDate: e.target.value })} />
            </div>
          </div>
          <label className="flex items-center gap-2 text-sm text-ink-700">
            <input
              type="checkbox"
              checked={form.adjustmentThresholdEnabled}
              onChange={(e) => setForm({ ...form, adjustmentThresholdEnabled: e.target.checked })}
            />
            {fieldLabel("adjustment_threshold_enabled")}
          </label>
        </section>

        <section className="space-y-4">
          <div>
            <h2 className="font-semibold text-ink-900">{t("admin_aff_set_levels")}</h2>
            <p className="text-sm text-ink-500">{t("admin_aff_set_level_hint")}</p>
          </div>
          {AFFILIATE_LEVELS.map((level) => (
            <div key={level} className="md-card-outlined space-y-4 p-5">
              <h3 className="font-semibold text-ink-900">{affLabel(t, level)}</h3>
              <div>
                <p className="mb-2 text-xs font-medium text-ink-500">{t("admin_aff_set_commission")}</p>
                <div className="grid grid-cols-2 gap-3 sm:grid-cols-5">
                  {COMMISSION_CATEGORIES.map((category) => (
                    <NumberField
                      key={category}
                      id={`l-${level}-pct-${category}`}
                      label={`${affLabel(t, category)} (%)`}
                      value={form.levels[level].pct[category]}
                      onChange={(v) => setLevelPct(level, category, v)}
                    />
                  ))}
                </div>
              </div>
              <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
                {LEVEL_FIELDS.map((key) => (
                  <NumberField
                    key={key}
                    id={`l-${level}-${key}`}
                    label={fieldLabel(key)}
                    value={form.levels[level].fields[key]}
                    onChange={(v) => setLevelField(level, key, v)}
                  />
                ))}
              </div>
            </div>
          ))}
        </section>

        <Button type="submit" disabled={update.isPending}>
          {t("admin_aff_set_save")}
        </Button>
      </form>

      <section className="md-card-outlined space-y-3 p-5">
        <h2 className="font-semibold text-ink-900">{t("admin_aff_ovr_title")}</h2>
        {overrides.data && overrides.data.length === 0 && <p className="text-sm text-ink-500">{t("admin_aff_ovr_empty")}</p>}
        <ul className="space-y-1 text-sm">
          {(overrides.data ?? []).map((o) => (
            <li key={o.product_id} className="flex items-center justify-between gap-3">
              <span className="font-mono text-xs text-ink-700">
                {o.product_id} · {o.percentage}%
              </span>
              <Button
                size="sm"
                variant="ghost"
                disabled={deleteOverride.isPending}
                onClick={() => deleteOverride.mutate(o.product_id, { onError: fail })}
              >
                {t("admin_aff_ovr_delete")}
              </Button>
            </li>
          ))}
        </ul>
        <form onSubmit={saveOverride} className="flex flex-wrap items-end gap-3">
          <div className="min-w-[260px] flex-1 space-y-1">
            <Label htmlFor="ovr-product">{t("admin_aff_ovr_product")}</Label>
            <Input id="ovr-product" value={ovrProduct} onChange={(e) => setOvrProduct(e.target.value)} />
          </div>
          <div className="w-28 space-y-1">
            <Label htmlFor="ovr-pct">{t("admin_aff_ovr_pct")}</Label>
            <Input id="ovr-pct" inputMode="decimal" value={ovrPct} onChange={(e) => setOvrPct(e.target.value)} />
          </div>
          <Button type="submit" disabled={upsertOverride.isPending}>
            {t("admin_aff_ovr_add")}
          </Button>
        </form>
      </section>

      <section className="md-card-outlined space-y-3 p-5">
        <div>
          <h2 className="font-semibold text-ink-900">{t("admin_aff_job_title")}</h2>
          <p className="text-sm text-ink-500">{t("admin_aff_job_hint")}</p>
        </div>
        <form onSubmit={run} className="flex flex-wrap items-end gap-3">
          <div className="space-y-1">
            <Label htmlFor="job-scope">{t("admin_aff_job_scope")}</Label>
            <select
              id="job-scope"
              value={jobScope}
              onChange={(e) => setJobScope(e.target.value)}
              className="h-9 rounded-md border border-line bg-surface px-3 text-sm"
            >
              {JOB_SCOPES.map((s) => (
                <option key={s} value={s}>
                  {affLabel(t, s)}
                </option>
              ))}
            </select>
          </div>
          {jobScope === "levels" && (
            <div className="space-y-1">
              <Label htmlFor="job-quarter">{t("admin_aff_job_quarter")}</Label>
              <Input id="job-quarter" value={jobQuarter} onChange={(e) => setJobQuarter(e.target.value)} placeholder="2026-Q3" />
            </div>
          )}
          <Button type="submit" disabled={runJob.isPending}>
            {t("admin_aff_job_run")}
          </Button>
        </form>
      </section>
    </div>
  );
}
