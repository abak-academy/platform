import { describe, expect, it } from "vitest";
import {
  courseRowText,
  eventRows,
  heroEvents,
  kompetisiCount,
  levelText,
  pendingCount,
  pickShipment,
  shipmentSteps,
  shopPicks,
} from "./student-home";
import type { Order, OrderItem, Product, PublicEdition, PublicEvent, RegistrationListItem } from "./types";

const NOW = Date.parse("2026-11-26T10:00:00Z");
const iso = (offsetMin: number) => new Date(NOW + offsetMin * 60_000).toISOString();

function reg(over: Partial<RegistrationListItem> = {}): RegistrationListItem {
  return {
    id: "r1", student_id: "s1", exam_id: "e1", token: "t", card_key: null, checked_in_at: null,
    attempts_used: 0, status: "registered", created_at: iso(-1000), exam_title: "Exam",
    scheduled_at: null, scheduled_end_at: null, is_free: false, requires_checkin: false,
    check_in_window_minutes: null, duration_minutes: 60, session_id: null, max_attempts: 1,
    navigation_policy: "free", timer_mode: "overall", section_count: 1, question_count: 10,
    sections_duration_minutes: null, ...over,
  };
}

function item(product_type: string): OrderItem {
  return { id: "i", order_id: "o", product_id: "p", product_type, name: "n", unit_price: 1, qty: 1, jumlah: 1 };
}

function order(over: Partial<Order> = {}): Order {
  return {
    id: "o1", student_id: "s1", status: "processing", subtotal: 1, discount: 0, shipping_cost: 0, total: 1,
    created_at: iso(-100), items: [item("book")], ...over,
  } as Order;
}

describe("kompetisiCount", () => {
  it("counts startable and in-progress cards only", () => {
    const regs = [
      reg({ id: "a" }),
      reg({ id: "b", status: "in_progress" }),
      reg({ id: "c", status: "submitted" }),
      reg({ id: "d", scheduled_at: iso(60), scheduled_end_at: iso(120) }),
      reg({ id: "e", scheduled_at: iso(-120), scheduled_end_at: iso(-60) }),
    ];
    expect(kompetisiCount(regs, NOW)).toBe(2);
  });
});

describe("pendingCount", () => {
  it("counts payment_pending orders", () => {
    expect(pendingCount([order({ status: "payment_pending" }), order({ status: "paid" }), order({ status: "payment_pending" })])).toBe(2);
    expect(pendingCount([])).toBe(0);
  });
});

describe("pickShipment", () => {
  it("picks the newest processing/shipped order with a physical item", () => {
    const older = order({ id: "old", created_at: iso(-300) });
    const newer = order({ id: "new", status: "shipped", created_at: iso(-10) });
    expect(pickShipment([older, newer])?.id).toBe("new");
  });
  it("excludes digital-only orders", () => {
    expect(pickShipment([order({ items: [item("exam"), item("course")] })])).toBeNull();
  });
  it("hides completed and other statuses", () => {
    expect(pickShipment([order({ status: "completed" }), order({ status: "paid" })])).toBeNull();
  });
  it("returns null for no orders", () => {
    expect(pickShipment([])).toBeNull();
  });
});

describe("shipmentSteps", () => {
  it("marks packed reached with a null date while processing", () => {
    const steps = shipmentSteps(order({ status: "processing", paid_at: iso(-50) }));
    expect(steps.map((s) => [s.key, s.reached])).toEqual([
      ["paid", true], ["packed", true], ["shipped", false], ["delivered", false],
    ]);
    expect(steps[1].date).toBeNull();
    expect(steps[0].date).toBe(iso(-50));
  });
  it("shipped reaches shipped with its date but not delivered", () => {
    const steps = shipmentSteps(order({ status: "shipped", paid_at: iso(-50), shipped_at: iso(-5) }));
    expect(steps[2]).toMatchObject({ key: "shipped", reached: true, date: iso(-5) });
    expect(steps[3].reached).toBe(false);
  });
  it("completed reaches everything", () => {
    const steps = shipmentSteps(order({ status: "completed", completed_at: iso(-1) }));
    expect(steps.every((s) => s.reached)).toBe(true);
    expect(steps[3].date).toBe(iso(-1));
  });
});

function edition(over: Partial<PublicEdition> = {}): PublicEdition {
  return {
    product_id: "p1", exam_ids: ["e1"], title: "T", stages: ["SD"], grade_min: 4, grade_max: 6, price: 1000,
    image_url: "", phase: "open", scheduled_at: null, scheduled_end_at: null, available_until: null, ...over,
  };
}
function event(name: string, eds: PublicEdition[]): PublicEvent {
  return {
    slug: name, name, short_name: null, scope: null, subjects: [], tone: "ember", illustration: "ill-" + name,
    logo_url: "", official_url: null, full_name: "", tagline: "", levels_label: "", story: [], rounds: [],
    categories: [], facts: [], schedule: [], open_editions: eds,
  };
}

