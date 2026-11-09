import { fireEvent, render, screen } from "@testing-library/react";
import { describe, expect, it, vi, beforeEach } from "vitest";
import { PartnerProgramPanel } from "./PartnerProgramPanel";

const h = vi.hoisted(() => ({ add: vi.fn(), schools: vi.fn(), picked: { value: null as { id: string; name: string } | null } }));

vi.mock("@/lib/hooks/admin-affiliates", () => ({
  useAddAffiliatorProgram: () => ({ mutate: h.add, isPending: false }),
  useSetPartnerSchools: () => ({ mutate: h.schools, isPending: false }),
}));
vi.mock("@/lib/hooks/students", () => ({ useSchoolById: (id: string) => ({ data: { name: `Sekolah ${id}` } }) }));
vi.mock("@/components/SchoolPicker", () => ({
  SchoolPicker: ({ onChange }: { onChange: (s: unknown) => void }) => <button onClick={() => onChange(h.picked.value)}>pilih sekolah</button>,
}));
vi.mock("sonner", () => ({ toast: { success: vi.fn(), error: vi.fn() } }));

describe("PartnerProgramPanel", () => {
  beforeEach(() => {
    h.add.mockReset();
    h.schools.mockReset();
    h.picked.value = { id: "s9", name: "Baru" };
  });

  it("lets an Affiliator be given the Mitra program, and never takes a program away", () => {
    render(<PartnerProgramPanel id="a1" programs={undefined} status="active" />);
    fireEvent.click(screen.getByRole("button", { name: "Tambah program Mitra" }));
    expect(h.add).toHaveBeenCalledWith({ id: "a1", program: "partner" }, expect.anything());
    expect(screen.queryByRole("button", { name: "Tambah program Affiliator" })).toBeNull();
    expect(screen.queryByText("Sekolah yang boleh dibelikan")).toBeNull();
  });

  it("offers the Affiliator program to a Mitra-only account", () => {
    render(<PartnerProgramPanel id="a1" programs={{ affiliate: false, partner: { school_ids: [] } }} status="active" />);
    fireEvent.click(screen.getByRole("button", { name: "Tambah program Affiliator" }));
    expect(h.add).toHaveBeenCalledWith({ id: "a1", program: "affiliate" }, expect.anything());
  });

  it("offers no new program while the application is still open", () => {
    render(<PartnerProgramPanel id="a1" programs={{ affiliate: false, partner: { school_ids: [] } }} status="pending_approval" />);
    expect(screen.queryByRole("button", { name: /tambah program/i })).toBeNull();
  });

  it("edits the schools of a Mitra and saves the whole list", () => {
    render(<PartnerProgramPanel id="a1" programs={{ affiliate: false, partner: { school_ids: ["s1"] } }} status="active" />);
    expect(screen.getByText("Sekolah s1")).toBeInTheDocument();

    fireEvent.click(screen.getByRole("button", { name: "pilih sekolah" }));
    fireEvent.click(screen.getByRole("button", { name: "Simpan sekolah" }));
    expect(h.schools).toHaveBeenCalledWith({ id: "a1", school_ids: ["s1", "s9"] }, expect.anything());

    fireEvent.click(screen.getByRole("button", { name: "remove Sekolah s1" }));
    fireEvent.click(screen.getByRole("button", { name: "Simpan sekolah" }));
    expect(h.schools).toHaveBeenLastCalledWith({ id: "a1", school_ids: ["s9"] }, expect.anything());
  });

  it("says when no school is assigned yet", () => {
    render(<PartnerProgramPanel id="a1" programs={{ partner: { school_ids: [] } }} status="active" />);
    expect(screen.getByText("Belum ada sekolah yang ditugaskan.")).toBeInTheDocument();
  });
});
