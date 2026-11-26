import { act, render, screen } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { RotatingWord } from "./RotatingWord";

const mockMotion = (reduce: boolean) =>
  vi.stubGlobal("matchMedia", (q: string) => ({ matches: reduce && q.includes("reduce"), media: q, addEventListener() {}, removeEventListener() {} }));

const active = (c: HTMLElement) => c.querySelector('[data-active="true"]')?.textContent;

beforeEach(() => {
  vi.useFakeTimers();
  mockMotion(false);
});
afterEach(() => {
  vi.useRealTimers();
  vi.unstubAllGlobals();
});

describe("RotatingWord", () => {
  it("shows the fallback first, then each word every 2.6s, then wraps", () => {
    const { container } = render(<RotatingWord words={["IKMC", "STEMCO"]} fallback="dunia" />);
    expect(active(container)).toBe("dunia");
    act(() => vi.advanceTimersByTime(2599));
    expect(active(container)).toBe("dunia");
    act(() => vi.advanceTimersByTime(1));
    expect(active(container)).toBe("IKMC");
    act(() => vi.advanceTimersByTime(2600));
    expect(active(container)).toBe("STEMCO");
    act(() => vi.advanceTimersByTime(2600));
    expect(active(container)).toBe("dunia");
  });

  it("exposes only the fallback to assistive tech", () => {
    render(
      <h1>
        Dari ruang kelas ke panggung <RotatingWord words={["IKMC"]} fallback="dunia" />
      </h1>,
    );
    expect(screen.getByRole("heading", { name: "Dari ruang kelas ke panggung dunia" })).toBeInTheDocument();
    act(() => vi.advanceTimersByTime(2600));
    expect(screen.getByRole("heading", { name: "Dari ruang kelas ke panggung dunia" })).toBeInTheDocument();
  });

  it("under reduced motion shows the fallback only and starts no interval", () => {
    mockMotion(true);
    const spy = vi.spyOn(globalThis, "setInterval");
    const { container } = render(<RotatingWord words={["IKMC"]} fallback="dunia" />);
    act(() => vi.advanceTimersByTime(10000));
    expect(active(container)).toBe("dunia");
    expect(spy).not.toHaveBeenCalled();
  });

  it("starts no interval without words and clears it on unmount", () => {
    const set = vi.spyOn(globalThis, "setInterval");
    render(<RotatingWord words={[]} fallback="dunia" />);
    expect(set).not.toHaveBeenCalled();
    const clear = vi.spyOn(globalThis, "clearInterval");
    const { unmount } = render(<RotatingWord words={["IKMC"]} fallback="dunia" />);
    unmount();
    expect(clear).toHaveBeenCalled();
  });
});
