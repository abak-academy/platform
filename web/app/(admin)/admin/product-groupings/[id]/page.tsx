"use client";

import { useEffect, useMemo, useRef, useState } from "react";
import { useParams, useRouter } from "next/navigation";
import Link from "next/link";
import { toast } from "sonner";
import { ArrowLeft, ArrowDown, ArrowUp, GripVertical, ImageIcon, Plus, Trash2, Upload, X } from "lucide-react";
import {
  DndContext,
  KeyboardSensor,
  PointerSensor,
  closestCenter,
  useSensor,
  useSensors,
  type DragEndEvent,
} from "@dnd-kit/core";
import {
  SortableContext,
  arrayMove,
  sortableKeyboardCoordinates,
  useSortable,
  verticalListSortingStrategy,
} from "@dnd-kit/sortable";
import { CSS } from "@dnd-kit/utilities";
import { Switch } from "@/components/ui/switch";
import { AdminPageHeader } from "@/components/admin/AdminPageHeader";
import { ExternalRoundsDialog } from "@/components/admin/ExternalRoundsDialog";
import { EVENT_ILLUSTRATIONS, EVENT_TONES, EventMark } from "@/components/admin/EventMark";
import { ProductLevelFields, gradeRangeInvalid, levelPayload } from "@/components/admin/ProductModal";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Skeleton } from "@/components/ui/skeleton";
import { fileUrl } from "@/lib/api";
import {
  groupingErrorParts,
  groupingMissingLabel,
  useAdminProductGrouping,
  useDeleteProductGrouping,
  useSetGroupingProducts,
  useUpdateEditionLevel,
  useUpdateProductGrouping,
} from "@/lib/hooks/admin-product-groupings";
import { useAdminProducts } from "@/lib/hooks/admin-products";
import { usePresignAdminImageUpload } from "@/lib/hooks/admin-uploads";
import { useTranslation } from "@/lib/i18n";
import { formatRupiah } from "@/lib/format";
import type {
  AdminUpdateProductGroupingInput,
  EditionPhase,
  EventIllustration,
  EventLocale,
  EventNarrative,
  EventScope,
  EventStage,
  EventTone,
  Product,
  ProductGroupingDetail,
  ProductGroupingStatus,
} from "@/lib/types";

const FIELD =
  "h-9 w-full rounded-md border border-input bg-transparent px-2 text-sm outline-none focus-visible:border-ring focus-visible:ring-[3px] focus-visible:ring-ring/50 disabled:opacity-50";
const AREA =
  "w-full rounded-md border border-input bg-transparent px-3 py-2 text-sm outline-none focus-visible:border-ring focus-visible:ring-[3px] focus-visible:ring-ring/50 disabled:opacity-50";

const PREFIX_HINT =
  "Dipakai di depan nomor peserta, mis. ESOL-251012-0007-000123. Mengubahnya mengubah nomor di kartu yang dibuat berikutnya.";

const PHASE_LABEL: Record<EditionPhase, string> = { open: "Dibuka", live: "Berlangsung", closed: "Ditutup", done: "Selesai" };
const PHASE_CLASS: Record<EditionPhase, string> = {
  open: "bg-teal-50 text-teal-700",
  live: "bg-ember-600 text-on-ember",
  closed: "bg-mute-50 text-mute-700",
  done: "bg-mute-50 text-mute-700",
};

interface NarrativeForm {
  fullName: string;
  levelsLabel: string;
  tagline: string;
  story: string;
  rounds: string;
  categories: string;
  facts: string;
}

interface ScheduleRow {
  key: number;
  label: string;
  labelEn: string;
  startsOn: string;
  endsOn: string;
}

interface Form {
  name: string;
  description: string;
  shortName: string;
  prefix: string;
  scope: EventScope | "";
  officialUrl: string;
  subjects: string;
  tone: EventTone;
  illustration: EventIllustration;
  status: ProductGroupingStatus;
  visible: boolean;
  logoKey: string;
  id: NarrativeForm;
  en: NarrativeForm;
  schedule: ScheduleRow[];
}

const paragraphs = (t: string) => t.split(/\n\s*\n/).map((p) => p.trim()).filter(Boolean);
const lines = (t: string) => t.split("\n").map((l) => l.trim()).filter(Boolean);
const pairs = (t: string) =>
  lines(t).flatMap((l) => {
    const at = l.indexOf("|");
    const left = (at === -1 ? l : l.slice(0, at)).trim();
    return left ? [[left, at === -1 ? "" : l.slice(at + 1).trim()] as const] : [];
  });
const pairText = (a: { [k: string]: string }[], k1: string, k2: string) => a.map((x) => `${x[k1]} | ${x[k2]}`).join("\n");

