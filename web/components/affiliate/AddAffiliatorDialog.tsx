"use client";

import { useState } from "react";
import { toast } from "sonner";
import { Download } from "lucide-react";
import { BulkFormatGuide } from "@/components/admin/BulkFormatGuide";
import { Button } from "@/components/ui/button";
import { Dialog, DialogContent, DialogFooter, DialogHeader, DialogTitle } from "@/components/ui/dialog";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { ApiError } from "@/lib/api";
import {
  AFFILIATOR_BULK_FIELDS,
  AFFILIATOR_GUIDE_PITFALL_KEYS,
  AFFILIATOR_TEMPLATE_HEADER,
  AFFILIATOR_TEMPLATE_ROWS,
  downloadTextFile,
} from "@/lib/bulk-import-format";
import { useBulkCreateAffiliators, useCreateAffiliator } from "@/lib/hooks/admin-affiliates";
import { useTranslation } from "@/lib/i18n";
import type { AffiliateCreateInput, AffiliateCreated } from "@/lib/types";

const BULK_MAX = 200;

export function parseAffiliatorCsv(text: string): AffiliateCreateInput[] {
  const lines = text.split(/\r?\n/).map((l) => l.trim()).filter(Boolean);
  const rows = lines.map((l) => l.split(",").map((c) => c.trim().replace(/^"|"$/g, "")));
  const header = rows[0]?.map((c) => c.toLowerCase());
  const hasHeader = header?.[0] === "name" && header?.[1] === "email";
  return (hasHeader ? rows.slice(1) : rows).map(([name = "", email = "", phone = "", password = ""]) => ({ name, email, phone, password }));
}

function credentialsCsv(items: AffiliateCreated[]): string {
  const esc = (v: string) => `"${v.replace(/"/g, '""')}"`;
  return ["name,email,phone,code,temp_password", ...items.map((c) => [c.name, c.email, c.phone, c.code, c.temp_password].map(esc).join(","))].join("\n");
}

function downloadCsv(items: AffiliateCreated[]) {
  const url = URL.createObjectURL(new Blob([credentialsCsv(items)], { type: "text/csv" }));
  const a = document.createElement("a");
  a.href = url;
  a.download = "affiliator-credentials.csv";
  a.click();
  URL.revokeObjectURL(url);
}

