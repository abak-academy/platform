import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import { render, screen, act, fireEvent, within } from "@testing-library/react";
import { EventCarousel } from "./EventCarousel";
import type { HeroSlide } from "@/lib/student-home";

const slide = (o: Partial<HeroSlide> = {}): HeroSlide => ({
  productId: "p1",
  eventName: "Olimpiade Sains",
  phase: "open",
  title: "Babak Penyisihan",
  stages: ["SD", "SMP"],
  gradeMin: 4,
  gradeMax: 6,
  price: 50000,
  imageUrl: null,
  illustration: "science",
  tone: "teal",
  ...o,
});

const A = slide();
const B = slide({ productId: "p2", eventName: "Kompetisi Bahasa", phase: "live", title: "Final", stages: [], gradeMin: null, gradeMax: null, price: 0, illustration: "language", tone: "sky" });

const mockMotion = (reduce: boolean) =>
  vi.stubGlobal("matchMedia", (q: string) => ({
    matches: reduce && q.includes("prefers-reduced-motion"),
    media: q,
    addEventListener: () => {},
    removeEventListener: () => {},
  }));

const slides = () => document.querySelectorAll<HTMLElement>("[data-slide]");
const activeIdx = () => [...slides()].findIndex((s) => !s.hasAttribute("inert"));
const tick = (ms: number) => act(() => void vi.advanceTimersByTime(ms));

beforeEach(() => {
  vi.useFakeTimers();
  mockMotion(false);
});
afterEach(() => {
  vi.useRealTimers();
  vi.unstubAllGlobals();
});

describe("EventCarousel", () => {
  it("renders open slide: overline, headline, CTA href, My Competitions, level and price", () => {
    render(<EventCarousel slides={[A]} />);
    expect(screen.getByRole("region", { name: "Event terbaru" })).toHaveAttribute("aria-roledescription", "carousel");
    expect(screen.getByText("Olimpiade Sains")).toBeInTheDocument();
    expect(screen.getByRole("heading", { name: "Pendaftaran dibuka" })).toBeInTheDocument();
    expect(screen.getByRole("link", { name: /Daftar sekarang/ })).toHaveAttribute("href", "/catalog/p1");
    expect(screen.getByRole("link", { name: "Kompetisi Saya" })).toHaveAttribute("href", "/exam");
    expect(screen.getByText(/Babak Penyisihan · SD\/SMP · Kelas 4–6 · Rp\s?50\.000/)).toBeInTheDocument();
  });

  it("renders live slide with Gratis and no level segment", () => {
    render(<EventCarousel slides={[B]} />);
    expect(screen.getByRole("heading", { name: "Sedang berlangsung" })).toBeInTheDocument();
    expect(screen.getByRole("link", { name: /Jelajahi sekarang/ })).toHaveAttribute("href", "/catalog/p2");
    expect(screen.getByText("Final · Gratis")).toBeInTheDocument();
  });

  it("renders the image when imageUrl is set, a fallback art otherwise", () => {
    const { container, rerender } = render(<EventCarousel slides={[slide({ imageUrl: "https://cdn.test/a.png" })]} />);
    expect(container.querySelector("img")).toHaveAttribute("src", "https://cdn.test/a.png");
    rerender(<EventCarousel slides={[A]} />);
    expect(container.querySelector("img")).toBeNull();
    expect(container.querySelector("svg")).not.toBeNull();
  });

  it("advances after 6000ms and wraps", () => {
    render(<EventCarousel slides={[A, B]} />);
    expect(activeIdx()).toBe(0);
    tick(5999);
    expect(activeIdx()).toBe(0);
    tick(1);
    expect(activeIdx()).toBe(1);
    tick(6000);
    expect(activeIdx()).toBe(0);
  });

  it("pauses while hovered and resumes after leave", () => {
    render(<EventCarousel slides={[A, B]} />);
    const region = screen.getByRole("region");
    fireEvent.mouseEnter(region);
    tick(20000);
    expect(activeIdx()).toBe(0);
    fireEvent.mouseLeave(region);
    tick(6000);
    expect(activeIdx()).toBe(1);
  });

  it("pauses while focus is inside and resumes after focus leaves", () => {
    render(<EventCarousel slides={[A, B]} />);
    const region = screen.getByRole("region");
    fireEvent.focusIn(within(region).getByRole("link", { name: /Daftar sekarang/ }));
    tick(20000);
    expect(activeIdx()).toBe(0);
    fireEvent.focusOut(region);
    tick(6000);
    expect(activeIdx()).toBe(1);
  });

  it("does not autoplay under prefers-reduced-motion", () => {
    mockMotion(true);
    render(<EventCarousel slides={[A, B]} />);
    tick(20000);
    expect(activeIdx()).toBe(0);
  });

  it("one slide: no autoplay target change and no dots", () => {
    render(<EventCarousel slides={[A]} />);
    tick(20000);
    expect(screen.queryAllByRole("button")).toHaveLength(0);
    expect(activeIdx()).toBe(0);
  });

  it("empty slides: fallback slide, CTA to My Competitions, no dots", () => {
    render(<EventCarousel slides={[]} />);
    expect(screen.getByText("Belum ada event baru")).toBeInTheDocument();
    expect(screen.getByRole("link", { name: /Kompetisi Saya/ })).toHaveAttribute("href", "/exam");
    expect(screen.queryAllByRole("button")).toHaveLength(0);
  });

  it("dots are labelled, mark the active one, and select on click", () => {
    render(<EventCarousel slides={[A, B]} />);
    const d1 = screen.getByRole("button", { name: "Event 1 dari 2: Olimpiade Sains" });
    const d2 = screen.getByRole("button", { name: "Event 2 dari 2: Kompetisi Bahasa" });
    expect(d1).toHaveAttribute("aria-current", "true");
    expect(d2).not.toHaveAttribute("aria-current");
    fireEvent.click(d2);
    expect(activeIdx()).toBe(1);
    expect(d2).toHaveAttribute("aria-current", "true");
    expect(d1).not.toHaveAttribute("aria-current");
  });

  it("inactive slides are inert and aria-hidden; active is neither", () => {
    render(<EventCarousel slides={[A, B]} />);
    const [s0, s1] = slides();
    expect(s0).not.toHaveAttribute("inert");
    expect(s0).not.toHaveAttribute("aria-hidden");
    expect(s1).toHaveAttribute("inert");
    expect(s1).toHaveAttribute("aria-hidden", "true");
  });
});
