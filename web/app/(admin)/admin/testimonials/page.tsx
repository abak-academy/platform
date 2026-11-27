"use client";

import { useMemo, useState } from "react";
import { toast } from "sonner";
import { Check, MessageSquareQuote, Pencil, Plus, Trash2, Trophy, TriangleAlert } from "lucide-react";
import { AdminPageHeader } from "@/components/admin/AdminPageHeader";
import { ListToolbar, type ListToolbarValue } from "@/components/admin/ListToolbar";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { EmptyState } from "@/components/ui/empty-state";
import { Switch } from "@/components/ui/switch";
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
import { AbakAvatar } from "@/components/ui/abak-avatar";

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
  const { t } = useTranslation();
  const src = fileUrl(item.photo_key);
  const chips = item.content.id?.achievements ?? [];
  const off = !item.is_visible || !item.has_consent;
  return (
    <article aria-label={item.name} className={`md-card-outlined flex flex-col gap-3 p-6 ${off ? "border-dashed bg-surface-sunken" : ""}`}>
      <div className="flex items-center gap-2">
        <AbakAvatar name={item.name} size="md" src={src || undefined} />
        <div className="min-w-0 flex-1">
          <h3 className="truncate font-[family-name:var(--font-display)] text-[15px] leading-[1.55] font-bold text-ink-900">{item.name}</h3>
          <p className="truncate text-[12.5px] leading-[1.55] text-ink-600">{item.role_label}</p>
        </div>
        <Switch
          size="sm"
          checked={item.is_visible}
          disabled={pending}
          aria-label={t("adm_show_on_landing_aria").replace("{name}", item.name)}
          onCheckedChange={onToggle}
        />
      </div>

      <blockquote className="line-clamp-4 flex-1 font-[family-name:var(--font-display)] text-[15px] leading-[1.5] font-semibold text-ink-700">
        {item.content.id?.quote?.trim() && <>“<span>{item.content.id.quote}</span>”</>}
      </blockquote>

      {chips.length > 0 && (
        <div className="flex flex-wrap gap-1.5">
          {chips.map((c) => (
            <Badge key={c} variant="gold"><Trophy className="size-3.5" aria-hidden />{c}</Badge>
          ))}
        </div>
      )}

      <div className="flex flex-wrap gap-1.5 border-t border-dashed border-line pt-2.5">
        {item.has_consent
          ? <Badge variant="teal"><Check className="size-3.5" aria-hidden />{t("adm_testi_consent_yes")}</Badge>
          : <Badge variant="danger"><TriangleAlert className="size-3.5" aria-hidden />{t("adm_testi_consent_no")}</Badge>}
        <Badge variant={idComplete(item) ? "mute" : "danger"}>{t(idComplete(item) ? "adm_testi_id_ok" : "adm_testi_id_empty")}</Badge>
        <Badge variant={enComplete(item) ? "mute" : "gold"}>{t(enComplete(item) ? "adm_testi_en_ok" : "adm_testi_en_missing")}</Badge>
        {!item.photo_key && <Badge variant="mute">{t("adm_testi_no_photo")}</Badge>}
      </div>

      <div className="flex items-center gap-2">
        <Button size="sm" variant="outline" onClick={onEdit}><Pencil aria-hidden />{t("adm_edit")}</Button>
        <Button size="sm" variant="ghost" onClick={onDelete}><Trash2 aria-hidden />{t("adm_delete")}</Button>
        <span className="ml-auto text-xs text-ink-600">{t(item.is_visible ? "adm_on_landing" : "adm_draft")}</span>
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

  const filtering = toolbar.q.trim() !== "" || Object.values(toolbar.filters).some(Boolean);
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
    <div className="flex flex-col gap-5 fade-in">
      <AdminPageHeader
        title={t("admin_nav_testimonials")}
        description={t("adm_testi_desc")}
        actions={
          <Button variant="dark" onClick={() => openEditor(null)}>
            <Plus aria-hidden />
            {t("adm_testi_add")}
          </Button>
        }
      />

      {isLoading && (
        <div className="grid grid-cols-[repeat(auto-fill,minmax(min(320px,100%),1fr))] gap-4">
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
            searchPlaceholder={t("adm_testi_search")}
          />
          {rows.length === 0 ? (
            <EmptyState framed icon={MessageSquareQuote} title={t("adm_testi_empty")} text={filtering ? t("adm_filter_hint") : undefined} />
          ) : (
            <div className="grid grid-cols-[repeat(auto-fill,minmax(min(320px,100%),1fr))] gap-4">
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
            <Button variant="outline" onClick={() => setDeleting(null)}>{t("adm_cancel_btn")}</Button>
            <Button variant="destructive" disabled={remove.isPending} onClick={handleDelete}>{t("adm_delete")}</Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  );
}
