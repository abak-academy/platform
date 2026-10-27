"use client";

import { Suspense, useEffect, useState } from "react";
import { usePathname, useRouter, useSearchParams } from "next/navigation";
import { Settings } from "lucide-react";
import { toast } from "sonner";
import { AdminPageHeader } from "@/components/admin/AdminPageHeader";
import { BulkFormatGuide } from "@/components/admin/BulkFormatGuide";
import { NoAccess } from "@/components/admin/NoAccess";
import { JobsPanel } from "@/components/affiliate/settings/JobsPanel";
import { OverridesPanel } from "@/components/affiliate/settings/OverridesPanel";
import { SettingField } from "@/components/affiliate/SettingField";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Tabs, TabsList, TabsTrigger } from "@/components/ui/tabs";
import { ApiError } from "@/lib/api";
import { affLabel } from "@/lib/affiliate-labels";
import { COMMISSION_RULE_TYPES } from "@/lib/affiliate-rules";
import {
  AFFILIATE_LEVELS,
  COMMISSION_CATEGORIES,
  GLOBAL_NUMBER_FIELDS,
  HIDDEN_NUMBER_FIELDS,
  LEVEL_FIELDS,
  formToInput,
  settingsToForm,
  type GlobalNumberField,
  type GlobalOptionalField,
  type RuleForm,
  type SettingsForm,
} from "@/lib/affiliate-settings-form";
import { GLOBAL_HELP, LEVEL_HELP, hintKey, levelHintKey, type GlobalField } from "@/lib/affiliate-settings-help";
import { useAdminAffiliateSettings, useUpdateAffiliateSettings } from "@/lib/hooks/admin-affiliates";
import { useHasCapability } from "@/lib/hooks/use-capability";
import { useTranslation, type I18nKey } from "@/lib/i18n";

const TABS = ["general", "levels", "overrides", "jobs"] as const;
type Tab = (typeof TABS)[number];

interface Group {
  title: I18nKey;
  desc: I18nKey;
  fields: GlobalField[];
}

const GROUPS: Group[] = [
  { title: "admin_aff_grp_link", desc: "admin_aff_grp_link_desc", fields: ["cookie_days", "binding_months", "min_extend_amount", "launch_date"] },
  { title: "admin_aff_grp_wait", desc: "admin_aff_grp_wait_desc", fields: ["digital_wait_days", "physical_wait_days"] },
  { title: "admin_aff_grp_payout", desc: "admin_aff_grp_payout_desc", fields: ["min_payout", "payout_day_of_month", "freeze_max_days", "adjustment_threshold"] },
  { title: "admin_aff_grp_level", desc: "admin_aff_grp_level_desc", fields: ["eval_day", "silver_threshold", "gold_threshold"] },
  { title: "admin_aff_grp_signup", desc: "admin_aff_grp_signup_desc", fields: ["daily_registration_cap"] },
];

export default function AdminAffiliateSettingsPage() {
  return (
    <Suspense fallback={null}>
      <SettingsContent />
    </Suspense>
  );
}

