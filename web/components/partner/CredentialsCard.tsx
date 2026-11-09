"use client";

import { Copy } from "lucide-react";
import { toast } from "sonner";
import { Button } from "@/components/ui/button";
import { useTranslation } from "@/lib/i18n";

// The temporary password is shown once: the server keeps only its hash.
export function CredentialsCard({ username, password }: { username: string; password: string }) {
  const { t } = useTranslation();
  const copy = async (value: string) => {
    try {
      await navigator.clipboard.writeText(value);
      toast.success(t("partner_copied"));
    } catch {
      toast.error(t("partner_copy_failed"));
    }
  };
  const row = (label: string, value: string) => (
    <div className="flex items-center justify-between gap-3">
      <div>
        <div className="text-xs text-ink-500">{label}</div>
        <div className="font-mono text-sm font-semibold text-ink-900">{value}</div>
      </div>
      <Button type="button" size="icon-xs" variant="ghost" aria-label={`${t("partner_copy")} ${label}`} onClick={() => copy(value)}>
        <Copy className="size-3.5" />
      </Button>
    </div>
  );
  return (
    <div className="md-card-outlined space-y-3 p-4">
      {row(t("partner_reg_username"), username)}
      {row(t("partner_reg_password"), password)}
      <p className="text-xs text-warn">{t("partner_reg_once")}</p>
    </div>
  );
}
