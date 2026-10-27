import { render, screen, fireEvent } from "@testing-library/react";
import { describe, expect, it, vi, beforeEach } from "vitest";
import FlagsPage from "./page";

const h = vi.hoisted(() => ({
  list: vi.fn(),
  review: vi.fn(),
  capability: vi.fn(),
  download: vi.fn(),
  replace: vi.fn(),
  search: { value: "" },
}));

vi.mock("next/navigation", () => ({
  useRouter: () => ({ replace: h.replace }),
  usePathname: () => "/admin/affiliates/flags",
  useSearchParams: () => new URLSearchParams(h.search.value),
}));
vi.mock("@/lib/hooks/admin-affiliates", () => ({
  downloadAffiliateCsv: h.download,
  useAdminAffiliateFlags: h.list,
  useReviewAffiliateFlag: () => ({ mutate: h.review, isPending: false }),
}));
vi.mock("@/lib/hooks/use-capability", () => ({ useHasCapability: h.capability }));
vi.mock("sonner", () => ({ toast: { success: vi.fn(), error: vi.fn() } }));

const flag = (id: string, status: string) => ({
  id,
  rule: "self_purchase",
  payload: { order_id: "o1" },
  status,
  created_at: "2026-10-02T00:00:00Z",
  affiliator_name: "Budi Santoso",
  affiliator_code: "BUDI1234",
});

describe("AdminAffiliateFlagsPage", () => {
  beforeEach(() => {
    Object.values(h).forEach((m) => "mockReset" in m && m.mockReset());
    h.search.value = "";
    h.capability.mockReturnValue(true);
    h.list.mockReturnValue({ isLoading: false, isError: false, data: { data: [flag("f1", "open"), flag("f2", "valid")], total: 2, page: 1, per_page: 20 } });
  });

  it("lists every status by default and shows the rule in words", () => {
    render(<FlagsPage />);

    expect(h.list).toHaveBeenCalledWith({ q: "", status: "", from: "", to: "", page: 1, per_page: 20 });
    expect(screen.getAllByText("Pembelian diri sendiri")).toHaveLength(2);
  });

  it("reviews only open flags", () => {
    render(<FlagsPage />);

    expect(screen.getAllByRole("button", { name: "Tandai valid" })).toHaveLength(1);
    fireEvent.click(screen.getByRole("button", { name: "Abaikan" }));
    expect(h.review).toHaveBeenCalledWith({ id: "f1", status: "ignored" }, expect.anything());
  });

  it("downloads the CSV with the filters in the URL", () => {
    h.search.value = "status=open&q=budi";
    h.download.mockResolvedValue(undefined);
    render(<FlagsPage />);

    fireEvent.click(screen.getByRole("button", { name: "Unduh CSV" }));

    expect(h.download).toHaveBeenCalledWith("flags", { q: "budi", status: "open", from: "", to: "" });
  });

  it("is blocked without the capability", () => {
    h.capability.mockReturnValue(false);
    render(<FlagsPage />);

    expect(screen.getByTestId("no-access")).toBeInTheDocument();
  });
});
