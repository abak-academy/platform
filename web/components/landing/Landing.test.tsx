import { render, screen, within } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { Landing } from "./Landing";
import { edition, event } from "@/components/events/fixtures";
import type { Product } from "@/lib/types";

const h = vi.hoisted(() => ({
  events: {} as { data?: unknown; isError?: boolean },
  stories: {} as { data?: unknown; isError?: boolean },
  courses: {} as { data?: unknown; isError?: boolean },
  all: {} as { data?: unknown; isError?: boolean },
}));

vi.mock("@/lib/hooks/event-series", () => ({ useEventSeries: () => h.events }));
vi.mock("@/lib/hooks/testimonials", () => ({ useTestimonials: () => h.stories }));
vi.mock("@/lib/hooks/products", () => ({ useProducts: (type?: string) => (type === "course" ? h.courses : h.all) }));

const product = (id: string, type: Product["type"]): Product => ({ id, type, name: `Produk ${id}`, price: 1000, stock: 5, status: "published" });
const story = (id: string) => ({ id, name: `Peserta ${id}`, role_label: "Siswa", photo_url: null, quote: `Kutipan ${id}`, achievements: [] });

const order = () => Array.from(document.querySelectorAll("[data-section]")).map((n) => n.getAttribute("data-section"));

beforeEach(() => {
  h.events = { data: [event({ slug: "a", name: "ESOLIO", open_editions: [edition()] }), event({ slug: "b", name: "Panjang", short_name: "IKMC" })] };
  h.stories = { data: [story("1"), story("2")] };
  h.courses = { data: [product("c1", "course")] };
  h.all = { data: [product("b1", "book"), product("e1", "exam"), product("c1", "course"), product("m1", "merchandise")] };
});

describe("Landing", () => {
  it("renders the sections in spec order with every optional section present", () => {
    render(<Landing />);
    expect(order()).toEqual(["hero", "programs", "about", "events", "features", "stories", "shop", "school", "faq", "cta"]);
  });

  it("has no Baru kicker and only the hero stats list (no end-of-page stats band)", () => {
    render(<Landing />);
    expect(screen.queryByText("Baru")).not.toBeInTheDocument();
    expect(screen.getAllByText("20.000+")).toHaveLength(1);
    expect(document.querySelector("[data-hero]")).toContainElement(screen.getByText("20.000+"));
  });

  it("hero rotating words are the API event names, short_name first", () => {
    render(<Landing />);
    const hero = document.querySelector("[data-hero]") as HTMLElement;
    const words = Array.from(hero.querySelectorAll("[data-active]")).map((n) => n.textContent);
    expect(words).toEqual(["dunia", "ESOLIO", "IKMC"]);
  });

  it("omits the stories section with zero testimonials and on error", () => {
    h.stories = { data: [] };
    const { unmount } = render(<Landing />);
    expect(order()).not.toContain("stories");
    unmount();
    h.stories = { isError: true };
    render(<Landing />);
    expect(order()).not.toContain("stories");
  });

  it("omits the course section when empty", () => {
    h.courses = { data: [] };
    render(<Landing />);
    expect(order()).not.toContain("courses");
  });

  it("one Toko section mixes courses with books/merchandise/medals, no exams, at most 8", () => {
    h.all = {
      data: [
        product("b1", "book"), product("e1", "exam"), product("c1", "course"), product("m1", "merchandise"),
        product("b2", "book"), product("b3", "book"), product("b4", "book"), product("b5", "book"),
        product("b6", "book"), product("b7", "book"),
      ],
    };
    render(<Landing />);
    expect(document.querySelector('[data-section="courses"]')).toBeNull();
    const shop = within(document.querySelector('[data-section="shop"]') as HTMLElement);
    expect(shop.getAllByRole("link", { name: /Produk|Kursus/ }).map((a) => a.getAttribute("href"))).toEqual([
      "/catalog/b1", "/catalog/c1", "/catalog/m1", "/catalog/b2", "/catalog/b3", "/catalog/b4", "/catalog/b5", "/catalog/b6",
    ]);
    expect(shop.getByRole("link", { name: "Ke toko" })).toHaveAttribute("href", "/catalog");
  });

  it("an event without open editions offers Tanya jadwal", () => {
    h.events = { data: [event({ open_editions: [] })] };
    render(<Landing />);
    expect(within(document.querySelector('[data-section="events"]') as HTMLElement).getByRole("link", { name: "Tanya jadwal" })).toBeInTheDocument();
  });

  it("on event API error still renders: hero shows only the fallback word and the event section is the empty state", () => {
    h.events = { isError: true };
    render(<Landing />);
    const hero = document.querySelector("[data-hero]") as HTMLElement;
    expect(Array.from(hero.querySelectorAll("[data-active]")).map((n) => n.textContent)).toEqual(["dunia"]);
    const events = within(document.querySelector('[data-section="events"]') as HTMLElement);
    expect(events.getByText("Event belum tersedia")).toBeInTheDocument();
    expect(events.getByRole("link", { name: "Tanya via WhatsApp" }).getAttribute("href")).toMatch(/^https:\/\/wa\.me\//);
    expect(order()).toContain("cta");
  });
});
