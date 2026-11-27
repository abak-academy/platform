"use client";

import { useMemo, useState } from "react";
import Link from "next/link";
import { useRouter } from "next/navigation";
import { toast } from "sonner";
import { ArrowDown, ArrowUp, CalendarDays, GripVertical, Layers, Pencil, Plus } from "lucide-react";
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
  rectSortingStrategy,
  sortableKeyboardCoordinates,
  useSortable,
} from "@dnd-kit/sortable";
import { CSS } from "@dnd-kit/utilities";
import { AdminPageHeader } from "@/components/admin/AdminPageHeader";
import { EVENT_ART_BG, EventMark } from "@/components/admin/EventMark";
import { ListToolbar, type ListToolbarValue } from "@/components/admin/ListToolbar";
import { ProductGroupingModal } from "@/components/admin/ProductGroupingModal";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { EmptyState } from "@/components/ui/empty-state";
import { Switch } from "@/components/ui/switch";
import { Skeleton } from "@/components/ui/skeleton";
import {
  groupingErrorParts,
  groupingMissingLabel,
  useAdminProductGroupings,
  useCreateProductGrouping,
  useReorderProductGroupings,
  useUpdateProductGrouping,
} from "@/lib/hooks/admin-product-groupings";
import { useTranslation } from "@/lib/i18n";
import type { AdminCreateProductGroupingInput, AdminProductGrouping } from "@/lib/types";

const SORTS = [
  { value: "order", label: "Urutan landing" },
  { value: "name", label: "Nama A–Z" },
  { value: "editions", label: "Edisi terbanyak" },
];

const FILTERS = [
  { key: "landing", label: "Landing page", options: [{ value: "shown", label: "Tampil" }, { value: "hidden", label: "Disembunyikan" }] },
  { key: "status", label: "Status", options: [{ value: "published", label: "Published" }, { value: "draft", label: "Draft" }] },
  { key: "scope", label: "Tingkat", options: [{ value: "national", label: "Nasional" }, { value: "international", label: "Internasional" }] },
  {
    key: "schedule",
    label: "Jadwal",
    options: [
      { value: "ongoing", label: "Sedang berlangsung" },
      { value: "upcoming", label: "Akan datang" },
      { value: "none", label: "Belum diisi" },
    ],
  },
];

const SCOPE_LABEL = { national: "Nasional", international: "Internasional" } as const;

function matches(g: AdminProductGrouping, f: Record<string, string>): boolean {
  if (f.landing && (f.landing === "shown") !== g.is_visible) return false;
  if (f.status && g.status !== f.status) return false;
  if (f.scope && g.scope !== f.scope) return false;
  if (f.schedule === "ongoing" && g.next_milestone?.state !== "ongoing") return false;
  if (f.schedule === "upcoming" && g.next_milestone?.state !== "upcoming") return false;
  if (f.schedule === "none" && g.next_milestone) return false;
  return true;
}

function formatRange(start: string, end: string | null): string {
  const fmt = (d: string) => new Date(`${d}T00:00:00`).toLocaleDateString("id-ID", { day: "numeric", month: "short", year: "numeric" });
  return end && end !== start ? `${fmt(start)} – ${fmt(end)}` : fmt(start);
}

