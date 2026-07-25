import type { ProductSpec, ProductType } from "@/lib/types";

// Canonical specification fields per product type. This catalogue lives in the
// frontend on purpose: the backend stores whatever it is given and only bounds
// the shape. Keys are canonical from day one so a future facet filter needs
// value normalisation, not a key clean-up across every product.
export const SPEC_FIELDS: Record<ProductType, { key: string; label: string }[]> = {
  book: [
    { key: "penerbit", label: "Perusahaan Penerbit" },
    { key: "tahun_terbit", label: "Tahun Terbit" },
    { key: "bahasa", label: "Bahasa" },
    { key: "jenis_cover", label: "Jenis Cover" },
    { key: "jenis_edisi", label: "Jenis Edisi" },
    { key: "jumlah_halaman", label: "Jumlah Halaman" },
    { key: "isbn", label: "ISBN" },
    { key: "impor_lokal", label: "Impor/Lokal" },
  ],
  merchandise: [
    { key: "bahan", label: "Bahan" },
    { key: "ukuran", label: "Ukuran" },
    { key: "warna", label: "Warna" },
    { key: "isi_paket", label: "Isi Paket" },
  ],
  medal: [
    { key: "bahan", label: "Bahan" },
    { key: "diameter", label: "Diameter" },
    { key: "finishing", label: "Finishing" },
    { key: "kemasan", label: "Kemasan" },
  ],
  course: [],
  exam: [],
};

// specRowsForType merges the canonical field list with whatever is already
// saved: canonical fields first in catalogue order carrying any saved value,
// then operator-added custom rows in their stored order.
export function specRowsForType(type: ProductType, saved: ProductSpec[]): ProductSpec[] {
  const canonical = SPEC_FIELDS[type] ?? [];
  const canonicalKeys = new Set(canonical.map((f) => f.key));

  const rows: ProductSpec[] = canonical.map((f) => {
    const hit = saved.find((s) => s.key === f.key);
    return { key: f.key, label: f.label, value: hit?.value ?? "" };
  });

  for (const s of saved) {
    if (!canonicalKeys.has(s.key)) rows.push({ ...s });
  }

  return rows;
}
