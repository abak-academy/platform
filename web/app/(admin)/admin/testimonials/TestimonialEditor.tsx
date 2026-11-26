"use client";

import { useEffect, useRef, useState, type ChangeEvent } from "react";
import { toast } from "sonner";
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from "@/components/ui/dialog";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { fileUrl } from "@/lib/api";
import { usePresignAdminImageUpload } from "@/lib/hooks/admin-uploads";
import { groupingErrorParts } from "@/lib/hooks/admin-product-groupings";
import {
  testimonialBlockedMessage,
  testimonialMissing,
  useCreateTestimonial,
  useUpdateTestimonial,
} from "@/lib/hooks/admin-testimonials";
import type { AdminTestimonial, AdminTestimonialInput } from "@/lib/types";
import { preparePhoto } from "./crop-photo";

const AREA =
  "w-full rounded-md border border-input bg-transparent px-3 py-2 text-sm outline-none focus-visible:border-ring focus-visible:ring-[3px] focus-visible:ring-ring/50 disabled:opacity-50";

interface Form {
  name: string;
  role_label: string;
  quoteId: string;
  quoteEn: string;
  achId: string;
  achEn: string;
  has_consent: boolean;
  consent_note: string;
  is_visible: boolean;
  sort_order: string;
}

const EMPTY: Form = {
  name: "", role_label: "", quoteId: "", quoteEn: "", achId: "", achEn: "",
  has_consent: false, consent_note: "", is_visible: false, sort_order: "0",
};

function formFrom(t: AdminTestimonial | null): Form {
  if (!t) return EMPTY;
  return {
    name: t.name,
    role_label: t.role_label,
    quoteId: t.content.id?.quote ?? "",
    quoteEn: t.content.en?.quote ?? "",
    achId: (t.content.id?.achievements ?? []).join("\n"),
    achEn: (t.content.en?.achievements ?? []).join("\n"),
    has_consent: t.has_consent,
    consent_note: t.consent_note ?? "",
    is_visible: t.is_visible,
    sort_order: String(t.sort_order),
  };
}

const lines = (s: string) => s.split("\n").map((l) => l.trim()).filter(Boolean);

interface Props {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  testimonial: AdminTestimonial | null;
}

