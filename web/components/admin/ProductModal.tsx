"use client";

import { useEffect, useRef, useState } from "react";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { useAdminCourses } from "@/lib/hooks/admin-courses";
import { useExams } from "@/lib/hooks/admin-exams";
import { usePresignUpload } from "@/lib/hooks/students";
import { fileUrl } from "@/lib/api";
import { ProductSpecsEditor } from "@/components/admin/ProductSpecsEditor";
import type { Product, ProductType, ProductStatus, ProductSpec, AdminCreateProductInput, AdminUpdateProductInput } from "@/lib/types";
import { useResolvedAdminRole } from "@/lib/hooks/use-capability";
import { writableProductTypes } from "@/lib/product-types";

interface ProductModalProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  product?: Product | null;
  onSubmit: (input: AdminCreateProductInput | AdminUpdateProductInput) => void;
  isPending: boolean;
}

const SELECT_CLASS =
  "h-9 w-full rounded-md border border-input bg-transparent px-2 text-sm outline-none focus-visible:border-ring focus-visible:ring-[3px] focus-visible:ring-ring/50 disabled:opacity-50";

const PRODUCT_STATUSES: ProductStatus[] = ["draft", "published", "hidden", "archived"];

const TYPE_LABELS: Record<ProductType, string> = {
  book: "Buku",
  course: "Kursus",
  exam: "Ujian",
  merchandise: "Merchandise",
  medal: "Medali",
};

const STATUS_LABELS: Record<ProductStatus, string> = {
  draft: "Draft",
  published: "Dipublikasikan",
  hidden: "Disembunyikan",
  archived: "Diarsipkan",
};

