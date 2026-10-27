import { describe, expect, it } from "vitest";
import { act, renderHook } from "@testing-library/react";
import { useCursorPaging } from "./use-cursor-paging";
import { useOffsetPaging } from "./use-offset-paging";

describe("useCursorPaging", () => {
  it("walks forward through cursors and back again", () => {
    const { result } = renderHook(() => useCursorPaging("k"));
    expect(result.current).toMatchObject({ cursor: undefined, page: 0, canPrev: false });

    act(() => result.current.next("c2"));
    act(() => result.current.next("c3"));
    expect(result.current).toMatchObject({ cursor: "c3", page: 2, canPrev: true });

    act(() => result.current.prev());
    expect(result.current).toMatchObject({ cursor: "c2", page: 1 });
  });

  it("returns to the first page when the reset key changes", () => {
    const { result, rerender } = renderHook(({ k }) => useCursorPaging(k), { initialProps: { k: "a" } });
    act(() => result.current.next("c2"));
    expect(result.current.page).toBe(1);

    rerender({ k: "b" });

    expect(result.current).toMatchObject({ cursor: undefined, page: 0, canPrev: false });
  });
});

describe("useOffsetPaging", () => {
  it("moves by the page size and never below zero", () => {
    const { result } = renderHook(() => useOffsetPaging("k", 20));
    expect(result.current).toMatchObject({ offset: 0, canPrev: false });

    act(() => result.current.next());
    act(() => result.current.next());
    expect(result.current).toMatchObject({ offset: 40, canPrev: true });

    act(() => result.current.prev());
    act(() => result.current.prev());
    act(() => result.current.prev());
    expect(result.current.offset).toBe(0);
  });

  it("returns to the first page when the period, search or page size changes", () => {
    const { result, rerender } = renderHook(({ k, n }) => useOffsetPaging(k, n), { initialProps: { k: "a", n: 20 } });
    act(() => result.current.next());
    expect(result.current.offset).toBe(20);

    rerender({ k: "b", n: 20 });
    expect(result.current.offset).toBe(0);
  });
});