function EventCard({
  grouping,
  reorder,
  index,
  total,
  onMove,
  onToggleVisible,
  togglePending,
}: {
  grouping: AdminProductGrouping;
  reorder: boolean;
  index: number;
  total: number;
  onMove: (index: number, delta: number) => void;
  onToggleVisible: (grouping: AdminProductGrouping) => void;
  togglePending: boolean;
}) {
  const { t } = useTranslation();
  const { attributes, listeners, setNodeRef, transform, transition, isDragging } = useSortable({ id: grouping.id, disabled: !reorder });
  const style = { transform: CSS.Transform.toString(transform), transition };
  const content = grouping.content?.id;
  const next = grouping.next_milestone;
  const levels = [grouping.scope ? SCOPE_LABEL[grouping.scope] : null, content?.levels_label].filter(Boolean).join(" · ");

  return (
    <article
      ref={setNodeRef}
      style={style}
      aria-label={grouping.name}
      className={`md-card-outlined md-card-flush flex flex-col ${isDragging ? "shadow-lg ring-2 ring-brand-600" : ""}`}
    >
      <div
        className="relative flex h-[120px] items-center justify-center"
        style={{ background: EVENT_ART_BG[grouping.tone] ?? EVENT_ART_BG.brand }}
      >
        <Badge variant="mute" className="absolute top-3 left-3 !bg-white/85 !text-ink-900 backdrop-blur-[8px]">
          {t(grouping.is_visible ? "adm_on_landing" : "adm_hidden")}
        </Badge>
        <EventMark
          logoKey={grouping.logo_key}
          logoUrl={grouping.logo_url}
          tone={grouping.tone}
          illustration={grouping.illustration}
          className="size-[90px] bg-transparent !text-white drop-shadow-[0_10px_14px_rgba(0,0,0,.2)]"
        />
        {reorder && (
          <div className="absolute top-2 right-2 flex items-center rounded-full bg-white/70 backdrop-blur">
            <button
              type="button"
              className="cursor-grab touch-none p-1 text-ink-600 active:cursor-grabbing"
              aria-label={t("grouping_drag_handle")}
              {...attributes}
              {...listeners}
            >
              <GripVertical className="size-4" aria-hidden />
            </button>
            <Button type="button" size="icon" variant="ghost" className="size-7" aria-label={t("grouping_move_up")} disabled={index === 0} onClick={() => onMove(index, -1)}>
              <ArrowUp className="size-4" aria-hidden />
            </Button>
            <Button type="button" size="icon" variant="ghost" className="size-7" aria-label={t("grouping_move_down")} disabled={index === total - 1} onClick={() => onMove(index, 1)}>
              <ArrowDown className="size-4" aria-hidden />
            </Button>
          </div>
        )}
      </div>

      <div className="flex flex-1 flex-col gap-2 px-[18px] pt-4 pb-[18px]">
        <p className="font-mono text-[11px] leading-[1.4] font-semibold tracking-[.12em] text-ink-600 uppercase">{levels || "—"}</p>
        <h3 className="truncate text-[19px] leading-[1.35] font-bold tracking-[-0.02em] text-ink-900">{grouping.name}</h3>
        <p className="line-clamp-4 flex-1 text-[13px] leading-[1.55] text-ink-600">{content?.tagline}</p>

        <div className={`flex items-center gap-2 rounded-[12px] px-2.5 py-2 text-[12.5px] leading-[1.55] ${next?.state === "ongoing" ? "bg-ember-50" : "bg-surface-sunken"}`}>
          <CalendarDays className="size-4 shrink-0 text-brand-600" aria-hidden />
          {next ? (
            <>
              <span className="min-w-0 flex-1 truncate">
                <b className="font-[family-name:var(--font-display)]">{next.label}</b> · {formatRange(next.starts_on, next.ends_on)}
              </span>
              {next.state === "ongoing" && <Badge variant="live" className="ml-auto h-5 text-[10.5px]">{t("adm_ongoing")}</Badge>}
            </>
          ) : (
            <span className="text-ink-600">{t("adm_pg_no_schedule")}</span>
          )}
        </div>

        <div className="flex flex-wrap gap-x-3.5 gap-y-1 border-t border-dashed border-line py-2 text-[12.5px] text-ink-600 tabular-nums [&_b]:font-[family-name:var(--font-display)] [&_b]:text-ink-900">
          <span>
            <b>{grouping.edition_count ?? 0}</b> edisi
          </span>
          <span>
            <b>{grouping.open_edition_count ?? 0}</b> dibuka
          </span>
          <span className="text-ink-600">{t(grouping.status === "published" ? "grouping_status_published" : "grouping_status_draft")}</span>
        </div>

        <div className="flex items-center justify-between gap-3">
          <Button size="sm" variant="outline" asChild>
            <Link href={`/admin/product-groupings/${grouping.id}`}>
              <Pencil className="size-3.5" aria-hidden />
              {t("grouping_manage_products")}
            </Link>
          </Button>
          <Switch
            size="sm"
            checked={grouping.is_visible}
            disabled={togglePending}
            aria-label={t("adm_show_on_landing_aria").replace("{name}", grouping.name)}
            onCheckedChange={() => onToggleVisible(grouping)}
          />
        </div>
      </div>
    </article>
  );
}

