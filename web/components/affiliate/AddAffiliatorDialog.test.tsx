import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import { AddAffiliatorDialog, parseAffiliatorCsv } from "./AddAffiliatorDialog";

const h = vi.hoisted(() => ({ bulk: vi.fn() }));

vi.mock("@/lib/hooks/admin-affiliates", () => ({
  useCreateAffiliator: () => ({ mutate: vi.fn(), isPending: false }),
  useBulkCreateAffiliators: () => ({ mutate: h.bulk, isPending: false }),
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
    expect(screen.getByRole("button", { name: "Unggah CSV" })).toHaveClass("bg-primary");
    expect(screen.getByRole("button", { name: "Satu akun" })).not.toHaveClass("bg-primary");
  });

  it("lets the admin set a password for a single account", () => {
    render(<AddAffiliatorDialog open onOpenChange={() => {}} />);

    expect(screen.getByLabelText("Password (opsional)")).toBeInTheDocument();
  });

  it("processes the CSV only after the upload button, filling empty passwords with the default", async () => {
    render(<AddAffiliatorDialog open onOpenChange={() => {}} />);
    fireEvent.click(screen.getByRole("button", { name: "Unggah CSV" }));

    const file = new File(["name,email,phone,password\nBudi,budi@example.com,0812,\nSiti,siti@example.com,0813,Sendiri123\n"], "a.csv", { type: "text/csv" });
    fireEvent.change(screen.getByLabelText("Pilih file CSV"), { target: { files: [file] } });
    fireEvent.change(screen.getByLabelText("Password default (opsional)"), { target: { value: "Bersama123" } });
    expect(h.bulk).not.toHaveBeenCalled();

    fireEvent.click(screen.getByRole("button", { name: "Unggah" }));

    await waitFor(() => expect(h.bulk).toHaveBeenCalled());
    expect(h.bulk.mock.calls[0][0]).toEqual([
      { name: "Budi", email: "budi@example.com", phone: "0812", password: "Bersama123" },
      { name: "Siti", email: "siti@example.com", phone: "0813", password: "Sendiri123" },
    ]);
  });
});
