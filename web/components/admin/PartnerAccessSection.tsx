"use client";

import { useEffect, useState } from "react";
import { toast } from "sonner";
import { PartnerPicker } from "@/components/admin/PartnerPicker";
import { Button } from "@/components/ui/button";
import { ApiError } from "@/lib/api";
import { usePartnerProductAccess, useSetPartnerProductAccess } from "@/lib/hooks/partner";
import type { PartnerAccessScope } from "@/lib/types";

const SCOPES: { value: PartnerAccessScope; label: string; hint: string }[] = [
  { value: "none", label: "Tidak tampil", hint: "Tidak ada Mitra yang dapat melihat atau membeli produk ini." },
  { value: "all", label: "Semua Mitra", hint: "Setiap Mitra aktif dapat melihat dan membeli produk ini." },
  { value: "selected", label: "Mitra terpilih", hint: "Hanya Mitra yang dipilih di bawah." },
];

// PartnerAccessSection sets which Mitra see and can buy a product. It saves on
// its own, through an endpoint apart from the product form, so saving the form
// can never overwrite it.
export function PartnerAccessSection({ productId, disabled }: { productId: string; disabled?: boolean }) {
  const access = usePartnerProductAccess(productId);
  const save = useSetPartnerProductAccess();
  const [scope, setScope] = useState<PartnerAccessScope>("none");
  const [ids, setIds] = useState<string[]>([]);

  useEffect(() => {
    if (!access.data) return;
    setScope(access.data.scope);
    setIds(access.data.ids);
  }, [access.data]);

  const submit = () =>
    save.mutate(
      { productId, access: { scope, ids: scope === "selected" ? ids : [] } },
      {
        onSuccess: () => toast.success("Akses Mitra disimpan"),
        onError: (err) => toast.error(err instanceof ApiError ? err.message : "Gagal menyimpan akses Mitra"),
      },
    );

  return (
    <fieldset className="grid gap-3 rounded-lg border border-line p-4" aria-label="Akses Mitra" disabled={disabled}>
      <legend className="px-1 text-sm font-semibold text-ink-900">Akses Mitra</legend>
      {access.isLoading ? (
        <p className="text-sm text-ink-500">Memuat...</p>
      ) : access.isError ? (
        <p role="alert" className="text-sm text-danger">
          Gagal memuat akses Mitra.
        </p>
      ) : (
        <>
          <div className="grid gap-2">
            {SCOPES.map((option) => (
              <label key={option.value} className="flex items-start gap-2 text-sm">
                <input
                  type="radio"
                  name="partner-scope"
                  value={option.value}
                  checked={scope === option.value}
                  onChange={() => setScope(option.value)}
                  className="mt-1"
                />
                <span>
                  <span className="font-medium text-ink-900">{option.label}</span>
                  <span className="block text-xs text-ink-500">{option.hint}</span>
                </span>
              </label>
            ))}
          </div>
          {scope === "selected" && (
            <PartnerPicker value={ids} onChange={setIds} emptyLabel="Pilih minimal satu Mitra" searchLabel="Cari Mitra" />
          )}
          <div className="flex justify-end">
            <Button type="button" size="sm" onClick={submit} disabled={save.isPending || (scope === "selected" && ids.length === 0)}>
              Simpan akses Mitra
            </Button>
          </div>
        </>
      )}
    </fieldset>
  );
}