export function AddAffiliatorDialog({ open, onOpenChange }: { open: boolean; onOpenChange: (open: boolean) => void }) {
  const { t } = useTranslation();
  const [mode, setMode] = useState<"single" | "bulk">("single");
  const [form, setForm] = useState<AffiliateCreateInput>({ name: "", email: "", phone: "", password: "" });
  const [file, setFile] = useState<File | null>(null);
  const [defaultPassword, setDefaultPassword] = useState("");
  const [created, setCreated] = useState<AffiliateCreated[]>([]);
  const [failures, setFailures] = useState<string[]>([]);
  const create = useCreateAffiliator();
  const bulk = useBulkCreateAffiliators();
  const pending = create.isPending || bulk.isPending;

  const close = () => {
    onOpenChange(false);
    setForm({ name: "", email: "", phone: "", password: "" });
    setFile(null);
    setDefaultPassword("");
    setCreated([]);
    setFailures([]);
  };
  const fail = (err: unknown) => toast.error(err instanceof ApiError ? err.message : t("aff_action_failed"));

  const submitSingle = () => {
    if (!form.name.trim() || !form.email.trim() || !form.phone.trim()) {
      toast.error(t("admin_aff_act_invalid"));
      return;
    }
    create.mutate(form, { onSuccess: (c) => setCreated([c]), onError: fail });
  };

  const onUpload = async () => {
    if (!file) {
      toast.error(t("bulk_register_no_file"));
      return;
    }
    const rows = parseAffiliatorCsv(await file.text()).map((r) => ({ ...r, password: r.password || defaultPassword }));
    if (rows.length === 0 || rows.length > BULK_MAX) {
      toast.error(t("admin_aff_add_bulk_hint"));
      return;
    }
    bulk.mutate(rows, {
      onSuccess: ({ data }) => {
        const ok = data.flatMap((r) => (r.created ? [r.created] : []));
        const bad = data.filter((r) => r.error);
        setCreated(ok);
        setFailures(
          bad.map((r) =>
            t("admin_aff_add_failed_row")
              .replace("{row}", String(r.row))
              .replace("{error}", t(`admin_aff_add_err_${r.error}` as Parameters<typeof t>[0])),
          ),
        );
        toast.success(t("admin_aff_add_done").replace("{ok}", String(ok.length)).replace("{fail}", String(bad.length)));
      },
      onError: fail,
    });
  };

  const showResult = created.length > 0 || failures.length > 0;

  return (
    <Dialog open={open} onOpenChange={(next) => !next && close()}>
      <DialogContent className="sm:max-h-[90vh] sm:max-w-xl overflow-y-auto">
        <DialogHeader>
          <DialogTitle>{t("admin_aff_add_title")}</DialogTitle>
        </DialogHeader>
        {showResult ? (
          <div className="space-y-3 text-sm">
            <p className="text-ink-700">{t("admin_aff_add_creds")}</p>
            <ul className="max-h-48 space-y-1 overflow-auto">
              {created.map((c) => (
                <li key={c.id} className="font-mono text-xs">
                  {c.email} · {c.code} · {c.temp_password}
                </li>
              ))}
            </ul>
            {failures.map((f) => (
              <p key={f} className="text-danger">
                {f}
              </p>
            ))}
            {created.length > 0 && (
              <Button variant="outline" size="sm" onClick={() => downloadCsv(created)}>
                {t("admin_aff_add_download")}
              </Button>
            )}
          </div>
        ) : (
          <div className="space-y-3">
            <div className="flex overflow-hidden rounded-[10px] border border-line" role="group">
              {(["single", "bulk"] as const).map((m) => (
                <button
                  key={m}
                  type="button"
                  aria-pressed={mode === m}
                  onClick={() => setMode(m)}
                  className={`flex-1 border-r border-line px-3 py-2 text-sm transition-colors last:border-r-0 ${
                    mode === m
                      ? "bg-primary font-semibold text-primary-foreground hover:bg-primary/90"
                      : "bg-surface text-ink-700 hover:bg-brand-50 hover:text-brand-700"
                  }`}
                >
                  {t(m === "single" ? "admin_aff_add_single" : "admin_aff_add_bulk")}
                </button>
              ))}
            </div>
            {mode === "single" ? (
              (["name", "email", "phone", "password"] as const).map((key) => (
                <div key={key} className="space-y-1">
                  <Label htmlFor={`aff-add-${key}`}>{t(`admin_aff_add_${key}` as Parameters<typeof t>[0])}</Label>
                  <Input
                    id={`aff-add-${key}`}
                    type={key === "email" ? "email" : key === "phone" ? "tel" : key === "password" ? "password" : "text"}
                    autoComplete={key === "password" ? "new-password" : undefined}
                    value={form[key] ?? ""}
                    onChange={(e) => setForm({ ...form, [key]: e.target.value })}
                    disabled={pending}
                  />
                  {key === "password" && <p className="text-xs text-ink-500">{t("admin_aff_add_password_hint")}</p>}
                </div>
              ))
            ) : (
              <div className="space-y-4">
                <section className="space-y-2">
                  <h3 className="text-sm font-semibold text-ink-900">{t("admin_aff_add_step_template")}</h3>
                  <Button
                    type="button"
                    variant="outline"
                    size="sm"
                    className="rounded-full"
                    onClick={() =>
                      downloadTextFile(
                        "affiliator_template.csv",
                        [AFFILIATOR_TEMPLATE_HEADER, ...AFFILIATOR_TEMPLATE_ROWS].join("\n"),
                        "text/csv;charset=utf-8",
                      )
                    }
                  >
                    <Download className="mr-2 size-4" aria-hidden />
                    {t("admin_aff_add_template")}
                  </Button>
                  <BulkFormatGuide fields={AFFILIATOR_BULK_FIELDS} pitfallKeys={AFFILIATOR_GUIDE_PITFALL_KEYS} />
                </section>
                <section className="space-y-2">
                  <h3 className="text-sm font-semibold text-ink-900">{t("admin_aff_add_step_upload")}</h3>
                  <div className="space-y-1">
                    <Label htmlFor="aff-add-file">{t("bulk_register_choose_file")}</Label>
                    <Input
                      id="aff-add-file"
                      type="file"
                      accept=".csv,text/csv"
                      disabled={pending}
                      onChange={(e) => setFile(e.target.files?.[0] ?? null)}
                    />
                    {file && <p className="text-sm text-ink-600">{file.name}</p>}
                    <p className="text-xs text-ink-500">{t("admin_aff_add_bulk_hint")}</p>
                  </div>
                  <div className="space-y-1">
                    <Label htmlFor="aff-add-default-password">{t("admin_aff_add_default_password")}</Label>
                    <Input
                      id="aff-add-default-password"
                      type="password"
                      autoComplete="new-password"
                      value={defaultPassword}
                      onChange={(e) => setDefaultPassword(e.target.value)}
                      disabled={pending}
                    />
                    <p className="text-xs text-ink-500">{t("admin_aff_add_default_password_hint")}</p>
                  </div>
                  <Button type="button" className="rounded-full" onClick={onUpload} disabled={pending || !file}>
                    {t("bulk_register_upload")}
                  </Button>
                </section>
              </div>
            )}
          </div>
        )}
        <DialogFooter>
          <Button variant="outline" onClick={close}>
            {t("cancel")}
          </Button>
          {!showResult && mode === "single" && (
            <Button onClick={submitSingle} disabled={pending}>
              {t("admin_aff_add_submit")}
            </Button>
          )}
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