// datetime-local <-> ISO conversions for the availability window.
function toLocalInput(iso?: string | null): string {
  if (!iso) return "";
  const d = new Date(iso);
  if (Number.isNaN(d.getTime())) return "";
  const pad = (n: number) => String(n).padStart(2, "0");
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())}T${pad(d.getHours())}:${pad(d.getMinutes())}`;
}
function fromLocalInput(v: string): string | null {
  if (!v) return null;
  const d = new Date(v);
  return Number.isNaN(d.getTime()) ? null : d.toISOString();
}

export function ProductModal({ open, onOpenChange, product, onSubmit, isPending }: ProductModalProps) {
  const isEdit = Boolean(product);
  const { role } = useResolvedAdminRole();
  const productTypes = writableProductTypes(role);
  const [name, setName] = useState("");
  const [type, setType] = useState<ProductType | "">("");
  const [price, setPrice] = useState("");
  const [stock, setStock] = useState("");
  const [weight, setWeight] = useState("");
  const [imageUrl, setImageUrl] = useState("");
  const [imageUploading, setImageUploading] = useState(false);
  const [status, setStatus] = useState<ProductStatus>("draft");
  const [description, setDescription] = useState("");
  const [courseIds, setCourseIds] = useState<string[]>([]);
  const [examIds, setExamIds] = useState<string[]>([]);
  const [availableFrom, setAvailableFrom] = useState("");
  const [availableUntil, setAvailableUntil] = useState("");
  const [specs, setSpecs] = useState<ProductSpec[]>([]);
  const { data: courses } = useAdminCourses();
  const { data: examsResp } = useExams();
  const presign = usePresignUpload();
  const imageInputRef = useRef<HTMLInputElement>(null);
  const exams = examsResp?.data ?? [];

  useEffect(() => {
    if (open) {
      if (product) {
        setName(product.name ?? "");
        setType(product.type ?? "");
        setPrice(String(product.price ?? ""));
        setStock(product.stock != null ? String(product.stock) : "");
        setWeight(product.weight_grams != null ? String(product.weight_grams) : "");
        setImageUrl(product.image_url ?? "");
        setStatus(product.status ?? "draft");
        setDescription(product.description ?? "");
        setCourseIds(product.course_ids ?? []);
        setExamIds(product.exam_ids ?? []);
        setAvailableFrom(toLocalInput(product.available_from));
        setAvailableUntil(toLocalInput(product.available_until));
        setSpecs(product.specs ?? []);
      } else {
        setName("");
        setType("");
        setPrice("");
        setStock("");
        setWeight("");
        setImageUrl("");
        setStatus("draft");
        setDescription("");
        setCourseIds([]);
        setExamIds([]);
        setAvailableFrom("");
        setAvailableUntil("");
        setSpecs([]);
      }
    }
  }, [open, product]);

  const showStock = type === "book" || type === "merchandise" || type === "medal";

  async function handleImageSelect(e: React.ChangeEvent<HTMLInputElement>) {
    const file = e.target.files?.[0];
    if (!file) return;
    setImageUploading(true);
    try {
      const presigned = await presign.mutateAsync({ filename: file.name, content_type: file.type, kind: "product" });
      const res = await fetch(presigned.url, {
        method: "PUT",
        body: file,
        headers: { "Content-Type": file.type },
      });
      if (!res.ok) throw new Error(`Upload failed: ${res.status}`);
      setImageUrl(presigned.key);
    } catch {
      // upload failed; leave existing image untouched
    } finally {
      setImageUploading(false);
      if (imageInputRef.current) imageInputRef.current.value = "";
    }
  }
  const effectiveType = isEdit ? product?.type : type;
  const showCourses = effectiveType === "course";
  const showExams = effectiveType === "exam";
  const canSubmit =
    name.trim() !== "" &&
    (isEdit || type !== "") &&
    price !== "" &&
    (!showCourses || courseIds.length > 0) &&
    (!showExams || examIds.length > 0);

  function handleSubmit(e: React.FormEvent) {
    e.preventDefault();
    if (!canSubmit || isPending) return;

    const base = {
      name: name.trim(),
      description: description.trim() || undefined,
      price: Number(price),
    };

    if (isEdit) {
      const input: AdminUpdateProductInput = {
        ...base,
        status,
        // Always send the window on update so clearing a field persists (the
        // backend preserves only when the key is absent).
        available_from: fromLocalInput(availableFrom),
        available_until: fromLocalInput(availableUntil),
        ...(showStock ? { stock: Number(stock) } : {}),
        ...(showStock && weight !== "" ? { weight_grams: Number(weight) } : {}),
        ...(imageUrl !== "" ? { image_url: imageUrl } : {}),
        specs: specs.filter((s) => s.label.trim() !== "" && s.value.trim() !== ""),
        ...(showCourses && courseIds.length > 0 ? { course_ids: courseIds } : {}),
        ...(showExams && examIds.length > 0 ? { exam_ids: examIds } : {}),
      };
      onSubmit(input);
      return;
    }

    if (!type) return;
    const input: AdminCreateProductInput = {
      ...base,
      type,
      ...(availableFrom ? { available_from: fromLocalInput(availableFrom) } : {}),
      ...(availableUntil ? { available_until: fromLocalInput(availableUntil) } : {}),
      ...(showStock ? { stock: Number(stock) } : {}),
      ...(showStock && weight !== "" ? { weight_grams: Number(weight) } : {}),
      ...(imageUrl !== "" ? { image_url: imageUrl } : {}),
      specs: specs.filter((s) => s.label.trim() !== "" && s.value.trim() !== ""),
      ...(showCourses && courseIds.length > 0 ? { course_ids: courseIds } : {}),
      ...(showExams && examIds.length > 0 ? { exam_ids: examIds } : {}),
    };
    onSubmit(input);
  }

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      {/* A book carries eight specification rows, which pushed the dialog past
          the viewport and took the title and the Save button off screen with
          it. The body scrolls; the header and footer stay put. */}
      <DialogContent className="flex max-h-[90vh] flex-col gap-0 overflow-hidden p-0 sm:max-w-3xl">
        <form onSubmit={handleSubmit} className="flex min-h-0 flex-1 flex-col">
          <DialogHeader className="shrink-0 border-b px-6 py-5">
            <DialogTitle>{isEdit ? "Edit produk" : "Buat produk"}</DialogTitle>
            <DialogDescription>
              {isEdit ? "Perbarui metadata produk." : "Tambahkan produk baru ke katalog."}
            </DialogDescription>
          </DialogHeader>

          <div className="min-h-0 flex-1 space-y-7 overflow-y-auto px-6 py-5">
            <Section title="Produk">
              <div className="grid gap-2">
                <Label htmlFor="product-name">Nama</Label>
                <Input
                  id="product-name"
                  value={name}
                  onChange={(e) => setName(e.target.value)}
                  placeholder="Nama produk"
                  disabled={isPending}
                />
              </div>

              <div className="grid grid-cols-1 gap-4 sm:grid-cols-2">
                <div className="grid gap-2">
                  <Label htmlFor="product-type">Jenis</Label>
                  <select
                    id="product-type"
                    value={type}
                    onChange={(e) => setType(e.target.value as ProductType)}
                    disabled={isEdit || isPending}
                    className={SELECT_CLASS}
                  >
                    <option value="" disabled>Pilih jenis</option>
                    {productTypes.map((t) => (
                      <option key={t} value={t}>
                        {TYPE_LABELS[t]}
                      </option>
                    ))}
                  </select>
                </div>

                <div className="grid gap-2">
                  <Label htmlFor="product-price">Harga (IDR)</Label>
                  <Input
                    id="product-price"
                    type="number"
                    min={0}
                    value={price}
                    onChange={(e) => setPrice(e.target.value)}
                    placeholder="0"
                    disabled={isPending}
                  />
                </div>
              </div>

              {isEdit && (
                <div className="grid gap-2 sm:max-w-[calc(50%-0.5rem)]">
                  <Label htmlFor="product-status">Status</Label>
                  <select
                    id="product-status"
                    value={status}
                    onChange={(e) => setStatus(e.target.value as ProductStatus)}
                    disabled={isPending}
                    className={SELECT_CLASS}
                  >
                    {PRODUCT_STATUSES.map((s) => (
                      <option key={s} value={s}>
                        {STATUS_LABELS[s]}
                      </option>
                    ))}
                  </select>
                </div>
              )}
            </Section>

            {showStock && (
              <Section title="Inventaris & pengiriman">
                <div className="grid grid-cols-1 items-start gap-4 sm:grid-cols-2">
                  <div className="grid gap-2">
                    <Label htmlFor="product-stock">Stok</Label>
                    <Input
                      id="product-stock"
                      type="number"
                      min={0}
                      value={stock}
                      onChange={(e) => setStock(e.target.value)}
                      placeholder="0"
                      disabled={isPending}
                    />
                  </div>
                  <div className="grid gap-2">
                    <Label htmlFor="product-weight">Berat (gram)</Label>
                    <Input
                      id="product-weight"
                      type="number"
                      min={0}
                      value={weight}
                      onChange={(e) => setWeight(e.target.value)}
                      placeholder="0"
                      disabled={isPending}
                    />
                    <p className="text-xs text-muted-foreground">Dipakai untuk menghitung ongkir.</p>
                  </div>
                </div>
              </Section>
            )}

            <Section title="Gambar produk">
              <div className="grid gap-2">
                <Label htmlFor="product-image">Gambar produk</Label>
                <div className="flex items-center gap-3">
                  {imageUrl && (
                    // eslint-disable-next-line @next/next/no-img-element
                    <img
                      src={fileUrl(imageUrl)}
                      alt="Pratinjau gambar"
                      className="h-16 w-16 shrink-0 rounded-md border border-input object-cover"
                    />
                  )}
                  <Input
                    id="product-image"
                    type="file"
                    accept="image/*"
                    ref={imageInputRef}
                    onChange={handleImageSelect}
                    disabled={isPending || imageUploading}
                  />
                </div>
              </div>
            </Section>

            {showCourses && (
              <Section title="Kursus terkait">
                <div className="max-h-40 overflow-y-auto rounded-md border border-input p-2">
                  {(courses ?? []).length === 0 ? (
                    <p className="px-1 py-2 text-sm text-muted-foreground">Belum ada kursus.</p>
                  ) : (
                    (courses ?? []).map((c) => {
                      const checked = courseIds.includes(c.id);
                      return (
                        <label key={c.id} className="flex items-center gap-2 px-1 py-1.5 text-sm">
                          <input
                            type="checkbox"
                            checked={checked}
                            disabled={isPending}
                            onChange={(e) =>
                              setCourseIds((prev) =>
                                e.target.checked ? [...prev, c.id] : prev.filter((id) => id !== c.id)
                              )
                            }
                          />
                          <span>{c.title}</span>
                        </label>
                      );
                    })
                  )}
                </div>
              </Section>
            )}

            {showExams && (
              <Section title="Ujian terkait">
                <div className="max-h-40 overflow-y-auto rounded-md border border-input p-2">
                  {exams.length === 0 ? (
                    <p className="px-1 py-2 text-sm text-muted-foreground">Belum ada ujian.</p>
                  ) : (
                    exams.map((e) => {
                      const checked = examIds.includes(e.id);
                      return (
                        <label key={e.id} className="flex items-center gap-2 px-1 py-1.5 text-sm">
                          <input
                            type="checkbox"
                            checked={checked}
                            disabled={isPending}
                            onChange={(ev) =>
                              setExamIds((prev) =>
                                ev.target.checked ? [...prev, e.id] : prev.filter((id) => id !== e.id)
                              )
                            }
                          />
                          <span>{e.title}</span>
                        </label>
                      );
                    })
                  )}
                </div>
              </Section>
            )}

            <ProductSpecsEditor type={type as ProductType} value={specs} onChange={setSpecs} />

            <Section title="Deskripsi">
              <textarea
                id="product-description"
                aria-label="Deskripsi"
                value={description}
                onChange={(e) => setDescription(e.target.value)}
                placeholder="Deskripsi singkat"
                disabled={isPending}
                rows={6}
                className="w-full rounded-md border border-input bg-transparent px-3 py-2 text-sm outline-none focus-visible:border-ring focus-visible:ring-[3px] focus-visible:ring-ring/50 disabled:opacity-50"
              />
              <p className="text-xs text-muted-foreground">
                Baris kosong memisahkan paragraf, dan halaman produk menampilkannya persis seperti yang diketik di sini.
              </p>
            </Section>

            <Section title="Ketersediaan di marketplace (opsional)">
              <p className="-mt-1 text-xs text-muted-foreground">
                Kosongkan untuk selalu tersedia. Produk hanya tampil dan bisa dibeli dalam rentang ini.
              </p>
              <div className="grid grid-cols-1 gap-4 sm:grid-cols-2">
                <div className="grid gap-1.5">
                  <Label htmlFor="product-available-from" className="text-xs font-normal text-muted-foreground">
                    Tersedia dari
                  </Label>
                  <Input
                    id="product-available-from"
                    type="datetime-local"
                    value={availableFrom}
                    onChange={(e) => setAvailableFrom(e.target.value)}
                    disabled={isPending}
                  />
                </div>
                <div className="grid gap-1.5">
                  <Label htmlFor="product-available-until" className="text-xs font-normal text-muted-foreground">
                    Tersedia sampai
                  </Label>
                  <Input
                    id="product-available-until"
                    type="datetime-local"
                    value={availableUntil}
                    onChange={(e) => setAvailableUntil(e.target.value)}
                    disabled={isPending}
                  />
                </div>
              </div>
            </Section>
          </div>

          <DialogFooter className="shrink-0 border-t px-6 py-4">
            <Button
              type="button"
              variant="outline"
              onClick={() => onOpenChange(false)}
              disabled={isPending}
            >
              Batal
            </Button>
            <Button type="submit" disabled={!canSubmit || isPending}>
              {isPending ? "Menyimpan..." : "Simpan"}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  );
}

// Each group gets a heading on a hairline so the form reads as a handful of
// topics rather than one run of fourteen fields.
function Section({ title, children }: { title: string; children: React.ReactNode }) {
  return (
    <section className="flex flex-col gap-3">
      <h3 className="border-b pb-2 text-[11px] font-semibold uppercase tracking-[0.08em] text-muted-foreground">
        {title}
      </h3>
      {children}
    </section>
  );
}
