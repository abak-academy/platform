import { fireEvent, render, screen } from "@testing-library/react";
import { describe, expect, it, vi, beforeEach } from "vitest";
import { PartnerSwitchCard } from "./PartnerSwitchCard";

const h = vi.hoisted(() => ({ status: { isLoading: false, data: { enabled: true, partner_enabled: false } as { enabled: boolean; partner_enabled?: boolean } | undefined }, set: vi.fn(), refetch: vi.fn() }));

vi.mock("@/lib/hooks/affiliate", () => ({ useAffiliateProgramStatus: () => ({ ...h.status, refetch: h.refetch }) }));
vi.mock("@/lib/hooks/admin-affiliates", () => ({ useSetPartnerProgram: () => ({ mutate: h.set, isPending: false }) }));
vi.mock("sonner", () => ({ toast: { success: vi.fn(), error: vi.fn() } }));

describe("PartnerSwitchCard", () => {
  beforeEach(() => {
    h.set.mockReset();
    h.status.data = { enabled: true, partner_enabled: false };
  });

  it("shows the Mitra program as off, and asks before switching it on", () => {
    render(<PartnerSwitchCard />);
    expect(screen.getByText("Program Mitra")).toBeInTheDocument();
    expect(screen.getByText("Nonaktif")).toBeInTheDocument();

    fireEvent.click(screen.getByRole("button", { name: "Aktifkan program" }));
    expect(h.set).not.toHaveBeenCalled();
    fireEvent.click(screen.getByRole("button", { name: "Konfirmasi" }));
    expect(h.set).toHaveBeenCalledWith(true, expect.anything());
  });

  it("is independent of the Affiliator switch: the Affiliator program being on says nothing about it", () => {
    h.status.data = { enabled: true };
    render(<PartnerSwitchCard />);
    expect(screen.getByText("Nonaktif")).toBeInTheDocument();
  });

  it("switches an active program off after confirmation", () => {
    h.status.data = { enabled: false, partner_enabled: true };
    render(<PartnerSwitchCard />);
    expect(screen.getByText("Aktif")).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "Nonaktifkan program" }));
    fireEvent.click(screen.getByRole("button", { name: "Konfirmasi" }));
    expect(h.set).toHaveBeenCalledWith(false, expect.anything());
  });
});
