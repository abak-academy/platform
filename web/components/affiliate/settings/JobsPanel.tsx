"use client";

import { useState } from "react";
import { toast } from "sonner";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { ApiError } from "@/lib/api";
import { affLabel } from "@/lib/affiliate-labels";
import { useRunAffiliateJob } from "@/lib/hooks/admin-affiliates";
import { useTranslation } from "@/lib/i18n";

const JOB_SCOPES = ["bindings", "commissions", "flags", "levels"] as const;

export function JobsPanel() {
  const { t } = useTranslation();
  const runJob = useRunAffiliateJob();
  const [scope, setScope] = useState<string>(JOB_SCOPES[0]);
  const [quarter, setQuarter] = useState("");

  const run = (e: React.FormEvent) => {
    e.preventDefault();
    runJob.mutate(
      { scope, quarter: scope === "levels" && quarter.trim() ? quarter.trim() : undefined },
      {
        onSuccess: () => toast.success(t("admin_aff_job_done")),
        onError: (err) => toast.error(err instanceof ApiError ? err.message : t("aff_action_failed")),
      },
    );
  };

  return (
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
            value={scope}
            onChange={(e) => setScope(e.target.value)}
            className="h-9 rounded-md border border-line bg-surface px-3 text-sm"
          >
            {JOB_SCOPES.map((s) => (
              <option key={s} value={s}>
                {affLabel(t, s)}
              </option>
            ))}
          </select>
        </div>
        {scope === "levels" && (
          <div className="space-y-1">
            <Label htmlFor="job-quarter">{t("admin_aff_job_quarter")}</Label>
            <Input id="job-quarter" value={quarter} onChange={(e) => setQuarter(e.target.value)} placeholder="2026-Q3" />
          </div>
        )}
        <Button type="submit" disabled={runJob.isPending}>
          {t("admin_aff_job_run")}
        </Button>
      </form>
    </section>
  );
}