export function TestimonialEditor({ open, onOpenChange, testimonial }: Props) {
  const create = useCreateTestimonial();
  const update = useUpdateTestimonial();
  const presign = usePresignAdminImageUpload();
  const fileInput = useRef<HTMLInputElement>(null);
  const [form, setForm] = useState<Form>(EMPTY);
  const [photo, setPhoto] = useState<{ blob: Blob; preview: string } | null>(null);
  const [photoRemoved, setPhotoRemoved] = useState(false);
  const [photoError, setPhotoError] = useState("");
  const [blocked, setBlocked] = useState("");
  const [saving, setSaving] = useState(false);

  useEffect(() => {
    if (!open) return;
    setForm(formFrom(testimonial));
    setPhoto(null);
    setPhotoRemoved(false);
    setPhotoError("");
    setBlocked("");
  }, [open, testimonial]);

  const previewRef = useRef<string | null>(null);
  previewRef.current = photo?.preview ?? null;
  useEffect(() => () => {
    if (previewRef.current) URL.revokeObjectURL(previewRef.current);
  }, []);
  useEffect(() => {
    if (!open && previewRef.current) {
      URL.revokeObjectURL(previewRef.current);
      setPhoto(null);
    }
  }, [open]);

  const set = (patch: Partial<Form>) => setForm((f) => ({ ...f, ...patch }));
  const existingKey = photoRemoved ? null : testimonial?.photo_key ?? null;
  const previewSrc = photo?.preview ?? fileUrl(existingKey);
  const hasPhoto = Boolean(photo) || Boolean(existingKey);

  function buildContent(f: Form = form): AdminTestimonial["content"] {
    return {
      id: { quote: f.quoteId.trim(), achievements: lines(f.achId) },
      en: { quote: f.quoteEn.trim(), achievements: lines(f.achEn) },
    };
  }

  function missingParts(): string[] {
    return testimonialMissing({
      has_consent: form.has_consent,
      consent_note: form.consent_note,
      content: buildContent(),
      photo_key: hasPhoto ? "pending" : null,
    });
  }

  function toggleVisible() {
    if (form.is_visible) return set({ is_visible: false });
    const missing = missingParts();
    if (missing.length > 0) return setBlocked(testimonialBlockedMessage(missing));
    setBlocked("");
    set({ is_visible: true });
  }

  function changeConsent(checked: boolean) {
    set({ has_consent: checked, ...(checked ? {} : { is_visible: false }) });
  }

  async function handlePick(e: ChangeEvent<HTMLInputElement>) {
    const file = e.target.files?.[0];
    if (!file) return;
    const res = await preparePhoto(file);
    if (fileInput.current) fileInput.current.value = "";
    if ("error" in res) {
      setPhotoError(res.error);
      return;
    }
    setPhotoError("");
    setPhotoRemoved(false);
    setPhoto((old) => {
      if (old) URL.revokeObjectURL(old.preview);
      return { blob: res.blob, preview: URL.createObjectURL(res.blob) };
    });
  }

  function removePhoto() {
    if (photo) URL.revokeObjectURL(photo.preview);
    setPhoto(null);
    setPhotoRemoved(true);
    set({ is_visible: false });
  }

  async function uploadPhoto(blob: Blob): Promise<string> {
    const presigned = await presign.mutateAsync({ filename: "testimonial.jpg", content_type: "image/jpeg" });
    const res = await fetch(presigned.url, { method: "PUT", body: blob, headers: { "Content-Type": "image/jpeg" } });
    if (!res.ok) throw new Error(`Upload gagal: ${res.status}`);
    return presigned.key;
  }

  async function handleSave() {
    if (!form.name.trim() || !form.role_label.trim()) return setBlocked("Nama dan sekolah/peran wajib diisi");
    if (lines(form.achId).length > 3 || lines(form.achEn).length > 3) return setBlocked("Maksimal 3 pencapaian per bahasa");
    if (!Number.isInteger(Number(form.sort_order))) return setBlocked("Urutan harus bilangan bulat");
    if (form.is_visible) {
      const missing = missingParts();
      if (missing.length > 0) return setBlocked(testimonialBlockedMessage(missing));
    }
    setBlocked("");
    setSaving(true);
    try {
      const input: AdminTestimonialInput = {
        name: form.name.trim(),
        role_label: form.role_label.trim(),
        content: buildContent(),
        has_consent: form.has_consent,
        consent_note: form.consent_note.trim() || null,
        is_visible: form.is_visible,
        sort_order: Number(form.sort_order) || 0,
      };
      if (photo) input.photo_key = await uploadPhoto(photo.blob);
      else if (photoRemoved && testimonial?.photo_key) input.photo_key = null;
      if (testimonial) {
        const init = formFrom(testimonial);
        const was: AdminTestimonialInput = {
          name: init.name.trim(),
          role_label: init.role_label.trim(),
          content: buildContent(init),
          has_consent: init.has_consent,
          consent_note: init.consent_note.trim() || null,
          is_visible: init.is_visible,
          sort_order: Number(init.sort_order) || 0,
        };
        const changed = Object.fromEntries(
          Object.entries(input).filter(([k, v]) => k === "photo_key" || JSON.stringify(v) !== JSON.stringify(was[k as keyof AdminTestimonialInput])),
        ) as AdminTestimonialInput;
        if (Object.keys(changed).length > 0) await update.mutateAsync({ id: testimonial.id, input: changed });
      } else await create.mutateAsync(input);
      toast.success("Testimoni disimpan");
      onOpenChange(false);
    } catch (e) {
      const { message, missing } = groupingErrorParts(e);
      setBlocked(missing.length > 0 ? testimonialBlockedMessage(missing) : message);
    } finally {
      setSaving(false);
    }
  }

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>{testimonial ? "Edit testimoni" : "Tambah testimoni"}</DialogTitle>
          <DialogDescription>Tayang di landing hanya dengan persetujuan tertulis, kutipan English, dan foto.</DialogDescription>
        </DialogHeader>

        <div className="space-y-4">
          <div className="space-y-1.5">
            <Label htmlFor="t-name">Nama</Label>
            <Input id="t-name" value={form.name} onChange={(e) => set({ name: e.target.value })} />
          </div>
          <div className="space-y-1.5">
            <Label htmlFor="t-role">Sekolah / peran</Label>
            <Input id="t-role" value={form.role_label} onChange={(e) => set({ role_label: e.target.value })} />
          </div>

          <div className="space-y-1.5">
            <Label htmlFor="t-photo">Foto peserta</Label>
            <div className="flex items-start gap-3">
              <div className="aspect-[4/5] w-24 shrink-0 overflow-hidden rounded-lg bg-surface-sunken">
                {previewSrc && <img src={previewSrc} alt="Pratinjau foto peserta" className="size-full object-cover" />}
              </div>
              <div className="space-y-2">
                <div className="flex flex-wrap gap-2">
                  <Button type="button" size="sm" variant="outline" onClick={() => fileInput.current?.click()}>
                    {hasPhoto ? "Ganti foto" : "Pilih foto"}
                  </Button>
                  {hasPhoto && (
                    <Button type="button" size="sm" variant="ghost" onClick={removePhoto}>Hapus foto</Button>
                  )}
                </div>
                <input ref={fileInput} id="t-photo" type="file" accept="image/jpeg,image/png,image/webp" hidden data-testid="testimonial-photo-input" onChange={handlePick} />
                {photoError && <p role="alert" className="text-sm text-danger-600">{photoError}</p>}
                <p className="text-xs text-ink-600">
                  Wajib untuk tayang. JPG/PNG/WebP, maks 5 MB, minimal 1200 × 1500 px. Dipotong otomatis ke 4:5 dan disimpan 1200 × 1500. Hanya unggah dengan izin peserta/orang tua.
                </p>
              </div>
            </div>
          </div>

          <div className="space-y-1.5">
            <Label htmlFor="t-quote-id">Kutipan (Indonesia)</Label>
            <textarea id="t-quote-id" rows={3} className={AREA} value={form.quoteId} onChange={(e) => set({ quoteId: e.target.value })} />
          </div>
          <div className="space-y-1.5">
            <Label htmlFor="t-quote-en">Kutipan (English)</Label>
            <textarea id="t-quote-en" rows={3} className={AREA} value={form.quoteEn} onChange={(e) => set({ quoteEn: e.target.value })} />
          </div>
          <div className="space-y-1.5">
            <Label htmlFor="t-ach-id">Pencapaian (Indonesia)</Label>
            <textarea id="t-ach-id" rows={3} className={AREA} value={form.achId} onChange={(e) => set({ achId: e.target.value })} />
            <p className="text-xs text-ink-600">Satu per baris, maksimal 3.</p>
          </div>
          <div className="space-y-1.5">
            <Label htmlFor="t-ach-en">Pencapaian (English)</Label>
            <textarea id="t-ach-en" rows={3} className={AREA} value={form.achEn} onChange={(e) => set({ achEn: e.target.value })} />
            <p className="text-xs text-ink-600">Satu per baris, maksimal 3.</p>
          </div>

          <div className="space-y-3 rounded-lg bg-surface-sunken p-3">
            <label className="flex items-center gap-2 text-sm font-medium">
              <input type="checkbox" checked={form.has_consent} onChange={(e) => changeConsent(e.target.checked)} />
              Ada persetujuan tertulis
            </label>
            <div className="space-y-1.5">
              <Label htmlFor="t-consent-note">Catatan lokasi persetujuan</Label>
              <Input id="t-consent-note" value={form.consent_note} placeholder="Formulir izin via WhatsApp, 12 Nov 2026" onChange={(e) => set({ consent_note: e.target.value })} />
            </div>
          </div>

          <div className="grid grid-cols-[1fr_auto] items-end gap-3">
            <div className="space-y-1.5">
              <Label htmlFor="t-order">Urutan</Label>
              <Input id="t-order" type="number" value={form.sort_order} onChange={(e) => set({ sort_order: e.target.value })} />
            </div>
            <label className="flex items-center gap-2 pb-2 text-sm">
              <span>Tampil di landing page</span>
              <button
                type="button"
                role="switch"
                aria-checked={form.is_visible}
                aria-label="Tampil di landing page"
                onClick={toggleVisible}
                className={`relative inline-flex h-6 w-11 shrink-0 cursor-pointer rounded-full border-2 border-transparent transition-colors ${form.is_visible ? "bg-brand-600" : "bg-line-strong"}`}
              >
                <span className={`pointer-events-none block size-5 rounded-full bg-white shadow transition-transform ${form.is_visible ? "translate-x-5" : "translate-x-0"}`} />
              </button>
            </label>
          </div>

          {blocked && <p role="alert" className="text-sm text-danger-600">{blocked}</p>}
        </div>

        <DialogFooter>
          <Button type="button" variant="outline" onClick={() => onOpenChange(false)}>Batal</Button>
          <Button type="button" variant="secondary" disabled={saving} onClick={handleSave}>Simpan</Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