export default function ProductGroupingsPage() {
  const { t } = useTranslation();
  const router = useRouter();
  const [modalOpen, setModalOpen] = useState(false);
  const [toolbar, setToolbar] = useState<ListToolbarValue>({ q: "", filters: {}, sort: "order" });

  const { data: groupings, isLoading, isError, error } = useAdminProductGroupings();
  const create = useCreateProductGrouping();
  const update = useUpdateProductGrouping();
  const reorder = useReorderProductGroupings();
  const sensors = useSensors(
    useSensor(PointerSensor, { activationConstraint: { distance: 4 } }),
    useSensor(KeyboardSensor, { coordinateGetter: sortableKeyboardCoordinates }),
  );

  const sort = toolbar.sort ?? "order";
  const byLandingOrder = useMemo(() => [...(groupings ?? [])].sort((a, b) => a.display_order - b.display_order), [groupings]);
  const filtering = toolbar.q.trim() !== "" || Object.keys(toolbar.filters).length > 0;
  const canReorder = sort === "order" && !filtering;

  const rows = useMemo(() => {
    const q = toolbar.q.trim().toLowerCase();
    const list = byLandingOrder.filter(
      (g) => matches(g, toolbar.filters) && (q === "" || `${g.name} ${g.short_name ?? ""}`.toLowerCase().includes(q)),
    );
    if (sort === "name") return [...list].sort((a, b) => a.name.localeCompare(b.name));
    if (sort === "editions") return [...list].sort((a, b) => (b.edition_count ?? 0) - (a.edition_count ?? 0));
    return list;
  }, [byLandingOrder, toolbar.q, toolbar.filters, sort]);

  function moveTo(ids: string[]) {
    reorder.mutate(ids, { onError: (e) => toast.error(groupingErrorParts(e).message) });
  }

  function move(index: number, delta: number) {
    const target = index + delta;
    if (target < 0 || target >= rows.length) return;
    moveTo(arrayMove(rows, index, target).map((g) => g.id));
  }

  function handleDragEnd({ active, over }: DragEndEvent) {
    if (!over || active.id === over.id) return;
    const from = rows.findIndex((g) => g.id === active.id);
    const to = rows.findIndex((g) => g.id === over.id);
    if (from === -1 || to === -1) return;
    moveTo(arrayMove(rows, from, to).map((g) => g.id));
  }

  async function handleCreate(input: AdminCreateProductGroupingInput) {
    try {
      const created = await create.mutateAsync(input);
      toast.success(t("grouping_created"));
      setModalOpen(false);
      router.push(`/admin/product-groupings/${created.id}`);
    } catch (e) {
      toast.error(groupingErrorParts(e).message);
    }
  }

  async function handleToggleVisible(grouping: AdminProductGrouping) {
    try {
      await update.mutateAsync({ id: grouping.id, input: { is_visible: !grouping.is_visible } });
    } catch (e) {
      const { message, missing } = groupingErrorParts(e);
      toast.error(missing.length > 0 ? `${message} (${missing.map(groupingMissingLabel).join(", ")})` : message);
    }
  }

  return (
    <div className="flex flex-col gap-5 fade-in">
      <AdminPageHeader
        title={t("grouping_page_title")}
        description={t("grouping_page_description")}
        actions={
          <Button variant="dark" onClick={() => setModalOpen(true)}>
            <Plus aria-hidden />
            {t("grouping_create")}
          </Button>
        }
      />

      {isLoading && (
        <div className="grid grid-cols-[repeat(auto-fill,minmax(min(260px,100%),1fr))] gap-4">
          {Array.from({ length: 3 }).map((_, i) => (
            <Skeleton key={i} className="h-56 w-full" />
          ))}
        </div>
      )}

      {isError && (
        <div className="rounded-lg border border-destructive/20 bg-destructive/10 p-4 text-destructive">
          {t("grouping_load_failed")}: {groupingErrorParts(error).message}
        </div>
      )}

      {!isLoading && !isError && (
        <>
          <ListToolbar
            variant="standalone"
            value={toolbar}
            onChange={setToolbar}
            filters={FILTERS}
            sorts={SORTS}
            count={rows.length}
            unit="event"
            searchPlaceholder="Cari nama event"
          />

          {rows.length === 0 ? (
            <EmptyState framed icon={Layers} title={t("empty_groupings")} text={filtering ? t("adm_filter_hint") : undefined} />
          ) : (
            <DndContext sensors={sensors} collisionDetection={closestCenter} onDragEnd={handleDragEnd}>
              <SortableContext items={rows.map((g) => g.id)} strategy={rectSortingStrategy}>
                <div className="grid grid-cols-[repeat(auto-fill,minmax(min(260px,100%),1fr))] gap-4">
                  {rows.map((g, i) => (
                    <EventCard
                      key={g.id}
                      grouping={g}
                      reorder={canReorder}
                      index={i}
                      total={rows.length}
                      onMove={move}
                      onToggleVisible={handleToggleVisible}
                      togglePending={update.isPending}
                    />
                  ))}
                </div>
              </SortableContext>
            </DndContext>
          )}
        </>
      )}

      <ProductGroupingModal open={modalOpen} onOpenChange={setModalOpen} onSubmit={handleCreate} isPending={create.isPending} />
    </div>
  );
}
