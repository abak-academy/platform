"use client";

import { useMemo, useState } from "react";
import { toast } from "sonner";
import { MessageSquareQuote } from "lucide-react";
import { AdminPageHeader } from "@/components/admin/AdminPageHeader";
import { ListToolbar, type ListToolbarValue } from "@/components/admin/ListToolbar";
import { Button } from "@/components/ui/button";
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from "@/components/ui/dialog";
import { Skeleton } from "@/components/ui/skeleton";
import { fileUrl } from "@/lib/api";
import { groupingErrorParts } from "@/lib/hooks/admin-product-groupings";
import {
  testimonialBlockedMessage,
  testimonialMissing,
  useAdminTestimonials,
  useDeleteTestimonial,
  useUpdateTestimonial,
} from "@/lib/hooks/admin-testimonials";
import { useTranslation } from "@/lib/i18n";
import type { AdminTestimonial } from "@/lib/types";
import { TestimonialEditor } from "./TestimonialEditor";

const FILTERS = [
  { key: "landing", label: "Landing page", options: [{ value: "shown", label: "Tampil" }, { value: "draft", label: "Draft" }] },
  { key: "consent", label: "Persetujuan", options: [{ value: "yes", label: "Ada" }, { value: "no", label: "Belum" }] },
  { key: "en", label: "English", options: [{ value: "complete", label: "Lengkap" }, { value: "missing", label: "Belum lengkap" }] },
];

const enComplete = (t: AdminTestimonial) => Boolean(t.content.en?.quote?.trim());
const idComplete = (t: AdminTestimonial) => Boolean(t.content.id?.quote?.trim());

function matches(t: AdminTestimonial, f: Record<string, string>): boolean {
  if (f.landing && (f.landing === "shown") !== t.is_visible) return false;
  if (f.consent && (f.consent === "yes") !== t.has_consent) return false;
  if (f.en && (f.en === "complete") !== enComplete(t)) return false;
  return true;
}

function TestimonialCard({
  item,
  onEdit,
  onDelete,
  onToggle,
  pending,
}: {
  item: AdminTestimonial;
  onEdit: () => void;
  onDelete: () => void;
  onToggle: () => void;
  pending: boolean;
}) {
  const src = fileUrl(item.photo_key);
  const chips = item.content.id?.achievements ?? [];
  return (
    <article aria-label={item.name} className="md-card-outlined flex flex-col gap-3 p-4">
      <div className="flex items-start gap-3">
        <div className="aspect-[4/5] w-16 shrink-0 overflow-hidden rounded-lg bg-surface-sunken">
          {src && <img src={src} alt={item.name} className="size-full object-cover" />}
        </div>
        <div className="min-w-0 flex-1">
          <h3 className="type-h3 truncate">{item.name}</h3>
          <p className="text-xs text-ink-600">{item.role_label}</p>
          <p className="mt-1 line-clamp-3 text-sm text-ink-700">{item.content.id?.quote}</p>
        </div>
      </div>

      {chips.length > 0 && (
        <div className="flex flex-wrap gap-2">
          {chips.map((c) => <span key={c} className="md-chip">{c}</span>)}
        </div>
      )}

      <div className="flex flex-wrap gap-2 text-xs">
        <span className="md-chip">{item.has_consent ? "Ada persetujuan" : "Belum ada persetujuan"}</span>
        <span className="md-chip">{idComplete(item) ? "ID lengkap" : "ID kosong"}</span>
        <span className="md-chip">{enComplete(item) ? "EN lengkap" : "EN belum lengkap"}</span>
        {!item.photo_key && <span className="md-chip">Belum ada foto</span>}
      </div>

      <div className="flex items-center justify-between gap-3">
        <div className="flex gap-2">
          <Button size="sm" variant="outline" onClick={onEdit}>Edit</Button>
          <Button size="sm" variant="ghost" onClick={onDelete}>Hapus</Button>
        </div>
        <label className="flex items-center gap-2 text-sm">
          <span className="text-ink-600">{item.is_visible ? "Tampil di landing" : "Draft"}</span>
          <button
            type="button"
            role="switch"
            aria-checked={item.is_visible}
            aria-label={`Tampilkan ${item.name} di landing`}
            disabled={pending}
            onClick={onToggle}
            className={`relative inline-flex h-6 w-11 shrink-0 cursor-pointer rounded-full border-2 border-transparent transition-colors ${item.is_visible ? "bg-brand-600" : "bg-line-strong"}`}
          >
            <span className={`pointer-events-none block size-5 rounded-full bg-white shadow transition-transform ${item.is_visible ? "translate-x-5" : "translate-x-0"}`} />
          </button>
        </label>
      </div>
    </article>
  );
}