function SettingsContent() {
  const { t } = useTranslation();
  const router = useRouter();
  const pathname = usePathname();
  const search = useSearchParams();
  const requested = search.get("tab");
  const tab: Tab = (TABS as readonly string[]).includes(requested ?? "") ? (requested as Tab) : "general";

  const canManage = useHasCapability("affiliates:*");
  const settings = useAdminAffiliateSettings();
  const update = useUpdateAffiliateSettings();
  const [form, setForm] = useState<SettingsForm | null>(null);

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

  const save = (e: React.FormEvent) => {
    e.preventDefault();
    const input = formToInput(form);
    if (!input) {
      toast.error(t("admin_aff_set_invalid"));
      return;
    }
    update.mutate(input, {
      onSuccess: () => toast.success(t("admin_aff_set_saved")),
      onError: (err) => toast.error(err instanceof ApiError ? err.message : t("aff_action_failed")),
    });
  };

  const changeTab = (value: string) => router.replace(value === "general" ? pathname : `${pathname}?tab=${value}`, { scroll: false });

  const setNumber = (key: GlobalNumberField, v: string) => setForm({ ...form, numbers: { ...form.numbers, [key]: v } });
  const setOptional = (key: GlobalOptionalField, v: string) => setForm({ ...form, optional: { ...form.optional, [key]: v } });
  const setLevelField = (level: string, key: keyof SettingsForm["levels"][string]["fields"], v: string) =>
    setForm({ ...form, levels: { ...form.levels, [level]: { ...form.levels[level], fields: { ...form.levels[level].fields, [key]: v } } } });
  const setLevelRule = (level: string, category: string, patch: Partial<RuleForm>) =>
    setForm({
      ...form,
      levels: {
        ...form.levels,
        [level]: {
          ...form.levels[level],
          rules: { ...form.levels[level].rules, [category]: { ...form.levels[level].rules[category], ...patch } },
        },
      },
    });

  const fieldLabel = (key: string) => t(`admin_aff_f_${key}` as I18nKey);

  const globalField = (key: GlobalField) => {
    const id = `g-${key}`;
    const help = GLOBAL_HELP[key];
    if (key === "launch_date") {
      return (
        <div key={key} className="space-y-1.5">
          <Label htmlFor={id}>{fieldLabel(key)}</Label>
          <Input id={id} type="date" value={form.launchDate} onChange={(e) => setForm({ ...form, launchDate: e.target.value })} />
          <p className="text-xs leading-relaxed text-ink-500">{t(hintKey(key))}</p>
        </div>
      );
    }
    const optional = key === "payout_day_of_month" || key === "daily_registration_cap";
    return (
      <SettingField
        key={key}
        id={id}
        label={fieldLabel(key)}
        hint={t(hintKey(key))}
        help={help}
        value={optional ? form.optional[key as GlobalOptionalField] : form.numbers[key as GlobalNumberField]}
        disabled={key === "adjustment_threshold"}
        onChange={(v) => (optional ? setOptional(key as GlobalOptionalField, v) : setNumber(key as GlobalNumberField, v))}
      />
    );
  };

  const guideFields = GROUPS.flatMap((g) => g.fields)
    .filter((key) => !(HIDDEN_NUMBER_FIELDS as readonly string[]).includes(key))
    .map((key) => ({
      column: fieldLabel(key),
      required: GLOBAL_HELP[key].required,
      ruleKey: hintKey(key),
      example: GLOBAL_HELP[key].example,
    }));

  const levelGuideFields = [
    { column: t("admin_aff_lvl_pct"), required: false, ruleKey: "admin_aff_rule_hint" as I18nKey, example: "10% / Rp5.000" },
    ...LEVEL_FIELDS.map((key) => ({
      column: fieldLabel(key),
      required: LEVEL_HELP[key].required,
      ruleKey: levelHintKey(key),
      example: LEVEL_HELP[key].example,
    })),
  ];

  const saveBar = (
    <div className="flex justify-end">
      <Button type="submit" disabled={update.isPending}>
        {t("admin_aff_set_save")}
      </Button>
    </div>
  );

  return (
    <div className="space-y-6">
      <AdminPageHeader icon={Settings} title={t("admin_aff_set_title")} description={t("admin_aff_set_desc")} />

      <Tabs value={tab} onValueChange={changeTab}>
        <TabsList>
          <TabsTrigger value="general">{t("admin_aff_tab_general")}</TabsTrigger>
          <TabsTrigger value="levels">{t("admin_aff_tab_levels")}</TabsTrigger>
          <TabsTrigger value="overrides">{t("admin_aff_tab_overrides")}</TabsTrigger>
          <TabsTrigger value="jobs">{t("admin_aff_tab_jobs")}</TabsTrigger>
        </TabsList>
      </Tabs>

      {tab === "general" && (
        <form onSubmit={save} className="space-y-6">
          <BulkFormatGuide fields={guideFields} pitfallKeys={[]} />
          {GROUPS.map((group) => (
            <section key={group.title} className="md-card-outlined space-y-4 p-5">
              <div>
                <h2 className="font-semibold text-ink-900">{t(group.title)}</h2>
                <p className="text-sm text-ink-600">{t(group.desc)}</p>
              </div>
              <div className="grid gap-5 sm:grid-cols-2 lg:grid-cols-3">
                {group.fields.filter((key) => !(HIDDEN_NUMBER_FIELDS as readonly string[]).includes(key)).map(globalField)}
              </div>
              {group.title === "admin_aff_grp_payout" && (
                <div className="space-y-1">
                  <label className="flex items-center gap-2 text-sm text-ink-500">
                    <input type="checkbox" checked={form.adjustmentThresholdEnabled} disabled readOnly />
                    {fieldLabel("adjustment_threshold_enabled")}
                  </label>
                  <p className="text-xs text-ink-500">{t("admin_aff_adjustment_locked")}</p>
                </div>
              )}
            </section>
          ))}
          {saveBar}
        </form>
      )}

      {tab === "levels" && (
        <form onSubmit={save} className="space-y-6">
          <BulkFormatGuide fields={levelGuideFields} pitfallKeys={["admin_aff_set_level_hint"]} />
          <div className="grid gap-4 lg:grid-cols-3">
            {AFFILIATE_LEVELS.map((level) => (
              <section key={level} className="md-card-outlined space-y-5 p-5">
                <h2 className="font-serif text-lg font-bold text-ink-900">{affLabel(t, level)}</h2>
                <div className="space-y-3">
                  <div>
                    <h3 className="text-sm font-semibold text-ink-900">{t("admin_aff_lvl_pct")}</h3>
                    <p className="text-xs text-ink-500">{t("admin_aff_rule_hint")}</p>
                  </div>
                  <div className="space-y-3">
                    {COMMISSION_CATEGORIES.map((category) => (
                      <RuleFieldCompact
                        key={category}
                        id={`l-${level}-rule-${category}`}
                        label={affLabel(t, category)}
                        rule={form.levels[level].rules[category]}
                        onChange={(patch) => setLevelRule(level, category, patch)}
                      />
                    ))}
                  </div>
                </div>
                <div className="space-y-4">
                  <h3 className="text-sm font-semibold text-ink-900">{t("admin_aff_lvl_rules")}</h3>
                  {LEVEL_FIELDS.map((key) => (
                    <SettingField
                      key={key}
                      id={`l-${level}-${key}`}
                      label={fieldLabel(key)}
                      hint={t(levelHintKey(key))}
                      help={LEVEL_HELP[key]}
                      value={form.levels[level].fields[key]}
                      placeholder={globalFallback(form, key)}
                      onChange={(v) => setLevelField(level, key, v)}
                    />
                  ))}
                </div>
              </section>
            ))}
          </div>
          {saveBar}
        </form>
      )}

      {tab === "overrides" && <OverridesPanel />}
      {tab === "jobs" && <JobsPanel />}
    </div>
  );
}

