import { renderHook, act } from "@testing-library/react";
import { describe, expect, it, vi, beforeEach } from "vitest";
import { useListParams } from "./use-list-params";

const replace = vi.fn();
let search = "";

vi.mock("next/navigation", () => ({
  useRouter: () => ({ replace }),
  usePathname: () => "/affiliate/commissions",
  useSearchParams: () => new URLSearchParams(search),
}));

describe("useListParams", () => {
  beforeEach(() => {
    replace.mockClear();
    search = "";
  });

  it("reads the list state from the URL with sane defaults", () => {
    search = "q=sari&status=pending&page=3&per_page=50&from=2026-10-01";
    const { result } = renderHook(() => useListParams());

    expect(result.current.params).toEqual({ q: "sari", status: "pending", type: "", from: "2026-10-01", to: "", page: 3, per_page: 50 });
    expect(result.current.hasFilters).toBe(true);
  });

  it("falls back for a junk page or page size", () => {
    search = "page=abc&per_page=7";
    const { result } = renderHook(() => useListParams());

    expect(result.current.params.page).toBe(1);
    expect(result.current.params.per_page).toBe(20);
    expect(result.current.hasFilters).toBe(false);
  });

  it("writes a filter change and drops the page back to 1", () => {
    search = "page=4&q=sari";
    const { result } = renderHook(() => useListParams());

    act(() => result.current.update({ status: "held" }));

    expect(replace).toHaveBeenCalledWith("/affiliate/commissions?q=sari&status=held", { scroll: false });
  });

  it("keeps the page when only the page changes", () => {
    search = "q=sari";
    const { result } = renderHook(() => useListParams());

    act(() => result.current.update({ page: 3 }));

    expect(replace).toHaveBeenCalledWith("/affiliate/commissions?q=sari&page=3", { scroll: false });
  });

  it("removes an emptied value and omits the default page size", () => {
    search = "q=sari&per_page=50";
    const { result } = renderHook(() => useListParams());

    act(() => result.current.update({ q: "  ", per_page: 20 }));

    expect(replace).toHaveBeenCalledWith("/affiliate/commissions", { scroll: false });
  });

  it("reset clears filters and page but keeps unrelated params and the page size", () => {
    search = "tab=payouts&status=paid&q=x&page=2&per_page=10";
    const { result } = renderHook(() => useListParams());

    act(() => result.current.reset());

    expect(replace).toHaveBeenCalledWith("/affiliate/commissions?tab=payouts&per_page=10", { scroll: false });
  });
});