function narrativeForm(n?: Partial<EventNarrative>): NarrativeForm {
  return {
    fullName: n?.full_name ?? "",
    levelsLabel: n?.levels_label ?? "",
    tagline: n?.tagline ?? "",
    story: (n?.story ?? []).join("\n\n"),
    rounds: (n?.rounds ?? []).join("\n"),
    categories: pairText(n?.categories ?? [], "name", "detail"),
    facts: pairText(n?.facts ?? [], "label", "value"),
  };
}

function narrativePayload(n: NarrativeForm): EventNarrative {
  return {
    full_name: n.fullName.trim(),
    tagline: n.tagline.trim(),
    levels_label: n.levelsLabel.trim(),
    story: paragraphs(n.story),
    rounds: lines(n.rounds),
    categories: pairs(n.categories).map(([name, detail]) => ({ name, detail })),
    facts: pairs(n.facts).map(([label, value]) => ({ label, value })),
  };
}

let rowKey = 0;

function formFromDetail(g: ProductGroupingDetail): Form {
  return {
    name: g.name,
    description: g.description ?? "",
    shortName: g.short_name ?? "",
    prefix: g.participant_prefix ?? "",
    scope: g.scope ?? "",
    officialUrl: g.official_url ?? "",
    subjects: (g.subjects ?? []).join(", "),
    tone: g.tone,
    illustration: g.illustration,
    status: g.status,
    visible: g.is_visible,
    logoKey: g.logo_key ?? "",
    id: narrativeForm(g.content?.id),
    en: narrativeForm(g.content?.en),
    schedule: (g.schedule ?? []).map((s) => ({
      key: ++rowKey,
      label: s.label,
      labelEn: s.label_en ?? "",
      startsOn: s.starts_on,
      endsOn: s.ends_on ?? "",
    })),
  };
}

const isBlankRow = (r: ScheduleRow) => !r.label.trim() && !r.labelEn.trim() && !r.startsOn && !r.endsOn;

function rowError(r: ScheduleRow): string | null {
  if (isBlankRow(r)) return null;
  if (!r.label.trim() || !r.startsOn) return "Isi label dan tanggal mulai.";
  if (r.endsOn && r.endsOn < r.startsOn) return "Tanggal selesai tidak boleh sebelum tanggal mulai.";
  return null;
}

function enIncomplete(f: Form): boolean {
  return (
    !f.en.tagline.trim() ||
    paragraphs(f.en.story).length === 0 ||
    f.schedule.some((r) => !isBlankRow(r) && !r.labelEn.trim())
  );
}

type MemberSection = "edition" | "other";

function MemberRow({
  product,
  index,
  total,
  onMove,
  onRemove,
  badges,
  actions,
  children,
}: {
  product: Product;
  index: number;
  total: number;
  onMove: (index: number, delta: number) => void;
  onRemove: (id: string) => void;
  badges?: React.ReactNode;
  actions?: React.ReactNode;
  children?: React.ReactNode;
}) {
  const { t } = useTranslation();
  const { attributes, listeners, setNodeRef, transform, transition, isDragging } = useSortable({ id: product.id });
  const style = { transform: CSS.Transform.toString(transform), transition };

  return (
    <li ref={setNodeRef} style={style} className={`space-y-3 px-3 py-2 ${isDragging ? "bg-muted" : ""}`}>
      <div className="flex items-center justify-between gap-3">
        <div className="flex min-w-0 items-center gap-2">
          <button
            type="button"
            className="cursor-grab touch-none text-muted-foreground active:cursor-grabbing"
            aria-label={t("grouping_drag_handle")}
            {...attributes}
            {...listeners}
          >
            <GripVertical className="size-4" aria-hidden />
          </button>
          <div className="min-w-0 truncate">
            <span className="font-medium">{product.name}</span>{" "}
            <span className="text-xs text-muted-foreground">{formatRupiah(product.price)}</span>
          </div>
          {badges}
        </div>
        <div className="flex items-center gap-1">
          {actions}
          <Button type="button" size="icon" variant="ghost" className="size-7" aria-label={t("grouping_move_up")} disabled={index === 0} onClick={() => onMove(index, -1)}>
            <ArrowUp className="size-4" aria-hidden />
          </Button>
          <Button type="button" size="icon" variant="ghost" className="size-7" aria-label={t("grouping_move_down")} disabled={index === total - 1} onClick={() => onMove(index, 1)}>
            <ArrowDown className="size-4" aria-hidden />
          </Button>
          <Button type="button" size="icon" variant="ghost" className="size-7" aria-label={t("grouping_remove_product")} onClick={() => onRemove(product.id)}>
            <X className="size-4" aria-hidden />
          </Button>
        </div>
      </div>
      {children}
    </li>
  );
}