// A level field left empty falls back to the global value, so show that value as the placeholder.
function globalFallback(form: SettingsForm, key: (typeof LEVEL_FIELDS)[number]): string | undefined {
  if (key === "payout_day_of_month") return form.optional.payout_day_of_month || undefined;
  if (key === "rise_threshold" || key === "hold_threshold") return undefined;
  return form.numbers[key] || undefined;
}

function RuleFieldCompact({ id, label, rule, onChange }: { id: string; label: string; rule: RuleForm; onChange: (patch: Partial<RuleForm>) => void }) {
  const { t } = useTranslation();
  return (
    <div className="space-y-1">
      <Label htmlFor={id}>{label}</Label>
      <div className="grid grid-cols-[7.5rem_minmax(0,1fr)] gap-2">
        <select
          aria-label={`${label} ${t("admin_aff_rule_type")}`}
          className="h-9 rounded-md border border-ink-200 bg-white px-2 text-sm text-ink-900"
          value={rule.type}
          onChange={(e) => onChange({ type: e.target.value as RuleForm["type"] })}
        >
          {COMMISSION_RULE_TYPES.map((type) => (
            <option key={type} value={type}>
              {t(type === "fixed" ? "admin_aff_rule_fixed" : "admin_aff_rule_percent")}
            </option>
          ))}
        </select>
        <Input id={id} inputMode="decimal" value={rule.value} onChange={(e) => onChange({ value: e.target.value })} />
      </div>
    </div>
  );
}
