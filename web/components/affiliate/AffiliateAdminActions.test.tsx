import { fireEvent, render, screen } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { AffiliateAdminActions } from "./AffiliateAdminActions";

const h = vi.hoisted(() => ({ add: vi.fn() }));

vi.mock("@/lib/hooks/admin-affiliates", () => ({
  useChangeAffiliateStatus: () => ({ mutate: vi.fn(), isPending: false }),
  useChangeAffiliateLevel: () => ({ mutate: vi.fn(), isPending: false }),
  useCreateAffiliateAdjustment: () => ({ mutate: vi.fn(), isPending: false }),
  useAddAffiliatorProgram: () => ({ mutate: h.add, isPending: false }),
}));
vi.mock("sonner", () => ({ toast: { success: vi.fn(), error: vi.fn() } }));

describe("AffiliateAdminActions — adding a program", () => {
  beforeEach(() => h.add.mockReset());

  it("asks first, says it cannot be undone, and only then adds the Mitra program", () => {
    render(<AffiliateAdminActions id="a1" currentStatus="active" currentLevel="bronze" programs={undefined} />);

    fireEvent.click(screen.getByRole("button", { name: "Tambah program Mitra" }));
    expect(h.add).not.toHaveBeenCalled();
    expect(screen.getByText(/tidak bisa dicabut/i)).toBeInTheDocument();

    fireEvent.click(screen.getByRole("button", { name: "Ya, tambahkan" }));
    expect(h.add).toHaveBeenCalledWith({ id: "a1", program: "partner" }, expect.anything());
  });

  it("does nothing when the admin backs out", () => {
    render(<AffiliateAdminActions id="a1" currentStatus="active" currentLevel="bronze" programs={undefined} />);
    fireEvent.click(screen.getByRole("button", { name: "Tambah program Mitra" }));
    fireEvent.click(screen.getByRole("button", { name: "Batal" }));
    expect(h.add).not.toHaveBeenCalled();
  });

  it("offers the Affiliator program to a Mitra-only account, and nothing to an account in both", () => {
    const { unmount } = render(
      <AffiliateAdminActions id="a1" currentStatus="active" currentLevel="bronze" affiliate={false} programs={{ affiliate: false, partner: { school_ids: [] } }} />,
    );
    expect(screen.getByRole("button", { name: "Tambah program Affiliator" })).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "Tambah program Mitra" })).toBeNull();
    unmount();

    render(<AffiliateAdminActions id="a1" currentStatus="active" currentLevel="bronze" programs={{ affiliate: true, partner: { school_ids: [] } }} />);
    expect(screen.queryByRole("button", { name: /tambah program/i })).toBeNull();
  });

  it("offers no new program while the application is still open", () => {
    render(<AffiliateAdminActions id="a1" currentStatus="pending_approval" currentLevel="bronze" programs={undefined} />);
    expect(screen.queryByRole("button", { name: /tambah program/i })).toBeNull();
  });
});
