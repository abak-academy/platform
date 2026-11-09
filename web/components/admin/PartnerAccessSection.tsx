"use client";

import { useEffect, useState } from "react";
import { PartnerPicker } from "@/components/admin/PartnerPicker";
import { Button } from "@/components/ui/button";
import { usePartnerProductAccess, useSetPartnerProductAccess } from "@/lib/hooks/partner";
import type { PartnerAccessScope } from "@/lib/types";

const SCOPES: { value: PartnerAccessScope; label: string; hint: string }[] = [
  { value: "none", label: "Tidak tampil", hint: "Tidak ada Mitra yang dapat melihat atau membeli produk ini." },
  { value: "all", label: "Semua Mitra", hint: "Setiap Mitra aktif dapat melihat dan membeli produk ini." },
  { value: "selected", label: "Mitra terpilih", hint: "Hanya Mitra yang dipilih di bawah." },
];

export interface PartnerAccessDraft {
  scope: PartnerAccessScope;
  ids: string[];
  setScope: (scope: PartnerAccessScope) => void;
  setIds: (ids: string[]) => void;
  isLoading: boolean;
  isError: boolean;
  /** The draft differs from what the server has. */
  dirty: boolean;
  /** "selected" needs at least one Mitra. */
  valid: boolean;
  save: () => Promise<unknown>;
}

// usePartnerAccessDraft holds the Mitra access being edited. It is saved through
// its own endpoint, apart from the product, so saving the product can never
// overwrite it; the product form saves both from its one Simpan button.
export function usePartnerAccessDraft(productId: string | null): PartnerAccessDraft {
  const access = usePartnerProductAccess(productId);
  const save = useSetPartnerProductAccess();
  const [scope, setScope] = useState<PartnerAccessScope>("none");
  const [ids, setIds] = useState<string[]>([]);

  useEffect(() => {
    if (!access.data) return;
    setScope(access.data.scope);
    setIds(access.data.ids);
  }, [access.data]);

  const sent = scope === "selected" ? ids : [];
  const server = access.data;
  const same = (a: string[], b: string[]) => a.length === b.length && [...a].sort().every((v, i) => v === [...b].sort()[i]);
  const dirty = Boolean(server) && (server!.scope !== scope || !same(server!.scope === "selected" ? server!.ids : [], sent));

  return {
    scope,
    ids,
    setScope,
    setIds,
    isLoading: access.isLoading,
    isError: access.isError,
    dirty,
    valid: scope !== "selected" || ids.length > 0,
    save: () => save.mutateAsync({ productId: productId as string, access: { scope, ids: sent } }),
  };
}

// PartnerAccessSection sets which Mitra see and can buy a product.
export function PartnerAccessSection({ draft, disabled }: { draft: PartnerAccessDraft; disabled?: boolean }) {
  const { scope, ids, setScope, setIds } = draft;

  return (
    <fieldset className="grid gap-3 rounded-lg border border-line p-4" aria-label="Akses Mitra" disabled={disabled}>
      <legend className="px-1 text-sm font-semibold text-ink-900">Akses Mitra</legend>
      {draft.isLoading ? (
        <p className="text-sm text-ink-500">Memuat...</p>
      ) : draft.isError ? (
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
          {!draft.valid && <p className="text-xs text-danger">Pilih minimal satu Mitra.</p>}
          <p className="text-xs text-ink-500">Disimpan bersama produk saat menekan Simpan.</p>
        </>
      )}
    </fieldset>
  );
}
