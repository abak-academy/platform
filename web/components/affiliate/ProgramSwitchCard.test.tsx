import { fireEvent, render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import { ProgramSwitchCard } from "./ProgramSwitchCard";

vi.mock("@/lib/hooks/admin-affiliates", () => ({ useSetAffiliateProgram: () => ({ mutate: vi.fn(), isPending: false }) }));
vi.mock("sonner", () => ({ toast: { success: vi.fn(), error: vi.fn() } }));

describe("ProgramSwitchCard", () => {
  it("cannot be switched on while required settings are empty, and names them", () => {
    render(<ProgramSwitchCard enabled={false} missing={["launch_date", "cookie_days"]} commissionUnset={false} />);

    expect(screen.getByRole("button", { name: "Aktifkan program" })).toBeDisabled();
    expect(screen.getByText(/Tanggal peluncuran, Masa berlaku cookie/)).toBeInTheDocument();
  });

  it("warns, without blocking, that no commission is set", () => {
    render(<ProgramSwitchCard enabled={false} missing={[]} commissionUnset />);

    expect(screen.getByRole("button", { name: "Aktifkan program" })).toBeEnabled();
    expect(screen.getByText(/tidak ada komisi yang akan dibuat/i)).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "Aktifkan program" }));
    expect(screen.getAllByText(/tidak ada komisi yang akan dibuat/i).length).toBeGreaterThan(1);
  });

  it("can always be switched off, whatever is missing", () => {
    render(<ProgramSwitchCard enabled missing={["cookie_days"]} commissionUnset />);
    expect(screen.getByRole("button", { name: "Nonaktifkan program" })).toBeEnabled();
  });
});
