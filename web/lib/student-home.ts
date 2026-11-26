import { computeCardState } from "@/lib/exam-card-state";
import { workingMinutes } from "@/lib/exam-schedule";
import type { Order, OrderItem, Product, PublicEvent, RegistrationListItem } from "@/lib/types";

export { workingMinutes as minutesFor };

const LIVE_KINDS = ["start", "checkin", "in_progress"];

export function kompetisiCount(regs: RegistrationListItem[], now: number): number {
  return regs.filter((r) => LIVE_KINDS.includes(computeCardState(r, now).kind)).length;
}

export function pendingCount(orders: Order[]): number {
  return orders.filter((o) => o.status === "payment_pending").length;
}

export function physicalItems(order: Order): OrderItem[] {
  return (order.items ?? []).filter((i) => i.product_type !== "exam" && i.product_type !== "course");
}

const time = (s?: string) => (s ? new Date(s).getTime() : 0);

export function pickShipment(orders: Order[]): Order | null {
  const eligible = orders.filter(
    (o) => (o.status === "processing" || o.status === "shipped") && physicalItems(o).length > 0,
  );
  return eligible.reduce<Order | null>((best, o) => (!best || time(o.created_at) > time(best.created_at) ? o : best), null);
}

export type ShipmentStepKey = "paid" | "packed" | "shipped" | "delivered";

export function shipmentSteps(order: Order): { key: ShipmentStepKey; reached: boolean; date: string | null }[] {
  const level = order.status === "completed" ? 3 : order.status === "shipped" ? 2 : order.status === "processing" ? 1 : 0;
  const dates: (string | null)[] = [order.paid_at ?? null, null, order.shipped_at ?? null, order.completed_at ?? null];
  return (["paid", "packed", "shipped", "delivered"] as const).map((key, i) => ({
    key,
    reached: dates[i] !== null || level >= i,
    date: dates[i],
  }));
}

export interface HeroSlide {
  productId: string;
  eventName: string;
  phase: "open" | "live";
  title: string;
  stages: string[];
  gradeMin: number | null;
  gradeMax: number | null;
  price: number;
  imageUrl: string | null;
  illustration: string;
  tone: string;
}

export function heroEvents(events: PublicEvent[], regs: RegistrationListItem[]): HeroSlide[] {
  const registered = new Set(regs.map((r) => r.exam_id));
  return events.flatMap((ev) =>
    ev.open_editions
      .filter((ed) => !(ed.exam_ids.length > 0 && ed.exam_ids.every((id) => registered.has(id))))
      .map((ed) => ({
        productId: ed.product_id,
        eventName: ev.name,
        phase: ed.phase,
        title: ed.title,
        stages: ed.stages,
        gradeMin: ed.grade_min,
        gradeMax: ed.grade_max,
        price: ed.price,
        imageUrl: ed.image_url || null,
        illustration: ev.illustration,
        tone: ev.tone,
      })),
  );
}

export function levelText(stages: string[], gradeMin: number | null, gradeMax: number | null, gradeWord: string): string {
  const stage = stages.join("/");
  let grade = "";
  if (gradeMin != null && gradeMax != null) {
    grade = gradeMin === gradeMax ? `${gradeWord} ${gradeMin}` : `${gradeWord} ${gradeMin}–${gradeMax}`;
  } else if (gradeMin != null || gradeMax != null) {
    grade = `${gradeWord} ${gradeMin ?? gradeMax}`;
  }
  return [stage, grade].filter(Boolean).join(" · ");
}

export interface EventRow {
  reg: RegistrationListItem;
  live: boolean;
  action: "external" | "result" | "expired" | "start" | "detail";
}

const MAX_EVENT_ROWS = 4;

export function eventRows(regs: RegistrationListItem[], now: number): EventRow[] {
  const rows = regs.map((reg): EventRow & { finished: boolean } => {
    const kind = computeCardState(reg, now).kind;
    const live = LIVE_KINDS.includes(kind);
    const action = reg.external_results?.length
      ? "external"
      : kind === "submitted"
        ? "result"
        : kind === "expired"
          ? "expired"
          : live
            ? "start"
            : "detail";
    return { reg, live, action, finished: kind === "submitted" || kind === "expired" };
  });
  const sched = (r: EventRow) => (r.reg.scheduled_at ? time(r.reg.scheduled_at) : Infinity);
  const end = (r: EventRow) => time(r.reg.scheduled_end_at ?? r.reg.scheduled_at ?? undefined);
  const open = rows.filter((r) => !r.finished).sort((a, b) => sched(a) - sched(b));
  const done = rows.filter((r) => r.finished).sort((a, b) => end(b) - end(a));
  return [...open, ...done].slice(0, MAX_EVENT_ROWS).map(({ finished: _, ...r }) => r);
}

export function courseRowText(c: { progress: number; done_lessons: number; total_lessons: number }) {
  return {
    percent: Math.round(c.progress),
    lesson: Math.min(c.done_lessons + 1, c.total_lessons),
    total: c.total_lessons,
    done: c.progress >= 100,
  };
}

const MAX_SHOP_PICKS = 4;

export function shopPicks(products: Product[], enrolledCourseIds: string[]): Product[] {
  const owned = new Set(enrolledCourseIds);
  return products
    .filter((p) => {
      if (p.type === "exam") return false;
      if (p.type === "course") return !(p.course_ids?.length && p.course_ids.every((id) => owned.has(id)));
      return p.stock !== 0;
    })
    .slice(0, MAX_SHOP_PICKS);
}
