import { render, screen } from "@testing-library/react";
import { describe, expect, it, vi, beforeEach } from "vitest";
import { ProgramGate, programForPath } from "./ProgramGate";
import type { AffiliatorProgramStates } from "@/lib/types";

const me = vi.hoisted(() => ({ value: { data: undefined, isLoading: false } as { data?: { affiliator_programs?: AffiliatorProgramStates }; isLoading: boolean } }));
const partnerMe = vi.hoisted(() => ({ value: { data: { program_enabled: true } } as { data?: { program_enabled: boolean } } }));

vi.mock("next/navigation", () => ({ usePathname: () => "/affiliate/partner" }));
vi.mock("@/lib/hooks/auth", () => ({ useMe: () => me.value }));
vi.mock("@/lib/hooks/partner", () => ({ usePartnerMe: () => partnerMe.value }));

function states(over: Partial<AffiliatorProgramStates>): AffiliatorProgramStates {
  return { affiliate: "active", partner: "active", account_active: true, ...over };
}

function setStates(value: AffiliatorProgramStates | undefined) {
  me.value = { data: value ? { affiliator_programs: value } : undefined, isLoading: false };
}

describe("programForPath", () => {
  it("maps every portal page to its program, and leaves the shared ones alone", () => {
    expect(programForPath("/affiliate/partner")).toBe("partner");
    expect(programForPath("/affiliate/partner/orders/new")).toBe("partner");
    expect(programForPath("/affiliate/overview")).toBe("affiliate");
    expect(programForPath("/affiliate/wallet")).toBe("affiliate");
    expect(programForPath("/affiliate/students")).toBe("affiliate");
    expect(programForPath("/affiliate/account")).toBeNull();
    expect(programForPath("/affiliate")).toBeNull();
  });
});

describe("ProgramGate", () => {
  beforeEach(() => {
    partnerMe.value = { data: { program_enabled: true } };
    setStates(states({}));
  });

  it("shows the pages to an account in the program", () => {
    render(<ProgramGate program="partner">halaman mitra</ProgramGate>);
    expect(screen.getByText("halaman mitra")).toBeInTheDocument();
    expect(screen.queryByRole("status")).toBeNull();
  });

  it("tells an account outside the program to contact an admin, instead of showing data", () => {
    setStates(states({ partner: "not_enrolled" }));
    render(<ProgramGate program="partner">halaman mitra</ProgramGate>);
    expect(screen.getByText("Anda belum terdaftar di program ini")).toBeInTheDocument();
    expect(screen.getByText(/hubungi admin/i)).toBeInTheDocument();
    expect(screen.queryByText("halaman mitra")).toBeNull();
  });

  it("tells a pending application apart from a missing one", () => {
    setStates(states({ partner: "pending" }));
    render(<ProgramGate program="partner">halaman mitra</ProgramGate>);
    expect(screen.getByText("Pendaftaran menunggu persetujuan")).toBeInTheDocument();
    expect(screen.queryByText("Anda belum terdaftar di program ini")).toBeNull();
  });

  it("shows why an application was rejected", () => {
    setStates(states({ partner: "rejected", rejection_reason: "dokumen kurang" }));
    render(<ProgramGate program="partner">halaman mitra</ProgramGate>);
    expect(screen.getByText("Pendaftaran ditolak")).toBeInTheDocument();
    expect(screen.getByText(/dokumen kurang/)).toBeInTheDocument();
  });

  it("judges each program on its own state", () => {
    setStates(states({ affiliate: "pending", partner: "active" }));
    const { unmount } = render(<ProgramGate program="affiliate">halaman affiliator</ProgramGate>);
    expect(screen.getByText("Pendaftaran menunggu persetujuan")).toBeInTheDocument();
    unmount();

    render(<ProgramGate program="partner">halaman mitra</ProgramGate>);
    expect(screen.getByText("halaman mitra")).toBeInTheDocument();
  });

  it("keeps the pages of a suspended account and says they are read-only", () => {
    setStates(states({ account_active: false }));
    render(<ProgramGate program="partner">halaman mitra</ProgramGate>);
    expect(screen.getByText("halaman mitra")).toBeInTheDocument();
    expect(screen.getByRole("status")).toHaveTextContent(/dinonaktifkan/i);
  });

  it("says the Mitra program is off, and only for the Mitra pages", () => {
    partnerMe.value = { data: { program_enabled: false } };
    const { unmount } = render(<ProgramGate program="partner">halaman mitra</ProgramGate>);
    expect(screen.getByRole("status")).toHaveTextContent(/program mitra sedang tidak aktif/i);
    unmount();

    render(<ProgramGate program="affiliate">halaman affiliator</ProgramGate>);
    expect(screen.queryByRole("status")).toBeNull();
  });

  it("waits while the account is loading, and does not block the page when it cannot tell", () => {
    me.value = { data: undefined, isLoading: true };
    const { unmount } = render(<ProgramGate program="partner">halaman mitra</ProgramGate>);
    expect(screen.getByTestId("program-gate-loading")).toBeInTheDocument();
    expect(screen.queryByText("halaman mitra")).toBeNull();
    unmount();

    setStates(undefined);
    render(<ProgramGate program="partner">halaman mitra</ProgramGate>);
    expect(screen.getByText("halaman mitra")).toBeInTheDocument();
  });
});