function EditionRow({
  groupingId,
  product,
  actions,
  ...row
}: {
  groupingId: string;
  product: Product;
  index: number;
  total: number;
  onMove: (index: number, delta: number) => void;
  onRemove: (id: string) => void;
  actions?: React.ReactNode;
}) {
  const saveLevel = useUpdateEditionLevel(groupingId);
  const [stages, setStages] = useState<EventStage[]>(product.stages ?? []);
  const [gradeMin, setGradeMin] = useState(product.grade_min != null ? String(product.grade_min) : "");
  const [gradeMax, setGradeMax] = useState(product.grade_max != null ? String(product.grade_max) : "");

  async function handleSaveLevel() {
    try {
      await saveLevel.mutateAsync({ productId: product.id, input: levelPayload(stages, gradeMin, gradeMax) });
      toast.success(`Level ${product.name} disimpan.`);
    } catch (e) {
      toast.error(groupingErrorParts(e).message);
    }
  }

  return (
    <MemberRow
      product={product}
      actions={actions}
      badges={
        <>
          {product.phase && (
            <span className={`shrink-0 rounded-full px-2 py-0.5 text-xs font-semibold ${PHASE_CLASS[product.phase]}`}>{PHASE_LABEL[product.phase]}</span>
          )}
          {product.multi_event && (
            <span className="shrink-0 rounded-full bg-gold-50 px-2 py-0.5 text-xs font-semibold text-gold-700">ada di dua event</span>
          )}
        </>
      }
      {...row}
    >
      <div role="group" aria-label={`Level ${product.name}`} className="space-y-2 pl-6">
        <ProductLevelFields
          idPrefix={`level-${product.id}`}
          stages={stages}
          gradeMin={gradeMin}
          gradeMax={gradeMax}
          disabled={saveLevel.isPending}
          onChange={(n) => {
            setStages(n.stages);
            setGradeMin(n.gradeMin);
            setGradeMax(n.gradeMax);
          }}
        />
        <Button
          type="button"
          size="sm"
          variant="outline"
          aria-label={`Simpan level ${product.name}`}
          disabled={saveLevel.isPending || gradeRangeInvalid(gradeMin, gradeMax)}
          onClick={handleSaveLevel}
        >
          Simpan level
        </Button>
      </div>
    </MemberRow>
  );
}

function Field({ id, label, children, hint }: { id: string; label: string; children: React.ReactNode; hint?: React.ReactNode }) {
  return (
    <div className="grid gap-1.5">
      <Label htmlFor={id}>{label}</Label>
      {children}
      {hint}
    </div>
  );
}

