import { beforeEach, describe, expect, it, vi } from "vitest";
import { fireEvent, render, screen, within } from "@testing-library/react";
import { AffiliatorPromoteTab } from "./AffiliatorPromoteTab";
import type { Affiliator, PromotableAdmin } from "@/lib/types";

vi.mock("@/lib/hooks/use-debounced-value", () => ({ useDebouncedValue: (v: string) => v }));
vi.mock("@/components/admin/AffiliatorDialogs", () => ({
  AffiliatorPromoteDialog: ({ admin }: { admin: PromotableAdmin | null }) => (admin ? <div role="dialog">promote {admin.id}</div> : null),
  AffiliatorSchoolsDialog: ({ id }: { id: string | null }) => (id ? <div role="dialog">schools {id}</div> : null),
}));

const promotableCalls: Record<string, unknown>[] = [];
const affiliatorCalls: Record<string, unknown>[] = [];
let promotable: { data?: { data: PromotableAdmin[]; total: number; next_cursor?: string }; isLoading: boolean; isError: boolean; refetch: () => void };
let affiliators: { data?: { data: Affiliator[]; total: number }; isLoading: boolean; isError: boolean; refetch: () => void };
vi.mock("@/lib/hooks/admin-affiliators", () => ({
  usePromotableAdmins: (p: Record<string, unknown>) => {
    promotableCalls.push(p);
    return promotable;
  },
  useAdminAffiliators: (p: Record<string, unknown>) => {
    affiliatorCalls.push(p);
    return affiliators;
  },
}));

const admins: PromotableAdmin[] = [
  { id: "u1", name: "Pak Guru", email: "guru@example.com", school_id: "s1", school_name: "SMAN 1", created_at: "" },
  { id: "u2", name: "Bu Sari", email: "sari@example.com", school_id: null, school_name: null, created_at: "" },
];

describe("AffiliatorPromoteTab", () => {
  beforeEach(() => {
    promotableCalls.length = 0;
    affiliatorCalls.length = 0;
    promotable = { data: { data: admins, total: 2 }, isLoading: false, isError: false, refetch: vi.fn() };
    affiliators = {
      data: { data: [{ id: "a1", name: "Andi", email: "andi@example.com", status: "active", school_ids: ["s1", "s2"], student_count: 0, order_count: 0, paid_amount: 0, created_at: "", updated_at: "" }], total: 1 },
      isLoading: false, isError: false, refetch: vi.fn(),
    };
  });

  it("says up front that promotion cannot be undone", () => {
    render(<AffiliatorPromoteTab />);

    expect(screen.getByRole("note")).toHaveTextContent("tidak bisa diturunkan kembali menjadi school admin");
  });

  it("lists the school admins with the school each one administers", () => {
    render(<AffiliatorPromoteTab />);

    const row = screen.getByText("Pak Guru").closest("tr")!;
    expect(within(row).getByText("guru@example.com")).toBeInTheDocument();
    expect(within(row).getByText("SMAN 1")).toBeInTheDocument();
    expect(within(screen.getByText("Bu Sari").closest("tr")!).getByText("-")).toBeInTheDocument();
  });

  it("opens the promote dialog for the chosen admin", () => {
    render(<AffiliatorPromoteTab />);

    fireEvent.click(within(screen.getByText("Pak Guru").closest("tr")!).getByRole("button", { name: "Jadikan affiliator" }));

    expect(screen.getByRole("dialog")).toHaveTextContent("promote u1");
  });

  it("searches on the server and pages with the cursor", () => {
    promotable = { data: { data: admins, total: 45, next_cursor: "c2" }, isLoading: false, isError: false, refetch: vi.fn() };
    render(<AffiliatorPromoteTab />);

    fireEvent.change(screen.getByRole("searchbox", { name: "Cari nama, email, atau sekolah" }), { target: { value: "sma" } });
    expect(promotableCalls.at(-1)).toMatchObject({ q: "sma" });

    fireEvent.click(screen.getByRole("button", { name: "Berikutnya" }));
    expect(promotableCalls.at(-1)).toMatchObject({ q: "sma", cursor: "c2" });
    expect(screen.getByTestId("pager-range")).toHaveTextContent("21–22 dari 45");
  });

  it("explains when nobody can be promoted, and retries on failure", () => {
    promotable = { data: { data: [], total: 0 }, isLoading: false, isError: false, refetch: vi.fn() };
    const { unmount } = render(<AffiliatorPromoteTab />);
    expect(screen.getByText("Tidak ada admin sekolah yang bisa dipromote.")).toBeInTheDocument();
    unmount();

    const refetch = vi.fn();
    promotable = { data: undefined, isLoading: false, isError: true, refetch };
    render(<AffiliatorPromoteTab />);
    fireEvent.click(screen.getByRole("button", { name: "Muat Ulang" }));
    expect(refetch).toHaveBeenCalled();
  });

  it("switches to the affiliators and sets their schools from there", () => {
    render(<AffiliatorPromoteTab />);

    fireEvent.click(screen.getByRole("radio", { name: "Affiliator (atur sekolah)" }));
    const row = screen.getByText("Andi").closest("tr")!;
    expect(within(row).getByText("2")).toBeInTheDocument();
    expect(screen.queryByText("Pak Guru")).not.toBeInTheDocument();

    fireEvent.click(within(row).getByRole("button", { name: "Atur sekolah" }));
    expect(screen.getByRole("dialog")).toHaveTextContent("schools a1");
    expect(affiliatorCalls.length).toBeGreaterThan(0);
  });
});
