import { fireEvent, render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import { AddAffiliatorDialog, parseAffiliatorCsv } from "./AddAffiliatorDialog";

vi.mock("@/lib/hooks/admin-affiliates", () => ({
  useCreateAffiliator: () => ({ mutate: vi.fn(), isPending: false }),
  useBulkCreateAffiliators: () => ({ mutate: vi.fn(), isPending: false }),
}));
vi.mock("sonner", () => ({ toast: { success: vi.fn(), error: vi.fn() } }));

describe("parseAffiliatorCsv", () => {
  it("reads name, email, phone and the optional password, skipping the header", () => {
    const rows = parseAffiliatorCsv('name,email,phone,password\nBudi,budi@example.com,0812,\n"Siti",siti@example.com,0813,SandiAwal123\n');

    expect(rows).toEqual([
      { name: "Budi", email: "budi@example.com", phone: "0812", password: "" },
      { name: "Siti", email: "siti@example.com", phone: "0813", password: "SandiAwal123" },
    ]);
  });
});

describe("AddAffiliatorDialog", () => {
  it("explains every CSV column and offers a template in bulk mode", () => {
    render(<AddAffiliatorDialog open onOpenChange={() => {}} />);

    fireEvent.click(screen.getByRole("button", { name: "Unggah CSV" }));

    expect(screen.getByRole("button", { name: "Unduh template CSV" })).toBeInTheDocument();
    for (const column of ["name", "email", "phone", "password"]) {
      expect(screen.getByText(column)).toBeInTheDocument();
    }
    expect(screen.getByRole("button", { name: "Unggah CSV" })).toHaveAttribute("aria-pressed", "true");
  });

  it("lets the admin set a password for a single account", () => {
    render(<AddAffiliatorDialog open onOpenChange={() => {}} />);

    expect(screen.getByLabelText("Password (opsional)")).toBeInTheDocument();
  });
});