export default function TestimonialsPage() {
  const { t } = useTranslation();
  const [toolbar, setToolbar] = useState<ListToolbarValue>({ q: "", filters: {} });
  const [editing, setEditing] = useState<AdminTestimonial | null>(null);
  const [editorOpen, setEditorOpen] = useState(false);
  const [deleting, setDeleting] = useState<AdminTestimonial | null>(null);

  const { data, isLoading, isError, error } = useAdminTestimonials();
  const update = useUpdateTestimonial();
  const remove = useDeleteTestimonial();

  const rows = useMemo(() => {
    const q = toolbar.q.trim().toLowerCase();
    return (data ?? []).filter(
      (x) =>
        matches(x, toolbar.filters) &&
        (q === "" || `${x.name} ${x.content.id?.quote ?? ""} ${x.content.en?.quote ?? ""}`.toLowerCase().includes(q)),
    );
  }, [data, toolbar]);

  function openEditor(item: AdminTestimonial | null) {
    setEditing(item);
    setEditorOpen(true);
  }

  async function handleToggle(item: AdminTestimonial) {
    if (!item.is_visible) {
      const missing = testimonialMissing(item);
      if (missing.length > 0) return toast.error(testimonialBlockedMessage(missing));
    }
    try {
      await update.mutateAsync({ id: item.id, input: { is_visible: !item.is_visible } });
    } catch (e) {
      const { message, missing } = groupingErrorParts(e);
      toast.error(missing.length > 0 ? testimonialBlockedMessage(missing) : message);
    }
  }

  async function handleDelete() {
    if (!deleting) return;
    try {
      await remove.mutateAsync(deleting.id);
      toast.success("Testimoni dihapus");
      setDeleting(null);
    } catch (e) {
      toast.error(groupingErrorParts(e).message);
    }
  }

  return (
    <div className="space-y-6 fade-in">
      <AdminPageHeader
        icon={MessageSquareQuote}
        title={t("admin_nav_testimonials")}
        description="Cerita peserta untuk landing page. Hanya tayang dengan persetujuan tertulis."
        actions={<Button onClick={() => openEditor(null)}>Tambah testimoni</Button>}
      />

      {isLoading && (
        <div className="grid gap-4 md:grid-cols-2 xl:grid-cols-3">
          {Array.from({ length: 3 }).map((_, i) => <Skeleton key={i} className="h-48 w-full" />)}
        </div>
      )}

      {isError && (
        <div className="rounded-lg border border-destructive/20 bg-destructive/10 p-4 text-destructive">
          Gagal memuat testimoni: {groupingErrorParts(error).message}
        </div>
      )}

      {!isLoading && !isError && (
        <>
          <ListToolbar
            variant="standalone"
            value={toolbar}
            onChange={setToolbar}
            filters={FILTERS}
            count={rows.length}
            unit="testimoni"
            searchPlaceholder="Cari nama atau kutipan"
          />
          {rows.length === 0 ? (
            <div className="md-card-outlined p-8 text-center text-ink-600">Belum ada testimoni.</div>
          ) : (
            <div className="grid gap-4 md:grid-cols-2 xl:grid-cols-3">
              {rows.map((x) => (
                <TestimonialCard
                  key={x.id}
                  item={x}
                  pending={update.isPending}
                  onEdit={() => openEditor(x)}
                  onDelete={() => setDeleting(x)}
                  onToggle={() => handleToggle(x)}
                />
              ))}
            </div>
          )}
        </>
      )}

      <TestimonialEditor open={editorOpen} onOpenChange={setEditorOpen} testimonial={editing} />

      <Dialog open={deleting !== null} onOpenChange={(o) => !o && setDeleting(null)}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>Hapus testimoni?</DialogTitle>
            <DialogDescription>
              {deleting?.name} akan dihapus permanen. Berkas foto di penyimpanan tidak ikut dihapus.
            </DialogDescription>
          </DialogHeader>
          <DialogFooter>
            <Button variant="outline" onClick={() => setDeleting(null)}>Batal</Button>
            <Button variant="destructive" disabled={remove.isPending} onClick={handleDelete}>Hapus</Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  );
}
