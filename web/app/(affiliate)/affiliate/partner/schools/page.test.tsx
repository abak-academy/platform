import { fireEvent, render, screen } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import PartnerSchoolsPage from "./page";

const hook = vi.hoisted(() => ({ state: {} as Record<string, unknown>, downloadCsv: vi.fn() }));

vi.mock("sonner", () => ({ toast: { error: vi.fn() } }));
vi.mock("@/lib/csv", () => ({ downloadCsv: (...a: unknown[]) => hook.downloadCsv(...a) }));
vi.mock("@/lib/hooks/partner", () => ({ usePartnerSchools: () => hook.state }));

describe("PartnerSchoolsPage", () => {
  beforeEach(() => hook.downloadCsv.mockClear());

  it("lists the allowed schools with the identifiers a CSV import needs, and downloads them", () => {
    hook.state = { isLoading: false, isError: false, data: { data: [{ id: "1", name: "SMA Contoh", code: "SMAC", npsn: "20100001", status: "active" }] } };
    render(<PartnerSchoolsPage />);

    expect(screen.getByText("SMA Contoh")).toBeInTheDocument();
    expect(screen.getByText("20100001")).toBeInTheDocument();
    expect(screen.getByText("SMAC")).toBeInTheDocument();

    fireEvent.click(screen.getByRole("button", { name: "Unduh CSV" }));
    expect(hook.downloadCsv).toHaveBeenCalledWith("sekolah-mitra.csv", ["name", "npsn", "code", "status"], [["SMA Contoh", "20100001", "SMAC", "active"]]);
  });

  it("says plainly when the admin has not allowed any school", () => {
    hook.state = { isLoading: false, isError: false, data: { data: [] } };
    render(<PartnerSchoolsPage />);
    expect(screen.getByText(/belum ada sekolah yang diizinkan/i)).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Unduh CSV" })).toBeDisabled();
  });
});
