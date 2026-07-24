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

interface ProductModalProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  product?: Product | null;
  onSubmit: (input: AdminCreateProductInput | AdminUpdateProductInput) => void;
  isPending: boolean;
}

const PRODUCT_TYPES: ProductType[] = ["book", "course", "exam", "merchandise", "medal"];
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

export function ProductModal({ open, onOpenChange, product, onSubmit, isPending }: ProductModalProps) {
  const isEdit = Boolean(product);
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
      const presigned = await presign.mutateAsync({ filename: file.name, content_type: file.type });
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
        ...(showStock ? { stock: Number(stock) } : {}),
        ...(showStock && weight !== "" ? { weight_grams: Number(weight) } : {}),
        ...(showStock && imageUrl !== "" ? { image_url: imageUrl } : {}),
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
      ...(showStock ? { stock: Number(stock) } : {}),
      ...(showStock && weight !== "" ? { weight_grams: Number(weight) } : {}),
      ...(showStock && imageUrl !== "" ? { image_url: imageUrl } : {}),
      specs: specs.filter((s) => s.label.trim() !== "" && s.value.trim() !== ""),
      ...(showCourses && courseIds.length > 0 ? { course_ids: courseIds } : {}),
      ...(showExams && examIds.length > 0 ? { exam_ids: examIds } : {}),
    };
    onSubmit(input);
  }

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="sm:max-w-2xl">
        <form onSubmit={handleSubmit}>
          <DialogHeader>
            <DialogTitle>{isEdit ? "Edit produk" : "Buat produk"}</DialogTitle>
            <DialogDescription>
              {isEdit ? "Perbarui metadata produk." : "Tambahkan produk baru ke katalog."}
            </DialogDescription>
          </DialogHeader>

          <div className="grid gap-4 py-4">
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

            <div className="grid grid-cols-2 gap-4">
              <div className="grid gap-2">
                <Label htmlFor="product-type">Jenis</Label>
                <select
                  id="product-type"
                  value={type}
                  onChange={(e) => setType(e.target.value as ProductType)}
                  disabled={isEdit || isPending}
                  className="h-9 w-full rounded-md border border-input bg-transparent px-2 text-sm outline-none focus-visible:border-ring focus-visible:ring-[3px] focus-visible:ring-ring/50 disabled:opacity-50"
                >
                  <option value="" disabled>Pilih jenis</option>
                  {PRODUCT_TYPES.map((t) => (
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
              <div className="grid gap-2">
                <Label htmlFor="product-status">Status</Label>
                <select
                  id="product-status"
                  value={status}
                  onChange={(e) => setStatus(e.target.value as ProductStatus)}
                  disabled={isPending}
                  className="h-9 w-full rounded-md border border-input bg-transparent px-2 text-sm outline-none focus-visible:border-ring focus-visible:ring-[3px] focus-visible:ring-ring/50 disabled:opacity-50"
                >
                  {PRODUCT_STATUSES.map((s) => (
                    <option key={s} value={s}>
                      {STATUS_LABELS[s]}
                    </option>
                  ))}
                </select>
              </div>
            )}

            {showStock && (
              <div className="grid grid-cols-2 gap-4">
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
                </div>
              </div>
            )}

            {showStock && (
              <div className="grid gap-2">
                <Label htmlFor="product-image">Gambar produk</Label>
                <div className="flex items-center gap-3">
                  {imageUrl && (
                    // eslint-disable-next-line @next/next/no-img-element
                    <img
                      src={fileUrl(imageUrl)}
                      alt="Pratinjau gambar"
                      className="h-16 w-16 rounded-md border border-input object-cover"
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
            )}

            {showCourses && (
              <div className="grid gap-2">
                <Label>Kursus terkait</Label>
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
              </div>
            )}

            {showExams && (
              <div className="grid gap-2">
                <Label>Ujian terkait</Label>
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
              </div>
            )}

            <ProductSpecsEditor type={type as ProductType} value={specs} onChange={setSpecs} />

            <div className="grid gap-2">
              <Label htmlFor="product-description">Deskripsi</Label>
              <textarea
                id="product-description"
                value={description}
                onChange={(e) => setDescription(e.target.value)}
                placeholder="Deskripsi singkat"
                disabled={isPending}
                rows={3}
                className="w-full rounded-md border border-input bg-transparent px-3 py-2 text-sm outline-none focus-visible:border-ring focus-visible:ring-[3px] focus-visible:ring-ring/50 disabled:opacity-50"
              />
            </div>
          </div>

          <DialogFooter>
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
