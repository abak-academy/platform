import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import { render, cleanup, act, waitFor } from "@testing-library/react";
import { PublicMotion } from "./PublicMotion";

type IOCb = (entries: { target: Element; isIntersecting: boolean }[]) => void;
let ioCb: IOCb;
const observed = new Set<Element>();

function mockMedia(reduced: boolean, coarse: boolean) {
  window.matchMedia = vi.fn().mockImplementation((q: string) => ({
    matches: q.includes("reduce") ? reduced : q.includes("coarse") ? coarse : false,
    addEventListener: vi.fn(),
    removeEventListener: vi.fn(),
  })) as unknown as typeof window.matchMedia;
}

function page() {
  document.body.innerHTML = `
    <section data-hero data-testid="hero"><div data-hero-photo id="photo"></div><div data-hero-content></div></section>
    <section data-reveal id="s1"><h2>A</h2><div data-reveal-grid><div data-tilt>1</div><div data-tilt>2</div></div></section>
    <section data-reveal id="s2"><h2>B</h2></section>
    <section data-reveal id="s3"><h2>C</h2></section>
    <section data-reveal id="live"><h2>D</h2></section>`;
  const rect = (top: number) => () => ({ top, height: 400, left: 0, width: 400 }) as DOMRect;
  for (const id of ["s1", "s2", "s3"]) document.getElementById(id)!.getBoundingClientRect = rect(3000);
  document.getElementById("live")!.getBoundingClientRect = rect(100);
}

const effectOf = (id: string) => [...document.getElementById(id)!.classList].find((c) => /^pm-(up|left|right|zoom|blur|tilt)$/.test(c));
const hidden = () => document.querySelectorAll("section.pm-reveal").length;

beforeEach(() => {
  observed.clear();
  vi.stubGlobal(
    "IntersectionObserver",
    class {
      constructor(cb: IOCb) {
        ioCb = cb;
      }
      observe = (el: Element) => observed.add(el);
      unobserve = (el: Element) => observed.delete(el);
      disconnect = () => observed.clear();
    },
  );
  page();
});
afterEach(() => {
  cleanup();
  vi.unstubAllGlobals();
  document.body.innerHTML = "";
});

describe.each([
  ["reduced motion", true, false],
  ["coarse pointer", false, true],
])("%s", (_n, reduced, coarse) => {
  it("initializes nothing", () => {
    mockMedia(reduced, coarse);
    const add = vi.spyOn(window, "addEventListener");
    render(<PublicMotion />);
    act(() => window.dispatchEvent(new Event("scroll")));
    expect(document.querySelector("canvas")).toBeNull();
    expect(hidden()).toBe(0);
    expect(document.getElementById("photo")!.style.transform).toBe("");
    expect(add.mock.calls.filter(([t]) => t === "pointermove" || t === "scroll")).toHaveLength(0);
    expect(observed.size).toBe(0);
  });
});

describe("motion allowed", () => {
  beforeEach(() => mockMedia(false, false));

  it("adds the swarm canvas, hides off-screen sections and reveals them when intersecting", () => {
    render(<PublicMotion />);
    expect(document.querySelector("canvas[aria-hidden='true']")).not.toBeNull();
    const s1 = document.getElementById("s1")!;
    expect(s1.classList.contains("pm-reveal")).toBe(true);
    expect(s1.classList.contains("pm-in")).toBe(false);
    act(() => ioCb([{ target: s1, isIntersecting: true }]));
    expect(s1.classList.contains("pm-in")).toBe(true);
    expect(observed.has(s1)).toBe(false);
  });

  it("never hides a section already in view on load", () => {
    render(<PublicMotion />);
    expect(document.getElementById("live")!.classList.contains("pm-reveal")).toBe(false);
    expect(observed.has(document.getElementById("live")!)).toBe(false);
  });

  it("gives adjacent sections different effects and staggers grid children", () => {
    render(<PublicMotion />);
    const [a, b, c] = ["s1", "s2", "s3"].map(effectOf);
    expect(new Set([a, b, c]).size).toBe(3);
    const kids = document.querySelectorAll("#s1 [data-reveal-grid] > .pm-kid");
    expect(kids).toHaveLength(2);
    expect((kids[1] as HTMLElement).style.getPropertyValue("--pm-d")).toBe("200ms");
  });

  it("handles sections that appear after load", async () => {
    render(<PublicMotion />);
    const late = document.createElement("section");
    late.setAttribute("data-reveal", "");
    late.getBoundingClientRect = () => ({ top: 3000 }) as DOMRect;
    document.body.appendChild(late);
    await Promise.resolve();
    expect(late.classList.contains("pm-reveal")).toBe(true);
    expect(observed.has(late)).toBe(true);
  });

  it("applies hero parallax", async () => {
    render(<PublicMotion />);
    await waitFor(() => expect(document.getElementById("photo")!.style.transform).toContain("translate3d"));
  });

  it("tears down on unmount", () => {
    const remove = vi.spyOn(window, "removeEventListener");
    const { unmount } = render(<PublicMotion />);
    unmount();
    expect(document.querySelector("canvas")).toBeNull();
    expect(remove.mock.calls.map(([t]) => t)).toEqual(expect.arrayContaining(["pointermove", "scroll"]));
    expect(hidden()).toBe(0);
    expect(observed.size).toBe(0);
  });
});
