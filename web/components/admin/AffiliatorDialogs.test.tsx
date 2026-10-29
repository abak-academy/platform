import { beforeEach, describe, expect, it, vi } from "vitest";
import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { toast } from "sonner";
import { AffiliatorDetailDialog, AffiliatorPromoteDialog, AffiliatorSchoolsDialog } from "./AffiliatorDialogs";
import type { PromotableAdmin } from "@/lib/types";

vi.mock("sonner", () => ({ toast: { success: vi.fn(), error: vi.fn() } }));
vi.mock("@/components/admin/SchoolMultiPicker", () => ({
  SchoolMultiPicker: ({ value, onChange }: { value: { id: string; name: string }[]; onChange: (v: { id: string; name: string }[]) => void }) => (
    <div>
      <span data-testid="picked">{value.map((s) => s.id).join(",")}</span>
      <button type="button" onClick={() => onChange([...value, { id: "s9", name: "SMAN 9" }])}>add school</button>
      <button type="button" onClick={() => onChange([])}>clear schools</button>
    </div>
  ),
}));

const promote = vi.fn();
const setSchools = vi.fn();
let detailState: { data?: unknown; isLoading: boolean };
vi.mock("@/lib/hooks/admin-affiliators", () => ({
  usePromoteAffiliator: () => ({ mutateAsync: promote, isPending: false }),
  useSetAffiliatorSchools: () => ({ mutateAsync: setSchools, isPending: false }),
  useAdminAffiliator: (id: string | null) => (id ? detailState : { data: undefined, isLoading: false }),
}));

const admin: PromotableAdmin = { id: "u1", name: "Pak Guru", email: "guru@example.com", school_id: "s1", school_name: "SMAN 1", created_at: "" };

describe("AffiliatorPromoteDialog", () => {
  beforeEach(() => {
    promote.mockReset().mockResolvedValue({});
    vi.mocked(toast.success).mockReset();
    vi.mocked(toast.error).mockReset();
  });

  it("warns that the account cannot become a school admin again, before the button", () => {
    render(<AffiliatorPromoteDialog admin={admin} onClose={vi.fn()} />);

    const alert = screen.getByRole("alert");
    expect(alert).toHaveTextContent("Tidak bisa diturunkan kembali");
    expect(alert).toHaveTextContent("tidak bisa turun lagi menjadi school admin");
    expect(alert).toHaveTextContent("Akses school admin ke sekolahnya");
  });

  it("shows the warning in the warning colours", () => {
    render(<AffiliatorPromoteDialog admin={admin} onClose={vi.fn()} />);

    const alert = screen.getByRole("alert");
    expect(alert.className).toContain("bg-warn-bg");
    expect(alert.className).toContain("border-warn");
  });

  it("starts with the school the admin administers today", () => {
    render(<AffiliatorPromoteDialog admin={admin} onClose={vi.fn()} />);

    expect(screen.getByTestId("picked")).toHaveTextContent("s1");
    expect(screen.getByText(/sudah dipilih/)).toBeInTheDocument();
  });

  it("promotes with the chosen schools and closes", async () => {
    const onClose = vi.fn();
    render(<AffiliatorPromoteDialog admin={admin} onClose={onClose} />);

    fireEvent.click(screen.getByRole("button", { name: "add school" }));
    fireEvent.click(screen.getByRole("button", { name: "Jadikan affiliator" }));

    await waitFor(() => expect(promote).toHaveBeenCalledWith({ id: "u1", school_ids: ["s1", "s9"] }));
    expect(toast.success).toHaveBeenCalledWith("Akun dijadikan affiliator.");
    expect(onClose).toHaveBeenCalled();
  });

  it("cannot promote without a school, e.g. an admin with none on file", () => {
    render(<AffiliatorPromoteDialog admin={{ ...admin, school_id: null, school_name: null }} onClose={vi.fn()} />);

    expect(screen.getByTestId("picked")).toHaveTextContent("");
    expect(screen.getByRole("button", { name: "Jadikan affiliator" })).toBeDisabled();
    expect(screen.queryByText(/sudah dipilih/)).not.toBeInTheDocument();
  });

  it("reports a server refusal and stays open", async () => {
    promote.mockRejectedValue(new Error("only an admin_school account can be promoted to affiliator"));
    const onClose = vi.fn();
    render(<AffiliatorPromoteDialog admin={admin} onClose={onClose} />);

    fireEvent.click(screen.getByRole("button", { name: "Jadikan affiliator" }));

    await waitFor(() => expect(toast.error).toHaveBeenCalledWith("only an admin_school account can be promoted to affiliator"));
    expect(onClose).not.toHaveBeenCalled();
  });

  it("renders nothing without a target", () => {
    render(<AffiliatorPromoteDialog admin={null} onClose={vi.fn()} />);
    expect(screen.queryByRole("dialog")).not.toBeInTheDocument();
  });
});

