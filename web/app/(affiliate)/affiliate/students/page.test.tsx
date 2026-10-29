import { render, screen } from "@testing-library/react";
import { describe, expect, it, vi, beforeEach } from "vitest";
import StudentsPage from "./page";

const h = vi.hoisted(() => ({ bindings: vi.fn(), search: { value: "" }, replace: vi.fn() }));

vi.mock("next/navigation", () => ({
  useRouter: () => ({ replace: h.replace }),
  usePathname: () => "/affiliate/students",
  useSearchParams: () => new URLSearchParams(h.search.value),
}));
vi.mock("@/lib/hooks/affiliate", () => ({ useAffiliateBindings: h.bindings }));

const ok = (data: unknown[]) => ({ isLoading: false, isError: false, data: { data, page: 1, per_page: 20, total: data.length } });

describe("AffiliateStudentsPage", () => {
  beforeEach(() => {
    h.bindings.mockReset();
    h.replace.mockReset();
    h.search.value = "";
  });

  it("lists referred students by display name only", () => {
    h.bindings.mockReturnValue(
      ok([
        {
          binding_id: "b1",
          name: "Budi S.",
          registered_at: "2026-09-01T00:00:00Z",
          status: "active",
          source: "link",
          started_at: "2026-09-02T00:00:00Z",
          expires_at: "2026-12-02T00:00:00Z",
          total_spend: 250000,
        },
      ]),
    );
    render(<StudentsPage />);

    expect(screen.getByText("Budi S.")).toBeInTheDocument();
    expect(screen.getAllByText("Tautan referral").length).toBeGreaterThan(0);
    expect(screen.getByText("Aktif")).toBeInTheDocument();
    expect(screen.getByRole("searchbox", { name: "Cari nama siswa" })).toBeInTheDocument();
  });

  it("passes the search, filters and page from the URL to the query", () => {
    h.search.value = "q=budi&status=ended&type=link&from=2026-10-01&page=2&per_page=10";
    h.bindings.mockReturnValue(ok([]));
    render(<StudentsPage />);

    expect(h.bindings).toHaveBeenCalledWith({
      page: 2,
      per_page: 10,
      q: "budi",
      status: "ended",
      type: "link",
      from: "2026-10-01",
      to: "",
    });
  });

  it("says nothing matches when a filter leaves the list empty, and nothing yet otherwise", () => {
    h.bindings.mockReturnValue(ok([]));
    h.search.value = "q=zzz";
    const { unmount } = render(<StudentsPage />);
    expect(screen.getByText("Tidak ada yang cocok dengan pencarian atau filter ini.")).toBeInTheDocument();
    unmount();

    h.search.value = "";
    render(<StudentsPage />);
    expect(screen.getByText("Belum ada siswa binaan.")).toBeInTheDocument();
  });
});