describe("heroEvents", () => {
  it("flattens in API order without re-sorting and maps fields", () => {
    const slides = heroEvents(
      [event("B", [edition({ product_id: "b1", image_url: "x.png", phase: "live" })]), event("A", [edition({ product_id: "a1" }), edition({ product_id: "a2" })])],
      [],
    );
    expect(slides.map((s) => s.productId)).toEqual(["b1", "a1", "a2"]);
    expect(slides[0]).toMatchObject({ eventName: "B", phase: "live", imageUrl: "x.png", illustration: "ill-B", tone: "ember", gradeMin: 4, price: 1000 });
    expect(slides[1].imageUrl).toBeNull();
  });
  it("drops an edition only when every exam is registered", () => {
    const eds = [
      edition({ product_id: "all", exam_ids: ["e1", "e2"] }),
      edition({ product_id: "partial", exam_ids: ["e1", "e3"] }),
    ];
    const regs = [reg({ exam_id: "e1" }), reg({ id: "r2", exam_id: "e2" })];
    expect(heroEvents([event("A", eds)], regs).map((s) => s.productId)).toEqual(["partial"]);
  });
});

describe("levelText", () => {
  it("formats stage with a grade range", () => expect(levelText(["SD"], 4, 6, "Kelas")).toBe("SD · Kelas 4–6"));
  it("uses a single grade when min equals max", () => expect(levelText(["SMP"], 5, 5, "Kelas")).toBe("SMP · Kelas 5"));
  it("uses the lone bound", () => {
    expect(levelText([], 4, null, "Kelas")).toBe("Kelas 4");
    expect(levelText([], null, 6, "Kelas")).toBe("Kelas 6");
  });
  it("joins several stages with a slash", () => expect(levelText(["SD", "SMP"], null, null, "Kelas")).toBe("SD/SMP"));
  it("is empty when nothing is set", () => expect(levelText([], null, null, "Kelas")).toBe(""));
});

describe("eventRows", () => {
  it("orders unfinished by schedule asc with unscheduled last, then finished newest first", () => {
    const regs = [
      reg({ id: "none" }),
      reg({ id: "late", scheduled_at: iso(600), scheduled_end_at: iso(700) }),
      reg({ id: "soon", scheduled_at: iso(60), scheduled_end_at: iso(120) }),
      reg({ id: "doneOld", status: "submitted", scheduled_at: iso(-900) }),
      reg({ id: "doneNew", status: "submitted", scheduled_at: iso(-500), scheduled_end_at: iso(-400) }),
    ];
    expect(eventRows(regs, NOW).map((r) => r.reg.id)).toEqual(["soon", "late", "none", "doneNew"]);
  });
  it("caps at 4", () => {
    const regs = Array.from({ length: 6 }, (_, i) => reg({ id: "r" + i }));
    expect(eventRows(regs, NOW)).toHaveLength(4);
  });
  it("resolves actions with external first, even when submitted", () => {
    const ext = [{ round_id: "x", round_label: "R", score: 1, rank: null, award: null, certificate_url: null, published_at: iso(0) }];
    const rows = eventRows(
      [
        reg({ id: "ext", status: "submitted", external_results: ext }),
        reg({ id: "sub", status: "submitted" }),
        reg({ id: "exp", scheduled_at: iso(-120), scheduled_end_at: iso(-60) }),
        reg({ id: "go", status: "in_progress" }),
      ],
      NOW,
    );
    const by = Object.fromEntries(rows.map((r) => [r.reg.id, r]));
    expect(by.ext.action).toBe("external");
    expect(by.sub.action).toBe("result");
    expect(by.exp.action).toBe("expired");
    expect(by.go).toMatchObject({ action: "start", live: true });
    const [locked] = eventRows([reg({ scheduled_at: iso(60), scheduled_end_at: iso(120) })], NOW);
    expect(locked).toMatchObject({ action: "detail", live: false });
  });
});

describe("courseRowText", () => {
  it("shows the next lesson and rounds percent", () => {
    expect(courseRowText({ progress: 0.334, done_lessons: 2, total_lessons: 6 })).toEqual({ percent: 33, lesson: 3, total: 6, done: false });
  });
  it("clamps lesson to total and flags done at a full fraction", () => {
    expect(courseRowText({ progress: 1, done_lessons: 6, total_lessons: 6 })).toEqual({ percent: 100, lesson: 6, total: 6, done: true });
  });
});

function prod(over: Partial<Product>): Product {
  return { id: "p", type: "book", name: "n", price: 1, ...over };
}

describe("shopPicks", () => {
  it("skips exams, owned courses and sold-out physical products", () => {
    const out = shopPicks(
      [
        prod({ id: "exam", type: "exam" }),
        prod({ id: "owned", type: "course", course_ids: ["c1"] }),
        prod({ id: "partial", type: "course", course_ids: ["c1", "c2"] }),
        prod({ id: "nocourses", type: "course" }),
        prod({ id: "soldout", type: "book", stock: 0 }),
        prod({ id: "ok", type: "medal", stock: 3 }),
      ],
      ["c1"],
    );
    expect(out.map((p) => p.id)).toEqual(["partial", "nocourses", "ok"]);
  });
  it("caps at 4 in input order and handles empty", () => {
    const many = Array.from({ length: 6 }, (_, i) => prod({ id: "p" + i }));
    expect(shopPicks(many, []).map((p) => p.id)).toEqual(["p0", "p1", "p2", "p3"]);
    expect(shopPicks([], [])).toEqual([]);
  });
});