describe("AffiliatorSchoolsDialog", () => {
  beforeEach(() => {
    setSchools.mockReset().mockResolvedValue({});
    detailState = {
      isLoading: false,
      data: { affiliator: { name: "Andi", schools: [{ id: "s2", name: "SMAN 2" }] }, totals: {}, recent_orders: [] },
    };
  });

  it("starts from the current schools and saves the new list", async () => {
    const onClose = vi.fn();
    render(<AffiliatorSchoolsDialog id="a1" onClose={onClose} />);
    expect(screen.getByTestId("picked")).toHaveTextContent("s2");

    fireEvent.click(screen.getByRole("button", { name: "add school" }));
    fireEvent.click(screen.getByRole("button", { name: "Simpan" }));

    await waitFor(() => expect(setSchools).toHaveBeenCalledWith({ id: "a1", school_ids: ["s2", "s9"] }));
    expect(onClose).toHaveBeenCalled();
  });

  it("cannot save an empty list", () => {
    render(<AffiliatorSchoolsDialog id="a1" onClose={vi.fn()} />);

    fireEvent.click(screen.getByRole("button", { name: "clear schools" }));

    expect(screen.getByRole("button", { name: "Simpan" })).toBeDisabled();
  });
});

describe("AffiliatorDetailDialog", () => {
  it("shows totals, schools and the latest orders with readable statuses", () => {
    detailState = {
      isLoading: false,
      data: {
        affiliator: { name: "Andi", email: "andi@example.com", student_count: 12, schools: [{ id: "s1", name: "SMAN 1" }] },
        totals: { paid_orders: 3, paid_units: 40, paid_amount: 900000, discount_total: 50000, pending_orders: 1 },
        recent_orders: [{ id: "o1", status: "paid", product_name: "Try Out UTBK", participant_count: 2, total: 90000 }],
      },
    };
    render(<AffiliatorDetailDialog id="a1" onClose={vi.fn()} />);

    expect(screen.getByText("Rp900.000")).toBeInTheDocument();
    expect(screen.getByText("SMAN 1")).toBeInTheDocument();
    expect(screen.getByText("Try Out UTBK · 2")).toBeInTheDocument();
    expect(screen.getByText("Rp90.000 · Dibayar")).toBeInTheDocument();
  });

  it("says so when there are no orders yet, and while loading", () => {
    detailState = { isLoading: false, data: { affiliator: { name: "Andi", student_count: 0 }, totals: { paid_orders: 0, paid_units: 0, paid_amount: 0, discount_total: 0, pending_orders: 0 }, recent_orders: [] } };
    const { unmount } = render(<AffiliatorDetailDialog id="a1" onClose={vi.fn()} />);
    expect(screen.getByText("Belum ada pembelian.")).toBeInTheDocument();
    unmount();

    detailState = { isLoading: true };
    render(<AffiliatorDetailDialog id="a1" onClose={vi.fn()} />);
    expect(screen.getByText("Memuat…")).toBeInTheDocument();
  });
});
