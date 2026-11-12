import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import PartnerSchoolsPage from "./page";

const hook = vi.hoisted(() => ({
  state: {} as Record<string, unknown>,
  params: vi.fn(),
  fetchAll: vi.fn(),
  downloadCsv: vi.fn(),
  search: { value: "" },
}));

vi.mock("next/navigation", () => ({
  useRouter: () => ({ replace: vi.fn() }),
  usePathname: () => "/affiliate/partner/schools",
  useSearchParams: () => new URLSearchParams(hook.search.value),
}));
vi.mock("sonner", () => ({ toast: { error: vi.fn() } }));
vi.mock("@/lib/csv", () => ({ downloadCsv: (...a: unknown[]) => hook.downloadCsv(...a) }));
vi.mock("@/lib/hooks/partner", () => ({
  usePartnerSchools: (params: unknown) => {
    hook.params(params);
    return hook.state;
  },
  fetchAllPartnerPages: (...a: unknown[]) => hook.fetchAll(...a),
}));

const school = { id: "1", name: "SMA Contoh", code: "SMAC", npsn: "20100001", status: "active" };
const page = (data: unknown[], total = data.length) => ({ isLoading: false, isError: false, data: { data, page: 1, per_page: 20, total } });

describe("PartnerSchoolsPage", () => {
  beforeEach(() => {
    hook.search.value = "";
    hook.params.mockClear();
    hook.downloadCsv.mockClear();
  });

  it("lists the allowed schools with the identifiers a CSV import needs", () => {
    hook.state = page([school]);
    render(<PartnerSchoolsPage />);

    expect(screen.getByText("SMA Contoh")).toBeInTheDocument();
    expect(screen.getByText("20100001")).toBeInTheDocument();
    expect(screen.getByText("SMAC")).toBeInTheDocument();
  });

  it("asks for the page and search the URL holds", () => {
    hook.search.value = "q=sma&page=3&per_page=10";
    hook.state = page([school], 25);
    render(<PartnerSchoolsPage />);
    expect(hook.params).toHaveBeenLastCalledWith({ q: "sma", status: "", page: 3, per_page: 10 });
  });

  it("downloads every school the search matches, not only the page on screen", async () => {
    hook.search.value = "q=sma";
    hook.state = page([school], 40);
    hook.fetchAll.mockResolvedValue([school, { ...school, id: "2", name: "SMK Lain", npsn: null }]);
    render(<PartnerSchoolsPage />);

    fireEvent.click(screen.getByRole("button", { name: "Unduh CSV" }));

    await waitFor(() => expect(hook.downloadCsv).toHaveBeenCalled());
    expect(hook.fetchAll).toHaveBeenCalledWith("/partners/me/schools", { q: "sma", status: "" });
    expect(hook.downloadCsv.mock.calls[0][2]).toHaveLength(2);
  });

  it("says plainly when the admin has not allowed any school", () => {
    hook.state = page([]);
    render(<PartnerSchoolsPage />);
    expect(screen.getByText(/belum ada sekolah yang diizinkan/i)).toBeInTheDocument();
  });
});
