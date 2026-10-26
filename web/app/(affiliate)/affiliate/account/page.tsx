"use client";

import { useEffect, useState } from "react";
import { UserCog } from "lucide-react";
import { toast } from "sonner";
import { AdminPageHeader } from "@/components/admin/AdminPageHeader";
import { StatusBadge } from "@/components/affiliate/StatusBadge";
import { ChangePasswordForm } from "@/components/profile/ChangePasswordForm";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { ApiError } from "@/lib/api";
import { affLabel, formatAffDate } from "@/lib/affiliate-labels";
import { useAffiliateMe, useUpdateAffiliateBank } from "@/lib/hooks/affiliate";
import { useMe } from "@/lib/hooks/auth";
import { useTranslation } from "@/lib/i18n";

export default function AffiliateAccountPage() {
  const { t } = useTranslation();
  const me = useMe();
  const profile = useAffiliateMe();
  const saveBank = useUpdateAffiliateBank();

  const [bankName, setBankName] = useState("");
  const [bankNumber, setBankNumber] = useState("");
  const [bankHolder, setBankHolder] = useState("");

  useEffect(() => {
    if (!profile.data) return;
    setBankName(profile.data.bank_name ?? "");
    setBankNumber(profile.data.bank_account_number ?? "");
    setBankHolder(profile.data.bank_account_holder ?? "");
  }, [profile.data]);

  const canEditBank = profile.data?.status === "active";

  const onSaveBank = (e: React.FormEvent) => {
    e.preventDefault();
    if (!bankName.trim() || !bankNumber.trim() || !bankHolder.trim()) {
      toast.error(t("aff_bank_required"));
      return;
    }
    saveBank.mutate(
      {
        bank_name: bankName.trim(),
        bank_account_number: bankNumber.trim(),
        bank_account_holder: bankHolder.trim(),
      },
      {
        onSuccess: () => toast.success(t("aff_bank_saved")),
        onError: (err) => toast.error(err instanceof ApiError ? err.message : t("aff_error")),
      },
    );
  };

  const user = me.data;
  const p = profile.data;
  const rows: { label: string; value: React.ReactNode }[] = [
    { label: t("aff_acc_name"), value: user?.name ?? "—" },
    { label: t("aff_acc_email"), value: user?.email ?? "—" },
    { label: t("aff_acc_phone"), value: user?.phone ?? "—" },
    { label: t("aff_acc_status"), value: p ? <StatusBadge status={p.status} /> : "—" },
    { label: t("aff_acc_level"), value: p ? affLabel(t, p.level) : "—" },
    { label: t("aff_acc_joined"), value: formatAffDate(user?.created_at) },
    { label: t("aff_acc_code"), value: p?.code || "—" },
    {
      label: t("aff_acc_terms"),
      value: p?.terms_version
        ? t("aff_acc_terms_value").replace("{version}", p.terms_version).replace("{date}", formatAffDate(p.terms_accepted_at))
        : "—",
    },
  ];

  return (
    <div className="space-y-6">
      <AdminPageHeader icon={UserCog} title={t("aff_acc_title")} description={t("aff_acc_desc")} />

      <section className="md-card-outlined space-y-4 p-5">
        <h2 className="font-semibold text-ink-900">{t("aff_acc_info")}</h2>
        <dl className="grid gap-4 sm:grid-cols-2">
          {rows.map((row) => (
            <div key={row.label}>
              <dt className="text-xs text-ink-500">{row.label}</dt>
              <dd className="mt-0.5 text-sm font-medium break-words text-ink-900">{row.value}</dd>
            </div>
          ))}
        </dl>
      </section>

      <section className="md-card-outlined space-y-3 p-5">
        <div>
          <h2 className="font-semibold text-ink-900">{t("aff_bank_title")}</h2>
          <p className="text-sm text-ink-600">{t("aff_acc_bank_desc")}</p>
        </div>
        {p && !canEditBank && <p className="text-sm text-warn">{t("aff_acc_bank_locked")}</p>}
        <form onSubmit={onSaveBank} className="grid gap-3 sm:grid-cols-3">
          <div>
            <Label htmlFor="bank_name">{t("aff_bank_name")}</Label>
            <Input id="bank_name" value={bankName} onChange={(e) => setBankName(e.target.value)} disabled={!canEditBank} />
          </div>
          <div>
            <Label htmlFor="bank_number">{t("aff_bank_number")}</Label>
            <Input
              id="bank_number"
              value={bankNumber}
              onChange={(e) => setBankNumber(e.target.value)}
              inputMode="numeric"
              disabled={!canEditBank}
            />
          </div>
          <div>
            <Label htmlFor="bank_holder">{t("aff_bank_holder")}</Label>
            <Input id="bank_holder" value={bankHolder} onChange={(e) => setBankHolder(e.target.value)} disabled={!canEditBank} />
          </div>
          <div className="sm:col-span-3">
            <Button type="submit" disabled={!canEditBank || saveBank.isPending}>
              {t("aff_bank_save")}
            </Button>
          </div>
        </form>
      </section>

      <section className="md-card-outlined space-y-3 p-5">
        <div>
          <h2 className="font-semibold text-ink-900">{t("aff_acc_password")}</h2>
          <p className="text-sm text-ink-600">{t("aff_acc_password_desc")}</p>
        </div>
        <ChangePasswordForm />
      </section>
    </div>
  );
}
