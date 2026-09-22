import { describe, it, expect, vi, afterEach } from "vitest";
import { act, renderHook } from "@testing-library/react";
import { useDebouncedValue, SEARCH_DEBOUNCE_MS } from "./use-debounced-value";

afterEach(() => {
  vi.useRealTimers();
});

describe("useDebouncedValue", () => {
  it("defaults to the shared 300ms search window", () => {
    expect(SEARCH_DEBOUNCE_MS).toBe(300);
  });

  it("holds the raw value until the window elapses", () => {
    vi.useFakeTimers();
    const { result, rerender } = renderHook(
      ({ value }) => useDebouncedValue(value),
      { initialProps: { value: "" } },
    );

    rerender({ value: "budi" });
    expect(result.current).toBe("");

    act(() => {
      vi.advanceTimersByTime(SEARCH_DEBOUNCE_MS - 1);
    });
    expect(result.current).toBe("");

    act(() => {
      vi.advanceTimersByTime(1);
    });
    expect(result.current).toBe("budi");
  });

  it("restarts the window on every change and emits only the last value", () => {
    vi.useFakeTimers();
    const { result, rerender } = renderHook(
      ({ value }) => useDebouncedValue(value),
      { initialProps: { value: "" } },
    );

    rerender({ value: "a" });
    act(() => {
      vi.advanceTimersByTime(100);
    });
    rerender({ value: "ab" });
    act(() => {
      vi.advanceTimersByTime(299);
    });
    expect(result.current).toBe("");

    act(() => {
      vi.advanceTimersByTime(1);
    });
    expect(result.current).toBe("ab");
  });

  it("honours a custom delay", () => {
    vi.useFakeTimers();
    const { result, rerender } = renderHook(
      ({ value }) => useDebouncedValue(value, 500),
      { initialProps: { value: "" } },
    );

    rerender({ value: "bina" });
    act(() => {
      vi.advanceTimersByTime(299);
    });
    expect(result.current).toBe("");

    act(() => {
      vi.advanceTimersByTime(201);
    });
    expect(result.current).toBe("bina");
  });
});