export default function ProductGroupingDetailPage() {
  const { t } = useTranslation();
  const router = useRouter();
  const params = useParams();
  const id = typeof params.id === "string" ? params.id : "";

  const { data: grouping, isLoading, isError, error } = useAdminProductGrouping(id);
  const { data: allProducts } = useAdminProducts();
  const [externalFor, setExternalFor] = useState<Product | null>(null);
  const setMembers = useSetGroupingProducts();
  const update = useUpdateProductGrouping();
  const remove = useDeleteProductGrouping();
  const presign = usePresignAdminImageUpload();
  const logoInput = useRef<HTMLInputElement>(null);

  const [form, setForm] = useState<Form | null>(null);
  const [memberIds, setMemberIds] = useState<string[] | null>(null);
  const [tab, setTab] = useState<EventLocale>("id");
  const [prefixError, setPrefixError] = useState("");
  const [missing, setMissing] = useState<string[]>([]);
  const [enNotice, setEnNotice] = useState(false);
  const [search, setSearch] = useState("");
  const sensors = useSensors(
    useSensor(PointerSensor, { activationConstraint: { distance: 4 } }),
    useSensor(KeyboardSensor, { coordinateGetter: sortableKeyboardCoordinates }),
  );

  useEffect(() => {
    if (!grouping) return;
    setForm((f) => f ?? formFromDetail(grouping));
    setMemberIds((m) => m ?? grouping.products.map((p) => p.id));
  }, [grouping]);

  const productsById = useMemo(() => {
    const map = new Map<string, Product>();
    for (const p of allProducts ?? []) map.set(p.id, p);
    for (const p of grouping?.products ?? []) map.set(p.id, p);
    return map;
  }, [allProducts, grouping]);

  const ids = memberIds ?? [];
  const members = ids.map((pid) => productsById.get(pid)).filter((p): p is Product => Boolean(p));
  const editions = members.filter((p) => p.type === "exam");
  const others = members.filter((p) => p.type !== "exam");

  const candidates = useMemo(() => {
    const q = search.trim().toLowerCase();
    return (allProducts ?? []).filter((p) => !ids.includes(p.id) && (q === "" || p.name.toLowerCase().includes(q)));
  }, [allProducts, ids, search]);

  function errorMessage(e: unknown): string {
    return e instanceof Error ? e.message : t("error_generic");
  }

  function set(patch: Partial<Form>) {
    setForm((f) => (f ? { ...f, ...patch } : f));
  }

  function setNarrative(loc: EventLocale, patch: Partial<NarrativeForm>) {
    setForm((f) => (f ? { ...f, [loc]: { ...f[loc], ...patch } } : f));
  }

  function setRow(key: number, patch: Partial<ScheduleRow>) {
    setForm((f) => (f ? { ...f, schedule: f.schedule.map((r) => (r.key === key ? { ...r, ...patch } : r)) } : f));
  }

  function commit(section: MemberSection, next: string[]) {
    const rest = (section === "edition" ? others : editions).map((p) => p.id);
    setMemberIds(section === "edition" ? [...next, ...rest] : [...rest, ...next]);
  }

  function move(section: MemberSection, index: number, delta: number) {
    const list = (section === "edition" ? editions : others).map((p) => p.id);
    const target = index + delta;
    if (target < 0 || target >= list.length) return;
    commit(section, arrayMove(list, index, target));
  }

  function handleDragEnd({ active, over }: DragEndEvent) {
    if (!over || active.id === over.id) return;
    for (const [section, list] of [["edition", editions], ["other", others]] as const) {
      const from = list.findIndex((p) => p.id === active.id);
      const to = list.findIndex((p) => p.id === over.id);
      if (from !== -1 && to !== -1) commit(section, arrayMove(list.map((p) => p.id), from, to));
    }
  }

  function toggleVisible() {
    if (!form) return;
    if (form.visible) return set({ visible: false });
    if (enIncomplete(form)) {
      setTab("en");
      setEnNotice(true);
      return;
    }
    setEnNotice(false);
    set({ visible: true });
  }

  function changeStatus(status: ProductGroupingStatus) {
    set(status === "draft" ? { status, visible: false } : { status });
  }

  async function handleLogo(e: React.ChangeEvent<HTMLInputElement>) {
    const file = e.target.files?.[0];
    if (!file) return;
    try {
      const presigned = await presign.mutateAsync({ filename: file.name, content_type: file.type });
      const res = await fetch(presigned.url, { method: "PUT", body: file, headers: { "Content-Type": file.type } });
      if (!res.ok) throw new Error(`Upload gagal: ${res.status}`);
      set({ logoKey: presigned.key });
    } catch (err) {
      toast.error(errorMessage(err));
    } finally {
      if (logoInput.current) logoInput.current.value = "";
    }
  }

  async function handleSave() {
    if (!form) return;
    const bad = form.schedule.findIndex((r) => rowError(r));
    if (bad !== -1) {
      toast.error(`Jadwal baris ${bad + 1}: ${rowError(form.schedule[bad])}`);
      return;
    }
    if (form.visible && enIncomplete(form)) {
      setTab("en");
      setEnNotice(true);
      set({ visible: false });
      return;
    }
    setPrefixError("");
    setMissing([]);
    const input: AdminUpdateProductGroupingInput = {
      name: form.name.trim(),
      description: form.description.trim(),
      status: form.status,
      short_name: form.shortName.trim(),
      scope: form.scope,
      subjects: form.subjects.split(",").map((s) => s.trim()).filter(Boolean),
      tone: form.tone,
      illustration: form.illustration,
      logo_key: form.logoKey,
      official_url: form.officialUrl.trim(),
      content: { id: narrativePayload(form.id), en: narrativePayload(form.en) },
      is_visible: form.visible,
      participant_prefix: form.prefix,
      schedule: form.schedule
        .filter((r) => !isBlankRow(r))
        .map((r) => ({ label: r.label.trim(), label_en: r.labelEn.trim() || null, starts_on: r.startsOn, ends_on: r.endsOn || null })),
    };
    try {
      const saved = await update.mutateAsync({ id, input });
      setForm(formFromDetail(saved));
      toast.success(t("changes_saved"));
    } catch (e) {
      const { message, missing: gaps, code } = groupingErrorParts(e);
      if (gaps.length > 0) {
        setMissing(gaps);
        if (gaps.some((g) => g.startsWith("en.") || g.startsWith("schedule["))) setTab("en");
        if (gaps.includes("participant_prefix")) setPrefixError(groupingMissingLabel("participant_prefix") + " wajib diisi.");
      } else if (code === "grouping_prefix_taken" || message.includes("participant_prefix must be")) {
        setPrefixError(message);
      } else {
        toast.error(message);
      }
    }
  }

  async function handleSaveMembers() {
    try {
      const saved = await setMembers.mutateAsync({ id, productIds: ids });
      setMemberIds(saved.products.map((p) => p.id));
      toast.success(t("grouping_members_saved"));
    } catch (e) {
      toast.error(errorMessage(e));
    }
  }

  async function handleDelete() {
    if (!confirm(t("grouping_confirm_delete"))) return;
    try {
      await remove.mutateAsync(id);
      toast.success(t("grouping_deleted"));
      router.push("/admin/product-groupings");
    } catch (e) {
      toast.error(errorMessage(e));
    }
  }

  const narrative = form?.[tab];
  const enGap = form ? enIncomplete(form) : false;
  const editionTotal = editions.length;

  return (
    <div className="space-y-6 fade-in">
      {!grouping && (
        <Link href="/admin/product-groupings" className="inline-flex w-fit items-center gap-1 type-label-sm text-ink-600 hover:text-ink-900">
          <ArrowLeft className="size-4" aria-hidden />
          {t("grouping_back_to_list")}
        </Link>
      )}

      {isLoading && <Skeleton className="h-16 w-full" />}

      {isError && (
        <div className="rounded-lg border border-destructive/20 bg-destructive/10 p-4 text-destructive">
          {t("grouping_load_failed")}: {errorMessage(error)}
        </div>
      )}

      {grouping && form && narrative && (
        <>
          <AdminPageHeader
            back={{ href: "/admin/product-groupings", label: t("grouping_back_to_list") }}
            title={grouping.name}
            description={`${grouping.products.filter((p) => p.type === "exam").length} edisi · ${grouping.is_visible ? "tampil di landing" : "disembunyikan dari landing"}`}
          />

          <div className="grid items-start gap-6 lg:grid-cols-[minmax(0,1fr)_340px]">
            <div className="min-w-0 space-y-4">
              <section className="md-card-outlined space-y-4 p-6" aria-label="Identitas">
                <h2 className="type-h2">Identitas</h2>
                <div className="grid gap-4 sm:grid-cols-2">
                  <Field id="event-name" label="Nama event">
                    <Input id="event-name" value={form.name} onChange={(e) => set({ name: e.target.value })} />
                  </Field>
                  <Field id="event-short-name" label="Nama pendek">
                    <Input id="event-short-name" value={form.shortName} onChange={(e) => set({ shortName: e.target.value })} placeholder="Dipakai di hero landing, mis. Einstein" />
                  </Field>
                  <Field
                    id="event-prefix"
                    label="Prefiks nomor peserta"
                    hint={
                      <>
                        <p className="text-xs text-ink-600">{PREFIX_HINT}</p>
                        {prefixError && <p role="alert" className="text-sm text-destructive">{prefixError}</p>}
                      </>
                    }
                  >
                    <Input
                      id="event-prefix"
                      value={form.prefix}
                      maxLength={6}
                      aria-invalid={prefixError ? true : undefined}
                      onChange={(e) => {
                        setPrefixError("");
                        set({ prefix: e.target.value.toUpperCase().replace(/[^A-Z]/g, "").slice(0, 6) });
                      }}
                      placeholder="ESOL"
                    />
                  </Field>
                  <Field id="event-scope" label="Tingkat">
                    <select id="event-scope" className={FIELD} value={form.scope} onChange={(e) => set({ scope: e.target.value as EventScope | "" })}>
                      <option value="">—</option>
                      <option value="national">Nasional</option>
                      <option value="international">Internasional</option>
                    </select>
                  </Field>
                  <Field id="event-url" label="Situs resmi">
                    <Input id="event-url" value={form.officialUrl} onChange={(e) => set({ officialUrl: e.target.value })} placeholder="https://" />
                  </Field>
                  <Field id="event-description" label="Deskripsi (filter Toko)">
                    <Input id="event-description" value={form.description} onChange={(e) => set({ description: e.target.value })} />
                  </Field>
                </div>
              </section>

              <section className="md-card-outlined space-y-4 p-6" aria-label={t("adm_event_narrative")}>
                <div className="flex flex-wrap items-center justify-between gap-2">
                  <h2 className="type-h2">{t("adm_event_narrative")}</h2>
                  <div role="tablist" aria-label={t("adm_pg_narration_lang")} className="inline-flex gap-1 rounded-full bg-surface-sunken p-1">
                    {(["id", "en"] as const).map((loc) => (
                      <button
                        key={loc}
                        type="button"
                        role="tab"
                        aria-selected={tab === loc}
                        onClick={() => setTab(loc)}
                        className={`inline-flex items-center gap-1 rounded-full px-3 py-1 text-sm font-semibold ${tab === loc ? "bg-brand-600 text-on-brand" : "text-ink-700"}`}
                      >
                        {loc === "id" ? "Indonesia" : "English"}
                        {loc === "en" && enGap && <span className="rounded-full bg-ember-600 px-1.5 text-xs text-on-ember">!</span>}
                      </button>
                    ))}
                  </div>
                </div>

                {tab === "en" && (
                  <p className={`text-sm ${enNotice ? "font-medium text-destructive" : "text-ink-600"}`} role={enNotice ? "alert" : undefined}>
                    Wajib diisi sebelum event bisa tampil di landing page: tagline, minimal satu paragraf cerita, dan label EN di setiap jadwal.
                  </p>
                )}

                <div className="grid gap-4">
                  <div className="grid gap-4 sm:grid-cols-2">
                    <Field id={`${tab}-full-name`} label={`Nama lengkap (${tab.toUpperCase()})`}>
                      <Input id={`${tab}-full-name`} value={narrative.fullName} onChange={(e) => setNarrative(tab, { fullName: e.target.value })} />
                    </Field>
                    <Field id={`${tab}-levels`} label={`Label jenjang (${tab.toUpperCase()})`}>
                      <Input id={`${tab}-levels`} value={narrative.levelsLabel} onChange={(e) => setNarrative(tab, { levelsLabel: e.target.value })} placeholder={tab === "id" ? "Kelas 1–12" : "Grades 1–12"} />
                    </Field>
                  </div>
                  <Field id={`${tab}-tagline`} label={`Tagline (${tab.toUpperCase()})`}>
                    <Input id={`${tab}-tagline`} value={narrative.tagline} onChange={(e) => setNarrative(tab, { tagline: e.target.value })} />
                  </Field>
                  <Field id={`${tab}-story`} label={`Cerita (${tab.toUpperCase()})`} hint={<p className="text-xs text-ink-600">Baris kosong memisahkan paragraf.</p>}>
                    <textarea id={`${tab}-story`} rows={6} className={AREA} value={narrative.story} onChange={(e) => setNarrative(tab, { story: e.target.value })} />
                  </Field>
                  <div className="grid gap-4 sm:grid-cols-2">
                    <Field id={`${tab}-rounds`} label={`Alur babak (${tab.toUpperCase()})`} hint={<p className="text-xs text-ink-600">Satu babak per baris.</p>}>
                      <textarea id={`${tab}-rounds`} rows={4} className={AREA} value={narrative.rounds} onChange={(e) => setNarrative(tab, { rounds: e.target.value })} />
                    </Field>
                    <Field id={`${tab}-categories`} label={`Kategori (${tab.toUpperCase()})`} hint={<p className="text-xs text-ink-600">Format &quot;Nama | keterangan&quot;, satu per baris.</p>}>
                      <textarea id={`${tab}-categories`} rows={4} className={AREA} value={narrative.categories} onChange={(e) => setNarrative(tab, { categories: e.target.value })} />
                    </Field>
                  </div>
                  <Field id={`${tab}-facts`} label={`Fakta singkat (${tab.toUpperCase()})`} hint={<p className="text-xs text-ink-600">Format &quot;Label | nilai&quot;, satu per baris.</p>}>
                    <textarea id={`${tab}-facts`} rows={4} className={AREA} value={narrative.facts} onChange={(e) => setNarrative(tab, { facts: e.target.value })} />
                  </Field>
                </div>
              </section>

              <section className="md-card-outlined space-y-3 p-6" aria-label={t("adm_event_schedule")}>
                <div className="flex items-center justify-between gap-2">
                  <h2 className="type-h2">{t("adm_event_schedule")}</h2>
                  <Button
                    type="button"
                    size="sm"
                    variant="outline"
                    onClick={() => set({ schedule: [...form.schedule, { key: ++rowKey, label: "", labelEn: "", startsOn: "", endsOn: "" }] })}
                  >
                    <Plus className="size-4" aria-hidden />
                    Tambah jadwal
                  </Button>
                </div>
                <p className="text-sm text-ink-600">Tahapan seperti pendaftaran, technical meeting, dan tiap babak. Tampil sebagai timeline di landing page.</p>
                {form.schedule.length === 0 && <p className="text-sm text-ink-600">Belum ada jadwal.</p>}
                <ul className="space-y-3">
                  {form.schedule.map((r, i) => {
                    const err = rowError(r);
                    const n = i + 1;
                    return (
                      <li key={r.key} className="grid gap-2 sm:grid-cols-[1fr_1fr_9rem_9rem_auto] sm:items-start">
                        <Input aria-label={`Label jadwal ${n}`} placeholder="Tahapan (ID)" value={r.label} onChange={(e) => setRow(r.key, { label: e.target.value })} />
                        <Input aria-label={`Label EN jadwal ${n}`} placeholder="Stage (EN)" value={r.labelEn} onChange={(e) => setRow(r.key, { labelEn: e.target.value })} />
                        <Input type="date" aria-label={`Mulai jadwal ${n}`} value={r.startsOn} onChange={(e) => setRow(r.key, { startsOn: e.target.value })} />
                        <Input type="date" aria-label={`Selesai jadwal ${n}`} value={r.endsOn} onChange={(e) => setRow(r.key, { endsOn: e.target.value })} />
                        <Button type="button" size="icon" variant="ghost" aria-label={`Hapus jadwal ${n}`} onClick={() => set({ schedule: form.schedule.filter((x) => x.key !== r.key) })}>
                          <Trash2 className="size-4" aria-hidden />
                        </Button>
                        {err && <p role="alert" className="text-sm text-destructive sm:col-span-5">Jadwal {n}: {err}</p>}
                      </li>
                    );
                  })}
                </ul>
              </section>

              <section className="md-card-outlined space-y-4 p-6" aria-label={t("grouping_members_title")}>
                <h2 className="type-h2">{t("grouping_members_title")}</h2>

                {members.length === 0 && <p className="text-sm text-muted-foreground">{t("grouping_members_empty")}</p>}

                <DndContext sensors={sensors} collisionDetection={closestCenter} onDragEnd={handleDragEnd}>
                  {editions.length > 0 && (
                    <div className="space-y-2">
                      <h3 className="text-sm font-semibold">Edisi ({editionTotal})</h3>
                      <SortableContext items={editions.map((p) => p.id)} strategy={verticalListSortingStrategy}>
                        <ul className="divide-y rounded-md border" aria-label="Edisi">
                          {editions.map((p, i) => (
                            <EditionRow
                              key={p.id}
                              groupingId={id}
                              product={p}
                              index={i}
                              total={editions.length}
                              actions={
                                p.type === "exam" && (
                                  <Button type="button" size="sm" variant="outline" aria-label={`Hasil ronde luar ${p.name}`} onClick={() => setExternalFor(p)}>
                                    Hasil ronde luar
                                  </Button>
                                )
                              }
                              onMove={(index, delta) => move("edition", index, delta)}
                              onRemove={(pid) => setMemberIds(ids.filter((x) => x !== pid))}
                            />
                          ))}
                        </ul>
                      </SortableContext>
                    </div>
                  )}
                  {others.length > 0 && (
                    <div className="space-y-2">
                      <h3 className="text-sm font-semibold">Produk lain</h3>
                      <SortableContext items={others.map((p) => p.id)} strategy={verticalListSortingStrategy}>
                        <ul className="divide-y rounded-md border" aria-label="Produk lain">
                          {others.map((p, i) => (
                            <MemberRow
                              key={p.id}
                              product={p}
                              index={i}
                              total={others.length}
                              onMove={(index, delta) => move("other", index, delta)}
                              onRemove={(pid) => setMemberIds(ids.filter((x) => x !== pid))}
                            />
                          ))}
                        </ul>
                      </SortableContext>
                    </div>
                  )}
                </DndContext>

                {externalFor && (
                  <ExternalRoundsDialog
                    key={externalFor.id}
                    open
                    onOpenChange={(o) => !o && setExternalFor(null)}
                    productName={externalFor.name}
                    examIds={externalFor.exam_ids ?? allProducts?.find((x) => x.id === externalFor.id)?.exam_ids ?? []}
                  />
                )}

                <div className="space-y-2">
                  <Input value={search} onChange={(e) => setSearch(e.target.value)} placeholder={t("grouping_add_product")} aria-label={t("grouping_add_product")} />
                  {search.trim() !== "" && (
                    <ul className="max-h-56 overflow-y-auto rounded-md border">
                      {candidates.length === 0 && <li className="px-3 py-2 text-sm text-muted-foreground">{t("grouping_add_product_empty")}</li>}
                      {candidates.map((p) => (
                        <li key={p.id}>
                          <button
                            type="button"
                            className="flex w-full items-center justify-between gap-3 px-3 py-2 text-left text-sm hover:bg-muted"
                            onClick={() => {
                              setMemberIds([...ids, p.id]);
                              setSearch("");
                            }}
                          >
                            <span className="min-w-0 truncate">{p.name}</span>
                            <span className="shrink-0 text-xs text-muted-foreground">{formatRupiah(p.price)}</span>
                          </button>
                        </li>
                      ))}
                    </ul>
                  )}
                </div>

                <div className="flex justify-end">
                  <Button onClick={handleSaveMembers} disabled={setMembers.isPending}>
                    {t("grouping_save_members")}
                  </Button>
                </div>
              </section>
            </div>

            <aside className="min-w-0 space-y-4 lg:sticky lg:top-4 lg:self-start">
              <section className="md-card-outlined space-y-4 p-6" aria-label="Tampilan">
                <h2 className="type-h2">Tampilan</h2>
                <div className="flex items-center gap-3 rounded-xl bg-surface-sunken p-3">
                  <EventMark logoKey={form.logoKey} tone={form.tone} illustration={form.illustration} className="size-12" />
                  <b className="min-w-0 truncate" data-testid="event-preview">{form.shortName.trim() || form.name}</b>
                </div>

                <div className="grid gap-2">
                  <span className="text-sm font-medium">Logo event</span>
                  <div className="flex items-center gap-3">
                    <span className="flex size-16 shrink-0 items-center justify-center overflow-hidden rounded-[18px] border-2 border-dashed border-line bg-surface">
                      {form.logoKey ? (
                        // eslint-disable-next-line @next/next/no-img-element
                        <img src={fileUrl(form.logoKey)} alt="Logo event" className="size-full object-contain p-1" />
                      ) : (
                        <ImageIcon className="size-6 text-ink-600" aria-hidden />
                      )}
                    </span>
                    <div className="flex flex-wrap gap-2">
                      <Button type="button" size="sm" variant="outline" disabled={presign.isPending} onClick={() => logoInput.current?.click()}>
                        <Upload className="size-4" aria-hidden />
                        {form.logoKey ? "Ganti logo" : "Upload logo"}
                      </Button>
                      {form.logoKey && (
                        <Button type="button" size="sm" variant="ghost" onClick={() => set({ logoKey: "" })}>
                          Hapus logo
                        </Button>
                      )}
                    </div>
                    <input ref={logoInput} type="file" accept="image/png,image/svg+xml,image/jpeg,image/webp" hidden aria-label="File logo" data-testid="event-logo-input" onChange={handleLogo} />
                  </div>
                  <p className="text-xs text-ink-600">PNG/SVG transparan. Tanpa logo, ilustrasi yang dipakai.</p>
                </div>

                <Field id="event-tone" label="Warna">
                  <select id="event-tone" className={FIELD} value={form.tone} onChange={(e) => set({ tone: e.target.value as EventTone })}>
                    {EVENT_TONES.map((o) => (
                      <option key={o.value} value={o.value}>{o.label}</option>
                    ))}
                  </select>
                </Field>
                <Field id="event-illustration" label="Ilustrasi">
                  <select id="event-illustration" className={FIELD} value={form.illustration} onChange={(e) => set({ illustration: e.target.value as EventIllustration })}>
                    {EVENT_ILLUSTRATIONS.map((o) => (
                      <option key={o.value} value={o.value}>{o.label}</option>
                    ))}
                  </select>
                </Field>
                <Field id="event-subjects" label="Bidang">
                  <Input id="event-subjects" value={form.subjects} onChange={(e) => set({ subjects: e.target.value })} placeholder="Matematika, Sains" />
                </Field>
                <Field id="event-status" label="Status">
                  <select id="event-status" className={FIELD} value={form.status} onChange={(e) => changeStatus(e.target.value as ProductGroupingStatus)}>
                    <option value="draft">Draft</option>
                    <option value="published">Published</option>
                  </select>
                </Field>

                <label className="flex items-center justify-between gap-3 text-sm font-medium">
                  {t("adm_show_on_landing_page")}
                  <Switch checked={form.visible} onCheckedChange={toggleVisible} aria-label={t("adm_show_on_landing_page")} />
                </label>

                {missing.length > 0 && (
                  <p role="alert" className="text-sm text-destructive">
                    Belum lengkap: {missing.map(groupingMissingLabel).join(", ")}.
                  </p>
                )}

                <Button className="w-full" onClick={handleSave} disabled={update.isPending || form.name.trim() === ""}>
                  Simpan event
                </Button>
                {grouping.products.length === 0 && (
                  <Button className="w-full" variant="ghost" onClick={handleDelete} disabled={remove.isPending}>
                    <Trash2 className="size-4" aria-hidden />
                    Hapus event
                  </Button>
                )}
              </section>
            </aside>
          </div>
        </>
      )}
    </div>
  );
}
