import { render, screen } from "@testing-library/react";
import { describe, expect, it, vi, beforeEach } from "vitest";
import AffiliateLandingPage from "./page";
import type { AffiliatorProgramStates } from "@/lib/types";

const replace = vi.hoisted(() => vi.fn());
const me = vi.hoisted(() => ({ value: { data: undefined, isLoading: false } as { data?: { affiliator_programs?: AffiliatorProgramStates }; isLoading: boolean } }));

vi.mock("next/navigation", () => ({ useRouter: () => ({ replace }) }));
vi.mock("@/lib/hooks/auth", () => ({ useMe: () => me.value }));
vi.mock("@/lib/hooks/affiliate", () => ({
  useAffiliateStats: () => ({ isLoading: false, data: { active_bindings: 4, commission_earned: 250000 } }),
}));
vi.mock("@/lib/hooks/partner", () => ({
  usePartnerDashboard: () => ({ isLoading: false, data: { students_registered: 7, orders_paid: 2 } }),
}));

function setStates(over: Partial<AffiliatorProgramStates>) {
  me.value = {
    data: { affiliator_programs: { affiliate: "not_enrolled", partner: "not_enrolled", account_active: true, ...over } },
    isLoading: false,
  };
}

describe("AffiliateLandingPage", () => {
  beforeEach(() => replace.mockClear());

  it("sends an account in one program straight to it", () => {
    setStates({ affiliate: "active" });
    const { unmount } = render(<AffiliateLandingPage />);
    expect(replace).toHaveBeenCalledWith("/affiliate/overview");
    unmount();

    replace.mockClear();
    setStates({ partner: "active" });
    render(<AffiliateLandingPage />);
    expect(replace).toHaveBeenCalledWith("/affiliate/partner");
  });

  it("counts a pending application as being in the program, so the notice is shown there", () => {
    setStates({ partner: "pending" });
    render(<AffiliateLandingPage />);
    expect(replace).toHaveBeenCalledWith("/affiliate/partner");
  });

  it("shows one card per program to an account in both, with each program's own figures", () => {
    setStates({ affiliate: "active", partner: "active" });
    render(<AffiliateLandingPage />);

    expect(replace).not.toHaveBeenCalled();
    expect(screen.getByRole("heading", { name: "Affiliator" })).toBeInTheDocument();
    expect(screen.getByRole("heading", { name: "Mitra" })).toBeInTheDocument();
    expect(screen.getByText("Siswa terikat").nextSibling).toHaveTextContent("4");
    expect(screen.getByText("Siswa terdaftar").nextSibling).toHaveTextContent("7");
    const links = screen.getAllByRole("link", { name: "Buka program" });
    expect(links.map((l) => l.getAttribute("href"))).toEqual(["/affiliate/overview", "/affiliate/partner"]);
  });

  it("shows the state instead of figures for a program that is not active yet", () => {
    setStates({ affiliate: "active", partner: "rejected" });
    render(<AffiliateLandingPage />);
    expect(screen.queryByText("Siswa terdaftar")).toBeNull();
    expect(screen.getByText("Pendaftaran Anda untuk program ini tidak disetujui.")).toBeInTheDocument();
  });

  it("tells an account with no program to contact an admin", () => {
    setStates({});
    render(<AffiliateLandingPage />);
    expect(screen.getByRole("status")).toHaveTextContent(/belum terdaftar di program mana pun/i);
    expect(replace).not.toHaveBeenCalled();
  });

  it("waits while the account loads", () => {
    me.value = { data: undefined, isLoading: true };
    render(<AffiliateLandingPage />);
    expect(screen.getByTestId("landing-loading")).toBeInTheDocument();
  });
});
